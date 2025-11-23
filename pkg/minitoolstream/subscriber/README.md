# MiniToolStream Subscriber Library

Clean Architecture based Go client library for subscribing to MiniToolStreamEgress.

## Architecture

This library follows Clean Architecture principles:

```
pkg/minitoolstream/subscriber/
├── domain/          # Business logic layer (entities, interfaces)
│   ├── entities.go  # Core domain entities
│   └── interfaces.go # Domain interfaces
├── client/          # Infrastructure layer (gRPC implementation)
│   └── grpc_client.go
├── usecase/         # Use cases layer (business logic)
│   └── subscriber.go
└── handler/         # Message handlers
    ├── file_saver.go
    ├── image_processor.go
    └── logger.go
```

### Dependency Direction

```
handler ──> usecase ──> client ──> domain
                   └────> domain
```

All dependencies point inward toward the domain layer.

## Features

- Clean separation of concerns
- Interface-based design for testability
- Concurrent multi-subject subscription
- Built-in logging and error handling
- Support for different message types (files, images, logs)
- Flexible configuration through builders
- Custom message handlers

## Usage

### Simple Usage

```go
import "github.com/moroshma/MiniToolStream/pkg/minitoolstream/subscriber"
import "github.com/moroshma/MiniToolStream/pkg/minitoolstream/subscriber/handler"
import "github.com/moroshma/MiniToolStream/pkg/minitoolstream/subscriber/domain"

// Create subscriber
sub, err := subscriber.NewSubscriber("localhost:50052", "my-subscriber")
if err != nil {
    log.Fatal(err)
}
defer sub.Stop()

// Create handlers
fileSaver, _ := handler.NewFileSaver(&handler.FileSaverConfig{
    OutputDir: "./downloads",
})

logHandler := handler.NewLoggerHandler(&handler.LoggerHandlerConfig{
    Prefix: "LOGS",
})

// Register handlers for subjects
sub.RegisterHandlers(map[string]domain.MessageHandler{
    "documents.pdf": fileSaver,
    "images.jpeg":   fileSaver,
    "logs.system":   logHandler,
})

// Start subscriptions
if err := sub.Start(); err != nil {
    log.Fatal(err)
}

// Wait for interrupt signal
sub.Wait()
```

### Advanced Usage with Builder

```go
// Build subscriber with custom configuration
sub, err := subscriber.NewSubscriberBuilder("localhost:50052").
    WithDurableName("custom-subscriber").
    WithBatchSize(50).
    WithLogger(customLogger).
    Build()
if err != nil {
    log.Fatal(err)
}
defer sub.Stop()

// Register handlers
imageHandler, _ := handler.NewImageProcessor(&handler.ImageProcessorConfig{
    OutputDir: "./images",
})

sub.RegisterHandler("images.png", imageHandler)

// Start
if err := sub.Start(); err != nil {
    log.Fatal(err)
}

// Wait
sub.Wait()
```

### Custom Message Handlers

```go
// Implement custom message handler
type CustomHandler struct {
    // your fields
}

func (h *CustomHandler) Handle(ctx context.Context, msg *domain.Message) error {
    // Process message
    log.Printf("Received: %s, seq=%d, size=%d",
        msg.Subject, msg.Sequence, len(msg.Data))

    // Custom processing logic
    return nil
}

// Use it
sub.RegisterHandler("custom.subject", &CustomHandler{})
```

## Design Principles

1. **Dependency Inversion**: High-level modules don't depend on low-level modules
2. **Interface Segregation**: Small, focused interfaces
3. **Single Responsibility**: Each component has one reason to change
4. **Open/Closed**: Open for extension, closed for modification

## Handlers

### FileSaver
Saves message data to files with automatic extension detection based on content-type.

```go
handler.NewFileSaver(&handler.FileSaverConfig{
    OutputDir: "./downloads/documents",
})
```

### ImageProcessor
Specialized handler for processing and saving images.

```go
handler.NewImageProcessor(&handler.ImageProcessorConfig{
    OutputDir: "./downloads/images",
})
```

### LoggerHandler
Logs message data without saving to disk.

```go
handler.NewLoggerHandler(&handler.LoggerHandlerConfig{
    Prefix: "SYSTEM",
})
```

## Testing

The architecture makes testing easy:

```go
// Mock the client
type MockClient struct{}

func (m *MockClient) Subscribe(ctx context.Context, config *domain.SubscriptionConfig) (domain.NotificationStream, error) {
    return &mockNotificationStream{}, nil
}

func (m *MockClient) Fetch(ctx context.Context, config *domain.SubscriptionConfig) (domain.MessageStream, error) {
    return &mockMessageStream{}, nil
}

func (m *MockClient) GetLastSequence(ctx context.Context, subject string) (uint64, error) {
    return 100, nil
}

func (m *MockClient) Close() error { return nil }

// Use mock in tests
sub, _ := usecase.New(&usecase.Config{
    Client:      &MockClient{},
    DurableName: "test",
})
```

## Integration with Publisher

This library works seamlessly with the publisher library:

```
Publisher (pkg/minitoolstream)
    ↓ publishes to
MiniToolStreamIngress
    ↓ stores in
Tarantool + MinIO
    ↓ reads from
MiniToolStreamEgress
    ↓ delivers to
Subscriber (pkg/minitoolstream/subscriber)
```

## Comparison with Old Implementation

**Before** (example/subscriber_client/internal):
- Tightly coupled to gRPC
- Hard to test
- No clear separation of concerns

**After** (pkg/minitoolstream/subscriber):
- Clean architecture
- Easy to test and extend
- Clear separation of concerns
- Reusable library
