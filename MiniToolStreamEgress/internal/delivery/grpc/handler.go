package grpc

import (
	"context"
	"fmt"

	pb "github.com/moroshma/MiniToolStreamConnector/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/domain/entity"
	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/usecase"
	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/pkg/logger"
	"github.com/moroshma/MiniToolStreamConnector/auth"
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
	// Check authorization if claims are present in context
	if claims, ok := auth.GetClaimsFromContext(stream.Context()); ok {
		h.logger.Info("Authenticated Subscribe request",
			logger.String("subject", req.Subject),
			logger.String("client_id", claims.ClientID),
			logger.String("durable_name", req.DurableName),
		)

		// Validate subscribe permission
		if err := claims.ValidateSubscribeAccess(req.Subject); err != nil {
			h.logger.Warn("Subscribe permission denied",
				logger.String("subject", req.Subject),
				logger.String("client_id", claims.ClientID),
				logger.Error(err),
			)
			return status.Errorf(codes.PermissionDenied, "subscribe permission denied")
		}
	} else {
		h.logger.Info("Unauthenticated Subscribe request",
			logger.String("subject", req.Subject),
			logger.String("durable_name", req.DurableName),
		)
	}

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
	// Check authorization if claims are present in context
	if claims, ok := auth.GetClaimsFromContext(stream.Context()); ok {
		h.logger.Info("Authenticated Fetch request",
			logger.String("subject", req.Subject),
			logger.String("client_id", claims.ClientID),
			logger.String("durable_name", req.DurableName),
			logger.Int("batch_size", int(req.BatchSize)),
		)

		// Validate fetch permission
		if err := claims.ValidateFetchAccess(req.Subject); err != nil {
			h.logger.Warn("Fetch permission denied",
				logger.String("subject", req.Subject),
				logger.String("client_id", claims.ClientID),
				logger.Error(err),
			)
			return status.Errorf(codes.PermissionDenied, "fetch permission denied")
		}
	} else {
		h.logger.Info("Unauthenticated Fetch request",
			logger.String("subject", req.Subject),
			logger.String("durable_name", req.DurableName),
			logger.Int("batch_size", int(req.BatchSize)),
		)
	}

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

// AckMessage implements the AckMessage RPC method for manual message acknowledgment
func (h *EgressHandler) AckMessage(ctx context.Context, req *pb.AckRequest) (*pb.AckResponse, error) {
	// Check authorization if claims are present in context
	if claims, ok := auth.GetClaimsFromContext(ctx); ok {
		h.logger.Debug("Authenticated AckMessage request",
			logger.String("subject", req.Subject),
			logger.String("durable_name", req.DurableName),
			logger.Uint64("sequence", req.Sequence),
			logger.String("client_id", claims.ClientID),
		)
	} else {
		h.logger.Debug("Unauthenticated AckMessage request",
			logger.String("subject", req.Subject),
			logger.String("durable_name", req.DurableName),
			logger.Uint64("sequence", req.Sequence),
		)
	}

	if req.Subject == "" {
		return &pb.AckResponse{
			Success:      false,
			ErrorMessage: "subject cannot be empty",
		}, nil
	}

	if req.DurableName == "" {
		return &pb.AckResponse{
			Success:      false,
			ErrorMessage: "durable_name cannot be empty",
		}, nil
	}

	// Update consumer position
	err := h.messageUC.AckMessage(ctx, req.DurableName, req.Subject, req.Sequence)
	if err != nil {
		h.logger.Warn("Failed to acknowledge message",
			logger.String("durable_name", req.DurableName),
			logger.String("subject", req.Subject),
			logger.Uint64("sequence", req.Sequence),
			logger.Error(err),
		)
		return &pb.AckResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	h.logger.Info("Message acknowledged",
		logger.String("durable_name", req.DurableName),
		logger.String("subject", req.Subject),
		logger.Uint64("sequence", req.Sequence),
	)

	return &pb.AckResponse{
		Success: true,
	}, nil
}
