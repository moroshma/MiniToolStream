package grpc

import (
	"context"
	"errors"
	"testing"

	pb "github.com/moroshma/MiniToolStreamConnector/model"

	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/internal/usecase"
	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/pkg/logger"
)

type mockPublishUseCase struct {
	publishFunc     func(ctx context.Context, req *usecase.PublishRequest) (*usecase.PublishResponse, error)
	healthCheckFunc func(ctx context.Context) error
}

func (m *mockPublishUseCase) Publish(ctx context.Context, req *usecase.PublishRequest) (*usecase.PublishResponse, error) {
	if m.publishFunc != nil {
		return m.publishFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockPublishUseCase) HealthCheck(ctx context.Context) error {
	if m.healthCheckFunc != nil {
		return m.healthCheckFunc(ctx)
	}
	return nil
}

func TestNewIngressHandler(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	handler := NewIngressHandler(&usecase.PublishUseCase{}, log)
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	if handler.publishUC == nil {
		t.Error("expected non-nil publishUC")
	}
	if handler.logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestIngressHandler_Publish_EmptySubject(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	handler := &IngressHandler{
		publishUC: &usecase.PublishUseCase{},
		logger:    log,
	}

	ctx := context.Background()
	req := &pb.PublishRequest{
		Subject: "",
		Data:    []byte("test data"),
		Headers: map[string]string{},
	}

	resp, err := handler.Publish(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.StatusCode != 1 {
		t.Errorf("expected status code 1, got %d", resp.StatusCode)
	}
	if resp.ErrorMessage != "subject cannot be empty" {
		t.Errorf("unexpected error message: %s", resp.ErrorMessage)
	}
	if resp.Sequence != 0 {
		t.Errorf("expected sequence 0, got %d", resp.Sequence)
	}
}

func TestIngressHandler_Publish_UseCaseError(t *testing.T) {
	msgRepo := &mockMessageRepository{
		publishFunc: func(subject string, headers map[string]string) (uint64, error) {
			return 0, errors.New("tarantool error")
		},
	}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := usecase.NewPublishUseCase(msgRepo, storageRepo, log)
	handler := NewIngressHandler(uc, log)

	ctx := context.Background()
	req := &pb.PublishRequest{
		Subject: "test.subject",
		Data:    []byte("test data"),
		Headers: map[string]string{},
	}

	resp, err := handler.Publish(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.StatusCode != 1 {
		t.Errorf("expected status code 1, got %d", resp.StatusCode)
	}
	if resp.ErrorMessage == "" {
		t.Error("expected error message")
	}
}

func TestIngressHandler_Publish_Success_WithData(t *testing.T) {
	var receivedSubject string
	var receivedData []byte
	var receivedHeaders map[string]string

	msgRepo := &mockMessageRepository{
		publishFunc: func(subject string, headers map[string]string) (uint64, error) {
			receivedSubject = subject
			receivedHeaders = headers
			return 42, nil
		},
	}
	storageRepo := &mockStorageRepository{
		uploadFunc: func(ctx context.Context, objectName string, data []byte, contentType string) error {
			receivedData = data
			return nil
		},
	}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := usecase.NewPublishUseCase(msgRepo, storageRepo, log)
	handler := NewIngressHandler(uc, log)

	ctx := context.Background()
	testData := []byte("test data content")
	req := &pb.PublishRequest{
		Subject: "test.subject",
		Data:    testData,
		Headers: map[string]string{
			"custom-header": "custom-value",
		},
	}

	resp, err := handler.Publish(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.StatusCode != 0 {
		t.Errorf("expected status code 0, got %d", resp.StatusCode)
	}
	if resp.ErrorMessage != "" {
		t.Errorf("unexpected error message: %s", resp.ErrorMessage)
	}
	if resp.Sequence != 42 {
		t.Errorf("expected sequence 42, got %d", resp.Sequence)
	}
	if resp.ObjectName != "test.subject_42" {
		t.Errorf("expected object name 'test.subject_42', got '%s'", resp.ObjectName)
	}

	if receivedSubject != "test.subject" {
		t.Errorf("expected subject 'test.subject', got '%s'", receivedSubject)
	}
	if string(receivedData) != "test data content" {
		t.Errorf("expected data 'test data content', got '%s'", string(receivedData))
	}
	if receivedHeaders["custom-header"] != "custom-value" {
		t.Errorf("expected custom header value 'custom-value', got '%s'", receivedHeaders["custom-header"])
	}
	if receivedHeaders["data-size"] != "17" {
		t.Errorf("expected data-size header '17', got '%s'", receivedHeaders["data-size"])
	}
}

func TestIngressHandler_Publish_Success_WithoutData(t *testing.T) {
	var receivedHeaders map[string]string
	uploadCalled := false

	msgRepo := &mockMessageRepository{
		publishFunc: func(subject string, headers map[string]string) (uint64, error) {
			receivedHeaders = headers
			return 100, nil
		},
	}
	storageRepo := &mockStorageRepository{
		uploadFunc: func(ctx context.Context, objectName string, data []byte, contentType string) error {
			uploadCalled = true
			return nil
		},
	}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := usecase.NewPublishUseCase(msgRepo, storageRepo, log)
	handler := NewIngressHandler(uc, log)

	ctx := context.Background()
	req := &pb.PublishRequest{
		Subject: "test.subject",
		Data:    []byte{},
		Headers: map[string]string{},
	}

	resp, err := handler.Publish(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.StatusCode != 0 {
		t.Errorf("expected status code 0, got %d", resp.StatusCode)
	}
	if resp.Sequence != 100 {
		t.Errorf("expected sequence 100, got %d", resp.Sequence)
	}

	if _, exists := receivedHeaders["data-size"]; exists {
		t.Error("expected data-size header not to be set for empty data")
	}
	if uploadCalled {
		t.Error("expected upload not to be called for empty data")
	}
}

func TestIngressHandler_Publish_HeadersConversion(t *testing.T) {
	var receivedHeaders map[string]string

	msgRepo := &mockMessageRepository{
		publishFunc: func(subject string, headers map[string]string) (uint64, error) {
			receivedHeaders = headers
			return 1, nil
		},
	}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := usecase.NewPublishUseCase(msgRepo, storageRepo, log)
	handler := NewIngressHandler(uc, log)

	ctx := context.Background()
	req := &pb.PublishRequest{
		Subject: "test.subject",
		Data:    []byte{},
		Headers: map[string]string{
			"header1": "value1",
			"header2": "value2",
			"header3": "value3",
		},
	}

	_, err := handler.Publish(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(receivedHeaders) != 3 {
		t.Errorf("expected 3 headers, got %d", len(receivedHeaders))
	}
	if receivedHeaders["header1"] != "value1" {
		t.Errorf("expected header1='value1', got '%s'", receivedHeaders["header1"])
	}
	if receivedHeaders["header2"] != "value2" {
		t.Errorf("expected header2='value2', got '%s'", receivedHeaders["header2"])
	}
	if receivedHeaders["header3"] != "value3" {
		t.Errorf("expected header3='value3', got '%s'", receivedHeaders["header3"])
	}
}

type mockMessageRepository struct {
	publishFunc func(subject string, headers map[string]string) (uint64, error)
	pingFunc    func() error
	closeFunc   func() error
}

func (m *mockMessageRepository) PublishMessage(subject string, headers map[string]string) (uint64, error) {
	if m.publishFunc != nil {
		return m.publishFunc(subject, headers)
	}
	return 0, nil
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
