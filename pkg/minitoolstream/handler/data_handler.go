package handler

import (
	"context"
	"log"
	"time"

	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/domain"
)

// DataHandler publishes raw data
type DataHandler struct {
	subject     string
	data        []byte
	contentType string
	headers     map[string]string
	logger      Logger
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

// DataHandlerConfig represents configuration for DataHandler
type DataHandlerConfig struct {
	Subject     string
	Data        []byte
	ContentType string
	Headers     map[string]string
	Logger      Logger
}

// NewDataHandler creates a new data handler
func NewDataHandler(config *DataHandlerConfig) *DataHandler {
	logger := config.Logger
	if logger == nil {
		logger = &defaultLogger{}
	}

	headers := config.Headers
	if headers == nil {
		headers = make(map[string]string)
	}

	contentType := config.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return &DataHandler{
		subject:     config.Subject,
		data:        config.Data,
		contentType: contentType,
		headers:     headers,
		logger:      logger,
	}
}

// WithHeaders adds custom headers to the handler
func (h *DataHandler) WithHeaders(headers map[string]string) *DataHandler {
	for k, v := range headers {
		h.headers[k] = v
	}
	return h
}

// Prepare prepares raw data for publishing
func (h *DataHandler) Prepare(ctx context.Context) (*domain.Message, error) {
	h.logger.Printf("[%s] Preparing data (%d bytes)", h.subject, len(h.data))

	// Build headers
	headers := make(map[string]string)
	headers["content-type"] = h.contentType
	headers["timestamp"] = time.Now().Format(time.RFC3339)

	// Add custom headers
	for k, v := range h.headers {
		headers[k] = v
	}

	return &domain.Message{
		Subject: h.subject,
		Data:    h.data,
		Headers: headers,
	}, nil
}
