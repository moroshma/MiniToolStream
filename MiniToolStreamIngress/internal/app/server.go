package app

import (
	"context"
	"fmt"

	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/internal/repository/minio"
	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/internal/repository/tarantool"
	pb "github.com/moroshma/MiniToolStreamConnector/model"
)

// IngressServer implements the gRPC IngressService
type IngressServer struct {
	pb.UnimplementedIngressServiceServer
	tarantoolClient *tarantool.Repository
	minioClient     *minio.Repository
}

// NewIngressServer creates a new gRPC app instance
func NewIngressServer(tarantoolClient *tarantool.Repository, minioClient *minio.Repository) *IngressServer {
	return &IngressServer{
		tarantoolClient: tarantoolClient,
		minioClient:     minioClient,
	}
}

// Publish implements the Publish RPC method
func (s *IngressServer) Publish(ctx context.Context, req *pb.PublishRequest) (*pb.PublishResponse, error) {
	// Validate request
	if req.Subject == "" {
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

	// Publish metadata to Tarantool
	sequence, err := s.tarantoolClient.PublishMessage(req.Subject, headers)
	if err != nil {
		return &pb.PublishResponse{
			Sequence:     0,
			ObjectName:   "",
			StatusCode:   1,
			ErrorMessage: err.Error(),
		}, nil
	}

	// Generate object_name using same pattern as Tarantool: {{subject}}_{{sequence}}
	objectName := fmt.Sprintf("%s_%d", req.Subject, sequence)

	// Upload data to MinIO if data is provided
	if len(req.Data) > 0 {
		contentType := headers["content-type"]
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		err = s.minioClient.UploadData(ctx, objectName, req.Data, contentType)
		if err != nil {
			return &pb.PublishResponse{
				Sequence:     sequence,
				ObjectName:   objectName,
				StatusCode:   1,
				ErrorMessage: fmt.Sprintf("metadata saved, but failed to upload data to MinIO: %v", err),
			}, nil
		}
	}

	// Return response
	return &pb.PublishResponse{
		Sequence:     sequence,
		ObjectName:   objectName,
		StatusCode:   0,
		ErrorMessage: "",
	}, nil
}
