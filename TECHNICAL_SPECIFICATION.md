# Техническое задание: MiniToolStream

## Платформа для потоковой обработки данных

**Версия документа:** 1.0
**Дата:** 03.12.2025
**Статус:** Утвержден

---

## 1. Введение

### 1.1. Назначение документа

Настоящий документ описывает технические требования к разработке платформы **MiniToolStream** — системы для эффективной транспортировки и обработки больших потоков данных в режиме реального времени.

### 1.2. Область применения

MiniToolStream предназначен для использования в высоконагруженных распределенных системах, требующих:
- Надежной асинхронной передачи данных между микросервисами
- Синхронизации данных между различными подразделениями или системами
- Построения event-driven архитектур
- Обработки сообщений произвольного размера и формата

### 1.3. Проблематика

Существующие решения для обмена сообщениями (Apache Kafka, RabbitMQ, Redis Streams) имеют ограничения:
- **Kafka**: ограничения на размер сообщения (по умолчанию 1MB), сложность масштабирования
- **RabbitMQ**: низкая производительность при больших объемах данных
- **Redis**: хранение только в памяти, отсутствие персистентности для больших объектов

**Проблема:** Необходимость эффективной работы с сообщениями произвольного размера (от байтов до гигабайтов) при сохранении высокой производительности и надежности.

### 1.4. Цель проекта

Повышение эффективности работы очередей сообщений в высоконагруженных системах за счет гибридного подхода к хранению данных:
- **Метаданные** → быстрая in-memory СУБД (Tarantool)
- **Полезная нагрузка (payload)** → масштабируемое объектное хранилище (MinIO/S3)

### 1.5. Задачи

1. Разработать инструмент синхронизации данных между гетерогенными системами
2. Обеспечить возможность хранения сообщений любого размера и формата
3. Реализовать высокопроизводительную систему с минимальной задержкой
4. Обеспечить надежность и персистентность данных
5. Предоставить простой и удобный API для разработчиков

---

## 2. Ключевые преимущества

### 2.1. Технологические преимущества

| Аспект | MiniToolStream | Apache Kafka |
|--------|---------------|--------------|
| **Размер сообщений** | Без ограничений (до TB) | До 1MB (настраиваемо до 100MB) |
| **Тип данных** | Любой формат, бинарные объекты | Байтовые массивы |
| **Хранение** | Гибридное (Tarantool + S3) | Файловая система |
| **Масштабирование** | Независимое для метаданных и данных | Партиционирование |
| **Сложность эксплуатации** | Средняя | Высокая |

### 2.2. Бизнес-преимущества

- Унифицированное решение для любых типов данных (события, файлы, видео, логи)
- Снижение стоимости хранения за счет использования S3
- Упрощение архитектуры (не нужны отдельные системы для больших файлов)
- Гибкость в выборе стратегий хранения и очистки данных

---

## 3. Архитектура системы

### 3.1. Общая архитектура

```
┌──────────────────┐
│   Producers      │  (Publisher Clients)
│  (Клиенты)       │
└────────┬─────────┘
         │ gRPC
         ▼
┌─────────────────────────────────────────┐
│      MiniToolStream Platform            │
│                                          │
│  ┌────────────────────────────────────┐ │
│  │   MiniToolStreamIngress            │ │
│  │   (Точка входа - gRPC Server)     │ │
│  └──────────┬───────────┬─────────────┘ │
│             │           │                │
│             ▼           ▼                │
│      ┌──────────┐  ┌──────────┐        │
│      │Tarantool │  │  MinIO   │        │
│      │(metadata)│  │  (data)  │        │
│      └──────────┘  └──────────┘        │
│             ▲           ▲                │
│             │           │                │
│  ┌──────────┴───────────┴─────────────┐ │
│  │   MiniToolStreamEgress             │ │
│  │   (Точка выхода - gRPC Server)    │ │
│  └────────────────────────────────────┘ │
│                                          │
└───────────────┬──────────────────────────┘
                │ gRPC
                ▼
┌──────────────────┐
│   Consumers      │  (Subscriber Clients)
│  (Клиенты)       │
└──────────────────┘
```

### 3.2. Компоненты системы

#### 3.2.1. MiniToolStreamIngress (Точка входа)

**Назначение:** Прием и обработка входящих сообщений от производителей.

**Функции:**
- Прием gRPC запросов на публикацию сообщений
- Валидация входящих данных
- Генерация уникальных идентификаторов (sequence)
- Сохранение метаданных в Tarantool
- Загрузка полезной нагрузки в MinIO
- Возврат подтверждения публикации клиенту

**Технологии:**
- Язык: Go 1.21+
- Протокол: gRPC
- Порт: 50051 (настраиваемый)

**Масштабирование:** Горизонтальное (stateless сервис)

#### 3.2.2. MiniToolStreamEgress (Точка выхода)

**Назначение:** Предоставление доступа к опубликованным сообщениям потребителям.

**Функции:**
- Прием gRPC запросов на чтение сообщений
- Получение последнего доступного sequence по каналу
- Извлечение метаданных из Tarantool
- Загрузка данных из MinIO
- Управление позицией чтения для durable consumers
- Поддержка стриминга сообщений

**Технологии:**
- Язык: Go 1.21+
- Протокол: gRPC (unary + server streaming)
- Порт: 50052 (настраиваемый)

**Масштабирование:** Горизонтальное (stateless сервис)

#### 3.2.3. Tarantool (Хранилище метаданных)

**Назначение:** Высокопроизводительное хранение метаданных сообщений.

**Структура данных:**

**Space: message**
```lua
{
    sequence: unsigned,      -- Глобальный уникальный номер сообщения (PK)
    headers: map,            -- Заголовки сообщения (метаданные)
    object_name: string,     -- Ключ объекта в MinIO (subject_sequence)
    subject: string,         -- Название канала/топика
    create_at: unsigned      -- Timestamp создания (для TTL)
}
```

**Индексы:**
- PRIMARY: sequence (unique)
- SECONDARY: subject (non-unique)
- SECONDARY: subject_sequence (unique, composite)
- SECONDARY: create_at (non-unique, для TTL)

**Space: consumers**
```lua
{
    durable_name: string,    -- Имя durable consumer
    subject: string,         -- Подписанный канал
    last_sequence: unsigned  -- Последний прочитанный sequence
}
```

**Индексы:**
- PRIMARY: (durable_name, subject) (composite unique)
- SECONDARY: subject (non-unique)

**Параметры:**
- Версия: Tarantool 2.11+
- Движок: memtx (in-memory с WAL)
- Память: 1GB (настраиваемо)
- WAL: включен (write mode)
- Репликация: standalone (1 нода)

#### 3.2.4. MinIO (Хранилище данных)

**Назначение:** Масштабируемое S3-совместимое хранилище полезной нагрузки сообщений.

**Параметры:**
- Bucket: `minitoolstream`
- Naming convention: `{subject}_{sequence}`
- Access: через SDK (MinIO Go Client)
- Политика доступа: private (только через API)

**Версия:** MinIO Latest (S3-compatible)

#### 3.2.5. HashiCorp Vault (Управление секретами)

**Назначение:** Централизованное управление конфигурацией и секретами.

**Хранимые секреты:**
- Credentials Tarantool
- Credentials MinIO
- API keys для сервисов
- TLS сертификаты

**Режим:** Development (для dev), HA (для production)

#### 3.2.6. MiniToolStreamConnector (Клиентская библиотека)

**Назначение:** SDK для упрощения интеграции с платформой.

**Функции:**
- Publisher API (публикация сообщений)
- Subscriber API (чтение сообщений)
- Управление подключением
- Автоматический retry при ошибках
- Logging и monitoring

**Архитектура:** Clean Architecture
```
minitoolstream_connector/
├── publisher.go          # Publisher API
├── subscriber.go         # Subscriber API
├── infrastructure/       # Реализации
│   ├── grpc_client/     # gRPC клиент
│   └── handler/         # Обработчики разных типов данных
└── domain/              # Domain entities
```

---

## 4. Поток данных (Data Flow)

### 4.1. Публикация сообщений (Publish Flow)

```
1. Producer ──[PublishRequest]──> MiniToolStreamIngress
                                         │
2. Validate request (subject not empty)  │
                                         │
3. Generate sequence number             ↓
   sequence = get_next_sequence()   [Tarantool]
                                         │
4. Save to MinIO                        │
   object_key = "{subject}_{sequence}"   │
   MinIO.Put(object_key, data)      ↓
                                   [MinIO]
5. Save metadata to Tarantool           │
   publish_message(subject, headers) ←──┘
                                         │
6. Response ←──[PublishResponse]────────┘
   {sequence, object_name, status}
```

