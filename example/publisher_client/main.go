package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/moroshma/MiniToolStream/model"
)

var (
	serverAddr = flag.String("app", "localhost:50051", "MiniToolStreamIngress gRPC app address")
	imagePath  = flag.String("image", "tst.jpeg", "Path to image file")
	subject    = flag.String("subject", "terminator.diff", "Subject/channel name")
)

func main() {
	flag.Parse()

	// Read image file
	imageData, err := os.ReadFile(*imagePath)
	if err != nil {
		log.Fatalf("Failed to read image file %s: %v", *imagePath, err)
	}
	log.Printf("Read image file: %s (%d bytes)", *imagePath, len(imageData))

	// Connect to MiniToolStreamIngress gRPC app
	log.Printf("Connecting to MiniToolStreamIngress at %s...", *serverAddr)
	conn, err := grpc.NewClient(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewIngressServiceClient(conn)

	// Publish image
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := &pb.PublishRequest{
		Subject: *subject,
		Data:    imageData,
		Headers: map[string]string{
			"content-type": "image/jpeg",
			"filename":     *imagePath,
			"timestamp":    time.Now().Format(time.RFC3339),
		},
	}

	log.Printf("Publishing image to subject '%s'...", *subject)
	resp, err := client.Publish(ctx, req)
	if err != nil {
		log.Fatalf("Publish failed: %v", err)
	}

	if resp.StatusCode != 0 {
		log.Fatalf("Server returned error: %s", resp.ErrorMessage)
	}

	log.Printf("✓ Image published successfully!")
	log.Printf("  Subject: %s", *subject)
	log.Printf("  Sequence: %d", resp.Sequence)
	log.Printf("  ObjectName: %s", resp.ObjectName)
	log.Printf("  Data size: %d bytes", len(imageData))
	log.Printf("\nNote: Upload the image data to MinIO using ObjectName '%s' as the key", resp.ObjectName)
}
