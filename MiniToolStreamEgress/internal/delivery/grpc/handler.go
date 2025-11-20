package grpc

import (
	"context"
	"fmt"

	pb "github.com/moroshma/MiniToolStream/model"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/domain/entity"
	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/usecase"
	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/pkg/logger"
)

// EgressHandler implements the gRPC EgressService
type EgressHandler struct {
	pb.UnimplementedEgressServiceServer
	messageUC *usecase.MessageUseCase
	logger    *logger.Logger
}

// NewEgressHandler creates a new gRPC handler
func NewEgressHandler(messageUC *usecase.MessageUseCase, logger *logger.Logger) *EgressHandler {
	return &EgressHandler{
		messageUC: messageUC,
		logger:    logger,
	}
}

// Subscribe implements the Subscribe RPC method
func (h *EgressHandler) Subscribe(req *pb.SubscribeRequest, stream pb.EgressService_SubscribeServer) error {
	h.logger.Info("Subscribe request",
		logger.String("subject", req.Subject),
		logger.String("durable_name", req.DurableName),
	)

	if req.Subject == "" {
		return fmt.Errorf("subject cannot be empty")
	}

	if req.DurableName == "" {
		return fmt.Errorf("durable_name cannot be empty")
	}

	// Create notification channel
	notificationChan := make(chan *entity.Notification, 100)
	defer close(notificationChan)

	// Start subscription in goroutine
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		err := h.messageUC.Subscribe(ctx, req.Subject, req.DurableName, req.StartSequence, notificationChan)
		if err != nil && err != context.Canceled {
			errChan <- err
		}
		close(errChan)
	}()

	// Forward notifications to gRPC stream
	for {
		select {
		case notification, ok := <-notificationChan:
			if !ok {
				return nil
			}

			err := stream.Send(&pb.Notification{
				Subject:  notification.Subject,
				Sequence: notification.Sequence,
			})
			if err != nil {
				return fmt.Errorf("failed to send notification: %w", err)
			}

		case err := <-errChan:
			if err != nil {
				return err
			}
			return nil

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Fetch implements the Fetch RPC method
func (h *EgressHandler) Fetch(req *pb.FetchRequest, stream pb.EgressService_FetchServer) error {
	h.logger.Info("Fetch request",
		logger.String("subject", req.Subject),
		logger.String("durable_name", req.DurableName),
		logger.Int("batch_size", int(req.BatchSize)),
	)

	if req.Subject == "" {
		return fmt.Errorf("subject cannot be empty")
	}

	if req.DurableName == "" {
		return fmt.Errorf("durable_name cannot be empty")
	}

	// Fetch messages
	messages, err := h.messageUC.FetchMessages(
		stream.Context(),
		req.Subject,
		req.DurableName,
		int(req.BatchSize),
	)
	if err != nil {
		return fmt.Errorf("failed to fetch messages: %w", err)
	}

	// Send each message
	for _, msg := range messages {
		pbMsg := &pb.Message{
			Subject:   msg.Subject,
			Sequence:  msg.Sequence,
			Data:      msg.Data,
			Headers:   msg.Headers,
			Timestamp: timestamppb.New(msg.Timestamp),
		}

		err = stream.Send(pbMsg)
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}
	}

	h.logger.Info("Fetch completed",
		logger.String("subject", req.Subject),
		logger.Int("count", len(messages)),
	)

	return nil
}

// GetLastSequence implements the GetLastSequence RPC method
func (h *EgressHandler) GetLastSequence(ctx context.Context, req *pb.GetLastSequenceRequest) (*pb.GetLastSequenceResponse, error) {
	h.logger.Info("GetLastSequence request", logger.String("subject", req.Subject))

	if req.Subject == "" {
		return nil, fmt.Errorf("subject cannot be empty")
	}

	lastSeq, err := h.messageUC.GetLastSequence(ctx, req.Subject)
	if err != nil {
		return nil, fmt.Errorf("failed to get last sequence: %w", err)
	}

	return &pb.GetLastSequenceResponse{
		LastSequence: lastSeq,
	}, nil
}
