package handler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/subscriber/domain"
)

// ImageProcessor processes and saves image messages
type ImageProcessor struct {
	outputDir string
	logger    Logger
}

// ImageProcessorConfig represents configuration for ImageProcessor
type ImageProcessorConfig struct {
	OutputDir string
	Logger    Logger
}

// NewImageProcessor creates a new image processor handler
func NewImageProcessor(config *ImageProcessorConfig) (*ImageProcessor, error) {
	logger := config.Logger
	if logger == nil {
		logger = &defaultLogger{}
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory %s: %w", config.OutputDir, err)
	}

	return &ImageProcessor{
		outputDir: config.OutputDir,
		logger:    logger,
	}, nil
}

// Handle processes and saves the image
func (h *ImageProcessor) Handle(ctx context.Context, msg *domain.Message) error {
	// Skip if no data
	if len(msg.Data) == 0 {
		h.logger.Printf("   No image data for sequence %d", msg.Sequence)
		return nil
	}

	// Get original filename from headers if available
	var filename string
	if origFilename, ok := msg.Headers["filename"]; ok {
		filename = filepath.Join(h.outputDir, fmt.Sprintf("%s_seq_%d_%s", msg.Subject, msg.Sequence, origFilename))
	} else {
		// Generate filename based on content-type
		filename = filepath.Join(h.outputDir, fmt.Sprintf("%s_seq_%d", msg.Subject, msg.Sequence))

		// Add extension based on content-type
		if contentType, ok := msg.Headers["content-type"]; ok {
			ext := getImageExtension(contentType)
			if ext != "" {
				filename += ext
			}
		}
	}

	// Log image metadata
	h.logger.Printf("   Image: %d bytes", len(msg.Data))
	if contentType, ok := msg.Headers["content-type"]; ok {
		h.logger.Printf("   Content-Type: %s", contentType)
	}

	// Save to file
	if err := os.WriteFile(filename, msg.Data, 0644); err != nil {
		return fmt.Errorf("failed to save image: %w", err)
	}

	h.logger.Printf("   ✓ Image saved to: %s", filename)
	return nil
}

// getImageExtension returns image file extension for content type
func getImageExtension(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	case "image/svg+xml":
		return ".svg"
	default:
		return ""
	}
}
