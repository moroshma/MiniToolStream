package handler

import (
	"context"
	pb "github.com/moroshma/MiniToolStream/model"
)

// PublishData represents data to be published
type PublishData struct {
	Subject string
	Data    []byte
	Headers map[string]string
}

// PublishHandler defines the interface for handling publish operations
type PublishHandler interface {
	// Prepare prepares the data for publishing
	// Returns PublishData or error
	Prepare(ctx context.Context) (*PublishData, error)
}

// PublishHandlerFunc is a function adapter for PublishHandler
type PublishHandlerFunc func(ctx context.Context) (*PublishData, error)

// Prepare implements PublishHandler interface
func (f PublishHandlerFunc) Prepare(ctx context.Context) (*PublishData, error) {
	return f(ctx)
}

// PublishResponse represents the response from publishing
type PublishResponse struct {
	Subject    string
	Sequence   uint64
	ObjectName string
	StatusCode int32
	Error      error
}

// ResponseHandler defines the interface for handling publish responses
type ResponseHandler interface {
	// Handle processes the publish response
	Handle(ctx context.Context, resp *pb.PublishResponse) error
}

// ResponseHandlerFunc is a function adapter for ResponseHandler
type ResponseHandlerFunc func(ctx context.Context, resp *pb.PublishResponse) error

// Handle implements ResponseHandler interface
func (f ResponseHandlerFunc) Handle(ctx context.Context, resp *pb.PublishResponse) error {
	return f(ctx, resp)
}
