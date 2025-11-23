package publisher

import (
	"context"
	"fmt"
	"log"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/moroshma/MiniToolStream/example/publisher_client/internal/handler"
	pb "github.com/moroshma/MiniToolStream/model"
)

// Config represents publisher manager configuration
type Config struct {
	ServerAddr string
	Timeout    int // timeout in seconds
}

// Manager manages multiple publish operations
type Manager struct {
	config   *Config
	client   pb.IngressServiceClient
	conn     *grpc.ClientConn
	handlers []handler.PublishHandler
	response handler.ResponseHandler
	mu       sync.RWMutex
}

// NewManager creates a new publisher manager
func NewManager(config *Config) (*Manager, error) {
	// Connect to Ingress server
	conn, err := grpc.NewClient(config.ServerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	client := pb.NewIngressServiceClient(conn)

	return &Manager{
		config:   config,
		client:   client,
		conn:     conn,
		handlers: make([]handler.PublishHandler, 0),
		response: handler.NewLoggerResponseHandler(true), // default response handler
	}, nil
}

// RegisterHandler registers a publish handler
func (m *Manager) RegisterHandler(h handler.PublishHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, h)
	log.Printf("✓ Registered publish handler (total: %d)", len(m.handlers))
}

// RegisterHandlers registers multiple handlers at once
func (m *Manager) RegisterHandlers(handlers []handler.PublishHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, h := range handlers {
		m.handlers = append(m.handlers, h)
	}
	log.Printf("✓ Registered %d publish handlers (total: %d)", len(handlers), len(m.handlers))
}

// SetResponseHandler sets a custom response handler
func (m *Manager) SetResponseHandler(h handler.ResponseHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.response = h
	log.Printf("✓ Custom response handler set")
}

// PublishAll publishes all registered handlers
func (m *Manager) PublishAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.handlers) == 0 {
		return fmt.Errorf("no handlers registered")
	}

	log.Printf("Publishing %d items...", len(m.handlers))

	var wg sync.WaitGroup
	errors := make(chan error, len(m.handlers))

	for i, h := range m.handlers {
		wg.Add(1)
		go func(idx int, handler handler.PublishHandler) {
			defer wg.Done()
			if err := m.publishOne(ctx, idx+1, handler); err != nil {
				errors <- fmt.Errorf("handler %d: %w", idx+1, err)
			}
		}(i, h)
	}

	wg.Wait()
	close(errors)

	// Collect errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to publish %d items: %v", len(errs), errs)
	}

	log.Printf("✓ All %d items published successfully", len(m.handlers))
	return nil
}

// publishOne publishes a single handler
func (m *Manager) publishOne(ctx context.Context, idx int, h handler.PublishHandler) error {
	log.Printf("[%d] Preparing data...", idx)

	// Prepare data
	data, err := h.Prepare(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare: %w", err)
	}

	// Publish
	log.Printf("[%d] Publishing to subject '%s'...", idx, data.Subject)
	req := &pb.PublishRequest{
		Subject: data.Subject,
		Data:    data.Data,
		Headers: data.Headers,
	}

	resp, err := m.client.Publish(ctx, req)
	if err != nil {
		return fmt.Errorf("publish failed: %w", err)
	}

	// Handle response
	if m.response != nil {
		if err := m.response.Handle(ctx, resp); err != nil {
			log.Printf("[%d] Response handler error: %v", idx, err)
		}
	}

	if resp.StatusCode != 0 {
		return fmt.Errorf("server error: %s", resp.ErrorMessage)
	}

	return nil
}

// Publish publishes a single handler immediately
func (m *Manager) Publish(ctx context.Context, h handler.PublishHandler) error {
	return m.publishOne(ctx, 1, h)
}

// Close closes the connection
func (m *Manager) Close() {
	if m.conn != nil {
		m.conn.Close()
		log.Printf("✓ Publisher manager closed")
	}
}
