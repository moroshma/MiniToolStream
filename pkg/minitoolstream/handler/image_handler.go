package handler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/domain"
)

// ImageHandler publishes image files
type ImageHandler struct {
	subject   string
	imagePath string
	logger    Logger
}

// ImageHandlerConfig represents configuration for ImageHandler
type ImageHandlerConfig struct {
	Subject   string
	ImagePath string
	Logger    Logger
}

// NewImageHandler creates a new image handler
func NewImageHandler(config *ImageHandlerConfig) *ImageHandler {
	logger := config.Logger
	if logger == nil {
		logger = &defaultLogger{}
	}

	return &ImageHandler{
		subject:   config.Subject,
		imagePath: config.ImagePath,
		logger:    logger,
	}
}

// Prepare reads the image file and prepares it for publishing
func (h *ImageHandler) Prepare(ctx context.Context) (*domain.Message, error) {
	// Check if file exists
	if _, err := os.Stat(h.imagePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("image file not found: %s", h.imagePath)
	}

	// Read image file
	imageData, err := os.ReadFile(h.imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image file %s: %w", h.imagePath, err)
	}

	h.logger.Printf("[%s] Read image file: %s (%d bytes)", h.subject, h.imagePath, len(imageData))

	// Determine content type from file extension
	contentType := detectImageContentType(h.imagePath)

	return &domain.Message{
		Subject: h.subject,
		Data:    imageData,
		Headers: map[string]string{
			"content-type": contentType,
			"filename":     filepath.Base(h.imagePath),
			"timestamp":    time.Now().Format(time.RFC3339),
		},
	}, nil
}

// detectImageContentType determines image content type from file extension
func detectImageContentType(imagePath string) string {
	ext := filepath.Ext(imagePath)
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".svg":
		return "image/svg+xml"
	default:
		return "image/jpeg" // default to jpeg
	}
}
