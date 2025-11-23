package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/moroshma/MiniToolStream/model"
	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/domain"
)

// Config represents the gRPC client configuration
type Config struct {
	ServerAddr string
	DialOpts   []grpc.DialOption
}

// GRPCClient implements domain.IngressClient using gRPC
type GRPCClient struct {
	conn   *grpc.ClientConn
	client pb.IngressServiceClient
}

// NewGRPCClient creates a new gRPC client for MiniToolStreamIngress
func NewGRPCClient(config *Config) (*GRPCClient, error) {
	if config.ServerAddr == "" {
		return nil, fmt.Errorf("server address is required")
	}

	// Default dial options if not provided
	dialOpts := config.DialOpts
	if dialOpts == nil {
		dialOpts = []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		}
	}

	// Connect to the server
	conn, err := grpc.NewClient(config.ServerAddr, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", config.ServerAddr, err)
	}

	client := pb.NewIngressServiceClient(conn)

	return &GRPCClient{
		conn:   conn,
		client: client,
	}, nil
}

// Publish publishes a message to the specified subject
func (c *GRPCClient) Publish(ctx context.Context, msg *domain.Message) (*domain.PublishResult, error) {
	if msg == nil {
		return nil, fmt.Errorf("message cannot be nil")
	}

	if msg.Subject == "" {
		return nil, fmt.Errorf("subject cannot be empty")
	}

	req := &pb.PublishRequest{
		Subject: msg.Subject,
		Data:    msg.Data,
		Headers: msg.Headers,
	}

	resp, err := c.client.Publish(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("publish failed: %w", err)
	}

	return &domain.PublishResult{
		Sequence:     resp.Sequence,
		ObjectName:   resp.ObjectName,
		StatusCode:   resp.StatusCode,
		ErrorMessage: resp.ErrorMessage,
	}, nil
}

// Close closes the gRPC connection
func (c *GRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
