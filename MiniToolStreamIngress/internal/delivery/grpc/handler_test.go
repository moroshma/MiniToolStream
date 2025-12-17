package grpc

import (
	"context"
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

type mockMessageRepository struct {
	getNextSeqFunc    func() (uint64, error)
	insertMessageFunc func(sequence uint64, subject string, headers map[string]string, objectName string) error
	pingFunc          func() error
	closeFunc         func() error
}

func (m *mockMessageRepository) PublishMessage(subject string, headers map[string]string) (uint64, error) {
	// Legacy method - not used in new architecture
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
