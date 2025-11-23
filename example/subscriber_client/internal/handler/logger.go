package handler

import (
	"context"
	"log"

	pb "github.com/moroshma/MiniToolStream/model"
)

// LoggerHandler logs message information without saving data
type LoggerHandler struct {
	prefix string
}

// NewLoggerHandler creates a new logger handler
func NewLoggerHandler(prefix string) *LoggerHandler {
	return &LoggerHandler{
		prefix: prefix,
	}
}

// Handle logs the message information
func (h *LoggerHandler) Handle(ctx context.Context, msg *pb.Message) error {
	log.Printf("[%s] Message: seq=%d, subject=%s, data_size=%d, headers=%v",
		h.prefix, msg.Sequence, msg.Subject, len(msg.Data), msg.Headers)
	return nil
}
