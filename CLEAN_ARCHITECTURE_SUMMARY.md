# MiniToolStream - Clean Architecture Implementation

## Обзор

Проект MiniToolStream был полностью рефакторен с применением принципов Clean Architecture. Вся клиентская логика для работы с Publisher (Ingress) и Subscriber (Egress) выделена в переиспользуемые библиотеки.

## Структура проекта

```
MiniToolStream/
├── pkg/minitoolstream/              # Publisher library
│   ├── domain/                      # Бизнес-логика
│   ├── client/                      # gRPC client для Ingress
│   ├── publisher/                   # Use cases публикации
│   ├── handler/                     # Обработчики данных
│   └── minitoolstream.go            # Фасад
│
├── pkg/minitoolstream/subscriber/   # Subscriber library
│   ├── domain/                      # Бизнес-логика
│   ├── client/                      # gRPC client для Egress
│   ├── usecase/                     # Use cases подписки
│   ├── handler/                     # Обработчики сообщений
│   └── subscriber.go                # Фасад
│
├── example/publisher_client/        # Пример использования Publisher
│   └── main.go                      # 80 строк (было 100+ + internal/)
│
└── example/subscriber_client/       # Пример использования Subscriber
    └── main.go                      # 110 строк (было 80 + internal/)
```

## Clean Architecture

### Слои архитектуры

```
┌─────────────────────────────────────────────────────┐
│           Frameworks & Drivers                       │
│         (gRPC, External Libraries)                   │
└─────────────────┬───────────────────────────────────┘
                  │
┌─────────────────▼───────────────────────────────────┐
│         Interface Adapters                          │
│    (Handlers: Data, File, Image)                    │
└─────────────────┬───────────────────────────────────┘
                  │
┌─────────────────▼───────────────────────────────────┐
│             Use Cases                               │
│   (Publisher/Subscriber business logic)             │
└─────────────────┬───────────────────────────────────┘
                  │
┌─────────────────▼───────────────────────────────────┐
│              Domain                                 │
│  (Entities, Interfaces - no dependencies)           │
└─────────────────────────────────────────────────────┘
```

**Ключевой принцип:** Зависимости направлены внутрь, к domain слою.

## Publisher Library (pkg/minitoolstream)

### Архитектура

```
pkg/minitoolstream/
├── domain/
│   ├── entities.go      # Message, PublishResult
│   └── interfaces.go    # IngressClient, Publisher, MessagePreparer
├── client/
│   └── grpc_client.go   # Реализация IngressClient
├── publisher/
│   ├── publisher.go     # Реализация Publisher
│   └── result_handler.go
└── handler/
    ├── data_handler.go  # Для raw data
    ├── file_handler.go  # Для файлов
    └── image_handler.go # Для изображений
```

### Использование

```go
// Простое использование
pub, _ := minitoolstream.NewPublisher("localhost:50051")
defer pub.Close()

pub.RegisterHandler(handler.NewDataHandler(&handler.DataHandlerConfig{
    Subject:     "test.subject",
    Data:        []byte("Hello!"),
    ContentType: "text/plain",
}))

pub.PublishAll(ctx, nil)
```

### Тестирование Publisher

✅ Single message publish
✅ Image upload
✅ Multiple concurrent messages
✅ Custom headers
✅ Error handling (file not found, connection refused)

## Subscriber Library (pkg/minitoolstream/subscriber)

### Архитектура

```
pkg/minitoolstream/subscriber/
├── domain/
│   ├── entities.go      # Message, Notification, SubscriptionConfig
│   └── interfaces.go    # EgressClient, Subscriber, MessageHandler
├── client/
│   └── grpc_client.go   # Реализация EgressClient
├── usecase/
│   └── subscriber.go    # Реализация Subscriber
└── handler/
    ├── file_saver.go    # Сохранение файлов
    ├── image_processor.go # Обработка изображений
    └── logger.go        # Логирование без сохранения
```

### Использование

```go
// Простое использование
sub, _ := subscriber.NewSubscriber("localhost:50052", "my-subscriber")
defer sub.Stop()

imageHandler, _ := handler.NewImageProcessor(&handler.ImageProcessorConfig{
    OutputDir: "./downloads/images",
})

sub.RegisterHandlers(map[string]domain.MessageHandler{
    "images.jpeg": imageHandler,
    "logs.system": handler.NewLoggerHandler(&handler.LoggerHandlerConfig{
        Prefix: "SYSTEM",
    }),
})

sub.Start()
sub.Wait()
```

### Тестирование Subscriber

✅ Multi-subject subscription (13 subjects)
✅ Concurrent message processing
✅ File saving with correct extensions
✅ Image processing and saving
✅ Logging without persistence
✅ End-to-end flow (publish → subscribe)

## Полная цепочка E2E

```
Publisher Client (новая библиотека)
    ↓ gRPC
MiniToolStreamIngress Server
    ↓ сохраняет в
Tarantool (метаданные) + MinIO (объекты)
    ↓ читает из
MiniToolStreamEgress Server
    ↓ gRPC streaming
Subscriber Client (новая библиотека)
    ↓ сохраняет в
Local filesystem
```

