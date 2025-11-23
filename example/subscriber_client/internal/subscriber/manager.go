package subscriber

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/moroshma/MiniToolStream/example/subscriber_client/internal/handler"
	pb "github.com/moroshma/MiniToolStream/model"
)

// Config represents subscriber manager configuration
type Config struct {
	ServerAddr  string
	DurableName string
	BatchSize   int32
}

// Manager manages multiple subject subscriptions
type Manager struct {
	config   *Config
	client   pb.EgressServiceClient
	conn     *grpc.ClientConn
	handlers map[string]handler.MessageHandler
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewManager creates a new subscriber manager
func NewManager(config *Config) (*Manager, error) {
	if config.BatchSize <= 0 {
		config.BatchSize = 10
	}

	// Connect to Egress server
	conn, err := grpc.NewClient(config.ServerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	client := pb.NewEgressServiceClient(conn)
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		config:   config,
		client:   client,
		conn:     conn,
		handlers: make(map[string]handler.MessageHandler),
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// RegisterHandler registers a message handler for a subject
func (m *Manager) RegisterHandler(subject string, h handler.MessageHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[subject] = h
	log.Printf("✓ Registered handler for subject: %s", subject)
}

// RegisterHandlers registers multiple handlers at once
func (m *Manager) RegisterHandlers(handlers map[string]handler.MessageHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for subject, h := range handlers {
		m.handlers[subject] = h
		log.Printf("✓ Registered handler for subject: %s", subject)
	}
}

// Start starts subscription for all registered handlers
func (m *Manager) Start() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.handlers) == 0 {
		return fmt.Errorf("no handlers registered")
	}

	log.Printf("Starting subscriptions for %d subjects...", len(m.handlers))

	// Start a goroutine for each subject
	for subject, h := range m.handlers {
		m.wg.Add(1)
		go m.subscribeToSubject(subject, h)
	}

	return nil
}

// subscribeToSubject handles subscription for a single subject
func (m *Manager) subscribeToSubject(subject string, h handler.MessageHandler) {
	defer m.wg.Done()

	log.Printf("[%s] Starting subscription...", subject)

	// Subscribe to notifications
	subscribeReq := &pb.SubscribeRequest{
		Subject:     subject,
		DurableName: m.config.DurableName,
	}

	subscribeStream, err := m.client.Subscribe(m.ctx, subscribeReq)
	if err != nil {
		log.Printf("[%s] Failed to subscribe: %v", subject, err)
		return
	}

	// Create notification channel
	notificationChan := make(chan *pb.Notification, 100)

	// Start notification receiver goroutine
	go func() {
		defer close(notificationChan)
		for {
			notification, err := subscribeStream.Recv()
			if err == io.EOF {
				log.Printf("[%s] Subscribe stream closed", subject)
				return
			}
			if err != nil {
				select {
				case <-m.ctx.Done():
					return
				default:
					log.Printf("[%s] Subscribe error: %v", subject, err)
					return
				}
			}
			log.Printf("[%s] 📬 Notification received: sequence=%d", subject, notification.Sequence)
			select {
			case notificationChan <- notification:
			case <-m.ctx.Done():
				return
			}
		}
	}()

	// Process notifications
	log.Printf("[%s] Waiting for notifications...", subject)
	for {
		select {
		case <-m.ctx.Done():
			log.Printf("[%s] Context cancelled, stopping subscription", subject)
			return

		case notification, ok := <-notificationChan:
			if !ok {
				log.Printf("[%s] Notification channel closed", subject)
				return
			}

			if err := m.processNotification(subject, notification, h); err != nil {
				log.Printf("[%s] Error processing notification: %v", subject, err)
			}
		}
	}
}

// processNotification fetches and processes messages for a notification
func (m *Manager) processNotification(subject string, notification *pb.Notification, h handler.MessageHandler) error {
	// Fetch messages
	fetchReq := &pb.FetchRequest{
		Subject:     notification.Subject,
		DurableName: m.config.DurableName,
		BatchSize:   m.config.BatchSize,
	}

	fetchStream, err := m.client.Fetch(m.ctx, fetchReq)
	if err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	messageCount := 0
	for {
		msg, err := fetchStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("fetch error: %w", err)
		}

		messageCount++
		log.Printf("[%s] 📨 Message received: sequence=%d, data_size=%d",
			subject, msg.Sequence, len(msg.Data))

		// Handle message
		if err := h.Handle(m.ctx, msg); err != nil {
			log.Printf("[%s] Handler error for sequence %d: %v", subject, msg.Sequence, err)
			// Continue processing other messages even if one fails
		}
	}

	log.Printf("[%s] Processed %d messages", subject, messageCount)
	return nil
}

// Stop gracefully stops all subscriptions
func (m *Manager) Stop() {
	log.Printf("Stopping subscriber manager...")
	m.cancel()
	m.wg.Wait()
	m.conn.Close()
	log.Printf("✓ Subscriber manager stopped")
}

// Wait blocks until all subscriptions finish
func (m *Manager) Wait() {
	m.wg.Wait()
}
