package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/pkg/logger"
)

type mockMessageRepository struct {
	publishFunc       func(subject string, headers map[string]string) (uint64, error)
	getNextSeqFunc    func() (uint64, error)
	insertMessageFunc func(sequence uint64, subject string, headers map[string]string, objectName string) error
	pingFunc          func() error
	closeFunc         func() error
}

func (m *mockMessageRepository) PublishMessage(subject string, headers map[string]string) (uint64, error) {
	if m.publishFunc != nil {
		return m.publishFunc(subject, headers)
	}
	return 0, nil
}

func (m *mockMessageRepository) GetNextSequence() (uint64, error) {
	if m.getNextSeqFunc != nil {
		return m.getNextSeqFunc()
	}
	return 0, nil
}

func (m *mockMessageRepository) InsertMessage(sequence uint64, subject string, headers map[string]string, objectName string) error {
	if m.insertMessageFunc != nil {
		return m.insertMessageFunc(sequence, subject, headers, objectName)
	}
	return nil
}

func (m *mockMessageRepository) Ping() error {
	if m.pingFunc != nil {
		return m.pingFunc()
	}
	return nil
}

func (m *mockMessageRepository) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

type mockStorageRepository struct {
	uploadFunc       func(ctx context.Context, objectName string, data []byte, contentType string) error
	getURLFunc       func(objectName string) string
	ensureBucketFunc func(ctx context.Context) error
}

func (m *mockStorageRepository) UploadData(ctx context.Context, objectName string, data []byte, contentType string) error {
	if m.uploadFunc != nil {
		return m.uploadFunc(ctx, objectName, data, contentType)
	}
	return nil
}

func (m *mockStorageRepository) GetObjectURL(objectName string) string {
	if m.getURLFunc != nil {
		return m.getURLFunc(objectName)
	}
	return ""
}

func (m *mockStorageRepository) EnsureBucket(ctx context.Context) error {
	if m.ensureBucketFunc != nil {
		return m.ensureBucketFunc(ctx)
	}
	return nil
}

func TestNewPublishUseCase(t *testing.T) {
	msgRepo := &mockMessageRepository{}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewPublishUseCase(msgRepo, storageRepo, log)
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
}