**Детали:**
1. **Валидация:** Проверка обязательных полей (subject)
2. **Sequence генерация:** Атомарный инкремент глобального счетчика
3. **Сохранение в MinIO:**
   - Формат ключа: `{subject}_{sequence}`
   - Content-Type берется из headers
   - Размер данных добавляется в headers (`data-size`)
4. **Сохранение в Tarantool:** Транзакционная вставка метаданных
5. **Ответ:** Возврат sequence и object_name клиенту

**Гарантии:**
- **At-least-once delivery**: Сообщение гарантированно сохранено
- **Ordering**: Глобальный порядок через sequence
- **Durability**: WAL в Tarantool + репликация MinIO

### 4.2. Потребление сообщений (Subscribe Flow)

**Модель:** Pull-based (потребитель запрашивает сообщения)

```
1. Consumer ──[GetLatestSequence]──> MiniToolStreamEgress
                                            │
2. Query Tarantool                         │
   latest = get_latest_sequence(subject)   │
                                           ↓
3. Response ←──[LatestSequenceResponse]───┘
   {latest_sequence}

4. Consumer compares with local position
   new_messages = latest - consumer_position

5. Consumer ──[FetchRequest]──> MiniToolStreamEgress
   {subject, start_sequence, limit}
                                            │
6. Fetch metadata from Tarantool          │
   messages = get_messages_by_subject()    │
                                           ↓
7. For each message:                  [Tarantool]
   - Get metadata (object_name)            │
   - Fetch data from MinIO                 │
   - object = MinIO.Get(object_name)       │
                                           ↓
8. Stream messages ←────────────────  [MinIO]
   [MessageResponse] (multiple)

9. Update consumer position
   update_consumer_position(durable, subject, last_seq)
```

**Режимы потребления:**

**A. Durable Consumer (с сохранением позиции)**
```go
Subscribe(subject, durable_name) → stream of messages
```
- Позиция читается из Tarantool (space: consumers)
- Автоматически обновляется после обработки
- Позволяет продолжить с последнего прочитанного

**B. Ephemeral Consumer (без сохранения)**
```go
Fetch(subject, start_sequence, limit) → batch of messages
```
- Клиент сам управляет позицией
- Нет записи в Tarantool
- Используется для одноразовых запросов

### 4.3. Очистка данных (TTL Cleanup)

**Архитектура:** Распределенная (Tarantool Fiber + MinIO Lifecycle Policies)

```
┌─────────────────────────────────────────────────────────────┐
│              TTL Cleanup Architecture                        │
└─────────────────────────────────────────────────────────────┘

Component 1: MinIO Lifecycle Policies (автоматическое удаление объектов)
─────────────────────────────────────────────────────────────
1. Setup (при запуске Ingress)
   ├── Read TTL config (default + per-channel)
   ├── Create lifecycle rules:
   │   ├── Default rule: expiration after {default} days
   │   └── Channel rules: expiration for {channel}_* prefix
   └── Apply to MinIO bucket

2. Runtime (автоматически MinIO)
   ├── Periodic scan bucket objects
   ├── Check object age vs lifecycle rules
   └── Delete expired objects

Component 2: Tarantool Background Fiber (очистка метаданных)
─────────────────────────────────────────────────────────────
1. Startup (при запуске Tarantool)
   ├── Configure TTL via configure_ttl()
   │   ├── enabled: true/false
   │   ├── default_ttl: seconds
   │   ├── check_interval: seconds
   │   └── channels: map[channel]ttl
   └── Start fiber via start_ttl_cleanup()

2. Background Fiber Loop
   ├── Sleep for check_interval
   ├── Group messages by subject
   ├── For each subject:
   │   ├── Get subject-specific TTL
   │   ├── Calculate cutoff_time = now - ttl
   │   ├── Delete old messages (create_at < cutoff_time)
   │   └── Log deletion count
   └── Repeat

3. Management Functions
   ├── configure_ttl(config) - update configuration
   ├── start_ttl_cleanup() - start background fiber
   ├── stop_ttl_cleanup() - stop fiber gracefully
   └── get_ttl_status() - query current status
```

**Конфигурация (config.yaml):**
```yaml
ttl:
  enabled: true
  default: 24h              # Глобальный TTL по умолчанию
  channels:                 # Per-channel TTL overrides
    - channel: "logs"
      duration: 7d          # Логи: 7 дней
    - channel: "metrics"
      duration: 30d         # Метрики: 30 дней
    - channel: "events"
      duration: 90d         # События: 90 дней
```

**Преимущества распределенной архитектуры:**
- **Независимость:** MinIO и Tarantool работают автономно
- **Эффективность:** Нет централизованного сервиса с overhead
- **Надежность:** Автоматическое восстановление после перезапуска
- **Гибкость:** Per-channel TTL настройка
- **Простота:** Нативные механизмы MinIO и Tarantool

**Tarantool Fiber Functions:**
- `configure_ttl(config)` — конфигурация TTL параметров
- `start_ttl_cleanup()` — запуск background fiber
- `stop_ttl_cleanup()` — остановка fiber
- `get_ttl_status()` — получение статуса TTL

**MinIO Lifecycle:**
- Правила настраиваются через `SetupTTLPolicies()` в Ingress
- Формат ID: `channel-{name}-ttl` или `default-ttl`
- Префикс фильтр: `{channel}_*` для channel-specific rules

---

## 5. API спецификация

### 5.1. Ingress gRPC API

**Service:** `IngressService`

#### 5.1.1. Publish RPC

Публикация одного сообщения.

**Request:**
```protobuf
message PublishRequest {
  string subject = 1;                    // Название канала (обязательно)
  bytes data = 2;                        // Полезная нагрузка (опционально, может быть пустым)
  map<string, string> headers = 3;       // Метаданные сообщения
}
```

**Response:**
```protobuf
message PublishResponse {
  uint64 sequence = 1;                   // Уникальный номер сообщения
  string object_name = 2;                // Ключ объекта в MinIO
  int64 status_code = 3;                 // 0 = success, != 0 = error
  string error_message = 4;              // Описание ошибки (если есть)
}
```

**Примеры:**
```bash
# Успешная публикация
Request: {subject: "orders.created", data: "order_123", headers: {"content-type": "text/plain"}}
Response: {sequence: 12345, object_name: "orders.created_12345", status_code: 0}

# Ошибка валидации
Request: {subject: "", data: "test"}
Response: {status_code: 1, error_message: "subject cannot be empty"}
```

#### 5.1.2. PublishBatch RPC (будущая функциональность)

Публикация пакета сообщений за один запрос.

**Request:**
```protobuf
message PublishBatchRequest {
  repeated PublishRequest messages = 1;
}
```

**Response:**
```protobuf
message PublishBatchResponse {
  repeated PublishResponse results = 1;
  int64 total_count = 2;
  int64 success_count = 3;
  int64 error_count = 4;
}
```

### 5.2. Egress gRPC API

**Service:** `EgressService`

#### 5.2.1. GetLatestSequence RPC

Получение последнего доступного sequence для канала.

**Request:**
```protobuf
message GetLatestSequenceRequest {
  string subject = 1;                    // Название канала
}
```

**Response:**
```protobuf
message GetLatestSequenceResponse {
  uint64 latest_sequence = 1;            // Последний sequence (0 если нет сообщений)
  int64 status_code = 2;
  string error_message = 3;
}
```

#### 5.2.2. Fetch RPC

Получение пакета сообщений по sequence.

**Request:**
```protobuf
message FetchRequest {
  string subject = 1;                    // Название канала
  uint64 start_sequence = 2;             // Начальный sequence (включительно)
  int32 limit = 3;                       // Максимальное количество сообщений
}
```

**Response:**
```protobuf
message FetchResponse {
  repeated Message messages = 1;
  int64 status_code = 2;
  string error_message = 3;
}

message Message {
  uint64 sequence = 1;
  string subject = 2;
  bytes data = 3;
  map<string, string> headers = 4;
  uint64 create_at = 5;
}
```

#### 5.2.3. Subscribe RPC (Server Streaming)

Подписка на канал с автоматическим получением новых сообщений.

**Request:**
```protobuf
message SubscribeRequest {
  string subject = 1;                    // Название канала
  string durable_name = 2;               // Имя durable consumer (опционально)
  uint64 start_sequence = 3;             // Начальная позиция (опционально)
  int32 batch_size = 4;                  // Размер пакета (по умолчанию 10)
}
```

**Response:** (stream)
```protobuf
message SubscribeResponse {
  oneof response_type {
    MessageBatch batch = 1;              // Пакет сообщений
    Notification notification = 2;        // Уведомление о новых сообщениях
    Error error = 3;                     // Ошибка
  }
}

message MessageBatch {
  repeated Message messages = 1;
}

message Notification {
  uint64 latest_sequence = 1;            // Последний доступный sequence
  int32 new_messages_count = 2;          // Количество новых сообщений
}
```

