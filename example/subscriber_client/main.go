package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/moroshma/MiniToolStream/model"
)

var (
	serverAddr  = flag.String("server", "localhost:50052", "MiniToolStreamEgress gRPC server address")
	subject     = flag.String("subject", "terminator.diff", "Subject/channel to subscribe to")
	durableName = flag.String("durable", "test-subscriber", "Durable consumer name")
	outputDir   = flag.String("output", "./downloads", "Directory to save downloaded files")
)

func main() {
	flag.Parse()

	log.Printf("MiniToolStream Subscriber Client")
	log.Printf("Connecting to: %s", *serverAddr)
	log.Printf("Subject: %s", *subject)
	log.Printf("Durable Name: %s", *durableName)

	// Create output directory
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Connect to MiniToolStreamEgress gRPC server
	conn, err := grpc.NewClient(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewEgressServiceClient(conn)
	ctx := context.Background()

	// First, subscribe to notifications
	log.Printf("Starting subscription...")
	subscribeReq := &pb.SubscribeRequest{
		Subject:     *subject,
		DurableName: *durableName,
	}

	subscribeStream, err := client.Subscribe(ctx, subscribeReq)
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	// Listen for notifications in a goroutine
	notificationChan := make(chan *pb.Notification, 10)
	go func() {
		for {
			notification, err := subscribeStream.Recv()
			if err == io.EOF {
				log.Printf("Subscribe stream closed")
				close(notificationChan)
				return
			}
			if err != nil {
				log.Printf("Subscribe error: %v", err)
				close(notificationChan)
				return
			}
			log.Printf("📬 Notification received: subject=%s, sequence=%d", notification.Subject, notification.Sequence)
			notificationChan <- notification
		}
	}()

	// Process notifications
	log.Printf("Waiting for notifications... (press Ctrl+C to exit)")
	for notification := range notificationChan {
		log.Printf("Processing notification for sequence %d", notification.Sequence)

		// Fetch messages
		fetchReq := &pb.FetchRequest{
			Subject:     notification.Subject,
			DurableName: *durableName,
			BatchSize:   10,
		}

		fetchStream, err := client.Fetch(ctx, fetchReq)
		if err != nil {
			log.Printf("Failed to fetch: %v", err)
			continue
		}

		// Receive messages
		messageCount := 0
		for {
			msg, err := fetchStream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("Fetch error: %v", err)
				break
			}

			messageCount++
			log.Printf("📨 Message received: sequence=%d, subject=%s, data_size=%d",
				msg.Sequence, msg.Subject, len(msg.Data))

			// Print headers
			if len(msg.Headers) > 0 {
				log.Printf("   Headers: %v", msg.Headers)
			}

			// Save data to file if present
			if len(msg.Data) > 0 {
				filename := fmt.Sprintf("%s/%s_seq_%d", *outputDir, msg.Subject, msg.Sequence)

				// Add extension based on content-type
				if contentType, ok := msg.Headers["content-type"]; ok {
					switch contentType {
					case "image/jpeg":
						filename += ".jpg"
					case "image/png":
						filename += ".png"
					case "text/plain":
						filename += ".txt"
					case "application/json":
						filename += ".json"
					}
				}

				err = os.WriteFile(filename, msg.Data, 0644)
				if err != nil {
					log.Printf("   Failed to save file: %v", err)
				} else {
					log.Printf("   ✓ Saved to: %s", filename)
				}
			}
		}

		log.Printf("Fetched %d messages", messageCount)
	}

	log.Printf("Subscriber client finished")
}
