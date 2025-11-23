# MiniToolStream Client Library - Clean Architecture

## Обзор

Клиентская библиотека для работы с MiniToolStreamIngress, построенная на принципах Clean Architecture.

## Архитектура

### Структура проекта

```
pkg/minitoolstream/
├── domain/              # Слой бизнес-логики (Entities & Interfaces)
│   ├── entities.go      # Доменные сущности (Message, PublishResult)
│   └── interfaces.go    # Доменные интерфейсы (IngressClient, Publisher)
├── client/              # Слой инфраструктуры (Frameworks & Drivers)
│   └── grpc_client.go   # gRPC реализация IngressClient
├── publisher/           # Слой бизнес-логики (Use Cases)
│   ├── publisher.go     # Реализация Publisher
│   └── result_handler.go # Обработчик результатов
├── handler/             # Адаптеры (Interface Adapters)
│   ├── data_handler.go  # Обработчик сырых данных
│   ├── file_handler.go  # Обработчик файлов
│   └── image_handler.go # Обработчик изображений
└── minitoolstream.go    # Фасад и builder'ы
```

### Диаграмма зависимостей

```
┌─────────────────────────────────────────────────┐
│               Application Layer                 │
│         (example/publisher_client)              │
└─────────────────┬───────────────────────────────┘
                  │ uses
┌─────────────────▼───────────────────────────────┐
│            Interface Adapters                   │
│    (handler: DataHandler, FileHandler,          │
│              ImageHandler)                      │
└─────────────────┬───────────────────────────────┘
                  │ implements
┌─────────────────▼───────────────────────────────┐
│              Use Cases Layer                    │
│        (publisher: SimplePublisher)             │
└─────────────────┬───────────────────────────────┘
                  │ uses
┌─────────────────▼───────────────────────────────┐
│           Domain Layer (Core)                   │
│   Entities: Message, PublishResult              │
│   Interfaces: IngressClient, Publisher,         │
│              MessagePreparer, ResultHandler     │
└─────────────────▲───────────────────────────────┘
                  │ implements
┌─────────────────┴───────────────────────────────┐
│       Infrastructure Layer                      │
│      (client: GRPCClient)                       │
└─────────────────────────────────────────────────┘
```

**Ключевой принцип:** Все зависимости направлены внутрь, к domain слою.

## Принципы Clean Architecture

### 1. Dependency Rule
Зависимости указывают только внутрь. Внешние слои зависят от внутренних, но не наоборот.

- `handler` зависит от `domain` (использует `MessagePreparer`)
- `publisher` зависит от `domain` (реализует `Publisher`, использует `IngressClient`)
- `client` зависит от `domain` (реализует `IngressClient`)
- `domain` не зависит ни от чего

### 2. Interface Segregation
Маленькие, сфокусированные интерфейсы:

- `MessagePreparer` - подготовка сообщения
- `ResultHandler` - обработка результата
- `IngressClient` - взаимодействие с сервером
- `Publisher` - публикация сообщений

### 3. Single Responsibility
Каждый компонент имеет одну причину для изменения:

- `domain/entities.go` - определения доменных сущностей
- `domain/interfaces.go` - контракты взаимодействия
- `client/grpc_client.go` - только gRPC коммуникация
- `publisher/publisher.go` - оркестрация публикаций
- `handler/*` - подготовка конкретных типов данных

### 4. Open/Closed Principle
Библиотека открыта для расширения, закрыта для модификации:

- Новые типы обработчиков: реализуйте `MessagePreparer`
- Новые протоколы: реализуйте `IngressClient`
- Новая обработка результатов: реализуйте `ResultHandler`

## Использование

### Простой пример

```go
import "github.com/moroshma/MiniToolStream/pkg/minitoolstream"
import "github.com/moroshma/MiniToolStream/pkg/minitoolstream/handler"

// Создание publisher
pub, err := minitoolstream.NewPublisher("localhost:50051")
if err != nil {
    log.Fatal(err)
}
defer pub.Close()

// Публикация данных
dataHandler := handler.NewDataHandler(&handler.DataHandlerConfig{
    Subject:     "test.subject",
    Data:        []byte("Hello, World!"),
    ContentType: "text/plain",
})

ctx := context.Background()
if err := pub.Publish(ctx, dataHandler); err != nil {
    log.Fatal(err)
}
```