**Логика работы:**
1. Клиент отправляет SubscribeRequest
2. Сервер возвращает начальный пакет сообщений
3. Сервер периодически проверяет наличие новых сообщений
4. При появлении новых - отправляет Notification
5. Клиент запрашивает следующий пакет
6. Цикл повторяется до закрытия stream

#### 5.2.4. UpdateConsumerPosition RPC

Обновление позиции durable consumer.

**Request:**
```protobuf
message UpdateConsumerPositionRequest {
  string durable_name = 1;
  string subject = 2;
  uint64 last_sequence = 3;
}
```

**Response:**
```protobuf
message UpdateConsumerPositionResponse {
  int64 status_code = 1;
  string error_message = 2;
}
```

### 5.3. Клиентская библиотека API (Go SDK)

**Библиотека:** `github.com/moroshma/MiniToolStream/MiniToolStreamConnector`

**Архитектура:** Clean Architecture с разделением на слои:
- `domain/` — интерфейсы и доменные типы
- `infrastructure/` — реализации (gRPC, handlers)
- `publisher.go` / `subscriber.go` — публичные API

#### 5.3.1. Publisher API

**Создание Publisher:**

```go
// Базовый конструктор
func NewPublisher(serverAddr string, opts ...grpc.DialOption) (Publisher, error)

// Параметры:
// - serverAddr: адрес Ingress сервера (например, "localhost:50051")
// - opts: дополнительные gRPC опции (TLS, interceptors и т.д.)
//
// Возвращает: Publisher интерфейс и ошибку

// Пример использования:
publisher, err := minitoolstream.NewPublisher("localhost:50051")
if err != nil {
    log.Fatal(err)
}
defer publisher.Close()
```

**Publisher Builder Pattern:**

```go
// Fluent API для создания Publisher с дополнительными параметрами
type PublisherBuilder struct {
    serverAddr string
    timeout    time.Duration
    logger     *zap.Logger
    grpcOpts   []grpc.DialOption
}

// Методы Builder:
func NewPublisherBuilder(serverAddr string) *PublisherBuilder
func (b *PublisherBuilder) WithTimeout(timeout time.Duration) *PublisherBuilder
func (b *PublisherBuilder) WithLogger(logger *zap.Logger) *PublisherBuilder
func (b *PublisherBuilder) WithGRPCOptions(opts ...grpc.DialOption) *PublisherBuilder
func (b *PublisherBuilder) Build() (Publisher, error)

// Пример использования:
publisher, err := minitoolstream.NewPublisherBuilder("localhost:50051").
    WithTimeout(10 * time.Second).
    WithLogger(logger).
    WithGRPCOptions(grpc.WithInsecure()).
    Build()
```

**Publisher Interface:**

```go
type Publisher interface {
    // Publish - публикация сообщения через MessagePreparer
    // MessagePreparer позволяет использовать разные типы данных
    Publish(ctx context.Context, preparer MessagePreparer) error

    // PublishAll - пакетная публикация сообщений
    PublishAll(ctx context.Context, preparers []MessagePreparer) error

    // RegisterHandler - регистрация обработчика для определенного типа
    RegisterHandler(preparer MessagePreparer)

    // SetResultHandler - установка обработчика результатов публикации
    SetResultHandler(handler ResultHandler)

    // Close - закрытие соединения
    Close() error
}
```

**MessagePreparer Interface:**

```go
// MessagePreparer - интерфейс для подготовки сообщений разных типов
type MessagePreparer interface {
    // Prepare - подготовка protobuf сообщения для отправки
    Prepare() (*pb.PublishRequest, error)
}

// Реализации MessagePreparer:
// 1. ByteMessagePreparer - для произвольных байтов
// 2. StringMessagePreparer - для текстовых данных
// 3. FileMessagePreparer - для файлов
// 4. ImageMessagePreparer - для изображений
// 5. JSONMessagePreparer - для JSON объектов
```

**Примеры публикации:**

```go
// 1. Публикация байтов
bytesPreparer := &domain.ByteMessagePreparer{
    Subject: "events.created",
    Data:    []byte("event data"),
    Headers: map[string]string{
        "content-type": "application/octet-stream",
        "source":       "api-service",
    },
}
err := publisher.Publish(ctx, bytesPreparer)

// 2. Публикация строки
stringPreparer := &domain.StringMessagePreparer{
    Subject: "logs.application",
    Message: "Application started successfully",
    Headers: map[string]string{
        "level": "info",
        "app":   "myapp",
    },
}
err := publisher.Publish(ctx, stringPreparer)

// 3. Публикация файла
filePreparer := &domain.FileMessagePreparer{
    Subject:  "documents.pdf",
    FilePath: "/path/to/document.pdf",
    Headers:  map[string]string{"author": "John Doe"},
}
err := publisher.Publish(ctx, filePreparer)

// 4. Публикация изображения
imagePreparer := &domain.ImageMessagePreparer{
    Subject:  "images.uploads",
    FilePath: "/path/to/image.jpg",
    Headers:  map[string]string{"resolution": "1920x1080"},
}
err := publisher.Publish(ctx, imagePreparer)

// 5. Публикация JSON
type Order struct {
    ID     string  `json:"id"`
    Amount float64 `json:"amount"`
}
jsonPreparer := &domain.JSONMessagePreparer{
    Subject: "orders.created",
    Data:    &Order{ID: "123", Amount: 99.99},
    Headers: map[string]string{"version": "v1"},
}
err := publisher.Publish(ctx, jsonPreparer)

// 6. Пакетная публикация
preparers := []domain.MessagePreparer{
    bytesPreparer,
    stringPreparer,
    filePreparer,
}
err := publisher.PublishAll(ctx, preparers)
```

**ResultHandler:**

```go
// ResultHandler - callback для обработки результатов публикации
type ResultHandler interface {
    OnSuccess(sequence uint64, objectName string)
    OnError(err error)
}

// Пример использования:
type MyResultHandler struct{}

func (h *MyResultHandler) OnSuccess(sequence uint64, objectName string) {
    log.Printf("Published successfully: seq=%d, object=%s", sequence, objectName)
}

func (h *MyResultHandler) OnError(err error) {
    log.Printf("Publish failed: %v", err)
}

publisher.SetResultHandler(&MyResultHandler{})
```

#### 5.3.2. Subscriber API

**Создание Subscriber:**

```go
// Базовый конструктор
func NewSubscriber(serverAddr string, durableName string, opts ...grpc.DialOption) (Subscriber, error)

// Параметры:
// - serverAddr: адрес Egress сервера (например, "localhost:50052")
// - durableName: имя durable consumer (для сохранения позиции)
// - opts: дополнительные gRPC опции
//
// Возвращает: Subscriber интерфейс и ошибку

// Пример использования:
subscriber, err := minitoolstream.NewSubscriber(
    "localhost:50052",
    "my-consumer-group",
)
if err != nil {
    log.Fatal(err)
}
defer subscriber.Stop()
```

**Subscriber Builder Pattern:**

```go
// Fluent API для создания Subscriber с дополнительными параметрами
type SubscriberBuilder struct {
    serverAddr   string
    durableName  string
    batchSize    int32
    pollInterval time.Duration
    logger       *zap.Logger
    grpcOpts     []grpc.DialOption
}

// Методы Builder:
func NewSubscriberBuilder(serverAddr, durableName string) *SubscriberBuilder
func (b *SubscriberBuilder) WithBatchSize(size int32) *SubscriberBuilder
func (b *SubscriberBuilder) WithPollInterval(interval time.Duration) *SubscriberBuilder
func (b *SubscriberBuilder) WithLogger(logger *zap.Logger) *SubscriberBuilder
func (b *SubscriberBuilder) WithGRPCOptions(opts ...grpc.DialOption) *SubscriberBuilder
func (b *SubscriberBuilder) Build() (Subscriber, error)

// Пример использования:
subscriber, err := minitoolstream.NewSubscriberBuilder(
    "localhost:50052",
    "my-consumer-group",
).
    WithBatchSize(50).
    WithPollInterval(1 * time.Second).
    WithLogger(logger).
    Build()
```

**Subscriber Interface:**

```go
type Subscriber interface {
    // RegisterHandler - регистрация обработчика для конкретного канала
    RegisterHandler(subject string, handler MessageHandler)

    // RegisterHandlers - регистрация нескольких обработчиков сразу
    RegisterHandlers(handlers map[string]MessageHandler)

    // Start - запуск subscriber (начинает получение сообщений)
    Start() error

    // Stop - остановка subscriber (graceful shutdown)
    Stop()

    // Wait - ожидание завершения работы
    Wait()
}
```

**MessageHandler Interface:**

