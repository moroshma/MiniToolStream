package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/domain/entity"
	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/domain/repository"
	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/pkg/logger"
)

// MessageUseCase handles business logic for message operations
type MessageUseCase struct {
	messageRepo  repository.MessageRepository
	storageRepo  repository.StorageRepository
	logger       *logger.Logger
	pollInterval time.Duration
}

// NewMessageUseCase creates a new message use case
func NewMessageUseCase(
	messageRepo repository.MessageRepository,
	storageRepo repository.StorageRepository,
	logger *logger.Logger,
	pollInterval time.Duration,
) *MessageUseCase {
	return &MessageUseCase{
		messageRepo:  messageRepo,
		storageRepo:  storageRepo,
		logger:       logger,
		pollInterval: pollInterval,
	}
}

// Subscribe polls for new messages and sends notifications
func (uc *MessageUseCase) Subscribe(
	ctx context.Context,
	subject string,
	durableName string,
	startSequence *uint64,
	notificationChan chan<- *entity.Notification,
) error {
	// Get initial consumer position
	lastSequence, err := uc.messageRepo.GetConsumerPosition(ctx, durableName, subject)
	if err != nil {
		return fmt.Errorf("failed to get consumer position: %w", err)
	}

	// Use start_sequence if provided
	if startSequence != nil && *startSequence > lastSequence {
		lastSequence = *startSequence
	}

	uc.logger.Info("Starting subscription",
		logger.String("subject", subject),
		logger.String("durable_name", durableName),
		logger.Uint64("start_sequence", lastSequence),
	)

	// Send initial notification if there are messages
	latestSeq, err := uc.messageRepo.GetLatestSequenceForSubject(ctx, subject)
	if err != nil {
		return fmt.Errorf("failed to get latest sequence: %w", err)
	}

	if latestSeq > lastSequence {
		select {
		case notificationChan <- &entity.Notification{
			Subject:  subject,
			Sequence: latestSeq,
		}:
			lastSequence = latestSeq
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Polling loop
	ticker := time.NewTicker(uc.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			uc.logger.Info("Subscription cancelled", logger.String("subject", subject))
			return ctx.Err()

		case <-ticker.C:
			// Check for new messages
			latestSeq, err := uc.messageRepo.GetLatestSequenceForSubject(ctx, subject)
			if err != nil {
				uc.logger.Error("Failed to get latest sequence", logger.Error(err))
				continue
			}

			if latestSeq > lastSequence {
				select {
				case notificationChan <- &entity.Notification{
					Subject:  subject,
					Sequence: latestSeq,
				}:
					uc.logger.Debug("Sent notification",
						logger.String("subject", subject),
						logger.Uint64("sequence", latestSeq),
					)
					lastSequence = latestSeq
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}
}

// FetchMessages fetches a batch of messages for a durable consumer
func (uc *MessageUseCase) FetchMessages(
	ctx context.Context,
	subject string,
	durableName string,
	batchSize int,
) ([]*entity.Message, error) {
	if batchSize <= 0 {
		batchSize = 10 // Default batch size
	}

	// Get consumer position
	lastSequence, err := uc.messageRepo.GetConsumerPosition(ctx, durableName, subject)
	if err != nil {
		return nil, fmt.Errorf("failed to get consumer position: %w", err)
	}

	uc.logger.Debug("Fetching messages",
		logger.String("subject", subject),
		logger.String("durable_name", durableName),
		logger.Uint64("last_sequence", lastSequence),
		logger.Int("batch_size", batchSize),
	)

	// Fetch messages from repository
	messages, err := uc.messageRepo.GetMessagesBySubject(ctx, subject, lastSequence+1, batchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	// Load data from storage for each message
	for _, msg := range messages {
		if msg.ObjectName != "" {
			data, err := uc.storageRepo.GetObject(ctx, msg.Subject, msg.ObjectName)
			if err != nil {
				uc.logger.Warn("Failed to get data from storage",
					logger.String("object_name", msg.ObjectName),
					logger.Error(err),
				)
				msg.Data = nil
			} else {
				msg.Data = data
			}
		}

		// Update consumer position
		err = uc.messageRepo.UpdateConsumerPosition(ctx, durableName, subject, msg.Sequence)
		if err != nil {
			uc.logger.Warn("Failed to update consumer position",
				logger.Uint64("sequence", msg.Sequence),
				logger.Error(err),
			)
		}
	}

	uc.logger.Info("Fetched messages",
		logger.String("subject", subject),
		logger.Int("count", len(messages)),
	)

	return messages, nil
}

// GetLastSequence returns the latest sequence number for a subject
func (uc *MessageUseCase) GetLastSequence(ctx context.Context, subject string) (uint64, error) {
	latestSeq, err := uc.messageRepo.GetLatestSequenceForSubject(ctx, subject)
	if err != nil {
		return 0, fmt.Errorf("failed to get latest sequence: %w", err)
	}

	uc.logger.Debug("Got latest sequence",
		logger.String("subject", subject),
		logger.Uint64("sequence", latestSeq),
	)

	return latestSeq, nil
}
