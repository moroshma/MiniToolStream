package usecase

import (
	"context"
	"fmt"

	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/pkg/logger"
)

// MessageRepository defines the interface for message storage
type MessageRepository interface {
	PublishMessage(subject string, headers map[string]string) (uint64, error)
	Ping() error
	Close() error
}

// StorageRepository defines the interface for object storage
type StorageRepository interface {
	UploadData(ctx context.Context, objectName string, data []byte, contentType string) error
	GetObjectURL(objectName string) string
	EnsureBucket(ctx context.Context) error
}

// PublishUseCase handles message publishing logic
type PublishUseCase struct {
	messageRepo MessageRepository
	storageRepo StorageRepository
	logger      *logger.Logger
}

// NewPublishUseCase creates a new publish use case
func NewPublishUseCase(
	messageRepo MessageRepository,
	storageRepo StorageRepository,
	log *logger.Logger,
) *PublishUseCase {
	return &PublishUseCase{
		messageRepo: messageRepo,
		storageRepo: storageRepo,
		logger:      log,
	}
}

// PublishRequest represents a publish request
type PublishRequest struct {
	Subject string
	Data    []byte
	Headers map[string]string
}

// PublishResponse represents a publish response
type PublishResponse struct {
	Sequence   uint64
	ObjectName string
}

// Publish publishes a message with optional data to storage
func (uc *PublishUseCase) Publish(ctx context.Context, req *PublishRequest) (*PublishResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	if req.Subject == "" {
		return nil, fmt.Errorf("subject cannot be empty")
	}

	uc.logger.Info("Publishing message",
		logger.String("subject", req.Subject),
		logger.Int("data_size", len(req.Data)),
	)

	// Publish message metadata to Tarantool
	sequence, err := uc.messageRepo.PublishMessage(req.Subject, req.Headers)
	if err != nil {
		uc.logger.Error("Failed to publish message metadata",
			logger.String("subject", req.Subject),
			logger.Error(err),
		)
		return nil, fmt.Errorf("failed to publish message: %w", err)
	}

	// Generate object name based on subject and sequence
	objectName := fmt.Sprintf("%s_%d", req.Subject, sequence)

	// Upload data to MinIO if present
	if len(req.Data) > 0 {
		contentType := "application/octet-stream"
		if ct, ok := req.Headers["content-type"]; ok {
			contentType = ct
		}

		err = uc.storageRepo.UploadData(ctx, objectName, req.Data, contentType)
		if err != nil {
			uc.logger.Error("Failed to upload data to storage",
				logger.String("subject", req.Subject),
				logger.Uint64("sequence", sequence),
				logger.String("object_name", objectName),
				logger.Error(err),
			)
			return nil, fmt.Errorf("failed to upload data: %w", err)
		}
	}

	uc.logger.Info("Message published successfully",
		logger.String("subject", req.Subject),
		logger.Uint64("sequence", sequence),
		logger.String("object_name", objectName),
	)

	return &PublishResponse{
		Sequence:   sequence,
		ObjectName: objectName,
	}, nil
}

// HealthCheck checks if all dependencies are healthy
func (uc *PublishUseCase) HealthCheck(ctx context.Context) error {
	// Check message repository
	if err := uc.messageRepo.Ping(); err != nil {
		return fmt.Errorf("message repository unhealthy: %w", err)
	}

	// Check storage repository
	if err := uc.storageRepo.EnsureBucket(ctx); err != nil {
		return fmt.Errorf("storage repository unhealthy: %w", err)
	}

	return nil
}
