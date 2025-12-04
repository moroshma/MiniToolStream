package ttl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockMessageRepository is a mock implementation of MessageRepository
type MockMessageRepository struct {
	mock.Mock
}

func (m *MockMessageRepository) DeleteOldMessages(ttlSeconds int) (int, []MessageInfo, error) {
	args := m.Called(ttlSeconds)
	if args.Get(1) == nil {
		return args.Int(0), nil, args.Error(2)
	}
	return args.Int(0), args.Get(1).([]MessageInfo), args.Error(2)
}

// MockStorageRepository is a mock implementation of StorageRepository
type MockStorageRepository struct {
	mock.Mock
}

func (m *MockStorageRepository) DeleteObject(ctx context.Context, objectName string) error {
	args := m.Called(ctx, objectName)
	return args.Error(0)
}

func TestNewService(t *testing.T) {
	messageRepo := &MockMessageRepository{}
	storageRepo := &MockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "info", Format: "json"})

	cfg := Config{
		Enabled:     true,
		TTLDuration: 24 * time.Hour,
		Interval:    1 * time.Hour,
	}

	service := NewService(messageRepo, storageRepo, cfg, log)

	assert.NotNil(t, service)
	assert.Equal(t, true, service.enabled)
	assert.Equal(t, 24*time.Hour, service.ttlDuration)
	assert.Equal(t, 1*time.Hour, service.interval)
}

func TestRunOnce_Success(t *testing.T) {
	messageRepo := &MockMessageRepository{}
	storageRepo := &MockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "info", Format: "json"})

	cfg := Config{
		Enabled:     true,
		TTLDuration: 24 * time.Hour,
		Interval:    1 * time.Hour,
	}

	service := NewService(messageRepo, storageRepo, cfg, log)

	// Mock deleted messages
	deletedMessages := []MessageInfo{
		{Sequence: 1, Subject: "test", ObjectName: "test_1"},
		{Sequence: 2, Subject: "test", ObjectName: "test_2"},
	}

	ctx := context.Background()

	messageRepo.On("DeleteOldMessages", 86400).Return(2, deletedMessages, nil)
	storageRepo.On("DeleteObject", ctx, "test_1").Return(nil)
	storageRepo.On("DeleteObject", ctx, "test_2").Return(nil)

	err := service.RunOnce(ctx)

	assert.NoError(t, err)
	messageRepo.AssertExpectations(t)
	storageRepo.AssertExpectations(t)
}

func TestRunOnce_NoMessagesToDelete(t *testing.T) {
	messageRepo := &MockMessageRepository{}
	storageRepo := &MockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "info", Format: "json"})

	cfg := Config{
		Enabled:     true,
		TTLDuration: 24 * time.Hour,
		Interval:    1 * time.Hour,
	}

	service := NewService(messageRepo, storageRepo, cfg, log)

	ctx := context.Background()

	messageRepo.On("DeleteOldMessages", 86400).Return(0, []MessageInfo{}, nil)

	err := service.RunOnce(ctx)

	assert.NoError(t, err)
	messageRepo.AssertExpectations(t)
	storageRepo.AssertNotCalled(t, "DeleteObject")
}

func TestRunOnce_MessageRepoError(t *testing.T) {
	messageRepo := &MockMessageRepository{}
	storageRepo := &MockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "info", Format: "json"})

	cfg := Config{
		Enabled:     true,
		TTLDuration: 24 * time.Hour,
		Interval:    1 * time.Hour,
	}

	service := NewService(messageRepo, storageRepo, cfg, log)

	ctx := context.Background()

	messageRepo.On("DeleteOldMessages", 86400).Return(0, nil, errors.New("connection error"))

	err := service.RunOnce(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection error")
	messageRepo.AssertExpectations(t)
	storageRepo.AssertNotCalled(t, "DeleteObject")
}

func TestRunOnce_PartialStorageFailure(t *testing.T) {
	messageRepo := &MockMessageRepository{}
	storageRepo := &MockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "info", Format: "json"})

	cfg := Config{
		Enabled:     true,
		TTLDuration: 24 * time.Hour,
		Interval:    1 * time.Hour,
	}

	service := NewService(messageRepo, storageRepo, cfg, log)

	// Mock deleted messages
	deletedMessages := []MessageInfo{
		{Sequence: 1, Subject: "test", ObjectName: "test_1"},
		{Sequence: 2, Subject: "test", ObjectName: "test_2"},
	}

	ctx := context.Background()

	messageRepo.On("DeleteOldMessages", 86400).Return(2, deletedMessages, nil)
	storageRepo.On("DeleteObject", ctx, "test_1").Return(nil)
	storageRepo.On("DeleteObject", ctx, "test_2").Return(errors.New("object not found"))

	err := service.RunOnce(ctx)

	// Should not return error, just log it
	assert.NoError(t, err)
	messageRepo.AssertExpectations(t)
	storageRepo.AssertExpectations(t)
}

func TestStart_Disabled(t *testing.T) {
	messageRepo := &MockMessageRepository{}
	storageRepo := &MockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "info", Format: "json"})

	cfg := Config{
		Enabled:     false,
		TTLDuration: 24 * time.Hour,
		Interval:    1 * time.Hour,
	}

	service := NewService(messageRepo, storageRepo, cfg, log)

	ctx := context.Background()
	err := service.Start(ctx)

	assert.NoError(t, err)
	messageRepo.AssertNotCalled(t, "DeleteOldMessages")
	storageRepo.AssertNotCalled(t, "DeleteObject")
}

func TestStart_Stop(t *testing.T) {
	messageRepo := &MockMessageRepository{}
	storageRepo := &MockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "info", Format: "json"})

	cfg := Config{
		Enabled:     true,
		TTLDuration: 24 * time.Hour,
		Interval:    100 * time.Millisecond, // Short interval for testing
	}

	service := NewService(messageRepo, storageRepo, cfg, log)

	ctx := context.Background()

	// Mock initial cleanup
	messageRepo.On("DeleteOldMessages", 86400).Return(0, []MessageInfo{}, nil).Maybe()

	err := service.Start(ctx)
	assert.NoError(t, err)

	// Let it run briefly
	time.Sleep(50 * time.Millisecond)

	// Stop the service
	service.Stop()

	// Should stop gracefully
	messageRepo.AssertExpectations(t)
}
