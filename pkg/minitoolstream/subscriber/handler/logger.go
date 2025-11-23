package handler

import (
	"context"

	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/subscriber/domain"
)

// LoggerHandler logs message data without saving
type LoggerHandler struct {
	prefix string
	logger Logger
}

// LoggerHandlerConfig represents configuration for LoggerHandler
type LoggerHandlerConfig struct {
	Prefix string
	Logger Logger
}

// NewLoggerHandler creates a new logger handler
func NewLoggerHandler(config *LoggerHandlerConfig) *LoggerHandler {
	logger := config.Logger
	if logger == nil {
		logger = &defaultLogger{}
	}

	return &LoggerHandler{
		prefix: config.Prefix,
		logger: logger,
	}
}

// Handle logs the message data
func (h *LoggerHandler) Handle(ctx context.Context, msg *domain.Message) error {
	h.logger.Printf("   [%s] Sequence=%d, Size=%d bytes", h.prefix, msg.Sequence, len(msg.Data))

	// Log headers if present
	if len(msg.Headers) > 0 {
		h.logger.Printf("   [%s] Headers: %v", h.prefix, msg.Headers)
	}

	// Log data content if it's text
	if contentType, ok := msg.Headers["content-type"]; ok {
		if contentType == "text/plain" && len(msg.Data) > 0 && len(msg.Data) < 1000 {
			h.logger.Printf("   [%s] Data: %s", h.prefix, string(msg.Data))
		}
	}

	return nil
}