```go
// MessageHandler - интерфейс для обработки полученных сообщений
type MessageHandler interface {
    Handle(msg *Message) error
}

// Message - структура полученного сообщения
type Message struct {
    Sequence  uint64            // Уникальный номер сообщения
    Subject   string            // Канал
    Data      []byte            // Полезная нагрузка
    Headers   map[string]string // Метаданные
    CreatedAt uint64            // Unix timestamp создания
}
```

**Примеры использования Subscriber:**

```go
// 1. Простой обработчик с функцией
type LogHandler struct{}

func (h *LogHandler) Handle(msg *domain.Message) error {
    log.Printf("Received message: seq=%d, subject=%s, size=%d bytes",
        msg.Sequence, msg.Subject, len(msg.Data))
    return nil
}

subscriber.RegisterHandler("logs.application", &LogHandler{})

// 2. Обработчик с сохранением в БД
type OrderHandler struct {
    db *sql.DB
}

func (h *OrderHandler) Handle(msg *domain.Message) error {
    var order Order
    if err := json.Unmarshal(msg.Data, &order); err != nil {
        return err
    }

    _, err := h.db.Exec("INSERT INTO orders VALUES (?, ?)", order.ID, order.Amount)
    return err
}

subscriber.RegisterHandler("orders.created", &OrderHandler{db: db})

// 3. Множественная регистрация
handlers := map[string]domain.MessageHandler{
    "logs.application": &LogHandler{},
    "orders.created":   &OrderHandler{db: db},
    "events.system":    &EventHandler{},
}
subscriber.RegisterHandlers(handlers)

// 4. Запуск и управление жизненным циклом
// Запуск в фоне
go func() {
    if err := subscriber.Start(); err != nil {
        log.Fatalf("Subscriber failed: %v", err)
    }
}()

// Graceful shutdown при получении сигнала
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
<-sigChan

log.Println("Shutting down subscriber...")
subscriber.Stop()
subscriber.Wait()
log.Println("Subscriber stopped")
```

#### 5.3.3. Дополнительные типы и утилиты

**Domain Types:**

```go
// PublishResult - результат публикации
type PublishResult struct {
    Sequence   uint64
    ObjectName string
    Error      error
}

// SubscriptionStatus - статус подписки
type SubscriptionStatus struct {
    Subject         string
    CurrentSequence uint64
    LatestSequence  uint64
    MessagesLag     int64
}
```

**Infrastructure Handlers:**

```go
// Файл: infrastructure/handler/byte_handler.go
type ByteMessageHandler struct {
    client pb.IngressServiceClient
}

// Файл: infrastructure/handler/file_handler.go
type FileMessageHandler struct {
    client pb.IngressServiceClient
}

// Файл: infrastructure/handler/image_handler.go
type ImageMessageHandler struct {
    client pb.IngressServiceClient
}

// Каждый handler реализует логику подготовки и отправки
// конкретного типа данных через gRPC
```

#### 5.3.4. Примеры полной интеграции

**Publisher пример:**

```go
package main

import (
    "context"
    "log"

    connector "github.com/moroshma/MiniToolStream/MiniToolStreamConnector/minitoolstream_connector"
    "github.com/moroshma/MiniToolStream/MiniToolStreamConnector/minitoolstream_connector/domain"
)

func main() {
    // Создание publisher
    publisher, err := connector.NewPublisher("localhost:50051")
    if err != nil {
        log.Fatal(err)
    }
    defer publisher.Close()

    // Установка обработчика результатов
    publisher.SetResultHandler(&MyResultHandler{})

    // Публикация разных типов данных
    ctx := context.Background()

    // Байты
    err = publisher.Publish(ctx, &domain.ByteMessagePreparer{
        Subject: "events.user.created",
        Data:    []byte(`{"user_id": "123"}`),
        Headers: map[string]string{"content-type": "application/json"},
    })

    // Файл
    err = publisher.Publish(ctx, &domain.FileMessagePreparer{
        Subject:  "documents.contracts",
        FilePath: "./contract.pdf",
        Headers:  map[string]string{"client": "ACME Corp"},
    })

    log.Println("All messages published successfully")
}

type MyResultHandler struct{}

func (h *MyResultHandler) OnSuccess(sequence uint64, objectName string) {
    log.Printf("✓ Published: seq=%d, object=%s", sequence, objectName)
}

func (h *MyResultHandler) OnError(err error) {
    log.Printf("✗ Error: %v", err)
}
```

**Subscriber пример:**

```go
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"

    connector "github.com/moroshma/MiniToolStream/MiniToolStreamConnector/minitoolstream_connector"
    "github.com/moroshma/MiniToolStream/MiniToolStreamConnector/minitoolstream_connector/domain"
)

func main() {
    // Создание subscriber
    subscriber, err := connector.NewSubscriber(
        "localhost:50052",
        "my-service-consumer",
    )
    if err != nil {
        log.Fatal(err)
    }

    // Регистрация обработчиков
    subscriber.RegisterHandlers(map[string]domain.MessageHandler{
        "events.user.created":   &UserEventHandler{},
        "documents.contracts":   &DocumentHandler{},
        "logs.application":      &LogHandler{},
    })

    // Запуск в фоне
    go func() {
        if err := subscriber.Start(); err != nil {
            log.Fatalf("Subscriber error: %v", err)
        }
    }()

    log.Println("Subscriber started, waiting for messages...")

    // Graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    <-sigChan

    log.Println("Shutting down...")
    subscriber.Stop()
    subscriber.Wait()
    log.Println("Shutdown complete")
}

// Обработчики
type UserEventHandler struct{}
func (h *UserEventHandler) Handle(msg *domain.Message) error {
    log.Printf("User event: %s", string(msg.Data))
    return nil
}

type DocumentHandler struct{}
func (h *DocumentHandler) Handle(msg *domain.Message) error {
    log.Printf("Document received: %d bytes", len(msg.Data))
    // Сохранение файла
    return nil
}

type LogHandler struct{}
func (h *LogHandler) Handle(msg *domain.Message) error {
    log.Printf("Log: %s", string(msg.Data))
    return nil
}
```

### 5.4. Tarantool Functions API

**Файл:** `tarantool/init.lua`

Tarantool предоставляет набор Lua функций для работы с метаданными сообщений, управления consumer positions и TTL cleanup.

#### 5.4.1. Sequence Management

**get_next_sequence()**
```lua
function get_next_sequence()
-- Возвращает следующий глобальный sequence number
-- Атомарная инкрементация in-memory счетчика
--
-- Returns: uint64 - следующий sequence
```

#### 5.4.2. Message Operations

**publish_message(subject, headers)**
```lua
function publish_message(subject, headers)
-- Публикация нового сообщения
--
-- Параметры:
--   subject: string - название канала
--   headers: table - map заголовков (может быть пустым)
--
-- Возвращает: uint64 - sequence number опубликованного сообщения
--
-- Действия:
--   1. Генерация sequence через get_next_sequence()
--   2. Создание object_name = "{subject}_{sequence}"
--   3. Запись в space 'message'
```

**get_message_by_sequence(sequence)**
```lua
function get_message_by_sequence(sequence)
-- Получение сообщения по sequence number
--
-- Параметры:
--   sequence: uint64 - номер сообщения
--
-- Возвращает: tuple или nil
--   [sequence, headers, object_name, subject, create_at]
```

**get_message_by_sequence_decoded(sequence)**
```lua
function get_message_by_sequence_decoded(sequence)
-- Получение сообщения в виде таблицы с именованными полями
--
-- Параметры:
--   sequence: uint64 - номер сообщения
--
-- Возвращает: table или nil
--   {
--     sequence: uint64,
--     headers: map,
--     object_name: string,
--     subject: string,
--     create_at: uint64
--   }
```

**get_messages_by_subject(subject, start_sequence, limit)**
```lua
function get_messages_by_subject(subject, start_sequence, limit)
-- Получение пакета сообщений по каналу
--
-- Параметры:
--   subject: string - название канала
--   start_sequence: uint64 - начальный sequence (включительно)
--   limit: number - максимальное количество сообщений
--
-- Возвращает: array of tuples
--   Использует индекс 'subject_sequence' для эффективного поиска
```

**get_latest_sequence_for_subject(subject)**
```lua
function get_latest_sequence_for_subject(subject)
-- Получение последнего sequence для канала
--
-- Параметры:
--   subject: string - название канала
--
-- Возвращает: uint64 - последний sequence или 0 если нет сообщений
```

**get_subject_message_count(subject)**
```lua
function get_subject_message_count(subject)
-- Подсчет количества сообщений в канале
--
-- Параметры:
--   subject: string - название канала
--
-- Возвращает: uint64 - количество сообщений
```

#### 5.4.3. Consumer Management

