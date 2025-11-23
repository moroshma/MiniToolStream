package handler

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/subscriber/domain"
)

// FileSaver saves message data to files
type FileSaver struct {
	outputDir string
	logger    Logger
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

// FileSaverConfig represents configuration for FileSaver
type FileSaverConfig struct {
	OutputDir string
	Logger    Logger
}

// NewFileSaver creates a new file saver handler
func NewFileSaver(config *FileSaverConfig) (*FileSaver, error) {
	logger := config.Logger
	if logger == nil {
		logger = &defaultLogger{}
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory %s: %w", config.OutputDir, err)
	}

	return &FileSaver{
		outputDir: config.OutputDir,
		logger:    logger,
	}, nil
}

// Handle saves the message data to a file
func (h *FileSaver) Handle(ctx context.Context, msg *domain.Message) error {
	// Skip if no data
	if len(msg.Data) == 0 {
		h.logger.Printf("   No data to save for sequence %d", msg.Sequence)
		return nil
	}

	// Print headers
	if len(msg.Headers) > 0 {
		h.logger.Printf("   Headers: %v", msg.Headers)
	}

	// Generate filename
	filename := filepath.Join(h.outputDir, fmt.Sprintf("%s_seq_%d", msg.Subject, msg.Sequence))

	// Add extension based on content-type
	if contentType, ok := msg.Headers["content-type"]; ok {
		ext := getFileExtension(contentType)
		if ext != "" {
			filename += ext
		}
	}

	// Save to file
	if err := os.WriteFile(filename, msg.Data, 0644); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	h.logger.Printf("   ✓ Saved to: %s", filename)
	return nil
}

// getFileExtension returns file extension for content type
func getFileExtension(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "text/plain":
		return ".txt"
	case "application/json":
		return ".json"
	case "application/xml":
		return ".xml"
	case "application/pdf":
		return ".pdf"
	case "application/octet-stream":
		return ".bin"
	default:
		return ""
	}
}
