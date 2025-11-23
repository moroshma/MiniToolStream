package handler

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// FilePublisherHandler publishes files
type FilePublisherHandler struct {
	subject     string
	filePath    string
	contentType string
}

// NewFilePublisherHandler creates a new file publisher handler
func NewFilePublisherHandler(subject, filePath, contentType string) *FilePublisherHandler {
	return &FilePublisherHandler{
		subject:     subject,
		filePath:    filePath,
		contentType: contentType,
	}
}

// Prepare reads the file and prepares it for publishing
func (h *FilePublisherHandler) Prepare(ctx context.Context) (*PublishData, error) {
	// Check if file exists
	if _, err := os.Stat(h.filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", h.filePath)
	}

	// Read file
	fileData, err := os.ReadFile(h.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", h.filePath, err)
	}

	log.Printf("[%s] Read file: %s (%d bytes)", h.subject, h.filePath, len(fileData))

	// Auto-detect content type if not provided
	contentType := h.contentType
	if contentType == "" {
		ext := filepath.Ext(h.filePath)
		switch ext {
		case ".json":
			contentType = "application/json"
		case ".xml":
			contentType = "application/xml"
		case ".pdf":
			contentType = "application/pdf"
		case ".txt":
			contentType = "text/plain"
		default:
			contentType = "application/octet-stream"
		}
	}

	return &PublishData{
		Subject: h.subject,
		Data:    fileData,
		Headers: map[string]string{
			"content-type": contentType,
			"filename":     filepath.Base(h.filePath),
			"timestamp":    time.Now().Format(time.RFC3339),
		},
	}, nil
}
