package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/moroshma/MiniToolStream/model"
	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/subscriber/domain"
)

// Config represents the gRPC client configuration
type Config struct {
	ServerAddr string
	DialOpts   []grpc.DialOption
}

// GRPCClient implements domain.EgressClient using gRPC
type GRPCClient struct {
	conn   *grpc.ClientConn
	client pb.EgressServiceClient
}

// NewGRPCClient creates a new gRPC client for MiniToolStreamEgress
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

	client := pb.NewEgressServiceClient(conn)

	return &GRPCClient{
		conn:   conn,
		client: client,
	}, nil
}

// Subscribe subscribes to notifications for a subject
func (c *GRPCClient) Subscribe(ctx context.Context, config *domain.SubscriptionConfig) (domain.NotificationStream, error) {
	if config == nil {
		return nil, fmt.Errorf("subscription config cannot be nil")
	}

	if config.Subject == "" {
		return nil, fmt.Errorf("subject cannot be empty")
	}

	req := &pb.SubscribeRequest{
		Subject:     config.Subject,
		DurableName: config.DurableName,
	}

	if config.StartSequence != nil {
		seq := *config.StartSequence
		req.StartSequence = &seq
	}

	stream, err := c.client.Subscribe(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("subscribe failed: %w", err)
	}

	return &notificationStreamAdapter{stream: stream}, nil
}

// Fetch fetches messages from a subject
func (c *GRPCClient) Fetch(ctx context.Context, config *domain.SubscriptionConfig) (domain.MessageStream, error) {
	if config == nil {
		return nil, fmt.Errorf("subscription config cannot be nil")
	}

	if config.Subject == "" {
		return nil, fmt.Errorf("subject cannot be empty")
	}

	req := &pb.FetchRequest{
		Subject:     config.Subject,
		DurableName: config.DurableName,
		BatchSize:   config.BatchSize,
	}

	stream, err := c.client.Fetch(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	return &messageStreamAdapter{stream: stream}, nil
}

// GetLastSequence gets the last sequence number for a subject
func (c *GRPCClient) GetLastSequence(ctx context.Context, subject string) (uint64, error) {
	if subject == "" {
		return 0, fmt.Errorf("subject cannot be empty")
	}

	req := &pb.GetLastSequenceRequest{
		Subject: subject,
	}

	resp, err := c.client.GetLastSequence(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("get last sequence failed: %w", err)
	}

	return resp.LastSequence, nil
}

// Close closes the gRPC connection
func (c *GRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// notificationStreamAdapter adapts gRPC stream to domain.NotificationStream
type notificationStreamAdapter struct {
	stream pb.EgressService_SubscribeClient
}

func (a *notificationStreamAdapter) Recv() (*domain.Notification, error) {
	notification, err := a.stream.Recv()
	if err != nil {
		return nil, err
	}

	return &domain.Notification{
		Subject:  notification.Subject,
		Sequence: notification.Sequence,
	}, nil
}

// messageStreamAdapter adapts gRPC stream to domain.MessageStream
type messageStreamAdapter struct {
	stream pb.EgressService_FetchClient
}

func (a *messageStreamAdapter) Recv() (*domain.Message, error) {
	msg, err := a.stream.Recv()
	if err != nil {
		return nil, err
	}

	return &domain.Message{
		Subject:   msg.Subject,
		Sequence:  msg.Sequence,
		Data:      msg.Data,
		Headers:   msg.Headers,
		Timestamp: msg.Timestamp.AsTime(),
	}, nil
}
