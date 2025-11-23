package domain

import (
	"context"
	"io"
)

// EgressClient represents the interface for communicating with MiniToolStreamEgress
type EgressClient interface {
	// Subscribe subscribes to notifications for a subject
	Subscribe(ctx context.Context, config *SubscriptionConfig) (NotificationStream, error)

	// Fetch fetches messages from a subject
	Fetch(ctx context.Context, config *SubscriptionConfig) (MessageStream, error)

	// GetLastSequence gets the last sequence number for a subject
	GetLastSequence(ctx context.Context, subject string) (uint64, error)

	// Close closes the client connection
	Close() error
}

// NotificationStream represents a stream of notifications
type NotificationStream interface {
	// Recv receives the next notification
	Recv() (*Notification, error)
}

// MessageStream represents a stream of messages
type MessageStream interface {
	// Recv receives the next message
	Recv() (*Message, error)
}

// MessageHandler processes received messages
type MessageHandler interface {
	// Handle processes a message
	Handle(ctx context.Context, msg *Message) error
}

// MessageHandlerFunc is a function adapter for MessageHandler
type MessageHandlerFunc func(ctx context.Context, msg *Message) error

// Handle implements MessageHandler interface
func (f MessageHandlerFunc) Handle(ctx context.Context, msg *Message) error {
	return f(ctx, msg)
}

// Subscriber represents the interface for subscribing to subjects
type Subscriber interface {
	// RegisterHandler registers a message handler for a subject
	RegisterHandler(subject string, handler MessageHandler)

	// RegisterHandlers registers multiple handlers at once
	RegisterHandlers(handlers map[string]MessageHandler)

	// Start starts all subscriptions
	Start() error

	// Stop stops all subscriptions
	Stop()

	// Wait blocks until all subscriptions finish
	Wait()
}

// NotificationReceiver receives notifications from a stream
type NotificationReceiver interface {
	// Receive receives notifications and sends them to the channel
	Receive(ctx context.Context, stream NotificationStream, ch chan<- *Notification) error
}

// MessageFetcher fetches and processes messages
type MessageFetcher interface {
	// Fetch fetches messages for a notification
	Fetch(ctx context.Context, notification *Notification, handler MessageHandler) error
}

// Helper function to check if error is EOF
func IsEOF(err error) bool {
	return err == io.EOF
}
