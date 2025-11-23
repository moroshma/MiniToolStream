package grpc

import (
	"context"
	"fmt"

	pb "github.com/moroshma/MiniToolStreamConnector/model"

	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/internal/usecase"
	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/pkg/logger"
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
	h.logger.Info("Received Publish request",
		logger.String("subject", req.Subject),
		logger.Int("data_size", len(req.Data)),
		logger.Int("headers_count", len(req.Headers)),
	)

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
