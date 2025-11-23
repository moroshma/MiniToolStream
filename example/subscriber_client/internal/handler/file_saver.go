package handler

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	pb "github.com/moroshma/MiniToolStream/model"
)

// FileSaverHandler saves message data to files
type FileSaverHandler struct {
	outputDir string
}

// NewFileSaverHandler creates a new file saver handler
func NewFileSaverHandler(outputDir string) *FileSaverHandler {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("Warning: Failed to create output directory %s: %v", outputDir, err)
	}

	return &FileSaverHandler{
		outputDir: outputDir,
	}
}

// Handle saves the message data to a file
func (h *FileSaverHandler) Handle(ctx context.Context, msg *pb.Message) error {
	// Skip if no data
	if len(msg.Data) == 0 {
		log.Printf("   No data to save for sequence %d", msg.Sequence)
		return nil
	}

	// Print headers
	if len(msg.Headers) > 0 {
		log.Printf("   Headers: %v", msg.Headers)
	}

	// Generate filename
	filename := filepath.Join(h.outputDir, fmt.Sprintf("%s_seq_%d", msg.Subject, msg.Sequence))

	// Add extension based on content-type
	if contentType, ok := msg.Headers["content-type"]; ok {
		switch contentType {
		case "image/jpeg":
			filename += ".jpg"
		case "image/png":
			filename += ".png"
		case "text/plain":
			filename += ".txt"
		case "application/json":
			filename += ".json"
		case "application/octet-stream":
			filename += ".bin"
		}
	}

	// Save to file
	if err := os.WriteFile(filename, msg.Data, 0644); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	log.Printf("   ✓ Saved to: %s", filename)
	return nil
}
