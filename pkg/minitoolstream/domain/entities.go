package domain

import "context"

// Message represents a message to be published
type Message struct {
	Subject string
	Data    []byte
	Headers map[string]string
}

// PublishResult represents the result of a publish operation
type PublishResult struct {
	Sequence     uint64
	ObjectName   string
	StatusCode   int64
	ErrorMessage string
}

// MessagePreparer prepares messages for publishing
type MessagePreparer interface {
	// Prepare prepares a message for publishing
	Prepare(ctx context.Context) (*Message, error)
}

// MessagePreparerFunc is a function adapter for MessagePreparer
type MessagePreparerFunc func(ctx context.Context) (*Message, error)

// Prepare implements MessagePreparer interface
func (f MessagePreparerFunc) Prepare(ctx context.Context) (*Message, error) {
	return f(ctx)
}

// ResultHandler processes publish results
type ResultHandler interface {
	// Handle processes the publish result
	Handle(ctx context.Context, result *PublishResult) error
}

// ResultHandlerFunc is a function adapter for ResultHandler
type ResultHandlerFunc func(ctx context.Context, result *PublishResult) error

// Handle implements ResultHandler interface
func (f ResultHandlerFunc) Handle(ctx context.Context, result *PublishResult) error {
	return f(ctx, result)
}
