package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "github.com/moroshma/MiniToolStreamConnector/model"

	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/domain/entity"
	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/usecase"
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

type mockSubscribeStream struct {
	ctx        context.Context
	sentNotifs []*pb.Notification
	sendErr    error
}

func (m *mockSubscribeStream) Send(notif *pb.Notification) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sentNotifs = append(m.sentNotifs, notif)
	return nil
}

func (m *mockSubscribeStream) Context() context.Context {
	return m.ctx
}

func (m *mockSubscribeStream) SetHeader(md interface{}) error  { return nil }
func (m *mockSubscribeStream) SendHeader(md interface{}) error { return nil }
func (m *mockSubscribeStream) SetTrailer(md interface{})       {}
func (m *mockSubscribeStream) SendMsg(msg interface{}) error   { return nil }
func (m *mockSubscribeStream) RecvMsg(msg interface{}) error   { return nil }

type mockFetchStream struct {
	ctx         context.Context
	sentMsgs    []*pb.Message
	sendErr     error
}

func (m *mockFetchStream) Send(msg *pb.Message) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sentMsgs = append(m.sentMsgs, msg)
	return nil
}

func (m *mockFetchStream) Context() context.Context {
	return m.ctx
}

func (m *mockFetchStream) SetHeader(md interface{}) error  { return nil }
func (m *mockFetchStream) SendHeader(md interface{}) error { return nil }
func (m *mockFetchStream) SetTrailer(md interface{})       {}
func (m *mockFetchStream) SendMsg(msg interface{}) error   { return nil }
func (m *mockFetchStream) RecvMsg(msg interface{}) error   { return nil }

func TestNewEgressHandler(t *testing.T) {
	msgRepo := &mockMessageRepository{}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := usecase.NewMessageUseCase(msgRepo, storageRepo, log, time.Second)
	handler := NewEgressHandler(uc, log)

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	if handler.messageUC == nil {
		t.Error("expected non-nil messageUC")
	}
	if handler.logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestEgressHandler_Subscribe_EmptySubject(t *testing.T) {
	msgRepo := &mockMessageRepository{}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := usecase.NewMessageUseCase(msgRepo, storageRepo, log, time.Second)
	handler := NewEgressHandler(uc, log)

	req := &pb.SubscribeRequest{
		Subject:     "",
		DurableName: "test-consumer",
	}

	stream := &mockSubscribeStream{ctx: context.Background()}
	err := handler.Subscribe(req, stream)

	if err == nil {
		t.Fatal("expected error for empty subject")
	}
}

func TestEgressHandler_Subscribe_EmptyDurableName(t *testing.T) {
	msgRepo := &mockMessageRepository{}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := usecase.NewMessageUseCase(msgRepo, storageRepo, log, time.Second)
	handler := NewEgressHandler(uc, log)

	req := &pb.SubscribeRequest{
		Subject:     "test.subject",
		DurableName: "",
	}

	stream := &mockSubscribeStream{ctx: context.Background()}
	err := handler.Subscribe(req, stream)

	if err == nil {
		t.Fatal("expected error for empty durable name")
	}
}

func TestEgressHandler_Fetch_EmptySubject(t *testing.T) {
	msgRepo := &mockMessageRepository{}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := usecase.NewMessageUseCase(msgRepo, storageRepo, log, time.Second)
	handler := NewEgressHandler(uc, log)

	req := &pb.FetchRequest{
		Subject:     "",
		DurableName: "test-consumer",
		BatchSize:   10,
	}

	stream := &mockFetchStream{ctx: context.Background()}
	err := handler.Fetch(req, stream)

	if err == nil {
		t.Fatal("expected error for empty subject")
	}
}

func TestEgressHandler_Fetch_EmptyDurableName(t *testing.T) {
	msgRepo := &mockMessageRepository{}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := usecase.NewMessageUseCase(msgRepo, storageRepo, log, time.Second)
	handler := NewEgressHandler(uc, log)

	req := &pb.FetchRequest{
		Subject:     "test.subject",
		DurableName: "",
		BatchSize:   10,
	}

	stream := &mockFetchStream{ctx: context.Background()}
	err := handler.Fetch(req, stream)

	if err == nil {
		t.Fatal("expected error for empty durable name")
	}
}

func TestEgressHandler_Fetch_Success(t *testing.T) {
	msgRepo := &mockMessageRepository{
		getConsumerPositionFunc: func(ctx context.Context, durableName, subject string) (uint64, error) {
			return 0, nil
		},
		getMessagesBySubjectFunc: func(ctx context.Context, subject string, startSeq uint64, limit int) ([]*entity.Message, error) {
			return []*entity.Message{
				{
					Sequence:   1,
					Subject:    "test.subject",
					Data:       []byte("test data"),
					Headers:    map[string]string{"key": "value"},
					ObjectName: "",
					Timestamp:  time.Now(),
				},
			}, nil
		},
		updateConsumerPositionFunc: func(ctx context.Context, durableName, subject string, sequence uint64) error {
			return nil
		},
	}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := usecase.NewMessageUseCase(msgRepo, storageRepo, log, time.Second)
	handler := NewEgressHandler(uc, log)

	req := &pb.FetchRequest{
		Subject:     "test.subject",
		DurableName: "test-consumer",
		BatchSize:   10,
	}

	stream := &mockFetchStream{ctx: context.Background()}
	err := handler.Fetch(req, stream)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(stream.sentMsgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(stream.sentMsgs))
	}

	msg := stream.sentMsgs[0]
	if msg.Subject != "test.subject" {
		t.Errorf("expected subject 'test.subject', got '%s'", msg.Subject)
	}
	if msg.Sequence != 1 {
		t.Errorf("expected sequence 1, got %d", msg.Sequence)
	}
	if string(msg.Data) != "test data" {
		t.Errorf("expected data 'test data', got '%s'", string(msg.Data))
	}
}

