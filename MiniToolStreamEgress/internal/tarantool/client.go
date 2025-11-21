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

// Message represents a message from Tarantool
type Message struct {
	Sequence   uint64
	Headers    map[string]string
	ObjectName string
	Subject    string
	CreateAt   uint64
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

// GetConsumerPosition gets the last read sequence for a durable consumer
func (c *Client) GetConsumerPosition(durableName, subject string) (uint64, error) {
	resp, err := c.Call("get_consumer_position", []interface{}{durableName, subject})
	if err != nil {
		return 0, fmt.Errorf("failed to get consumer position: %w", err)
	}

	if len(resp) == 0 {
		return 0, nil
	}

	return toUint64(resp[0]), nil
}

// UpdateConsumerPosition updates the last read sequence for a durable consumer
func (c *Client) UpdateConsumerPosition(durableName, subject string, lastSequence uint64) error {
	_, err := c.Call("update_consumer_position", []interface{}{durableName, subject, lastSequence})
	if err != nil {
		return fmt.Errorf("failed to update consumer position: %w", err)
	}
	return nil
}

// GetLatestSequenceForSubject gets the latest sequence number for a subject
func (c *Client) GetLatestSequenceForSubject(subject string) (uint64, error) {
	resp, err := c.Call("get_latest_sequence_for_subject", []interface{}{subject})
	if err != nil {
		return 0, fmt.Errorf("failed to get latest sequence: %w", err)
	}

	if len(resp) == 0 {
		return 0, nil
	}

	return toUint64(resp[0]), nil
}

// GetMessagesBySubject fetches messages for a subject starting from a sequence
func (c *Client) GetMessagesBySubject(subject string, startSequence uint64, limit int) ([]Message, error) {
	resp, err := c.Call("get_messages_by_subject", []interface{}{subject, startSequence, limit})
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	if len(resp) == 0 {
		return []Message{}, nil
	}

	// Parse response - it's an array of tuples
	tuples, ok := resp[0].([]interface{})
	if !ok {
		return []Message{}, nil
	}

	messages := make([]Message, 0, len(tuples))
	for _, tupleRaw := range tuples {
		tuple, ok := tupleRaw.([]interface{})
		if !ok || len(tuple) < 5 {
			continue
		}

		// Parse headers (may be map or empty)
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

		msg := Message{
			Sequence:   toUint64(tuple[0]),
			Headers:    headers,
			ObjectName: toString(tuple[2]),
			Subject:    toString(tuple[3]),
			CreateAt:   toUint64(tuple[4]),
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// GetMessageBySequence gets a single message by its sequence number
func (c *Client) GetMessageBySequence(sequence uint64) (*Message, error) {
	resp, err := c.Call("get_message_by_sequence_decoded", []interface{}{sequence})
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

	msg := &Message{
		Sequence:   toUint64(msgMap["sequence"]),
		Headers:    headers,
		ObjectName: toString(msgMap["object_name"]),
		Subject:    toString(msgMap["subject"]),
		CreateAt:   toUint64(msgMap["create_at"]),
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
