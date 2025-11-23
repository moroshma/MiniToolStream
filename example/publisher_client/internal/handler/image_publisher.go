package handler

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// ImagePublisherHandler publishes image files
type ImagePublisherHandler struct {
	subject   string
	imagePath string
}

// NewImagePublisherHandler creates a new image publisher handler
func NewImagePublisherHandler(subject, imagePath string) *ImagePublisherHandler {
	return &ImagePublisherHandler{
		subject:   subject,
		imagePath: imagePath,
	}
}

// Prepare reads the image file and prepares it for publishing
func (h *ImagePublisherHandler) Prepare(ctx context.Context) (*PublishData, error) {
	// Check if file exists
	if _, err := os.Stat(h.imagePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("image file not found: %s", h.imagePath)
	}

	// Read image file
	imageData, err := os.ReadFile(h.imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image file %s: %w", h.imagePath, err)
	}

	log.Printf("[%s] Read image file: %s (%d bytes)", h.subject, h.imagePath, len(imageData))

	// Determine content type from file extension
	contentType := "image/jpeg"
	ext := filepath.Ext(h.imagePath)
	switch ext {
	case ".png":
		contentType = "image/png"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".gif":
		contentType = "image/gif"
	case ".webp":
		contentType = "image/webp"
	}

	return &PublishData{
		Subject: h.subject,
		Data:    imageData,
		Headers: map[string]string{
			"content-type": contentType,
			"filename":     filepath.Base(h.imagePath),
			"timestamp":    time.Now().Format(time.RFC3339),
		},
	}, nil
}
