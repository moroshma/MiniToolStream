package repository

import (
	"context"

	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/domain/entity"
)

// MessageRepository defines the interface for message storage operations
type MessageRepository interface {
	// GetConsumerPosition returns the last read sequence for a durable consumer
	GetConsumerPosition(ctx context.Context, durableName, subject string) (uint64, error)

	// UpdateConsumerPosition updates the last read sequence for a durable consumer
	UpdateConsumerPosition(ctx context.Context, durableName, subject string, lastSequence uint64) error

	// GetLatestSequenceForSubject returns the latest sequence number for a subject
	GetLatestSequenceForSubject(ctx context.Context, subject string) (uint64, error)

	// GetMessagesBySubject fetches messages for a subject starting from a sequence
	GetMessagesBySubject(ctx context.Context, subject string, startSequence uint64, limit int) ([]*entity.Message, error)

	// GetMessageBySequence gets a single message by its sequence number
	GetMessageBySequence(ctx context.Context, sequence uint64) (*entity.Message, error)
}