func TestEgressHandler_Fetch_UseCaseError(t *testing.T) {
	msgRepo := &mockMessageRepository{
		getConsumerPositionFunc: func(ctx context.Context, durableName, subject string) (uint64, error) {
			return 0, errors.New("database error")
		},
	}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := usecase.NewMessageUseCase(msgRepo, storageRepo, log, time.Second)
	handler := NewEgressHandler(uc, log)

	req := &pb.FetchRequest{
		Subject:     "test.subject",
		DurableName: "test-consumer",
		BatchSize:   10,
	}

	stream := &mockFetchStream{ctx: context.Background()}
	err := handler.Fetch(req, stream)

	if err == nil {
		t.Fatal("expected error from use case")
	}
}

func TestEgressHandler_Fetch_SendError(t *testing.T) {
	msgRepo := &mockMessageRepository{
		getConsumerPositionFunc: func(ctx context.Context, durableName, subject string) (uint64, error) {
			return 0, nil
		},
		getMessagesBySubjectFunc: func(ctx context.Context, subject string, startSeq uint64, limit int) ([]*entity.Message, error) {
			return []*entity.Message{
				{
					Sequence:  1,
					Subject:   "test.subject",
					Data:      []byte("test data"),
					Headers:   map[string]string{},
					Timestamp: time.Now(),
				},
			}, nil
		},
		updateConsumerPositionFunc: func(ctx context.Context, durableName, subject string, sequence uint64) error {
			return nil
		},
	}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := usecase.NewMessageUseCase(msgRepo, storageRepo, log, time.Second)
	handler := NewEgressHandler(uc, log)

	req := &pb.FetchRequest{
		Subject:     "test.subject",
		DurableName: "test-consumer",
		BatchSize:   10,
	}

	stream := &mockFetchStream{
		ctx:     context.Background(),
		sendErr: errors.New("send error"),
	}
	err := handler.Fetch(req, stream)

	if err == nil {
		t.Fatal("expected error from stream.Send")
	}
}

func TestEgressHandler_GetLastSequence_EmptySubject(t *testing.T) {
	msgRepo := &mockMessageRepository{}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := usecase.NewMessageUseCase(msgRepo, storageRepo, log, time.Second)
	handler := NewEgressHandler(uc, log)

	req := &pb.GetLastSequenceRequest{
		Subject: "",
	}

	_, err := handler.GetLastSequence(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty subject")
	}
}

func TestEgressHandler_GetLastSequence_Success(t *testing.T) {
	msgRepo := &mockMessageRepository{
		getLatestSequenceForSubjectFunc: func(ctx context.Context, subject string) (uint64, error) {
			return 123, nil
		},
	}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := usecase.NewMessageUseCase(msgRepo, storageRepo, log, time.Second)
	handler := NewEgressHandler(uc, log)

	req := &pb.GetLastSequenceRequest{
		Subject: "test.subject",
	}

	resp, err := handler.GetLastSequence(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.LastSequence != 123 {
		t.Errorf("expected last sequence 123, got %d", resp.LastSequence)
	}
}

func TestEgressHandler_GetLastSequence_Error(t *testing.T) {
	msgRepo := &mockMessageRepository{
		getLatestSequenceForSubjectFunc: func(ctx context.Context, subject string) (uint64, error) {
			return 0, errors.New("database error")
		},
	}
	storageRepo := &mockStorageRepository{}
	log, _ := logger.New(logger.Config{Level: "debug", Format: "json", OutputPath: "stdout"})

	uc := usecase.NewMessageUseCase(msgRepo, storageRepo, log, time.Second)
	handler := NewEgressHandler(uc, log)

	req := &pb.GetLastSequenceRequest{
		Subject: "test.subject",
	}

	_, err := handler.GetLastSequence(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from use case")
	}
}