**update_consumer_position(durable_name, subject, last_sequence)**
```lua
function update_consumer_position(durable_name, subject, last_sequence)
-- Обновление позиции durable consumer
--
-- Параметры:
--   durable_name: string - имя consumer group
--   subject: string - название канала
--   last_sequence: uint64 - последний прочитанный sequence
--
-- Возвращает: bool - true
--
-- Действия:
--   - Если позиция не существует - создает (INSERT)
--   - Если существует - обновляет (UPDATE)
```

**get_consumer_position(durable_name, subject)**
```lua
function get_consumer_position(durable_name, subject)
-- Получение текущей позиции consumer
--
-- Параметры:
--   durable_name: string - имя consumer group
--   subject: string - название канала
--
-- Возвращает: uint64 - last_sequence или 0 если не найдено
```

**get_consumers_by_subject(subject)**
```lua
function get_consumers_by_subject(subject)
-- Получение всех consumers подписанных на канал
--
-- Параметры:
--   subject: string - название канала
--
-- Возвращает: array of tuples
--   [{durable_name, subject, last_sequence}, ...]
```

#### 5.4.4. TTL Management Functions

**configure_ttl(config)**
```lua
function configure_ttl(config)
-- Конфигурация TTL параметров
--
-- Параметры:
--   config: table
--     {
--       enabled: bool,              -- включить/выключить TTL
--       default_ttl: number,        -- TTL по умолчанию (секунды)
--       check_interval: number,     -- интервал проверки (секунды)
--       channels: table             -- map[channel_name] = ttl_seconds
--     }
--
-- Возвращает: bool - true
--
-- Действия:
--   1. Обновляет глобальную конфигурацию ttl_config
--   2. Перезапускает TTL fiber если enabled=true
--   3. Останавливает fiber если enabled=false
--
-- Пример:
--   configure_ttl({
--     enabled = true,
--     default_ttl = 86400,  -- 24 часа
--     check_interval = 3600, -- 1 час
--     channels = {
--       ["logs"] = 604800,     -- 7 дней
--       ["metrics"] = 2592000  -- 30 дней
--     }
--   })
```

**start_ttl_cleanup()**
```lua
function start_ttl_cleanup()
-- Запуск background fiber для TTL cleanup
--
-- Возвращает: bool
--   true - fiber запущен успешно
--   false - TTL отключен или fiber уже работает
--
-- Действия:
--   1. Проверяет что TTL включен
--   2. Создает новый fiber если не запущен
--   3. Fiber периодически сканирует и удаляет старые записи
```

**stop_ttl_cleanup()**
```lua
function stop_ttl_cleanup()
-- Остановка background fiber
--
-- Возвращает: bool - true
--
-- Действия:
--   1. Устанавливает ttl_config.enabled = false
--   2. Ждет завершения fiber (max 5 секунд)
--   3. Если не завершился - force cancel
```

**get_ttl_status()**
```lua
function get_ttl_status()
-- Получение текущего статуса TTL
--
-- Возвращает: table
--   {
--     enabled: bool,              -- TTL включен
--     default_ttl: number,        -- TTL по умолчанию
--     check_interval: number,     -- интервал проверки
--     fiber_running: bool,        -- fiber работает
--     channels: table             -- per-channel TTL
--   }
```

**delete_old_messages(ttl_seconds)**
```lua
function delete_old_messages(ttl_seconds)
-- Удаление старых сообщений (legacy, используется только для manual cleanup)
--
-- Параметры:
--   ttl_seconds: number - время жизни в секундах
--
-- Возвращает: deleted_count, deleted_messages
--   deleted_count: number - количество удаленных
--   deleted_messages: array - массив удаленных сообщений
--     [{sequence, subject, object_name}, ...]
--
-- Примечание:
--   В production используется background fiber через start_ttl_cleanup()
```

#### 5.4.5. Monitoring and Statistics

**get_new_messages_count(subject, durable_name, since_sequence)**
```lua
function get_new_messages_count(subject, durable_name, since_sequence)
-- Подсчет новых сообщений для consumer
--
-- Параметры:
--   subject: string - название канала
--   durable_name: string (опционально) - имя consumer
--   since_sequence: uint64 (опционально) - позиция
--
-- Возвращает: uint64 - количество новых сообщений
--
-- Логика:
--   - Если указан durable_name - использует позицию из БД
--   - Если указан since_sequence - использует его
--   - Иначе - считает все сообщения
```

**check_new_messages(subject, durable_name)**
```lua
function check_new_messages(subject, durable_name)
-- Проверка наличия новых сообщений для consumer
--
-- Параметры:
--   subject: string - название канала
--   durable_name: string - имя consumer
--
-- Возвращает: table
--   {
--     has_new: bool,              -- есть ли новые
--     latest_sequence: uint64,    -- последний sequence
--     consumer_position: uint64,  -- позиция consumer
--     new_count: number           -- количество новых
--   }
--
-- Используется для Subscribe notifications
```

#### 5.4.6. TTL Background Fiber (Internal)

**ttl_cleanup_fiber()**
```lua
local function ttl_cleanup_fiber()
-- Background корутина для автоматической очистки
--
-- Логика работы:
--   1. Бесконечный цикл пока ttl_config.enabled = true
--   2. Sleep на check_interval секунд
--   3. Сканирование всех сообщений
--   4. Группировка по subject
--   5. Для каждого subject:
--      a. Получение TTL (per-channel или default)
--      b. Вычисление cutoff_time = now - ttl
--      c. Удаление сообщений где create_at < cutoff_time
--   6. Логирование количества удаленных
--   7. Повтор цикла
--
-- Запуск: автоматически через start_ttl_cleanup()
-- Остановка: через stop_ttl_cleanup()
```

**get_subject_ttl(subject)**
```lua
local function get_subject_ttl(subject)
-- Получение TTL для конкретного subject (internal helper)
--
-- Параметры:
--   subject: string - название канала
--
-- Возвращает: number - TTL в секундах
--
-- Логика:
--   1. Если есть в ttl_config.channels[subject] - возвращает его
--   2. Иначе возвращает ttl_config.default_ttl
```

#### 5.4.7. Инициализация

**init_global_sequence()**
```lua
local function init_global_sequence()
-- Инициализация глобального sequence счетчика
--
-- Вызывается: Автоматически при старте Tarantool
--
-- Действия:
--   1. Находит максимальный sequence в space 'message'
--   2. Устанавливает global_sequence = max_seq
--   3. Логирует значение
--
-- Обеспечивает: Восстановление sequence после перезапуска
```

### 5.5. MinIO Repository Functions

**Файл:** `MiniToolStreamIngress/internal/repository/minio/client.go`

#### SetupTTLPolicies(ctx, ttlConfig)

```go
func (r *Repository) SetupTTLPolicies(ctx context.Context, ttlConfig config.TTLConfig) error
// Настройка MinIO lifecycle policies для автоматического удаления объектов
//
// Параметры:
//   ctx: context.Context - контекст выполнения
//   ttlConfig: config.TTLConfig - конфигурация TTL
//
// Возвращает: error или nil
//
// Действия:
//   1. Создает lifecycle.Configuration
//   2. Добавляет default rule (expiration после X дней)
//   3. Для каждого channel из config.Channels:
//      a. Создает rule с ID "channel-{name}-ttl"
//      b. Устанавливает prefix filter "{channel}_"
//      c. Устанавливает expiration days
//   4. Применяет конфигурацию к bucket через SetBucketLifecycle
//
// Пример:
//   ttlConfig := config.TTLConfig{
//     Enabled: true,
//     Default: 24 * time.Hour,
//     Channels: []config.ChannelTTLConfig{
//       {Channel: "logs", Duration: 7 * 24 * time.Hour},
//       {Channel: "metrics", Duration: 30 * 24 * time.Hour},
//     },
//   }
//   err := repo.SetupTTLPolicies(ctx, ttlConfig)
```

### 5.6. Tarantool Repository Functions

**Файл:** `MiniToolStreamIngress/internal/repository/tarantool/client.go`

#### StartTTLCleanup(ttlConfig)

```go
func (r *Repository) StartTTLCleanup(ttlConfig config.TTLConfig) error
// Запуск Tarantool background fiber для TTL cleanup
//
// Параметры:
//   ttlConfig: config.TTLConfig - конфигурация TTL
//
// Возвращает: error или nil
//
// Действия:
//   1. Преобразует config.TTLConfig в Lua table
//   2. Создает map для per-channel TTL
//   3. Вызывает Tarantool функцию configure_ttl()
//   4. Логирует результат
//
// Вызывается: При старте Ingress service
```

#### GetTTLStatus()

```go
func (r *Repository) GetTTLStatus() (map[string]interface{}, error)
// Получение статуса TTL от Tarantool
//
// Возвращает: map с полями:
//   - enabled: bool
//   - default_ttl: number
//   - check_interval: number
//   - fiber_running: bool
//   - channels: map[string]int
//
// Используется: Для мониторинга и debugging
```

---

