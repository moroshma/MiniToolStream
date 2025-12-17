package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/domain/entity"
	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/pkg/logger"
)

type mockMessageRepository struct {
	getConsumerPositionFunc        func(ctx context.Context, durableName, subject string) (uint64, error)
	getLatestSequenceForSubjectFunc func(ctx context.Context, subject string) (uint64, error)
	getMessagesBySubjectFunc       func(ctx context.Context, subject string, startSeq uint64, limit int) ([]*entity.Message, error)
	updateConsumerPositionFunc     func(ctx context.Context, durableName, subject string, sequence uint64) error
	getMessageBySequenceFunc       func(ctx context.Context, sequence uint64) (*entity.Message, error)
}

func (m *mockMessageRepository) GetConsumerPosition(ctx context.Context, durableName, subject string) (uint64, error) {
	if m.getConsumerPositionFunc != nil {
		return m.getConsumerPositionFunc(ctx, durableName, subject)
	}
	return 0, nil
}

func (m *mockMessageRepository) GetLatestSequenceForSubject(ctx context.Context, subject string) (uint64, error) {
	if m.getLatestSequenceForSubjectFunc != nil {
		return m.getLatestSequenceForSubjectFunc(ctx, subject)
	}
	return 0, nil
}

func (m *mockMessageRepository) GetMessagesBySubject(ctx context.Context, subject string, startSeq uint64, limit int) ([]*entity.Message, error) {
	if m.getMessagesBySubjectFunc != nil {
		return m.getMessagesBySubjectFunc(ctx, subject, startSeq, limit)
	}
	return nil, nil
}

func (m *mockMessageRepository) UpdateConsumerPosition(ctx context.Context, durableName, subject string, sequence uint64) error {
	if m.updateConsumerPositionFunc != nil {
		return m.updateConsumerPositionFunc(ctx, durableName, subject, sequence)
	}
	return nil
}

func (m *mockMessageRepository) GetMessageBySequence(ctx context.Context, sequence uint64) (*entity.Message, error) {
	if m.getMessageBySequenceFunc != nil {
		return m.getMessageBySequenceFunc(ctx, sequence)
	}
	return nil, nil
}

type mockStorageRepository struct {
	getObjectFunc    func(ctx context.Context, subject, objectName string) ([]byte, error)
	getObjectURLFunc func(objectName string) string
}

func (m *mockStorageRepository) GetObject(ctx context.Context, subject, objectName string) ([]byte, error) {
	if m.getObjectFunc != nil {
		return m.getObjectFunc(ctx, subject, objectName)
	}
	return nil, nil
}

func (m *mockStorageRepository) GetObjectURL(subject, objectName string) string {
	if m.getObjectURLFunc != nil {
		return m.getObjectURLFunc(objectName)
	}
	return ""
}

func TestNewMessageUseCase(t *testing.T) {
	msgRepo := &mockMessageRepository{}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewMessageUseCase(msgRepo, storageRepo, log, time.Second)
	if uc == nil {
		t.Fatal("expected non-nil usecase")
	}
	if uc.messageRepo == nil {
		t.Error("expected non-nil messageRepo")
	}
	if uc.storageRepo == nil {
		t.Error("expected non-nil storageRepo")
	}
	if uc.logger == nil {
		t.Error("expected non-nil logger")
	}
	if uc.pollInterval != time.Second {
		t.Errorf("expected pollInterval 1s, got %v", uc.pollInterval)
	}
}

