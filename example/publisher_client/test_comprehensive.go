package main

import (
	"context"
	"log"
	"time"

	"github.com/moroshma/MiniToolStream/pkg/minitoolstream"
	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/domain"
	"github.com/moroshma/MiniToolStream/pkg/minitoolstream/handler"
)

func main() {
	log.Printf("=== MiniToolStream Comprehensive Integration Test ===")
	log.Printf("")

	// Test 1: Single data message
	log.Printf("Test 1: Single data message")
	testSingleMessage()
	time.Sleep(500 * time.Millisecond)

	// Test 2: Image upload
	log.Printf("\nTest 2: Image upload")
	testImageUpload()
	time.Sleep(500 * time.Millisecond)

	// Test 3: Multiple concurrent messages
	log.Printf("\nTest 3: Multiple concurrent messages")
	testMultipleMessages()
	time.Sleep(500 * time.Millisecond)

	// Test 4: Custom headers
	log.Printf("\nTest 4: Custom headers")
	testCustomHeaders()
	time.Sleep(500 * time.Millisecond)

	log.Printf("\n=== All tests completed successfully! ===")
}

func testSingleMessage() {
	pub, err := minitoolstream.NewPublisher("localhost:50051")
	if err != nil {
		log.Fatalf("Failed to create publisher: %v", err)
	}
	defer pub.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dataHandler := handler.NewDataHandler(&handler.DataHandlerConfig{
		Subject:     "test.single",
		Data:        []byte("Single test message"),
		ContentType: "text/plain",
	})

	if err := pub.Publish(ctx, dataHandler); err != nil {
		log.Fatalf("Test 1 failed: %v", err)
	}

	log.Printf("✓ Test 1 passed")
}

func testImageUpload() {
	pub, err := minitoolstream.NewPublisher("localhost:50051")
	if err != nil {
		log.Fatalf("Failed to create publisher: %v", err)
	}
	defer pub.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	imageHandler := handler.NewImageHandler(&handler.ImageHandlerConfig{
		Subject:   "images.comprehensive",
		ImagePath: "/Users/moroshma/go/MiniToolStream/example/publisher_client/tst.jpeg",
	})

	if err := pub.Publish(ctx, imageHandler); err != nil {
		log.Fatalf("Test 2 failed: %v", err)
	}

	log.Printf("✓ Test 2 passed")
}

func testMultipleMessages() {
	pub, err := minitoolstream.NewPublisher("localhost:50051")
	if err != nil {
		log.Fatalf("Failed to create publisher: %v", err)
	}
	defer pub.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messages := []domain.MessagePreparer{
		handler.NewDataHandler(&handler.DataHandlerConfig{
			Subject:     "test.multi.1",
			Data:        []byte("Message 1"),
			ContentType: "text/plain",
		}),
		handler.NewDataHandler(&handler.DataHandlerConfig{
			Subject:     "test.multi.2",
			Data:        []byte("Message 2"),
			ContentType: "text/plain",
		}),
		handler.NewDataHandler(&handler.DataHandlerConfig{
			Subject:     "test.multi.3",
			Data:        []byte("Message 3"),
			ContentType: "text/plain",
		}),
		handler.NewImageHandler(&handler.ImageHandlerConfig{
			Subject:   "test.multi.image",
			ImagePath: "/Users/moroshma/go/MiniToolStream/example/publisher_client/tst.jpeg",
		}),
	}

	if err := pub.PublishAll(ctx, messages); err != nil {
		log.Fatalf("Test 3 failed: %v", err)
	}

	log.Printf("✓ Test 3 passed")
}

func testCustomHeaders() {
	pub, err := minitoolstream.NewPublisher("localhost:50051")
	if err != nil {
		log.Fatalf("Failed to create publisher: %v", err)
	}
	defer pub.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dataHandler := handler.NewDataHandler(&handler.DataHandlerConfig{
		Subject:     "test.headers",
		Data:        []byte("Message with custom headers"),
		ContentType: "text/plain",
		Headers: map[string]string{
			"x-custom-header": "custom-value",
			"x-test-id":       "12345",
		},
	})

	if err := pub.Publish(ctx, dataHandler); err != nil {
		log.Fatalf("Test 4 failed: %v", err)
	}

	log.Printf("✓ Test 4 passed")
}