## 6. Функциональные требования

| ID | Требование | Описание | Приоритет |
|----|-----------|----------|-----------|
| **ФТ-1** | Прием сообщений | Система должна принимать сообщения от производителей через gRPC API | Критический |
| **ФТ-2** | Доставка сообщений | Система должна предоставлять API для чтения сообщений по sequence | Критический |
| **ФТ-3** | Поддержка каналов (Topics) | Система должна поддерживать множество независимых каналов | Критический |
| **ФТ-4** | Множество потребителей | Один канал может читаться неограниченным числом потребителей | Критический |
| **ФТ-5** | Durable consumers | Поддержка сохранения позиции чтения для consumer groups | Высокий |
| **ФТ-6** | Управление жизненным циклом | Автоматическое удаление устаревших данных по TTL | Высокий |
| **ФТ-7** | Клиентская библиотека | SDK на Go для упрощения интеграции | Высокий |
| **ФТ-8** | Гибкость формата данных | Поддержка любых форматов данных без ограничений | Критический |
| **ФТ-9** | Batch публикация | Возможность публиковать несколько сообщений за один запрос | Средний |
| **ФТ-10** | Streaming подписка | Server-side streaming для real-time доставки | Высокий |
| **ФТ-11** | Мониторинг и метрики | Экспорт метрик производительности (Prometheus) | Средний |
| **ФТ-12** | Логирование | Структурированное логирование всех операций | Высокий |
| **ФТ-13** | Health checks | Проверки здоровья сервисов для Kubernetes | Высокий |
| **ФТ-14** | Graceful shutdown | Корректная остановка с завершением активных запросов | Высокий |
| **ФТ-15** | Документация API | Полная документация всех API endpoints | Высокий |

---

## 7. Нефункциональные требования

### 7.1. Производительность

| ID | Требование | Описание | Метрика |
|----|-----------|----------|---------|
| **НФТ-1** | Throughput (Ingress) | Система должна обрабатывать минимум 1000 RPS на публикацию | >= 1000 RPS |
| **НФТ-2** | Latency (Ingress) | Задержка публикации сообщения до 10KB | p95 < 50ms |
| **НФТ-3** | Latency (Egress) | Задержка чтения сообщения | p95 < 100ms |
| **НФТ-4** | Throughput (Egress) | Система должна обрабатывать минимум 500 RPS на чтение | >= 500 RPS |
| **НФТ-5** | Размер сообщения | Поддержка сообщений до 1GB без деградации | 1GB max |
| **НФТ-6** | Concurrent connections | Поддержка минимум 1000 одновременных соединений | >= 1000 |

### 7.2. Надежность

| ID | Требование | Описание |
|----|-----------|----------|
| **НФТ
├── Namespace: minitoolstream
│   ├── Deployment: minitoolstream-ingress (3 replicas)
│   │   └── Service: ClusterIP (port 50051)
│   ├── Deployment: minitoolstream-egress (3 replicas)
│   │   └── Service: ClusterIP (port 50052)
│   ├── StatefulSet: tarantool (1 replica)
│   │   ├── Service: ClusterIP (port 3301)
│   │   └── PersistentVolumeClaim (10Gi)
│   ├── StatefulSet: minio (4 replicas)
│   │   ├── Service: ClusterIP (port 9000)
│   │   └── PersistentVolumeClaim (100Gi per node)
│   └── Deployment: vault (1 replica)
│       └── Service: ClusterIP (port 8200)
│
├── ConfigMaps:
│   ├── ingress-config
│   ├── egress-config
│   └── tarantool-config
│
├── Secrets:
│   ├── tarantool-credentials
│   ├── minio-credentials
│   └── vault-token
│
└── HorizontalPodAutoscaler:
├── ingress-hpa (min: 3, max: 10)
└── egress-hpa (min: 3, max: 10)
```

### 9.3. Процесс развертывания

#### 9.3.1. Локальное развертывание (Development)

**Шаг 1: Запуск инфраструктуры**
```bash
# Docker Compose для Tarantool, MinIO, Vault
cd MiniToolStream
docker-compose up -d
```

**Шаг 2: Создание k3d кластера**
```bash
k3d cluster create minitoolstream \
  --agents 2 \
  --api-port 6550 \
  --port "8080:80@loadbalancer" \
  --port "8443:443@loadbalancer"
```

**Шаг 3: Развертывание Dashboard**
```bash
helm install kubernetes-dashboard kubernetes-dashboard/kubernetes-dashboard \
  --create-namespace \
  --namespace kubernetes-dashboard
```

**Шаг 4: Сборка образов**
```bash
# Ingress
cd MiniToolStreamIngress
docker build -t minitoolstream-ingress:latest --platform linux/arm64 .
k3d image import minitoolstream-ingress:latest -c minitoolstream

# Egress
cd MiniToolStreamEgress
docker build -t minitoolstream-egress:latest --platform linux/arm64 .
k3d image import minitoolstream-egress:latest -c minitoolstream
```

**Шаг 5: Развертывание сервисов**
```bash
# Ingress
kubectl apply -k MiniToolStreamIngress/k8s/

# Egress
kubectl apply -k MiniToolStreamEgress/k8s/

# Создание ExternalName сервисов для Tarantool и MinIO
kubectl apply -f infrastructure-services.yaml
```

**Шаг 6: Проверка**
```bash
kubectl get all -n minitoolstream
kubectl logs -n minitoolstream -l app=minitoolstream-ingress
```

#### 9.3.2. Production развертывание

**Шаг 1: Подготовка окружения**
```bash
# Создание namespace
kubectl create namespace minitoolstream

# Установка Tarantool Operator
kubectl apply -f https://raw.githubusercontent.com/tarantool/tarantool-operator/master/deploy/bundle.yaml

# Установка MinIO Operator
kubectl apply -k github.com/minio/operator
```

**Шаг 2: Настройка секретов**
```bash
# Vault
helm install vault hashicorp/vault \
  --namespace minitoolstream \
  --set server.ha.enabled=true \
  --set server.ha.replicas=3

# Заполнение секретов
kubectl create secret generic tarantool-credentials \
  --from-literal=user=minitoolstream_connector \
  --from-literal=password=<strong-password> \
  -n minitoolstream

kubectl create secret generic minio-credentials \
  --from-literal=accessKey=<access-key> \
  --from-literal=secretKey=<secret-key> \
  -n minitoolstream
```

**Шаг 3: Развертывание Tarantool**
```bash
kubectl apply -f tarantool-cluster.yaml
```

**Шаг 4: Развертывание MinIO**
```bash
kubectl apply -f minio-tenant.yaml
```

**Шаг 5: Развертывание приложений**
```bash
# Через Helm Chart
helm install minitoolstream ./charts/minitoolstream \
  --namespace minitoolstream \
  --values production-values.yaml
```

**Шаг 6: Настройка мониторинга**
```bash
# Prometheus
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring

# Grafana dashboards
kubectl apply -f monitoring/grafana-dashboards/
```

### 9.4. Конфигурационные файлы

#### 9.4.1. Ingress ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: minitoolstream-ingress-config
  namespace: minitoolstream
data:
  config.yaml: |
    server:
      port: 50051
      grpc:
        max_concurrent_streams: 1000
        max_connection_idle: 5m

    tarantool:
      address: "tarantool-service:3301"
      user: "minitoolstream_connector"
      timeout: 5s
      pool_size: 10

    minio:
      endpoint: "minio-service:9000"
      bucket: "minitoolstream"
      use_ssl: false

    vault:
      enabled: true
      address: "http://vault:8200"
      secrets_path: "minitoolstream/ingress"

    logging:
      level: info
      format: json
```

#### 9.4.2. Egress ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: minitoolstream-egress-config
  namespace: minitoolstream
data:
  config.yaml: |
    server:
      port: 50052
      grpc:
        max_concurrent_streams: 1000
        keepalive: 30s

    tarantool:
      address: "tarantool-service:3301"
      user: "minitoolstream_connector"
      timeout: 5s
      pool_size: 20

    minio:
      endpoint: "minio-service:9000"
      bucket: "minitoolstream"
      use_ssl: false

    subscription:
      check_interval: 1s
      batch_size: 10
      max_wait_time: 30s

    logging:
      level: info
      format: json
```

### 9.5. Docker образы

#### 9.5.1. Ingress Dockerfile

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/minitoolstream-ingress \
    ./cmd/server

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
RUN addgroup -g 1000 appgroup && \
    adduser -D -u 1000 -G appgroup appuser

WORKDIR /app
COPY --from=builder /app/minitoolstream-ingress .

RUN chown -R appuser:appgroup /app
USER appuser

EXPOSE 50051

ENTRYPOINT ["/app/minitoolstream-ingress"]
```

### 9.6. Мониторинг и Observability

#### 9.6.1. Метрики (Prometheus)

