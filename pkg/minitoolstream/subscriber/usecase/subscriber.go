package usecase

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/subscriber/domain"
)

// Config represents subscriber configuration
type Config struct {
	Client      domain.EgressClient
	DurableName string
	BatchSize   int32
	Logger      Logger
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

// MultiSubject implements domain.Subscriber for multiple subjects
type MultiSubject struct {
	client      domain.EgressClient
	durableName string
	batchSize   int32
	logger      Logger
	handlers    map[string]domain.MessageHandler
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// New creates a new multi-subject subscriber
func New(config *Config) (*MultiSubject, error) {
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

	batchSize := config.BatchSize
	if batchSize <= 0 {
		batchSize = 10
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &MultiSubject{
		client:      config.Client,
		durableName: config.DurableName,
		batchSize:   batchSize,
		logger:      logger,
		handlers:    make(map[string]domain.MessageHandler),
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

// RegisterHandler registers a message handler for a subject
func (s *MultiSubject) RegisterHandler(subject string, handler domain.MessageHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[subject] = handler
	s.logger.Printf("✓ Registered handler for subject: %s", subject)
}

// RegisterHandlers registers multiple handlers at once
func (s *MultiSubject) RegisterHandlers(handlers map[string]domain.MessageHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for subject, handler := range handlers {
		s.handlers[subject] = handler
		s.logger.Printf("✓ Registered handler for subject: %s", subject)
	}
}

// Start starts all subscriptions
func (s *MultiSubject) Start() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.handlers) == 0 {
		return fmt.Errorf("no handlers registered")
	}

	s.logger.Printf("Starting subscriptions for %d subjects...", len(s.handlers))

	// Start a goroutine for each subject
	for subject, handler := range s.handlers {
		s.wg.Add(1)
		go s.subscribeToSubject(subject, handler)
	}

	return nil
}

// subscribeToSubject handles subscription for a single subject
func (s *MultiSubject) subscribeToSubject(subject string, handler domain.MessageHandler) {
	defer s.wg.Done()

	s.logger.Printf("[%s] Starting subscription...", subject)

	config := &domain.SubscriptionConfig{
		Subject:     subject,
		DurableName: s.durableName,
		BatchSize:   s.batchSize,
	}

	// Subscribe to notifications
	notificationStream, err := s.client.Subscribe(s.ctx, config)
	if err != nil {
		s.logger.Printf("[%s] Failed to subscribe: %v", subject, err)
		return
	}

	// Create notification channel
	notificationChan := make(chan *domain.Notification, 100)

	// Start notification receiver goroutine
	go func() {
		defer close(notificationChan)
		for {
			notification, err := notificationStream.Recv()
			if err == io.EOF {
				s.logger.Printf("[%s] Subscribe stream closed", subject)
				return
			}
			if err != nil {
				select {
				case <-s.ctx.Done():
					return
				default:
					s.logger.Printf("[%s] Subscribe error: %v", subject, err)
					return
				}
			}
			s.logger.Printf("[%s] 📬 Notification received: sequence=%d", subject, notification.Sequence)
			select {
			case notificationChan <- notification:
			case <-s.ctx.Done():
				return
			}
		}
	}()

	// Process notifications
	s.logger.Printf("[%s] Waiting for notifications...", subject)
	for {
		select {
		case <-s.ctx.Done():
			s.logger.Printf("[%s] Context cancelled, stopping subscription", subject)
			return

		case notification, ok := <-notificationChan:
			if !ok {
				s.logger.Printf("[%s] Notification channel closed", subject)
				return
			}

			if err := s.processNotification(subject, notification, handler); err != nil {
				s.logger.Printf("[%s] Error processing notification: %v", subject, err)
			}
		}
	}
}

// processNotification fetches and processes messages for a notification
func (s *MultiSubject) processNotification(subject string, notification *domain.Notification, handler domain.MessageHandler) error {
	config := &domain.SubscriptionConfig{
		Subject:     notification.Subject,
		DurableName: s.durableName,
		BatchSize:   s.batchSize,
	}

	// Fetch messages
	messageStream, err := s.client.Fetch(s.ctx, config)
	if err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	messageCount := 0
	for {
		msg, err := messageStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("fetch error: %w", err)
		}

		messageCount++
		s.logger.Printf("[%s] 📨 Message received: sequence=%d, data_size=%d",
			subject, msg.Sequence, len(msg.Data))

		// Handle message
		if err := handler.Handle(s.ctx, msg); err != nil {
			s.logger.Printf("[%s] Handler error for sequence %d: %v", subject, msg.Sequence, err)
			// Continue processing other messages even if one fails
		}
	}

	s.logger.Printf("[%s] Processed %d messages", subject, messageCount)
	return nil
}

// Stop gracefully stops all subscriptions
func (s *MultiSubject) Stop() {
	s.logger.Printf("Stopping subscriber...")
	s.cancel()
	s.wg.Wait()
	if err := s.client.Close(); err != nil {
		s.logger.Printf("Error closing client: %v", err)
	}
	s.logger.Printf("✓ Subscriber stopped")
}

// Wait blocks until all subscriptions finish
func (s *MultiSubject) Wait() {
	s.wg.Wait()
}
