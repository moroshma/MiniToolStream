package handler

import (
	"context"
	"log"
	"time"
)

// DataPublisherHandler publishes raw data
type DataPublisherHandler struct {
	subject     string
	data        []byte
	contentType string
	headers     map[string]string
}

// NewDataPublisherHandler creates a new data publisher handler
func NewDataPublisherHandler(subject string, data []byte, contentType string) *DataPublisherHandler {
	return &DataPublisherHandler{
		subject:     subject,
		data:        data,
		contentType: contentType,
		headers:     make(map[string]string),
	}
}

// WithHeaders adds custom headers to the handler
func (h *DataPublisherHandler) WithHeaders(headers map[string]string) *DataPublisherHandler {
	for k, v := range headers {
		h.headers[k] = v
	}
	return h
}

// Prepare prepares raw data for publishing
func (h *DataPublisherHandler) Prepare(ctx context.Context) (*PublishData, error) {
	log.Printf("[%s] Preparing data (%d bytes)", h.subject, len(h.data))

	// Build headers
	headers := make(map[string]string)
	headers["content-type"] = h.contentType
	headers["timestamp"] = time.Now().Format(time.RFC3339)

	// Add custom headers
	for k, v := range h.headers {
		headers[k] = v
	}

	return &PublishData{
		Subject: h.subject,
		Data:    h.data,
		Headers: headers,
	}, nil
}