**Ingress метрики:**
```
minitoolstream_ingress_requests_total{status}
minitoolstream_ingress_request_duration_seconds
minitoolstream_ingress_active_connections
minitoolstream_ingress_tarantool_operations_total{operation, status}
minitoolstream_ingress_minio_operations_total{operation, status}
```

**Egress метрики:**
```
minitoolstream_egress_requests_total{method, status}
minitoolstream_egress_request_duration_seconds
minitoolstream_egress_active_subscriptions
minitoolstream_egress_messages_delivered_total
```

#### 9.6.2. Health Checks

**Liveness Probe:**
```yaml
livenessProbe:
  exec:
    command:
      - /bin/sh
      - -c
      - "nc -z localhost 50051"
  initialDelaySeconds: 30
  periodSeconds: 10
```

**Readiness Probe:**
```yaml
readinessProbe:
  exec:
    command:
      - /bin/sh
      - -c
      - "nc -z localhost 50051"
  initialDelaySeconds: 10
  periodSeconds: 5
```

---

## 10. Ограничения и допущения

### 10.1. Ограничения

1. **Операционные системы:**
    - Production: Linux (Ubuntu 20.04+, RHEL 8+, Debian 11+)
    - Development: Linux, macOS (Apple Silicon)
    - Архитектуры: amd64, arm64

2. **Сетевые требования:**
    - Минимальная пропускная способность: 1 Gbps
    - Задержка между нодами: < 10ms (LAN)
    - Открытые порты: 50051, 50052, 3301, 9000, 8200

3. **Отказоустойчивость:**
    - Single point of failure: Tarantool (в базовой конфигурации)
    - Требуется внешнее решение для резервного копирования
    - RPO (Recovery Point Objective): 5 минут
    - RTO (Recovery Time Objective): 5 минут

4. **Масштабирование:**
    - Вертикальное масштабирование Tarantool ограничено размером RAM
    - Горизонтальное масштабирование Tarantool требует настройки репликации
    - Максимальное количество каналов: не ограничено
    - Максимальное количество потребителей: не ограничено

5. **Размеры данных:**
    - Рекомендуемый максимум для одного сообщения: 1 GB
    - Максимальное количество сообщений: ограничено дисковым пространством MinIO

### 10.2. Допущения

1. **Инфраструктура:**
    - У пользователей есть подготовленный Kubernetes кластер
    - Доступны persistent volumes для хранения данных
    - Настроен ingress controller для внешнего доступа

2. **Железо:**
    - Ноды оснащены NVMe SSD для высокой производительности I/O
    - Достаточный объем RAM для работы Tarantool (минимум 8GB)
    - Стабильное сетевое соединение между нодами

3. **Пользователи:**
    - Пользователи являются профессионально подготовленными разработчиками
    - Знание Go, gRPC, Kubernetes на базовом уровне
    - Понимание принципов работы message brokers

4. **Эксплуатация:**
    - Наличие команды DevOps для поддержки инфраструктуры
    - Настроены процедуры резервного копирования и восстановления
    - Есть план реагирования на инциденты

---

## 11. Тестирование

### 11.1. Виды тестирования

| Тип | Описание | Инструменты |
|-----|----------|-------------|
| **Unit tests** | Тестирование отдельных функций и модулей | Go testing, testify |
| **Integration tests** | Тестирование взаимодействия компонентов | Go testing, testcontainers |
| **Load tests** | Нагрузочное тестирование | k6, vegeta |
| **Stress tests** | Тестирование предельных нагрузок | k6 |
| **E2E tests** | End-to-end сценарии | Go testing |
| **Security tests** | Проверка безопасности | gosec, trivy |

### 11.2. Сценарии тестирования

#### 11.2.1. Функциональные тесты

**Сценарий 1: Публикация и чтение сообщения**
```
1. Publisher публикует сообщение в канал "test.channel"
2. Проверка: получен sequence и object_name
3. Subscriber читает последний sequence
4. Subscriber запрашивает сообщение по sequence
5. Проверка: полученные данные совпадают с опубликованными
```

**Сценарий 2: Множество потребителей**
```
1. Publisher публикует 100 сообщений в канал
2. Запускаются 10 Subscribers (durable consumers)
3. Каждый читает все 100 сообщений
4. Проверка: все получили все сообщения
5. Проверка: позиции сохранены в Tarantool
```

**Сценарий 3: TTL и очистка данных**
```
1. Публикация сообщений с TTL = 1 час
2. Ожидание 1 час + 5 минут
3. Запуск cleaner
4. Проверка: старые сообщения удалены из Tarantool и MinIO
5. Проверка: новые сообщения остались
```

#### 11.2.2. Нагрузочные тесты

**Сценарий 4: 1000 RPS публикация**
```javascript
// k6 script
export let options = {
  vus: 100,
  duration: '5m',
  thresholds: {
    'grpc_req_duration{method="Publish"}': ['p(95)<50'],
  },
};

export default function () {
  let response = grpc.invoke('IngressService/Publish', {
    subject: 'load.test',
    data: randomBytes(10 * 1024), // 10KB
    headers: { 'content-type': 'application/octet-stream' },
  });
  check(response, {
    'status is OK': (r) => r.status === grpc.StatusOK,
  });
}
```

**Результаты:**
- Target RPS: 1000
- Expected p95 latency: < 50ms
- Expected error rate: < 0.1%

#### 11.2.3. Тесты отказоустойчивости

**Сценарий 5: Restart Ingress под**
```
1. Publisher начинает публиковать сообщения (100 RPS)
2. Один из Ingress подов перезапускается
3. Проверка: публикация продолжается без ошибок
4. Проверка: все сообщения сохранены
```

**Сценарий 6: Недоступность Tarantool**
```
1. Tarantool останавливается
2. Попытка публикации сообщения
3. Проверка: ошибка connection timeout
4. Tarantool запускается
5. Проверка: публикация восстанавливается
```

### 11.3. Критерии приемки

| Критерий | Пороговое значение | Метод проверки |
|----------|-------------------|----------------|
| Unit tests coverage | >= 80% | go test -cover |
| Integration tests pass rate | 100% | CI/CD pipeline |
| Load test (1000 RPS) | p95 < 50ms | k6 |
| Error rate under load | < 0.1% | k6 |
| Data integrity | 100% | E2E tests |
| Security vulnerabilities | 0 critical/high | gosec, trivy |

---

## 12. Документация

### 12.1. Требуемая документация

1. **API Documentation**
    - gRPC API reference (auto-generated from protobuf)
    - Go SDK documentation (godoc)
    - Examples и tutorials

2. **Architecture Documentation**
    - Диаграммы компонентов
    - Data flow диаграммы
    - Sequence диаграммы

3. **Operations Documentation**
    - Installation guide
    - Configuration guide
    - Monitoring guide
    - Troubleshooting guide
    - Backup and recovery procedures

4. **Developer Documentation**
    - Contributing guide
    - Code style guide
    - Testing guide
    - Release process

### 12.2. Формат документации

- Markdown файлы в репозитории
- Auto-generated API docs (protoc-gen-doc)
- Godoc для Go кода
- Confluence/Wiki для операционной документации
- Диаграммы: PlantUML, Mermaid

---

## 13. Сравнение с альтернативами

### 13.1. MiniToolStream vs Apache Kafka

| Критерий | MiniToolStream | Apache Kafka |
|----------|---------------|--------------|
| **Размер сообщений** | Без ограничений (до TB) | До 1MB (default), 100MB (max) |
| **Хранилище** | Гибридное (Tarantool + S3) | Файловая система |
| **Типы данных** | Любые (файлы, бинарные) | Байтовые массивы |
| **Сложность установки** | Средняя | Высокая |
| **Производительность** | 1000+ RPS | 100k+ RPS |
| **Масштабируемость** | Хорошая | Отличная |
| **Стоимость хранения** | Низкая (S3) | Средняя (диски) |
| **Зрелость экосистемы** | Новый проект | Mature |

**Вывод:** MiniToolStream подходит для сценариев с большими объектами, Kafka — для высокопроизводительных event streams.

### 13.2. MiniToolStream vs Redis Streams

| Критерий | MiniToolStream | Redis Streams |
|----------|---------------|---------------|
| **Персистентность** | Durability (WAL + S3) | In-memory (с AOF) |
| **Размер сообщений** | Без ограничений | Ограничено RAM |
| **Стоимость** | Средняя | Высокая (RAM) |
| **Latency** | 50ms (p95) | 1ms (p95) |
| **TTL** | Гибкий (по каналам) | Ограниченный |

**Вывод:** Redis быстрее для небольших сообщений, MiniToolStream — для больших объемов и длительного хранения.

---

## 14. Roadmap

### 14.1. Phase 1 (MVP) — 3 месяца

