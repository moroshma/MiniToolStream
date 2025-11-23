package minitoolstream

import (
	"fmt"

	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/client"
	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/domain"
	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/publisher"
)

// NewPublisher creates a new publisher with default configuration
// This is a convenience function that combines client and publisher creation
func NewPublisher(serverAddr string) (domain.Publisher, error) {
	if serverAddr == "" {
		return nil, fmt.Errorf("server address is required")
	}

	// Create gRPC client
	grpcClient, err := client.NewGRPCClient(&client.Config{
		ServerAddr: serverAddr,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	// Create publisher
	pub, err := publisher.New(&publisher.Config{
		Client: grpcClient,
	})
	if err != nil {
		grpcClient.Close()
		return nil, fmt.Errorf("failed to create publisher: %w", err)
	}

	return pub, nil
}

// PublisherBuilder provides a fluent interface for building publishers
type PublisherBuilder struct {
	serverAddr    string
	resultHandler domain.ResultHandler
	logger        publisher.Logger
	err           error
}

// NewPublisherBuilder creates a new publisher builder
func NewPublisherBuilder(serverAddr string) *PublisherBuilder {
	return &PublisherBuilder{
		serverAddr: serverAddr,
	}
}

// WithResultHandler sets a custom result handler
func (b *PublisherBuilder) WithResultHandler(handler domain.ResultHandler) *PublisherBuilder {
	b.resultHandler = handler
	return b
}

// WithLogger sets a custom logger
func (b *PublisherBuilder) WithLogger(logger publisher.Logger) *PublisherBuilder {
	b.logger = logger
	return b
}

// Build creates the publisher instance
func (b *PublisherBuilder) Build() (domain.Publisher, error) {
	if b.err != nil {
		return nil, b.err
	}

	if b.serverAddr == "" {
		return nil, fmt.Errorf("server address is required")
	}

	// Create gRPC client
	grpcClient, err := client.NewGRPCClient(&client.Config{
		ServerAddr: b.serverAddr,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}

	// Create publisher
	pub, err := publisher.New(&publisher.Config{
		Client:        grpcClient,
		ResultHandler: b.resultHandler,
		Logger:        b.logger,
	})
	if err != nil {
		grpcClient.Close()
		return nil, fmt.Errorf("failed to create publisher: %w", err)
	}

	return pub, nil
}
