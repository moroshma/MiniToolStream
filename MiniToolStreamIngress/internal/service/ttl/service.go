package ttl

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/pkg/logger"
)

// MessageInfo represents information about a deleted message
type MessageInfo struct {
	Sequence   uint64
	Subject    string
	ObjectName string
}

// MessageRepository defines the interface for message storage operations
type MessageRepository interface {
	DeleteOldMessages(ttlSeconds int) (int, []MessageInfo, error)
}

// StorageRepository defines the interface for object storage operations
type StorageRepository interface {
	DeleteObject(ctx context.Context, objectName string) error
}

// Service handles TTL cleanup operations
type Service struct {
	messageRepo MessageRepository
	storageRepo StorageRepository
	logger      *logger.Logger
	ttlDuration time.Duration
	interval    time.Duration
	enabled     bool

	stopCh chan struct{}
	wg     sync.WaitGroup
	mu     sync.Mutex
}

// Config represents TTL service configuration
type Config struct {
	Enabled     bool
	TTLDuration time.Duration
	Interval    time.Duration
}

// NewService creates a new TTL cleanup service
func NewService(
	messageRepo MessageRepository,
	storageRepo StorageRepository,
	cfg Config,
	log *logger.Logger,
) *Service {
	return &Service{
		messageRepo: messageRepo,
		storageRepo: storageRepo,
		logger:      log,
		ttlDuration: cfg.TTLDuration,
		interval:    cfg.Interval,
		enabled:     cfg.Enabled,
		stopCh:      make(chan struct{}),
	}
}

// Start starts the TTL cleanup service
func (s *Service) Start(ctx context.Context) error {
	if !s.enabled {
		s.logger.Info("TTL cleanup service is disabled")
		return nil
	}

	s.logger.Info("Starting TTL cleanup service",
		logger.Duration("ttl_duration", s.ttlDuration),
		logger.Duration("interval", s.interval),
	)

	s.wg.Add(1)
	go s.cleanupLoop(ctx)

	return nil
}

// Stop stops the TTL cleanup service
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopCh == nil {
		return
	}

	s.logger.Info("Stopping TTL cleanup service...")
	close(s.stopCh)
	s.wg.Wait()
	s.logger.Info("TTL cleanup service stopped")
}

// cleanupLoop runs the cleanup process periodically
func (s *Service) cleanupLoop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run cleanup immediately on start
	if err := s.runCleanup(ctx); err != nil {
		s.logger.Error("Initial TTL cleanup failed", logger.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("TTL cleanup service context cancelled")
			return
		case <-s.stopCh:
			s.logger.Info("TTL cleanup service received stop signal")
			return
		case <-ticker.C:
			if err := s.runCleanup(ctx); err != nil {
				s.logger.Error("TTL cleanup failed", logger.Error(err))
			}
		}
	}
}

// runCleanup executes the cleanup process
func (s *Service) runCleanup(ctx context.Context) error {
	startTime := time.Now()
	ttlSeconds := int(s.ttlDuration.Seconds())

	s.logger.Info("Running TTL cleanup",
		logger.Int("ttl_seconds", ttlSeconds),
	)

	// Delete old messages from Tarantool and get list of deleted messages
	deletedCount, deletedMessages, err := s.messageRepo.DeleteOldMessages(ttlSeconds)
	if err != nil {
		return fmt.Errorf("failed to delete old messages from Tarantool: %w", err)
	}

	if deletedCount == 0 {
		s.logger.Info("No messages to clean up")
		return nil
	}

	s.logger.Info("Deleted old messages from Tarantool",
		logger.Int("count", deletedCount),
	)

	// Delete corresponding objects from MinIO
	deletedFromMinIO := 0
	failedDeletes := 0

	for _, msg := range deletedMessages {
		if err := s.storageRepo.DeleteObject(ctx, msg.ObjectName); err != nil {
			s.logger.Error("Failed to delete object from MinIO",
				logger.String("object_name", msg.ObjectName),
				logger.Uint64("sequence", msg.Sequence),
				logger.String("subject", msg.Subject),
				logger.Error(err),
			)
			failedDeletes++
			continue
		}
		deletedFromMinIO++
	}

	duration := time.Since(startTime)

	s.logger.Info("TTL cleanup completed",
		logger.Int("tarantool_deleted", deletedCount),
		logger.Int("minio_deleted", deletedFromMinIO),
		logger.Int("minio_failed", failedDeletes),
		logger.Duration("duration", duration),
	)

	return nil
}

// RunOnce runs the cleanup process once (useful for testing)
func (s *Service) RunOnce(ctx context.Context) error {
	return s.runCleanup(ctx)
}
