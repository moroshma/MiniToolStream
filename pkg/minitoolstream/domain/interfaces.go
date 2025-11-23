package domain

import "context"

// IngressClient represents the interface for communicating with MiniToolStreamIngress
type IngressClient interface {
	// Publish publishes a message to the specified subject
	Publish(ctx context.Context, msg *Message) (*PublishResult, error)

	// Close closes the client connection
	Close() error
}

// Publisher represents the interface for publishing messages
type Publisher interface {
	// Publish publishes a single message
	Publish(ctx context.Context, preparer MessagePreparer) error

	// PublishAll publishes multiple messages concurrently
	PublishAll(ctx context.Context, preparers []MessagePreparer) error

	// RegisterHandler registers a message preparer
	RegisterHandler(preparer MessagePreparer)

	// RegisterHandlers registers multiple message preparers
	RegisterHandlers(preparers []MessagePreparer)

	// SetResultHandler sets a custom result handler
	SetResultHandler(handler ResultHandler)

	// Close closes the publisher and underlying client
	Close() error
}