func TestPublishUseCase_Publish_NilRequest(t *testing.T) {
	msgRepo := &mockMessageRepository{}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewPublishUseCase(msgRepo, storageRepo, log)
	ctx := context.Background()

	_, err := uc.Publish(ctx, nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
	if err.Error() != "request cannot be nil" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPublishUseCase_Publish_EmptySubject(t *testing.T) {
	msgRepo := &mockMessageRepository{}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewPublishUseCase(msgRepo, storageRepo, log)
	ctx := context.Background()

	req := &PublishRequest{
		Subject: "",
		Data:    []byte("test data"),
		Headers: make(map[string]string),
	}

	_, err := uc.Publish(ctx, req)
	if err == nil {
		t.Fatal("expected error for empty subject")
	}
	if err.Error() != "subject cannot be empty" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPublishUseCase_Publish_MessageRepoError(t *testing.T) {
	msgRepo := &mockMessageRepository{
		getNextSeqFunc: func() (uint64, error) {
			return 0, errors.New("tarantool error")
		},
	}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewPublishUseCase(msgRepo, storageRepo, log)
	ctx := context.Background()

	req := &PublishRequest{
		Subject: "test.subject",
		Data:    []byte("test data"),
		Headers: make(map[string]string),
	}

	_, err := uc.Publish(ctx, req)
	if err == nil {
		t.Fatal("expected error from message repository")
	}
}

func TestPublishUseCase_Publish_StorageRepoError(t *testing.T) {
	msgRepo := &mockMessageRepository{
		getNextSeqFunc: func() (uint64, error) {
			return 42, nil
		},
		insertMessageFunc: func(sequence uint64, subject string, headers map[string]string, objectName string) error {
			return nil
		},
	}
	storageRepo := &mockStorageRepository{
		uploadFunc: func(ctx context.Context, objectName string, data []byte, contentType string) error {
			return errors.New("minio error")
		},
	}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewPublishUseCase(msgRepo, storageRepo, log)
	ctx := context.Background()

	req := &PublishRequest{
		Subject: "test.subject",
		Data:    []byte("test data"),
		Headers: make(map[string]string),
	}

	_, err := uc.Publish(ctx, req)
	if err == nil {
		t.Fatal("expected error from storage repository")
	}
}

func TestPublishUseCase_Publish_Success_WithData(t *testing.T) {
	var uploadedData []byte
	var uploadedObjectName string
	var uploadedContentType string

	msgRepo := &mockMessageRepository{
		getNextSeqFunc: func() (uint64, error) {
			return 123, nil
		},
		insertMessageFunc: func(sequence uint64, subject string, headers map[string]string, objectName string) error {
			return nil
		},
	}
	storageRepo := &mockStorageRepository{
		uploadFunc: func(ctx context.Context, objectName string, data []byte, contentType string) error {
			uploadedData = data
			uploadedObjectName = objectName
			uploadedContentType = contentType
			return nil
		},
	}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewPublishUseCase(msgRepo, storageRepo, log)
	ctx := context.Background()

	testData := []byte("test data")
	req := &PublishRequest{
		Subject: "test.subject",
		Data:    testData,
		Headers: map[string]string{
			"content-type": "text/plain",
		},
	}

	resp, err := uc.Publish(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Sequence != 123 {
		t.Errorf("expected sequence 123, got %d", resp.Sequence)
	}
	if resp.ObjectName != "test.subject_123" {
		t.Errorf("expected object name 'test.subject_123', got '%s'", resp.ObjectName)
	}

	if string(uploadedData) != "test data" {
		t.Errorf("expected uploaded data 'test data', got '%s'", string(uploadedData))
	}
	if uploadedObjectName != "test.subject_123" {
		t.Errorf("expected uploaded object name 'test.subject_123', got '%s'", uploadedObjectName)
	}
	if uploadedContentType != "text/plain" {
		t.Errorf("expected content type 'text/plain', got '%s'", uploadedContentType)
	}
}

func TestPublishUseCase_Publish_Success_WithoutData(t *testing.T) {
	uploadCalled := false
	msgRepo := &mockMessageRepository{
		getNextSeqFunc: func() (uint64, error) {
			return 456, nil
		},
		insertMessageFunc: func(sequence uint64, subject string, headers map[string]string, objectName string) error {
			return nil
		},
	}
	storageRepo := &mockStorageRepository{
		uploadFunc: func(ctx context.Context, objectName string, data []byte, contentType string) error {
			uploadCalled = true
			return nil
		},
	}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewPublishUseCase(msgRepo, storageRepo, log)
	ctx := context.Background()

	req := &PublishRequest{
		Subject: "test.subject",
		Data:    []byte{},
		Headers: make(map[string]string),
	}

	resp, err := uc.Publish(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Sequence != 456 {
		t.Errorf("expected sequence 456, got %d", resp.Sequence)
	}
	if resp.ObjectName != "test.subject_456" {
		t.Errorf("expected object name 'test.subject_456', got '%s'", resp.ObjectName)
	}

	if uploadCalled {
		t.Error("expected upload not to be called for empty data")
	}
}

func TestPublishUseCase_Publish_DefaultContentType(t *testing.T) {
	var uploadedContentType string

	msgRepo := &mockMessageRepository{
		getNextSeqFunc: func() (uint64, error) {
			return 789, nil
		},
		insertMessageFunc: func(sequence uint64, subject string, headers map[string]string, objectName string) error {
			return nil
		},
	}
	storageRepo := &mockStorageRepository{
		uploadFunc: func(ctx context.Context, objectName string, data []byte, contentType string) error {
			uploadedContentType = contentType
			return nil
		},
	}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewPublishUseCase(msgRepo, storageRepo, log)
	ctx := context.Background()

	req := &PublishRequest{
		Subject: "test.subject",
		Data:    []byte("test data"),
		Headers: make(map[string]string),
	}

	_, err := uc.Publish(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if uploadedContentType != "application/octet-stream" {
		t.Errorf("expected default content type 'application/octet-stream', got '%s'", uploadedContentType)
	}
}

func TestPublishUseCase_HealthCheck_Success(t *testing.T) {
	msgRepo := &mockMessageRepository{
		pingFunc: func() error {
			return nil
		},
	}
	storageRepo := &mockStorageRepository{
		ensureBucketFunc: func(ctx context.Context) error {
			return nil
		},
	}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewPublishUseCase(msgRepo, storageRepo, log)
	ctx := context.Background()

	err := uc.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPublishUseCase_HealthCheck_MessageRepoUnhealthy(t *testing.T) {
	msgRepo := &mockMessageRepository{
		pingFunc: func() error {
			return errors.New("connection failed")
		},
	}
	storageRepo := &mockStorageRepository{
		ensureBucketFunc: func(ctx context.Context) error {
			return nil
		},
	}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewPublishUseCase(msgRepo, storageRepo, log)
	ctx := context.Background()

	err := uc.HealthCheck(ctx)
	if err == nil {
		t.Fatal("expected error for unhealthy message repository")
	}
}

func TestPublishUseCase_HealthCheck_StorageRepoUnhealthy(t *testing.T) {
	msgRepo := &mockMessageRepository{
		pingFunc: func() error {
			return nil
		},
	}
	storageRepo := &mockStorageRepository{
		ensureBucketFunc: func(ctx context.Context) error {
			return errors.New("bucket not available")
		},
	}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := NewPublishUseCase(msgRepo, storageRepo, log)
	ctx := context.Background()

	err := uc.HealthCheck(ctx)
	if err == nil {
		t.Fatal("expected error for unhealthy storage repository")
	}
}