### Множественная публикация

```go
pub.RegisterHandlers([]domain.MessagePreparer{
    handler.NewImageHandler(&handler.ImageHandlerConfig{
        Subject:   "images.jpeg",
        ImagePath: "photo.jpg",
    }),
    handler.NewDataHandler(&handler.DataHandlerConfig{
        Subject:     "logs.app",
        Data:        []byte("Application started"),
        ContentType: "text/plain",
    }),
})

// Публикация всех зарегистрированных обработчиков конкурентно
if err := pub.PublishAll(ctx, nil); err != nil {
    log.Fatal(err)
}
```

### Расширение функциональности

Создание пользовательского обработчика:

```go
type CustomPreparer struct {
    apiEndpoint string
}

func (p *CustomPreparer) Prepare(ctx context.Context) (*domain.Message, error) {
    // Получение данных из API
    data, err := fetchFromAPI(p.apiEndpoint)
    if err != nil {
        return nil, err
    }

    return &domain.Message{
        Subject: "custom.subject",
        Data:    data,
        Headers: map[string]string{
            "content-type": "application/json",
        },
    }, nil
}

// Использование
pub.Publish(ctx, &CustomPreparer{apiEndpoint: "https://api.example.com"})
```

## Тестирование

Архитектура упрощает тестирование через mock'и:

```go
type MockClient struct{}

func (m *MockClient) Publish(ctx context.Context, msg *domain.Message) (*domain.PublishResult, error) {
    return &domain.PublishResult{
        Sequence:   1,
        ObjectName: "test_obj",
        StatusCode: 0,
    }, nil
}

func (m *MockClient) Close() error { return nil }

// Использование в тестах
pub, _ := publisher.New(&publisher.Config{
    Client: &MockClient{},
})
```

## Результаты тестирования

### Test Suite Results

✅ **Test 1: Single message publish** - PASSED
- Публикация одного сообщения с текстовыми данными
- Sequence: 71, Object: test.single_71

✅ **Test 2: Image upload** - PASSED
- Публикация изображения (159 bytes)
- Sequence: 72, Object: images.comprehensive_72

✅ **Test 3: Multiple concurrent messages** - PASSED
- Конкурентная публикация 4 сообщений (3 текстовых + 1 изображение)
- Sequences: 73-76

✅ **Test 4: Custom headers** - PASSED
- Публикация с пользовательскими заголовками
- Sequence: 77, Object: test.headers_77

✅ **Error Handling Tests** - PASSED
- Обработка несуществующих файлов: корректное сообщение об ошибке
- Обработка отсутствия соединения: корректное сообщение об ошибке

## Преимущества архитектуры

1. **Тестируемость** - легко создавать моки и стабы
2. **Расширяемость** - новые обработчики без изменения кода
3. **Независимость от фреймворков** - domain не зависит от gRPC
4. **Гибкость** - можно заменить gRPC на другой протокол
5. **Поддерживаемость** - четкое разделение ответственности
6. **Переиспользуемость** - библиотеку можно использовать в любом Go проекте

## Миграция с старого кода

Старая реализация (example/publisher_client/internal) была полностью заменена на использование библиотеки:

**До:**
```go
import "github.com/moroshma/MiniToolStream/example/publisher_client/internal/publisher"
import "github.com/moroshma/MiniToolStream/example/publisher_client/internal/handler"

manager, err := publisher.NewManager(config)
```

**После:**
```go
import "github.com/moroshma/MiniToolStream/pkg/minitoolstream"
import "github.com/moroshma/MiniToolStream/pkg/minitoolstream/handler"

pub, err := minitoolstream.NewPublisher("localhost:50051")
```

Код стал проще, чище и более переиспользуемым.