func TestMessageUseCase_FetchMessages_DefaultBatchSize(t *testing.T) {
	var actualLimit int
	msgRepo := &mockMessageRepository{
		getConsumerPositionFunc: func(ctx context.Context, durableName, subject string) (uint64, error) {
			return 10, nil
		},
		getMessagesBySubjectFunc: func(ctx context.Context, subject string, startSeq uint64, limit int) ([]*entity.Message, error) {
			actualLimit = limit
			return []*entity.Message{}, nil
		},
	}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewMessageUseCase(msgRepo, storageRepo, log, time.Second)
	ctx := context.Background()

	_, err := uc.FetchMessages(ctx, "test.subject", "test-consumer", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if actualLimit != 10 {
		t.Errorf("expected default batch size 10, got %d", actualLimit)
	}
}

func TestMessageUseCase_FetchMessages_GetConsumerPositionError(t *testing.T) {
	msgRepo := &mockMessageRepository{
		getConsumerPositionFunc: func(ctx context.Context, durableName, subject string) (uint64, error) {
			return 0, errors.New("database error")
		},
	}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewMessageUseCase(msgRepo, storageRepo, log, time.Second)
	ctx := context.Background()

	_, err := uc.FetchMessages(ctx, "test.subject", "test-consumer", 5)
	if err == nil {
		t.Fatal("expected error from GetConsumerPosition")
	}
}

func TestMessageUseCase_FetchMessages_GetMessagesError(t *testing.T) {
	msgRepo := &mockMessageRepository{
		getConsumerPositionFunc: func(ctx context.Context, durableName, subject string) (uint64, error) {
			return 5, nil
		},
		getMessagesBySubjectFunc: func(ctx context.Context, subject string, startSeq uint64, limit int) ([]*entity.Message, error) {
			return nil, errors.New("fetch error")
		},
	}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewMessageUseCase(msgRepo, storageRepo, log, time.Second)
	ctx := context.Background()

	_, err := uc.FetchMessages(ctx, "test.subject", "test-consumer", 5)
	if err == nil {
		t.Fatal("expected error from GetMessagesBySubject")
	}
}

func TestMessageUseCase_FetchMessages_Success_WithData(t *testing.T) {
	testData := []byte("test message data")
	msgRepo := &mockMessageRepository{
		getConsumerPositionFunc: func(ctx context.Context, durableName, subject string) (uint64, error) {
			return 0, nil
		},
		getMessagesBySubjectFunc: func(ctx context.Context, subject string, startSeq uint64, limit int) ([]*entity.Message, error) {
			return []*entity.Message{
				{
					Sequence:   1,
					Subject:    "test.subject",
					ObjectName: "test_object_1",
					Headers:    map[string]string{"key": "value"},
					Timestamp:  time.Now(),
				},
			}, nil
		},
		updateConsumerPositionFunc: func(ctx context.Context, durableName, subject string, sequence uint64) error {
			return nil
		},
	}
	storageRepo := &mockStorageRepository{
		getObjectFunc: func(ctx context.Context, subject, objectName string) ([]byte, error) {
			return testData, nil
		},
	}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewMessageUseCase(msgRepo, storageRepo, log, time.Second)
	ctx := context.Background()

	messages, err := uc.FetchMessages(ctx, "test.subject", "test-consumer", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	if string(messages[0].Data) != string(testData) {
		t.Errorf("expected data '%s', got '%s'", string(testData), string(messages[0].Data))
	}
}

func TestMessageUseCase_FetchMessages_UpdatePositionError(t *testing.T) {
	msgRepo := &mockMessageRepository{
		getConsumerPositionFunc: func(ctx context.Context, durableName, subject string) (uint64, error) {
			return 0, nil
		},
		getMessagesBySubjectFunc: func(ctx context.Context, subject string, startSeq uint64, limit int) ([]*entity.Message, error) {
			return []*entity.Message{
				{
					Sequence:   1,
					Subject:    "test.subject",
					ObjectName: "",
					Headers:    map[string]string{},
					Timestamp:  time.Now(),
				},
			}, nil
		},
		updateConsumerPositionFunc: func(ctx context.Context, durableName, subject string, sequence uint64) error {
			return errors.New("update error")
		},
	}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewMessageUseCase(msgRepo, storageRepo, log, time.Second)
	ctx := context.Background()

	messages, err := uc.FetchMessages(ctx, "test.subject", "test-consumer", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
}

func TestMessageUseCase_GetLastSequence_Success(t *testing.T) {
	msgRepo := &mockMessageRepository{
		getLatestSequenceForSubjectFunc: func(ctx context.Context, subject string) (uint64, error) {
			return 42, nil
		},
	}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewMessageUseCase(msgRepo, storageRepo, log, time.Second)
	ctx := context.Background()

	seq, err := uc.GetLastSequence(ctx, "test.subject")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if seq != 42 {
		t.Errorf("expected sequence 42, got %d", seq)
	}
}

func TestMessageUseCase_GetLastSequence_Error(t *testing.T) {
	msgRepo := &mockMessageRepository{
		getLatestSequenceForSubjectFunc: func(ctx context.Context, subject string) (uint64, error) {
			return 0, errors.New("database error")
		},
	}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewMessageUseCase(msgRepo, storageRepo, log, time.Second)
	ctx := context.Background()

	_, err := uc.GetLastSequence(ctx, "test.subject")
	if err == nil {
		t.Fatal("expected error from GetLatestSequenceForSubject")
	}
}

func TestMessageUseCase_Subscribe_GetConsumerPositionError(t *testing.T) {
	msgRepo := &mockMessageRepository{
		getConsumerPositionFunc: func(ctx context.Context, durableName, subject string) (uint64, error) {
			return 0, errors.New("position error")
		},
	}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewMessageUseCase(msgRepo, storageRepo, log, time.Millisecond*10)
	ctx := context.Background()
	notifChan := make(chan *entity.Notification, 10)

	err := uc.Subscribe(ctx, "test.subject", "test-consumer", nil, notifChan)
	if err == nil {
		t.Fatal("expected error from GetConsumerPosition")
	}
}

func TestMessageUseCase_Subscribe_GetLatestSequenceError(t *testing.T) {
	msgRepo := &mockMessageRepository{
		getConsumerPositionFunc: func(ctx context.Context, durableName, subject string) (uint64, error) {
			return 0, nil
		},
		getLatestSequenceForSubjectFunc: func(ctx context.Context, subject string) (uint64, error) {
			return 0, errors.New("latest seq error")
		},
	}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewMessageUseCase(msgRepo, storageRepo, log, time.Millisecond*10)
	ctx := context.Background()
	notifChan := make(chan *entity.Notification, 10)

	err := uc.Subscribe(ctx, "test.subject", "test-consumer", nil, notifChan)
	if err == nil {
		t.Fatal("expected error from GetLatestSequenceForSubject")
	}
}

func TestMessageUseCase_Subscribe_WithStartSequence(t *testing.T) {
	startSeq := uint64(50)
	msgRepo := &mockMessageRepository{
		getConsumerPositionFunc: func(ctx context.Context, durableName, subject string) (uint64, error) {
			return 10, nil
		},
		getLatestSequenceForSubjectFunc: func(ctx context.Context, subject string) (uint64, error) {
			return 100, nil
		},
	}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewMessageUseCase(msgRepo, storageRepo, log, time.Millisecond*10)
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	defer cancel()

	notifChan := make(chan *entity.Notification, 10)
	go func() {
		uc.Subscribe(ctx, "test.subject", "test-consumer", &startSeq, notifChan)
	}()

	select {
	case notif := <-notifChan:
		if notif.Sequence != 100 {
			t.Errorf("expected sequence 100, got %d", notif.Sequence)
		}
	case <-time.After(time.Millisecond * 100):
		t.Fatal("timeout waiting for notification")
	}
}

func TestMessageUseCase_Subscribe_CancelContext(t *testing.T) {
	msgRepo := &mockMessageRepository{
		getConsumerPositionFunc: func(ctx context.Context, durableName, subject string) (uint64, error) {
			return 0, nil
		},
		getLatestSequenceForSubjectFunc: func(ctx context.Context, subject string) (uint64, error) {
			return 0, nil
		},
	}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewMessageUseCase(msgRepo, storageRepo, log, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	notifChan := make(chan *entity.Notification, 10)

	errChan := make(chan error, 1)
	go func() {
		errChan <- uc.Subscribe(ctx, "test.subject", "test-consumer", nil, notifChan)
	}()

	time.Sleep(time.Millisecond * 10)
	cancel()

	select {
	case err := <-errChan:
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for subscription to cancel")
	}
}
