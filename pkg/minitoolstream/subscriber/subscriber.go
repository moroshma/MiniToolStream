package subscriber

import (
	"fmt"

	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/subscriber/client"
	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/subscriber/domain"
	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/subscriber/usecase"
)

// NewSubscriber creates a new subscriber with default configuration
// This is a convenience function that combines client and subscriber creation
func NewSubscriber(serverAddr string, durableName string) (domain.Subscriber, error) {
	if serverAddr == "" {
		return nil, fmt.Errorf("server address is required")
	}

	if durableName == "" {
		durableName = "default-subscriber"
	}

	// Create gRPC client
	grpcClient, err := client.NewGRPCClient(&client.Config{
		ServerAddr: serverAddr,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	// Create subscriber
	sub, err := usecase.New(&usecase.Config{
		Client:      grpcClient,
		DurableName: durableName,
		BatchSize:   10,
	})
	if err != nil {
		grpcClient.Close()
		return nil, fmt.Errorf("failed to create subscriber: %w", err)
	}

	return sub, nil
}

// SubscriberBuilder provides a fluent interface for building subscribers
type SubscriberBuilder struct {
	serverAddr  string
	durableName string
	batchSize   int32
	logger      usecase.Logger
	err         error
}

// NewSubscriberBuilder creates a new subscriber builder
func NewSubscriberBuilder(serverAddr string) *SubscriberBuilder {
	return &SubscriberBuilder{
		serverAddr: serverAddr,
		batchSize:  10,
	}
}

// WithDurableName sets the durable name
func (b *SubscriberBuilder) WithDurableName(durableName string) *SubscriberBuilder {
	b.durableName = durableName
	return b
}

// WithBatchSize sets the batch size for fetching messages
func (b *SubscriberBuilder) WithBatchSize(batchSize int32) *SubscriberBuilder {
	b.batchSize = batchSize
	return b
}

// WithLogger sets a custom logger
func (b *SubscriberBuilder) WithLogger(logger usecase.Logger) *SubscriberBuilder {
	b.logger = logger
	return b
}

// Build creates the subscriber instance
func (b *SubscriberBuilder) Build() (domain.Subscriber, error) {
	if b.err != nil {
		return nil, b.err
	}

	if b.serverAddr == "" {
		return nil, fmt.Errorf("server address is required")
	}

	if b.durableName == "" {
		b.durableName = "default-subscriber"
	}

	// Create gRPC client
	grpcClient, err := client.NewGRPCClient(&client.Config{
		ServerAddr: b.serverAddr,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	// Create subscriber
	sub, err := usecase.New(&usecase.Config{
		Client:      grpcClient,
		DurableName: b.durableName,
		BatchSize:   b.batchSize,
		Logger:      b.logger,
	})
	if err != nil {
		grpcClient.Close()
		return nil, fmt.Errorf("failed to create subscriber: %w", err)
	}

	return sub, nil
}