- [x] Базовая архитектура
- [x] Ingress service (gRPC)
- [x] Egress service (gRPC)
- [x] Интеграция с Tarantool
- [x] Интеграция с MinIO
- [x] Клиентская библиотека (Go SDK)
- [x] Kubernetes deployment
- [ ] Базовая документация

### 14.2. Phase 2 (Production Ready) — 2 месяца

- [ ] Authentication/Authorization
- [ ] TLS support
- [ ] Monitoring (Prometheus)
- [ ] Grafana dashboards
- [ ] Load testing
- [ ] Security audit
- [ ] Production documentation

### 14.3. Phase 3 (Advanced Features) — 3 месяца

- [ ] Batch publishing API
- [ ] Dead letter queues
- [ ] Message replay
- [ ] Schema registry
- [ ] Tarantool clustering (HA)
- [ ] Multi-region support

### 14.4. Phase 4 (Ecosystem) — ongoing

- [ ] Python SDK
- [ ] Java SDK
- [ ] CLI tool
- [ ] Web UI для мониторинга
- [ ] Kafka compatibility layer
- [ ] Benchmarking suite

---

## 15. Риски и митигация

| Риск | Вероятность | Влияние | Митигация |
|------|-------------|---------|-----------|
| **Tarantool single point of failure** | Высокая | Критическое | Настроить репликацию в Phase 3 |
| **MinIO недоступность** | Средняя | Высокое | Использовать распределенный режим MinIO |
| **Недостаточная производительность** | Средняя | Высокое | Провести load testing на ранней стадии |
| **Сложность эксплуатации** | Средняя | Среднее | Детальная документация, автоматизация |
| **Безопасность данных** | Низкая | Критическое | Security audit, encryption |

---

## 16. Контакты и поддержка

### 16.1. Команда разработки

- **Tech Lead:** [Имя]
- **Backend Engineers:** [Имена]
- **DevOps Engineers:** [Имена]
- **QA Engineers:** [Имена]

### 16.2. Ресурсы

- **Репозиторий:** https://github.com/moroshma/MiniToolStream
- **Документация:** https://docs.minitoolstream.io
- **Issues:** https://github.com/moroshma/MiniToolStream/issues
- **Slack:** #minitoolstream

---

## 17. Приложения

### Приложение A: Глоссарий

- **Sequence** — уникальный монотонно возрастающий номер сообщения (аналог offset в Kafka)
- **Subject** — название канала/топика для логической группировки сообщений
- **Object name** — ключ объекта в MinIO, формат: `{subject}_{sequence}`
- **Durable consumer** — потребитель с сохранением позиции чтения в БД
- **Ephemeral consumer** — потребитель без сохранения позиции
- **TTL** — время жизни сообщения до удаления
- **WAL** — Write-Ahead Log, механизм персистентности Tarantool

### Приложение B: Примеры использования

См. директорию `/examples` в репозитории:
- `publisher_client/` — пример публикации
- `subscriber_client/` — пример подписки
- `batch_publisher/` — пример batch публикации
- `integration_test/` — интеграционные тесты

### Приложение C: Конфигурационные файлы

См. директорию `/configs` в репозитории:
- `ingress.yaml` — конфигурация Ingress
- `egress.yaml` — конфигурация Egress
- `tarantool.lua` — схема Tarantool
- `k8s/` — Kubernetes манифесты

---

**Конец документа**

**Утверждено:**
Дата: 03.12.2025
Версия: 1.0
-7** | Durability | Сообщения должны сохраняться персистентно с использованием WAL |
| **НФТ-8** | Data integrity | Гарантия целостности данных при сбоях |
| **НФТ-9** | At-least-once delivery | Гарантия доставки каждого сообщения минимум один раз |
| **НФТ-10** | Availability | Целевая доступность 99.9% (SLA) |
| **НФТ-11** | Recovery time | Время восстановления после сбоя < 5 минут (RTO) |
| **НФТ-12** | Backup | Ежедневное резервное копирование Tarantool |

### 7.3. Безопасность

| ID | Требование | Описание |
|----|-----------|----------|
| **НФТ-13** | Authentication | Аутентификация через gRPC Interceptors (JWT/mTLS) |
| **НФТ-14** | Authorization | Role-Based Access Control (RBAC) для каналов |
| **НФТ-15** | Encryption in transit | TLS 1.3 для всех gRPC соединений |
| **НФТ-16** | Encryption at rest | Опциональное шифрование данных в MinIO (SSE-S3) |
| **НФТ-17** | Secrets management | Хранение всех credentials в Vault |
| **НФТ-18** | Audit logging | Логирование всех операций с данными |

### 7.4. Масштабируемость

| ID | Требование | Описание |
|----|-----------|----------|
| **НФТ-19** | Horizontal scaling (Ingress) | Возможность добавления новых инстансов Ingress |
| **НФТ-20** | Horizontal scaling (Egress) | Возможность добавления новых инстансов Egress |
| **НФТ-21** | Storage scaling | Независимое масштабирование MinIO кластера |
| **НФТ-22** | Channel isolation | Изоляция каналов для предотвращения interference |

### 7.5. Операционные требования

| ID | Требование | Описание |
|----|-----------|----------|
| **НФТ-23** | Deployment | Развертывание в Kubernetes (k8s/k3s) |
| **НФТ-24** | Containerization | Все компоненты упакованы в Docker образы |
| **НФТ-25** | Configuration | Управление конфигурацией через Vault + ConfigMaps |
| **НФТ-26** | Monitoring | Интеграция с Prometheus/Grafana |
| **НФТ-27** | Logging | Централизованное логирование (ELK/Loki) |
| **НФТ-28** | Alerting | Настройка алертов для критических событий |
| **НФТ-29** | Documentation | Полная операционная документация |

### 7.6. Совместимость

| ID | Требование | Описание |
|----|-----------|----------|
| **НФТ-30** | OS compatibility | Linux (amd64, arm64), macOS (arm64) |
| **НФТ-31** | Kubernetes versions | K8s 1.25+, K3s latest |
| **НФТ-32** | Go version | Go 1.21+ |
| **НФТ-33** | gRPC compatibility | gRPC 1.50+ |
| **НФТ-34** | S3 API | Полная совместимость с AWS S3 API |

---

## 8. Технологический стек

### 8.1. Основные технологии

| Компонент | Технология | Версия | Назначение |
|-----------|-----------|--------|------------|
| **Язык программирования** | Go (Golang) | 1.21+ | Разработка всех сервисов |
| **RPC протокол** | gRPC | 1.50+ | Межсервисное взаимодействие |
| **Метаданные БД** | Tarantool | 2.11+ | In-memory хранилище метаданных |
| **Объектное хранилище** | MinIO | Latest | S3-compatible хранилище данных |
| **Секреты** | HashiCorp Vault | Latest | Управление конфигурацией и секретами |
| **Оркестрация** | Kubernetes | 1.25+ | Развертывание и управление |
| **Контейнеризация** | Docker | Latest | Упаковка приложений |
| **Registry** | Docker Hub | - | Хранение образов |

### 8.2. Вспомогательные инструменты

| Инструмент | Назначение |
|-----------|-----------|
| **Protocol Buffers** | Определение gRPC API |
| **Tarantool Operator** | Управление кластером Tarantool в K8s |
| **MinIO Operator** | Управление MinIO в K8s |
| **Helm** | Package manager для K8s |
| **Prometheus** | Сбор метрик |
| **Grafana** | Визуализация метрик |
| **Loki** | Агрегация логов |

### 8.3. Go библиотеки

```go
// gRPC и protobuf
google.golang.org/grpc
google.golang.org/protobuf

// Tarantool client
github.com/tarantool/go-tarantool

// MinIO client
github.com/minio/minio-go/v7

// Vault client
github.com/hashicorp/vault/api

// Логирование
go.uber.org/zap

// Конфигурация
github.com/spf13/viper

// Метрики
github.com/prometheus/client_golang
```

---

## 9. Развертывание

### 9.1. Требования к инфраструктуре

#### 9.1.1. Минимальные требования (Development)

**Одна нода:**
- CPU: 4 cores
- RAM: 8 GB
- Disk: 50 GB SSD
- Network: 1 Gbps

**Kubernetes:**
- k3s (для локальной разработки)
- kubectl

#### 9.1.2. Рекомендуемые требования (Production)

**Kubernetes кластер:**
- Минимум 3 ноды (master + 2 workers)
- CPU: 8 cores per node
- RAM: 16 GB per node
- Disk: 200 GB NVMe SSD per node
- Network: 10 Gbps

**Компоненты:**
- Ingress: 3 реплики
- Egress: 3 реплики
- Tarantool: 1 нода (3 для HA)
- MinIO: 4 ноды (распределенный режим)

### 9.2. Схема развертывания

```
Kubernetes Cluster