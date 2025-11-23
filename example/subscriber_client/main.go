package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/subscriber"
	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/subscriber/domain"
	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/subscriber/handler"
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

	// Create subscriber using the library
	sub, err := subscriber.NewSubscriberBuilder(*serverAddr).
		WithDurableName(*durableName).
		WithBatchSize(int32(*batchSize)).
		Build()
	if err != nil {
		log.Fatalf("Failed to create subscriber: %v", err)
	}
	defer sub.Stop()

	// Create handlers
	imageHandler, err := handler.NewImageProcessor(&handler.ImageProcessorConfig{
		OutputDir: *outputDir + "/images",
	})
	if err != nil {
		log.Fatalf("Failed to create image handler: %v", err)
	}

	documentHandler, err := handler.NewFileSaver(&handler.FileSaverConfig{
		OutputDir: *outputDir + "/documents",
	})
	if err != nil {
		log.Fatalf("Failed to create document handler: %v", err)
	}

	testHandler, err := handler.NewFileSaver(&handler.FileSaverConfig{
		OutputDir: *outputDir + "/test",
	})
	if err != nil {
		log.Fatalf("Failed to create test handler: %v", err)
	}

	systemLogHandler := handler.NewLoggerHandler(&handler.LoggerHandlerConfig{
		Prefix: "SYSTEM",
	})

	appLogHandler := handler.NewLoggerHandler(&handler.LoggerHandlerConfig{
		Prefix: "APP",
	})

	// Register handlers for different subjects
	sub.RegisterHandlers(map[string]domain.MessageHandler{
		// Images: save to ./downloads/images/
		"images.jpeg": imageHandler,
		"images.png":  imageHandler,

		// Documents: save to ./downloads/documents/
		"documents.pdf":  documentHandler,
		"documents.json": documentHandler,

		// Test channels: save to ./downloads/test/
		"test.debug":     testHandler,
		"test.fullchain": testHandler,
		"test.config":    testHandler,
		"test.default":   testHandler,
		"test.envvar":    testHandler,
		"test.vault":     testHandler,
		"final.test":     testHandler,
		"test.single":    testHandler,
		"test.multi.1":   testHandler,
		"test.multi.2":   testHandler,
		"test.multi.3":   testHandler,

		// Logs: just log without saving
		"logs.system": systemLogHandler,
		"logs.app":    appLogHandler,
	})

	// Start all subscriptions
	if err := sub.Start(); err != nil {
		log.Fatalf("Failed to start subscriptions: %v", err)
	}

	log.Printf("✓ All subscriptions started, waiting for messages...")
	log.Printf("Press Ctrl+C to stop")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Printf("\nShutting down...")
	sub.Stop()
	log.Printf("✓ Subscriber client finished")
}
