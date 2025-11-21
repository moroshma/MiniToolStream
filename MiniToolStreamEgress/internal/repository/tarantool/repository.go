package tarantool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tarantool/go-tarantool/v2"

	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/domain/entity"
	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/pkg/logger"
)

// Repository implements domain.MessageRepository using Tarantool
type Repository struct {
	conn   *tarantool.Connection
	logger *logger.Logger
	mu     sync.RWMutex
	closed bool
}

// Config represents Tarantool repository configuration
type Config struct {
	Address  string
	User     string
	Password string
	Timeout  time.Duration
}

// NewRepository creates a new Tarantool repository
func NewRepository(cfg *Config, log *logger.Logger) (*Repository, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	ctx := context.Background()

	// Create Tarantool dialer
	dialer := tarantool.NetDialer{
		Address:  cfg.Address,
		User:     cfg.User,
		Password: cfg.Password,
	}

	// Connection options
	opts := tarantool.Opts{
		Timeout: cfg.Timeout,
	}

	// Connect to Tarantool
	conn, err := tarantool.Connect(ctx, dialer, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Tarantool: %w", err)
	}

	repo := &Repository{
		conn:   conn,
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

// Call executes a Tarantool function
func (r *Repository) call(functionName string, args []interface{}) ([]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return nil, fmt.Errorf("repository is closed")
	}

	req := tarantool.NewCall17Request(functionName).Args(args)
	future := r.conn.Do(req)
	resp, err := future.Get()
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// GetConsumerPosition returns the last read sequence for a durable consumer
func (r *Repository) GetConsumerPosition(ctx context.Context, durableName, subject string) (uint64, error) {
	resp, err := r.call("get_consumer_position", []interface{}{durableName, subject})
	if err != nil {
		return 0, fmt.Errorf("failed to get consumer position: %w", err)
	}

	if len(resp) == 0 {
		return 0, nil
	}

	return toUint64(resp[0]), nil
}

// UpdateConsumerPosition updates the last read sequence for a durable consumer
func (r *Repository) UpdateConsumerPosition(ctx context.Context, durableName, subject string, lastSequence uint64) error {
	_, err := r.call("update_consumer_position", []interface{}{durableName, subject, lastSequence})
	if err != nil {
		return fmt.Errorf("failed to update consumer position: %w", err)
	}
	return nil
}

// GetLatestSequenceForSubject returns the latest sequence number for a subject
func (r *Repository) GetLatestSequenceForSubject(ctx context.Context, subject string) (uint64, error) {
	resp, err := r.call("get_latest_sequence_for_subject", []interface{}{subject})
	if err != nil {
		return 0, fmt.Errorf("failed to get latest sequence: %w", err)
	}

	if len(resp) == 0 {
		return 0, nil
	}

	return toUint64(resp[0]), nil
}

// GetMessagesBySubject fetches messages for a subject starting from a sequence
func (r *Repository) GetMessagesBySubject(ctx context.Context, subject string, startSequence uint64, limit int) ([]*entity.Message, error) {
	resp, err := r.call("get_messages_by_subject", []interface{}{subject, startSequence, limit})
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	if len(resp) == 0 {
		return []*entity.Message{}, nil
	}

	// Parse response - it's an array of tuples
	tuples, ok := resp[0].([]interface{})
	if !ok {
		return []*entity.Message{}, nil
	}

	messages := make([]*entity.Message, 0, len(tuples))
	for _, tupleRaw := range tuples {
		tuple, ok := tupleRaw.([]interface{})
		if !ok || len(tuple) < 5 {
			continue
		}

		// Parse headers
		headers := make(map[string]string)
		if headersRaw, ok := tuple[1].(map[interface{}]interface{}); ok {
			for k, v := range headersRaw {
				if keyStr, ok := k.(string); ok {
					if valStr, ok := v.(string); ok {
						headers[keyStr] = valStr
					}
				}
			}
		}

		msg := &entity.Message{
			Sequence:   toUint64(tuple[0]),
			Headers:    headers,
			ObjectName: toString(tuple[2]),
			Subject:    toString(tuple[3]),
			Timestamp:  time.Unix(int64(toUint64(tuple[4])), 0),
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// GetMessageBySequence gets a single message by its sequence number
func (r *Repository) GetMessageBySequence(ctx context.Context, sequence uint64) (*entity.Message, error) {
	resp, err := r.call("get_message_by_sequence_decoded", []interface{}{sequence})
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	if len(resp) == 0 {
		return nil, fmt.Errorf("message not found")
	}

	// Parse the response map
	msgMap, ok := resp[0].(map[interface{}]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	// Parse headers
	headers := make(map[string]string)
	if headersRaw, ok := msgMap["headers"].(map[interface{}]interface{}); ok {
		for k, v := range headersRaw {
			if keyStr, ok := k.(string); ok {
				if valStr, ok := v.(string); ok {
					headers[keyStr] = valStr
				}
			}
		}
	}

	msg := &entity.Message{
		Sequence:   toUint64(msgMap["sequence"]),
		Headers:    headers,
		ObjectName: toString(msgMap["object_name"]),
		Subject:    toString(msgMap["subject"]),
		Timestamp:  time.Unix(int64(toUint64(msgMap["create_at"])), 0),
	}

	return msg, nil
}

// Helper function for type conversion to uint64
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

// Helper function for type conversion to string
func toString(val interface{}) string {
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}
