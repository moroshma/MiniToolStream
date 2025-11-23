package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/moroshma/MiniToolStream/example/subscriber_client/internal/handler"
	"github.com/moroshma/MiniToolStream/example/subscriber_client/internal/subscriber"
)

var (
	serverAddr  = flag.String("server", "localhost:50052", "MiniToolStreamEgress gRPC server address")
	durableName = flag.String("durable", "multi-subscriber", "Durable consumer name")
	outputDir   = flag.String("output", "./downloads", "Directory to save downloaded files")
	batchSize   = flag.Int("batch", 10, "Batch size for fetching messages")
)

func main() {
	flag.Parse()

	log.Printf("MiniToolStream Multi-Channel Subscriber")
	log.Printf("Connecting to: %s", *serverAddr)
	log.Printf("Durable Name: %s", *durableName)
	log.Printf("Output Directory: %s", *outputDir)

	// Create subscriber manager
	config := &subscriber.Config{
		ServerAddr:  *serverAddr,
		DurableName: *durableName,
		BatchSize:   int32(*batchSize),
	}

	manager, err := subscriber.NewManager(config)
	if err != nil {
		log.Fatalf("Failed to create subscriber manager: %v", err)
	}
	defer manager.Stop()

	// Register handlers for different subjects
	// This is the pattern similar to your example
	manager.RegisterHandlers(map[string]handler.MessageHandler{
		// Images: save to ./downloads/images/
		"images.jpeg": handler.NewImageProcessorHandler(*outputDir + "/images"),
		"images.png":  handler.NewImageProcessorHandler(*outputDir + "/images"),

		// Documents: save to ./downloads/documents/
		"documents.pdf":  handler.NewFileSaverHandler(*outputDir + "/documents"),
		"documents.json": handler.NewFileSaverHandler(*outputDir + "/documents"),

		// Test channels: save to ./downloads/test/
		"test.debug":     handler.NewFileSaverHandler(*outputDir + "/test"),
		"test.fullchain": handler.NewFileSaverHandler(*outputDir + "/test"),
		"final.test":     handler.NewFileSaverHandler(*outputDir + "/test"),

		// Logs: just log without saving
		"logs.system": handler.NewLoggerHandler("SYSTEM"),
		"logs.app":    handler.NewLoggerHandler("APP"),
	})

	// Start all subscriptions
	if err := manager.Start(); err != nil {
		log.Fatalf("Failed to start subscriptions: %v", err)
	}

	log.Printf("✓ All subscriptions started, waiting for messages...")
	log.Printf("Press Ctrl+C to stop")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Printf("\nShutting down...")
	manager.Stop()
	log.Printf("✓ Subscriber client finished")
}
