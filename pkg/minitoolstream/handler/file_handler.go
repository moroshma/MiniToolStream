package handler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/domain"
)

// FileHandler publishes files
type FileHandler struct {
	subject     string
	filePath    string
	contentType string
	logger      Logger
}

// FileHandlerConfig represents configuration for FileHandler
type FileHandlerConfig struct {
	Subject     string
	FilePath    string
	ContentType string
	Logger      Logger
}

// NewFileHandler creates a new file handler
func NewFileHandler(config *FileHandlerConfig) *FileHandler {
	logger := config.Logger
	if logger == nil {
		logger = &defaultLogger{}
	}

	return &FileHandler{
		subject:     config.Subject,
		filePath:    config.FilePath,
		contentType: config.ContentType,
		logger:      logger,
	}
}

// Prepare reads the file and prepares it for publishing
func (h *FileHandler) Prepare(ctx context.Context) (*domain.Message, error) {
	// Check if file exists
	if _, err := os.Stat(h.filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", h.filePath)
	}

	// Read file
	fileData, err := os.ReadFile(h.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", h.filePath, err)
	}

	h.logger.Printf("[%s] Read file: %s (%d bytes)", h.subject, h.filePath, len(fileData))

	// Determine content type if not specified
	contentType := h.contentType
	if contentType == "" {
		contentType = detectContentType(h.filePath)
	}

	return &domain.Message{
		Subject: h.subject,
		Data:    fileData,
		Headers: map[string]string{
			"content-type": contentType,
			"filename":     filepath.Base(h.filePath),
			"timestamp":    time.Now().Format(time.RFC3339),
		},
	}, nil
}

// detectContentType attempts to detect content type from file extension
func detectContentType(filePath string) string {
	ext := filepath.Ext(filePath)
	switch ext {
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".txt":
		return "text/plain"
	case ".html":
		return "text/html"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}
