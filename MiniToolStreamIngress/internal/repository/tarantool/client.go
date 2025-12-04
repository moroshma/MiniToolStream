package tarantool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tarantool/go-tarantool/v2"

	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/internal/config"
	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/pkg/logger"
)

// Config represents configuration for Tarantool connection
type Config struct {
	Address  string
	User     string
	Password string
	Timeout  time.Duration
}

// Repository represents a connection to Tarantool
type Repository struct {
	conn   *tarantool.Connection
	config *Config
	logger *logger.Logger
	mu     sync.RWMutex
	closed bool
}

// NewRepository creates a new Tarantool repository
func NewRepository(config *Config, log *logger.Logger) (*Repository, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	ctx := context.Background()

	// Create Tarantool dialer
	dialer := tarantool.NetDialer{
		Address:  config.Address,
		User:     config.User,
		Password: config.Password,
	}

	// Connection options
	opts := tarantool.Opts{
		Timeout: config.Timeout,
	}

	// Connect to Tarantool
	conn, err := tarantool.Connect(ctx, dialer, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Tarantool: %w", err)
	}

	repo := &Repository{
		conn:   conn,
		config: config,
		logger: log,
		closed: false,
	}

	return repo, nil
}

// Close closes the Tarantool connection
func (r *Repository) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}

	r.closed = true
	return r.conn.Close()
}

// Ping checks if the connection to Tarantool is alive
func (r *Repository) Ping() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return fmt.Errorf("repository is closed")
	}

	_, err := r.conn.Ping()
	return err
}

// call executes a Tarantool function
func (r *Repository) call(functionName string, args []interface{}) ([]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return nil, fmt.Errorf("repository is closed")
	}

	// Use Call17 for better type support
	req := tarantool.NewCall17Request(functionName).Args(args)
	future := r.conn.Do(req)
	resp, err := future.Get()
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// PublishMessage publishes a message to Tarantool
// Returns sequence number
func (r *Repository) PublishMessage(subject string, headers map[string]string) (uint64, error) {
	if subject == "" {
		return 0, fmt.Errorf("subject cannot be empty")
	}

	if headers == nil {
		headers = make(map[string]string)
	}

	r.logger.Debug("Publishing message to Tarantool",
		logger.String("subject", subject),
		logger.Any("headers", headers),
	)

	// Call Tarantool function
	resp, err := r.call("publish_message", []interface{}{
		subject,
		headers,
	})
	if err != nil {
		r.logger.Error("Failed to publish message to Tarantool",
			logger.String("subject", subject),
			logger.Error(err),
		)
		return 0, fmt.Errorf("failed to publish message: %w", err)
	}

	// Parse response - returns sequence number
	if len(resp) == 0 {
		return 0, fmt.Errorf("empty response from Tarantool")
	}

	// Call17 returns the sequence number directly
	sequence := toUint64(resp[0])

	r.logger.Debug("Message published successfully",
		logger.String("subject", subject),
		logger.Uint64("sequence", sequence),
	)

	return sequence, nil
}

// StartTTLCleanup starts the TTL cleanup background fiber in Tarantool
func (r *Repository) StartTTLCleanup(ttlConfig config.TTLConfig) error {
	r.logger.Info("Configuring Tarantool TTL cleanup",
		logger.Any("default_ttl", ttlConfig.Default),
		logger.Int("channels_count", len(ttlConfig.Channels)),
	)

	// Prepare channel-specific TTL map
	channelsMap := make(map[string]interface{})
	for _, ch := range ttlConfig.Channels {
		channelsMap[ch.Channel] = int(ch.Duration.Seconds())
	}

	// Prepare configuration
	ttlConfigMap := map[string]interface{}{
		"enabled":        ttlConfig.Enabled,
		"default_ttl":    int(ttlConfig.Default.Seconds()),
		"check_interval": int(ttlConfig.Default.Seconds()) / 24, // Run 24 times during TTL period
		"channels":       channelsMap,
	}

	// Call configure_ttl function
	resp, err := r.call("configure_ttl", []interface{}{ttlConfigMap})
	if err != nil {
		r.logger.Error("Failed to configure TTL in Tarantool",
			logger.Error(err),
		)
		return fmt.Errorf("failed to configure TTL: %w", err)
	}

	if len(resp) > 0 {
		if success, ok := resp[0].(bool); ok && success {
			r.logger.Info("Tarantool TTL cleanup configured successfully")
			return nil
		}
	}

	return fmt.Errorf("unexpected response from configure_ttl")
}

// GetTTLStatus returns the current TTL configuration status from Tarantool
func (r *Repository) GetTTLStatus() (map[string]interface{}, error) {
	resp, err := r.call("get_ttl_status", []interface{}{})
	if err != nil {
		return nil, fmt.Errorf("failed to get TTL status: %w", err)
	}

	if len(resp) > 0 {
		if status, ok := resp[0].([]interface{}); ok && len(status) > 0 {
			if statusMap, ok := status[0].(map[interface{}]interface{}); ok {
				result := make(map[string]interface{})
				for k, v := range statusMap {
					if key, ok := k.(string); ok {
						result[key] = v
					}
				}
				return result, nil
			}
		}
	}

	return nil, fmt.Errorf("unexpected response format from get_ttl_status")
}

// Helper function for type conversion
func toUint64(val interface{}) uint64 {
	switch v := val.(type) {
	case uint64:
		return v
	case int64:
		return uint64(v)
	case int:
		return uint64(v)
	case int8:
		return uint64(v)
	case int16:
		return uint64(v)
	case int32:
		return uint64(v)
	case uint:
		return uint64(v)
	case uint8:
		return uint64(v)
	case uint16:
		return uint64(v)
	case uint32:
		return uint64(v)
	case float64:
		return uint64(v)
	case float32:
		return uint64(v)
	default:
		return 0
	}
}

// Helper function for string conversion
func toString(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
