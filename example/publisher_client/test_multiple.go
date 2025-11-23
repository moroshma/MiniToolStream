package main

import (
	"context"
	"log"
	"time"

	"github.com/moroshma/MiniToolStream/example/publisher_client/internal/handler"
	"github.com/moroshma/MiniToolStream/example/publisher_client/internal/publisher"
)

func main() {
	log.Printf("MiniToolStream Publisher Client - Multiple Handlers Test")
	log.Printf("Connecting to: localhost:50051")

	// Create publisher manager
	config := &publisher.Config{
		ServerAddr: "localhost:50051",
		Timeout:    30,
	}

	manager, err := publisher.NewManager(config)
	if err != nil {
		log.Fatalf("Failed to create publisher manager: %v", err)
	}
	defer manager.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Register multiple handlers for different subjects
	log.Printf("Registering multiple handlers...")
	manager.RegisterHandlers([]handler.PublishHandler{
		// Images
		handler.NewImagePublisherHandler("images.jpeg", "tst.jpeg"),

		// Raw data
		handler.NewDataPublisherHandler("logs.system", []byte("System initialized"), "text/plain"),
		handler.NewDataPublisherHandler("logs.app", []byte("Application started"), "text/plain"),

		// Test data
		handler.NewDataPublisherHandler("test.debug", []byte("Debug message #1"), "text/plain"),
		handler.NewDataPublisherHandler("final.test", []byte("Final test message"), "text/plain"),
	})

	// Publish all registered handlers
	log.Printf("Publishing all handlers...")
	if err := manager.PublishAll(ctx); err != nil {
		log.Fatalf("Failed to publish: %v", err)
	}

	log.Printf("✓ Test completed successfully!")
}
