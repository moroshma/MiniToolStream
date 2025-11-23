package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/moroshma/MiniToolStream/example/publisher_client/internal/handler"
	"github.com/moroshma/MiniToolStream/example/publisher_client/internal/publisher"
)

var (
	serverAddr = flag.String("server", "localhost:50051", "MiniToolStreamIngress gRPC server address")
	imagePath  = flag.String("image", "", "Path to image file (optional)")
	subject    = flag.String("subject", "", "Subject/channel name (required if using -image or -file)")
	filePath   = flag.String("file", "", "Path to file (optional)")
	data       = flag.String("data", "", "Raw data to publish (optional)")
	timeout    = flag.Int("timeout", 10, "Timeout in seconds")
)

func main() {
	flag.Parse()

	log.Printf("MiniToolStream Publisher Client")
	log.Printf("Connecting to: %s", *serverAddr)

	// Create publisher manager
	config := &publisher.Config{
		ServerAddr: *serverAddr,
		Timeout:    *timeout,
	}

	manager, err := publisher.NewManager(config)
	if err != nil {
		log.Fatalf("Failed to create publisher manager: %v", err)
	}
	defer manager.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	// Register handlers based on flags
	if *imagePath != "" {
		if *subject == "" {
			log.Fatalf("Subject is required when using -image")
		}
		log.Printf("Registering image handler: %s -> %s", *imagePath, *subject)
		manager.RegisterHandler(handler.NewImagePublisherHandler(*subject, *imagePath))
	}

	if *filePath != "" {
		if *subject == "" {
			log.Fatalf("Subject is required when using -file")
		}
		log.Printf("Registering file handler: %s -> %s", *filePath, *subject)
		manager.RegisterHandler(handler.NewFilePublisherHandler(*subject, *filePath, ""))
	}

	if *data != "" {
		if *subject == "" {
			log.Fatalf("Subject is required when using -data")
		}
		log.Printf("Registering data handler: %d bytes -> %s", len(*data), *subject)
		manager.RegisterHandler(handler.NewDataPublisherHandler(*subject, []byte(*data), "text/plain"))
	}

	// Example: Register multiple handlers programmatically
	// Uncomment to use:
	/*
		manager.RegisterHandlers([]handler.PublishHandler{
			handler.NewImagePublisherHandler("images.jpeg", "test1.jpg"),
			handler.NewImagePublisherHandler("images.png", "test2.png"),
			handler.NewFilePublisherHandler("documents.json", "config.json", "application/json"),
			handler.NewDataPublisherHandler("logs.system", []byte("System started"), "text/plain"),
		})
	*/

	// Publish all registered handlers
	if err := manager.PublishAll(ctx); err != nil {
		log.Fatalf("Failed to publish: %v", err)
	}

	log.Printf("✓ Publisher client finished")
}
