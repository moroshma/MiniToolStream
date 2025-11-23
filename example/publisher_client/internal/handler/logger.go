package handler

import (
	"context"
	"log"

	pb "github.com/moroshma/MiniToolStream/model"
)

// LoggerResponseHandler logs publish responses
type LoggerResponseHandler struct {
	verbose bool
}

// NewLoggerResponseHandler creates a new logger response handler
func NewLoggerResponseHandler(verbose bool) *LoggerResponseHandler {
	return &LoggerResponseHandler{
		verbose: verbose,
	}
}

// Handle logs the publish response
func (h *LoggerResponseHandler) Handle(ctx context.Context, resp *pb.PublishResponse) error {
	if resp.StatusCode != 0 {
		log.Printf("✗ Publish failed: %s", resp.ErrorMessage)
		return nil
	}

	if h.verbose {
		log.Printf("✓ Published successfully!")
		log.Printf("  Sequence: %d", resp.Sequence)
		log.Printf("  ObjectName: %s", resp.ObjectName)
	} else {
		log.Printf("✓ Published: seq=%d, object=%s", resp.Sequence, resp.ObjectName)
	}

	return nil
}
