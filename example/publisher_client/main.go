package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/moroshma/MiniToolStream/pkg/minitoolstream"
	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/handler"
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

	// Create publisher using the library
	pub, err := minitoolstream.NewPublisher(*serverAddr)
	if err != nil {
		log.Fatalf("Failed to create publisher: %v", err)
	}
	defer pub.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	// Register handlers based on flags
	if *imagePath != "" {
		if *subject == "" {
			log.Fatalf("Subject is required when using -image")
		}
		log.Printf("Registering image handler: %s -> %s", *imagePath, *subject)
		pub.RegisterHandler(handler.NewImageHandler(&handler.ImageHandlerConfig{
			Subject:   *subject,
			ImagePath: *imagePath,
		}))
	}

	if *filePath != "" {
		if *subject == "" {
			log.Fatalf("Subject is required when using -file")
		}
		log.Printf("Registering file handler: %s -> %s", *filePath, *subject)
		pub.RegisterHandler(handler.NewFileHandler(&handler.FileHandlerConfig{
			Subject:  *subject,
			FilePath: *filePath,
		}))
	}

	if *data != "" {
		if *subject == "" {
			log.Fatalf("Subject is required when using -data")
		}
		log.Printf("Registering data handler: %d bytes -> %s", len(*data), *subject)
		pub.RegisterHandler(handler.NewDataHandler(&handler.DataHandlerConfig{
			Subject:     *subject,
			Data:        []byte(*data),
			ContentType: "text/plain",
		}))
	}

	// Publish all registered handlers
	if err := pub.PublishAll(ctx, nil); err != nil {
		log.Fatalf("Failed to publish: %v", err)
	}

	log.Printf("✓ Publisher client finished")
}
