package main

import (
	"context"
	"log"
	"time"

	"github.com/moroshma/MiniToolStreamConnector/minitoolstream_connector"
	"github.com/moroshma/MiniToolStreamConnector/minitoolstream_connector/domain"
	"github.com/moroshma/MiniToolStreamConnector/minitoolstream_connector/handler"
)

func main() {
	log.Printf("MiniToolStream Publisher Client - Multiple Handlers Test")
	log.Printf("Connecting to: localhost:50051")

	// Create publisher using the library
	pub, err := minitoolstream_connector.NewPublisher("localhost:50051")
	if err != nil {
		log.Fatalf("Failed to create publisher: %v", err)
	}
	defer pub.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Register multiple handlers for different subjects
	log.Printf("Registering multiple handlers...")
	pub.RegisterHandlers([]domain.MessagePreparer{
		// Images
		handler.NewImageHandler(&handler.ImageHandlerConfig{
			Subject:   "images.jpeg",
			ImagePath: "/Users/moroshma/go/MiniToolStream/example/publisher_client/tst.jpeg",
		}),

		// Raw data
		handler.NewDataHandler(&handler.DataHandlerConfig{
			Subject:     "logs.system",
			Data:        []byte("System initialized"),
			ContentType: "text/plain",
		}),
		handler.NewDataHandler(&handler.DataHandlerConfig{
			Subject:     "logs.app",
			Data:        []byte("Application started"),
			ContentType: "text/plain",
		}),

		// Test data
		handler.NewDataHandler(&handler.DataHandlerConfig{
			Subject:     "test.debug",
			Data:        []byte("Debug message #1"),
			ContentType: "text/plain",
		}),
		handler.NewDataHandler(&handler.DataHandlerConfig{
			Subject:     "final.test",
			Data:        []byte("Final test message"),
			ContentType: "text/plain",
		}),
	})

	// Publish all registered handlers
	log.Printf("Publishing all handlers...")
	if err := pub.PublishAll(ctx, nil); err != nil {
		log.Fatalf("Failed to publish: %v", err)
	}

	log.Printf("✓ Test completed successfully!")
}
