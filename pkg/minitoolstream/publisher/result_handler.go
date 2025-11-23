package publisher

import (
	"context"
	"fmt"

	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/domain"
)

// LoggingResultHandler logs publish results
type LoggingResultHandler struct {
	logger  Logger
	verbose bool
}

// NewLoggingResultHandler creates a new logging result handler
func NewLoggingResultHandler(logger Logger, verbose bool) *LoggingResultHandler {
	if logger == nil {
		logger = &defaultLogger{}
	}
	return &LoggingResultHandler{
		logger:  logger,
		verbose: verbose,
	}
}

// Handle logs the publish result
func (h *LoggingResultHandler) Handle(ctx context.Context, result *domain.PublishResult) error {
	if result == nil {
		return fmt.Errorf("result cannot be nil")
	}

	if result.StatusCode != 0 {
		h.logger.Printf("✗ Publish failed: error=%s", result.ErrorMessage)
		return nil
	}

	if h.verbose {
		h.logger.Printf("✓ Published: sequence=%d, object=%s",
			result.Sequence, result.ObjectName)
	} else {
		h.logger.Printf("✓ Published: sequence=%d", result.Sequence)
	}

	return nil
}
