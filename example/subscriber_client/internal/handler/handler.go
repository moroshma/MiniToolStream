package handler

import (
	"context"
	pb "github.com/moroshma/MiniToolStream/model"
)

// MessageHandler defines the interface for handling messages from a subject
type MessageHandler interface {
	// Handle processes a message from the stream
	Handle(ctx context.Context, msg *pb.Message) error
}

// MessageHandlerFunc is a function adapter for MessageHandler
type MessageHandlerFunc func(ctx context.Context, msg *pb.Message) error

// Handle implements MessageHandler interface
func (f MessageHandlerFunc) Handle(ctx context.Context, msg *pb.Message) error {
	return f(ctx, msg)
}
