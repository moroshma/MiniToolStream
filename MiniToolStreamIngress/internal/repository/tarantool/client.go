package tarantool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tarantool/go-tarantool/v2"
)

// Config represents configuration for Tarantool connection
type Config struct {
	Address  string
	User     string
	Password string
	Timeout  time.Duration
}

// Client represents a connection to Tarantool
type Client struct {
	conn   *tarantool.Connection
	config *Config
	mu     sync.RWMutex
	closed bool
}

// NewClient creates a new Tarantool client
func NewClient(config *Config) (*Client, error) {
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

	client := &Client{
		conn:   conn,
		config: config,
		closed: false,
	}

	return client, nil
}

// Close closes the Tarantool connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	return c.conn.Close()
}

// Ping checks if the connection to Tarantool is alive
func (c *Client) Ping() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	_, err := c.conn.Ping()
	return err
}

// Call executes a Tarantool function
func (c *Client) Call(functionName string, args []interface{}) ([]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	// Use Call17 for better type support
	req := tarantool.NewCall17Request(functionName).Args(args)
	future := c.conn.Do(req)
	resp, err := future.Get()
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// PublishMessage publishes a message to Tarantool
// Returns sequence number
func (c *Client) PublishMessage(subject string, headers map[string]string) (uint64, error) {
	if subject == "" {
		return 0, fmt.Errorf("subject cannot be empty")
	}

	if headers == nil {
		headers = make(map[string]string)
	}

	// Call Tarantool function
	resp, err := c.Call("publish_message", []interface{}{
		subject,
		headers,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to publish message: %w", err)
	}

	// Parse response - returns sequence number
	if len(resp) == 0 {
		return 0, fmt.Errorf("empty response from Tarantool")
	}

	// Call17 returns the sequence number directly
	sequence := toUint64(resp[0])
	return sequence, nil
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
