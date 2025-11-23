package handler

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	pb "github.com/moroshma/MiniToolStream/model"
)

// ImageProcessorHandler processes image messages
type ImageProcessorHandler struct {
	outputDir string
}

// NewImageProcessorHandler creates a new image processor handler
func NewImageProcessorHandler(outputDir string) *ImageProcessorHandler {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("Warning: Failed to create output directory %s: %v", outputDir, err)
	}

	return &ImageProcessorHandler{
		outputDir: outputDir,
	}
}

// Handle processes image messages
func (h *ImageProcessorHandler) Handle(ctx context.Context, msg *pb.Message) error {
	// Check if it's an image
	contentType, ok := msg.Headers["content-type"]
	if !ok || (contentType != "image/jpeg" && contentType != "image/png") {
		log.Printf("   Skipping non-image message: content-type=%s", contentType)
		return nil
	}

	if len(msg.Data) == 0 {
		return fmt.Errorf("image message has no data")
	}

	// Generate filename
	extension := ".jpg"
	if contentType == "image/png" {
		extension = ".png"
	}
	filename := filepath.Join(h.outputDir, fmt.Sprintf("image_%s_seq_%d%s", msg.Subject, msg.Sequence, extension))

	// Save image
	if err := os.WriteFile(filename, msg.Data, 0644); err != nil {
		return fmt.Errorf("failed to save image: %w", err)
	}

	log.Printf("   📷 Processed image: %s (%d bytes)", filename, len(msg.Data))

	// Here you could add actual image processing:
	// - Resize
	// - Watermark
	// - Format conversion
	// - Thumbnail generation
	// etc.

	return nil
}
