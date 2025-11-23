# MiniToolStream Client Library

Clean Architecture based Go client library for MiniToolStreamIngress.

## Architecture

This library follows Clean Architecture principles:

```
pkg/minitoolstream/
├── domain/          # Business logic layer (entities, interfaces)
│   ├── entities.go  # Core domain entities
│   └── interfaces.go # Domain interfaces
├── client/          # Infrastructure layer (gRPC implementation)
│   └── grpc_client.go
├── publisher/       # Use cases layer (business logic)
│   ├── publisher.go
│   └── result_handler.go
└── handler/         # Adapters for different data types
    ├── data_handler.go
    ├── file_handler.go
    └── image_handler.go
```

### Dependency Direction

```
handler ──> publisher ──> client ──> domain
                              └────> domain
```

All dependencies point inward toward the domain layer.

## Features

- Clean separation of concerns
- Interface-based design for testability
- Concurrent message publishing
- Built-in logging and error handling
- Support for multiple data types (raw data, files, images)
- Flexible configuration through builders
- Custom result handlers

## Usage

### Simple Usage

```go
import "github.com/moroshma/MiniToolStream/pkg/minitoolstream"
import "github.com/moroshma/MiniToolStream/pkg/minitoolstream/handler"

// Create publisher
pub, err := minitoolstream.NewPublisher("localhost:50051")
if err != nil {
    log.Fatal(err)
}
defer pub.Close()

// Create a data handler
dataHandler := handler.NewDataHandler(&handler.DataHandlerConfig{
    Subject:     "test.subject",
    Data:        []byte("Hello, World!"),
    ContentType: "text/plain",
})

// Publish
ctx := context.Background()
if err := pub.Publish(ctx, dataHandler); err != nil {
    log.Fatal(err)
}
```

### Advanced Usage with Builder

```go
// Build publisher with custom configuration
pub, err := minitoolstream.NewPublisherBuilder("localhost:50051").
    WithLogger(customLogger).
    WithResultHandler(customHandler).
    Build()
if err != nil {
    log.Fatal(err)
}
defer pub.Close()

// Register multiple handlers
pub.RegisterHandlers([]domain.MessagePreparer{
    handler.NewImageHandler(&handler.ImageHandlerConfig{
        Subject:   "images.jpeg",
        ImagePath: "photo.jpg",
    }),
    handler.NewFileHandler(&handler.FileHandlerConfig{
        Subject:  "documents.json",
        FilePath: "config.json",
    }),
    handler.NewDataHandler(&handler.DataHandlerConfig{
        Subject:     "logs.app",
        Data:        []byte("Application started"),
        ContentType: "text/plain",
    }),
})

// Publish all concurrently
ctx := context.Background()
if err := pub.PublishAll(ctx, nil); err != nil {
    log.Fatal(err)
}
```

### Custom Message Preparers

```go
// Implement custom message preparer
type CustomPreparer struct {
    // your fields
}

func (p *CustomPreparer) Prepare(ctx context.Context) (*domain.Message, error) {
    return &domain.Message{
        Subject: "custom.subject",
        Data:    []byte("custom data"),
        Headers: map[string]string{
            "content-type": "application/custom",
        },
    }, nil
}

// Use it
pub.Publish(ctx, &CustomPreparer{})
```

## Design Principles

1. **Dependency Inversion**: High-level modules don't depend on low-level modules. Both depend on abstractions.
2. **Interface Segregation**: Small, focused interfaces (MessagePreparer, ResultHandler, IngressClient).
3. **Single Responsibility**: Each component has one reason to change.
4. **Open/Closed**: Open for extension (custom handlers), closed for modification.

## Testing

The architecture makes testing easy:

```go
// Mock the client
type MockClient struct{}

func (m *MockClient) Publish(ctx context.Context, msg *domain.Message) (*domain.PublishResult, error) {
    return &domain.PublishResult{StatusCode: 0}, nil
}

func (m *MockClient) Close() error { return nil }

// Use mock in tests
pub, _ := publisher.New(&publisher.Config{
    Client: &MockClient{},
})
```
