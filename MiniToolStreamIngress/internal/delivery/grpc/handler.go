package grpc

import (
	"context"
	"fmt"

	pb "github.com/moroshma/MiniToolStreamConnector/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/internal/usecase"
	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/pkg/logger"
	"github.com/moroshma/MiniToolStreamConnector/auth"
)

// IngressHandler implements the gRPC IngressService
type IngressHandler struct {
	pb.UnimplementedIngressServiceServer
	publishUC *usecase.PublishUseCase
	logger    *logger.Logger
}

// NewIngressHandler creates a new gRPC handler instance
func NewIngressHandler(publishUC *usecase.PublishUseCase, log *logger.Logger) *IngressHandler {
	return &IngressHandler{
		publishUC: publishUC,
		logger:    log,
	}
}

// Publish implements the Publish RPC method
func (h *IngressHandler) Publish(ctx context.Context, req *pb.PublishRequest) (*pb.PublishResponse, error) {
	// Validate request is not nil
	if req == nil {
		h.logger.Warn("Publish request rejected: nil request")
		return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
	}

	// Check authorization if claims are present in context
	if claims, ok := auth.GetClaimsFromContext(ctx); ok {
		h.logger.Info("Received authenticated Publish request",
			logger.String("subject", req.Subject),
			logger.String("client_id", claims.ClientID),
			logger.Int("data_size", len(req.Data)),
			logger.Int("headers_count", len(req.Headers)),
		)

		// Validate publish permission
		if err := claims.ValidatePublishAccess(req.Subject); err != nil {
			h.logger.Warn("Publish permission denied",
				logger.String("subject", req.Subject),
				logger.String("client_id", claims.ClientID),
				logger.Error(err),
			)
			return nil, status.Errorf(codes.PermissionDenied, "publish permission denied")
		}
	} else {
		h.logger.Info("Received unauthenticated Publish request",
			logger.String("subject", req.Subject),
			logger.Int("data_size", len(req.Data)),
			logger.Int("headers_count", len(req.Headers)),
		)
	}

	// Validate request
	if req.Subject == "" {
		h.logger.Warn("Publish request rejected: empty subject")
		return &pb.PublishResponse{
			Sequence:     0,
			ObjectName:   "",
			StatusCode:   1,
			ErrorMessage: "subject cannot be empty",
		}, nil
	}

	// Convert headers from proto map to Go map
	headers := make(map[string]string)
	for k, v := range req.Headers {
		headers[k] = v
	}

	// Add data size to headers if data is provided
	if len(req.Data) > 0 {
		headers["data-size"] = fmt.Sprintf("%d", len(req.Data))
	}

	// Call use case
	ucReq := &usecase.PublishRequest{
		Subject: req.Subject,
		Data:    req.Data,
		Headers: headers,
	}

	resp, err := h.publishUC.Publish(ctx, ucReq)
	if err != nil {
		h.logger.Error("Publish use case failed",
			logger.String("subject", req.Subject),
			logger.Error(err),
		)
		return &pb.PublishResponse{
			Sequence:     0,
			ObjectName:   "",
			StatusCode:   1,
			ErrorMessage: err.Error(),
		}, nil
	}

	h.logger.Info("Publish request completed successfully",
		logger.String("subject", req.Subject),
		logger.Uint64("sequence", resp.Sequence),
		logger.String("object_name", resp.ObjectName),
	)

	// Return response
	return &pb.PublishResponse{
		Sequence:     resp.Sequence,
		ObjectName:   resp.ObjectName,
		StatusCode:   0,
		ErrorMessage: "",
	}, nil
}
