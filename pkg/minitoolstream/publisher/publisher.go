package publisher

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/domain"
)

// Config represents publisher configuration
type Config struct {
	Client        domain.IngressClient
	ResultHandler domain.ResultHandler
	Logger        Logger
}

// Logger defines the logging interface
type Logger interface {
	Printf(format string, v ...interface{})
}

// defaultLogger is a default logger implementation
type defaultLogger struct{}

func (l *defaultLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

// SimplePublisher implements domain.Publisher
type SimplePublisher struct {
	client        domain.IngressClient
	resultHandler domain.ResultHandler
	logger        Logger
	preparers     []domain.MessagePreparer
	mu            sync.RWMutex
}

// New creates a new publisher instance
func New(config *Config) (*SimplePublisher, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.Client == nil {
		return nil, fmt.Errorf("client cannot be nil")
	}

	logger := config.Logger
	if logger == nil {
		logger = &defaultLogger{}
	}

	resultHandler := config.ResultHandler
	if resultHandler == nil {
		resultHandler = NewLoggingResultHandler(logger, true)
	}

	return &SimplePublisher{
		client:        config.Client,
		resultHandler: resultHandler,
		logger:        logger,
		preparers:     make([]domain.MessagePreparer, 0),
	}, nil
}

// RegisterHandler registers a message preparer
func (p *SimplePublisher) RegisterHandler(preparer domain.MessagePreparer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.preparers = append(p.preparers, preparer)
	p.logger.Printf("✓ Registered message preparer (total: %d)", len(p.preparers))
}

// RegisterHandlers registers multiple message preparers
func (p *SimplePublisher) RegisterHandlers(preparers []domain.MessagePreparer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, preparer := range preparers {
		p.preparers = append(p.preparers, preparer)
	}
	p.logger.Printf("✓ Registered %d message preparers (total: %d)", len(preparers), len(p.preparers))
}

// SetResultHandler sets a custom result handler
func (p *SimplePublisher) SetResultHandler(handler domain.ResultHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resultHandler = handler
	p.logger.Printf("✓ Custom result handler set")
}

// Publish publishes a single message
func (p *SimplePublisher) Publish(ctx context.Context, preparer domain.MessagePreparer) error {
	return p.publishOne(ctx, 1, preparer)
}

// PublishAll publishes all registered message preparers concurrently
func (p *SimplePublisher) PublishAll(ctx context.Context, preparers []domain.MessagePreparer) error {
	if len(preparers) == 0 {
		p.mu.RLock()
		preparers = p.preparers
		p.mu.RUnlock()
	}

	if len(preparers) == 0 {
		return fmt.Errorf("no message preparers to publish")
	}

	p.logger.Printf("Publishing %d messages...", len(preparers))

	var wg sync.WaitGroup
	errChan := make(chan error, len(preparers))

	for i, preparer := range preparers {
		wg.Add(1)
		go func(idx int, prep domain.MessagePreparer) {
			defer wg.Done()
			if err := p.publishOne(ctx, idx+1, prep); err != nil {
				errChan <- fmt.Errorf("message %d: %w", idx+1, err)
			}
		}(i, preparer)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to publish %d messages: %v", len(errs), errs)
	}

	p.logger.Printf("✓ All %d messages published successfully", len(preparers))
	return nil
}

// publishOne publishes a single message
func (p *SimplePublisher) publishOne(ctx context.Context, idx int, preparer domain.MessagePreparer) error {
	p.logger.Printf("[%d] Preparing message...", idx)

	// Prepare message
	msg, err := preparer.Prepare(ctx)
	if err != nil {
		return fmt.Errorf("failed to prepare message: %w", err)
	}

	if msg == nil {
		return fmt.Errorf("preparer returned nil message")
	}

	// Publish message
	p.logger.Printf("[%d] Publishing to subject '%s'...", idx, msg.Subject)
	result, err := p.client.Publish(ctx, msg)
	if err != nil {
		return fmt.Errorf("publish failed: %w", err)
	}

	// Handle result
	if p.resultHandler != nil {
		if err := p.resultHandler.Handle(ctx, result); err != nil {
			p.logger.Printf("[%d] Result handler error: %v", idx, err)
		}
	}

	if result.StatusCode != 0 {
		return fmt.Errorf("server error: %s", result.ErrorMessage)
	}

	return nil
}

// Close closes the publisher and underlying client
func (p *SimplePublisher) Close() error {
	if p.client != nil {
		if err := p.client.Close(); err != nil {
			return fmt.Errorf("failed to close client: %w", err)
		}
		p.logger.Printf("✓ Publisher closed")
	}
	return nil
}