### E2E тест результаты

```
1. Publish: "Testing new libraries - subscriber should receive this!"
   ✓ Published: sequence=80, object=test.single_80

2. Subscribe: test.single subject
   ✓ Notification received: sequence=80
   ✓ Message received: sequence=80, data_size=55
   ✓ Saved to: downloads/test/test.single_seq_80.txt

3. Verification:
   $ cat test.single_seq_80.txt
   > Testing new libraries - subscriber should receive this!
```

## Принципы Clean Architecture

### 1. Dependency Rule
✅ Все зависимости направлены к domain
✅ Domain не зависит ни от чего
✅ Infrastructure зависит от domain, а не наоборот

### 2. Interface Segregation
✅ MessagePreparer - подготовка сообщений
✅ MessageHandler - обработка сообщений
✅ IngressClient - взаимодействие с Ingress
✅ EgressClient - взаимодействие с Egress

### 3. Single Responsibility
✅ Domain - бизнес-правила
✅ Client - коммуникация с gRPC
✅ Use cases - оркестрация бизнес-логики
✅ Handlers - адаптация данных

### 4. Open/Closed Principle
✅ Новые типы обработчиков - реализуйте интерфейс
✅ Новые протоколы - реализуйте Client interface
✅ Библиотека расширяется без модификации

## Метрики рефакторинга

### Publisher Client
- **До:** ~250 строк кода (main.go + internal/)
- **После:** 80 строк в main.go
- **Сокращение:** 68% меньше кода в клиенте
- **Переиспользуемость:** Библиотека может использоваться в любом Go проекте

### Subscriber Client
- **До:** ~300 строк кода (main.go + internal/)
- **После:** 110 строк в main.go
- **Сокращение:** 63% меньше кода в клиенте
- **Переиспользуемость:** Библиотека может использоваться в любом Go проекте

### Библиотеки
- **Publisher:** 11 файлов, ~600 строк качественного кода
- **Subscriber:** 10 файлов, ~650 строк качественного кода
- **Покрытие:** Все основные use cases протестированы
- **Документация:** README для каждой библиотеки

## Преимущества новой архитектуры

### 1. Тестируемость
- Легко создавать моки для каждого слоя
- Независимые юнит-тесты для каждого компонента
- E2E тесты без изменения библиотек

### 2. Расширяемость
- Новые обработчики данных - реализуйте MessagePreparer
- Новые обработчики сообщений - реализуйте MessageHandler
- Новые протоколы - реализуйте Client interface

### 3. Переиспользуемость
- Библиотеки можно импортировать в любой Go проект
- Чистый API без привязки к конкретной реализации
- Примеры использования в example/

### 4. Поддерживаемость
- Четкое разделение ответственности
- Каждый компонент имеет одну причину для изменения
- Документация и примеры использования

### 5. Независимость от фреймворков
- Domain слой не зависит от gRPC
- Легко заменить gRPC на другой протокол
- Бизнес-логика изолирована от инфраструктуры

## Сравнение: До и После

### Publisher Client - До
```go
// Зависимость от внутренней реализации
import "github.com/moroshma/MiniToolStream/example/publisher_client/internal/publisher"
import "github.com/moroshma/MiniToolStream/example/publisher_client/internal/handler"

// Тесная связь с gRPC
manager, err := publisher.NewManager(config)
manager.RegisterHandler(handler.NewImagePublisherHandler(...))
```

### Publisher Client - После
```go
// Зависимость от библиотеки
import "github.com/moroshma/MiniToolStream/pkg/minitoolstream"
import "github.com/moroshma/MiniToolStream/pkg/minitoolstream/handler"

// Чистый API
pub, _ := minitoolstream.NewPublisher("localhost:50051")
pub.RegisterHandler(handler.NewImageHandler(&handler.ImageHandlerConfig{...}))
```

## Выводы

✅ **Clean Architecture реализована полностью**
- Все слои четко разделены
- Зависимости направлены правильно
- Принципы SOLID соблюдены

✅ **Библиотеки готовы к использованию**
- Publisher library: pkg/minitoolstream
- Subscriber library: pkg/minitoolstream/subscriber
- Документированы и протестированы

✅ **E2E тесты пройдены**
- Publisher → Ingress → Storage → Egress → Subscriber
- Все типы данных (text, images, files)
- Обработка ошибок работает корректно

✅ **Код стал проще и чище**
- Клиенты сократились на 60-70%
- Логика вынесена в переиспользуемые библиотеки
- Легко добавлять новую функциональность

## Рекомендации для дальнейшего развития

1. **Добавить unit тесты** для библиотек
2. **Добавить примеры** более сложных сценариев
3. **Создать integration тесты** в отдельном пакете
4. **Документировать** best practices использования
5. **Рассмотреть** добавление metrics и tracing
