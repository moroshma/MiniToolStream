# gRPC API → Tarantool Function Mapping

Этот документ описывает соответствие между gRPC методами (из proto-файлов) и функциями Tarantool.

## IngressService (Публикация сообщений)

### `Publish(PublishRequest) → PublishResponse`

**Proto определение:**
```protobuf
message PublishRequest {
  string subject = 1;
  bytes data = 2;
  map<string, string> headers = 3;
}

message PublishResponse {
  uint64 sequence = 1;
  int64 StatusCode = 2;
  string ResponderError = 3;
}
```

**Tarantool функция:**
```lua
grpc_publish(subject, object_name, headers) → {sequence, status_code, error_message}
```

**Пример вызова из Go:**
```go
resp, err := conn.Call("grpc_publish", []interface{}{
    "orders",                    // subject
    "minio/orders/123.json",     // object_name (путь к данным в MinIO)
    map[string]interface{}{      // headers
        "content-type": "application/json",
        "source": "api-gateway",
    },
})

result := resp[0].(map[interface{}]interface{})
sequence := result["sequence"]       // uint64
statusCode := result["status_code"]  // 0 = success, 1 = error
errorMsg := result["error_message"]  // string or nil
```

**Примечание:**
- В текущей реализации `data` не сохраняется в Tarantool
- Вместо этого сохраняется `object_name` - путь к объекту в MinIO/S3
- Полная схема: сначала данные загружаются в MinIO, затем путь сохраняется в Tarantool

---

## EgressService (Чтение сообщений)

### 1. `GetLastSequence(GetLastSequenceRequest) → GetLastSequenceResponse`

**Proto определение:**
```protobuf
message GetLastSequenceRequest {
  string subject = 1;
}

message GetLastSequenceResponse {
  uint64 last_sequence = 1;
}
```

**Tarantool функция:**
```lua
grpc_get_last_sequence(subject) → {last_sequence}
```

**Пример вызова:**
```go
resp, err := conn.Call("grpc_get_last_sequence", []interface{}{"orders"})
result := resp[0].(map[interface{}]interface{})
lastSeq := result["last_sequence"] // uint64
```

**Альтернатива (прямой доступ):**
```go
resp, err := conn.Call("get_latest_sequence_for_subject", []interface{}{"orders"})
lastSeq := resp[0].(uint64)
```

---

### 2. `Fetch(FetchRequest) → stream Message`

**Proto определение:**
```protobuf
message FetchRequest {
  string subject = 1;
  string durable_name = 2;
  int32 batch_size = 3;
}

message Message {
  string subject = 1;
  uint64 sequence = 2;
  bytes data = 3;
  map<string, string> headers = 4;
  google.protobuf.Timestamp timestamp = 5;
}
```

**Tarantool функции:**

#### Вариант 1: Fetch с ручным подтверждением (рекомендуется)
```lua
grpc_fetch(subject, durable_name, batch_size, auto_ack=false) → messages[]
```

**Пример:**
```go
// 1. Получить сообщения
resp, err := conn.Call("grpc_fetch", []interface{}{
    "orders",        // subject
    "consumer-1",    // durable_name
    10,              // batch_size
    false,           // auto_ack (не обновлять позицию автоматически)
})

messages := resp[0].([]interface{})

// 2. Обработать сообщения
for _, m := range messages {
    msg := m.([]interface{})
    sequence := msg[0].(uint64)
    headers := msg[1].(map[interface{}]interface{})
    objectName := msg[2].(string)
    subject := msg[3].(string)
    createAt := msg[4].(uint64)

    // Загрузить данные из MinIO используя objectName
    data := minioClient.Get(objectName)

    // Обработать data
    process(data)
}

// 3. Подтвердить обработку последнего сообщения
lastMsg := messages[len(messages)-1].([]interface{})
lastSeq := lastMsg[0].(uint64)

conn.Call("grpc_ack", []interface{}{
    "consumer-1",  // durable_name
    "orders",      // subject
    lastSeq,       // sequence
})
```

#### Вариант 2: Fetch с авто-подтверждением
```lua
grpc_fetch(subject, durable_name, batch_size, auto_ack=true) → messages[]
```

**Пример:**
```go
resp, err := conn.Call("grpc_fetch", []interface{}{
    "orders",
    "consumer-1",
    10,
    true, // auto_ack - позиция обновится автоматически
})

messages := resp[0].([]interface{})
// Позиция consumer уже обновлена до последнего сообщения в batch
```

**⚠️ Внимание:** Auto-ack обновляет позицию сразу после выборки, до обработки. Используйте с осторожностью.

---

### 3. `Subscribe(SubscribeRequest) → stream Notification`

**Proto определение:**
```protobuf
message SubscribeRequest {
  string subject = 1;
  optional uint64 start_sequence = 2;
  string consumer_group = 3;
}

message Notification {
  string subject = 1;
  uint64 sequence = 2; // последний доступный sequence
}
```

**Tarantool функции:**

Subscribe - это **server streaming** gRPC метод. Для его реализации нужно периодически проверять наличие новых сообщений.

#### Функция для проверки новых сообщений:
```lua
check_new_messages(subject, consumer_group) → {
    has_new,           -- boolean
    latest_sequence,   -- uint64
    consumer_position, -- uint64
    new_count         -- uint64
}
```

**Пример реализации Subscribe в Go gRPC сервере:**
```go
func (s *EgressServer) Subscribe(req *pb.SubscribeRequest, stream pb.EgressService_SubscribeServer) error {
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    lastNotifiedSeq := uint64(0)
    if req.StartSequence != nil {
        lastNotifiedSeq = *req.StartSequence
    }

    for {
        select {
        case <-stream.Context().Done():
            return nil
        case <-ticker.C:
            // Проверить наличие новых сообщений
            resp, err := tarantoolConn.Call("check_new_messages", []interface{}{
                req.Subject,
                req.ConsumerGroup,
            })
            if err != nil {
                return err
            }

            result := resp[0].(map[interface{}]interface{})
            hasNew := result["has_new"].(bool)
            latestSeq := result["latest_sequence"].(uint64)

            // Отправить нотификацию если есть новые сообщения
            if hasNew && latestSeq > lastNotifiedSeq {
                notification := &pb.Notification{
                    Subject:  req.Subject,
                    Sequence: latestSeq,
                }

                if err := stream.Send(notification); err != nil {
                    return err
                }

                lastNotifiedSeq = latestSeq
            }
        }
    }
}
```

#### Альтернативная функция (только счетчик новых):
```lua
get_new_messages_count(subject, durable_name, since_sequence) → count
```

---

## Дополнительные вспомогательные функции

### `grpc_peek` - Предпросмотр без изменения позиции

Получить сообщения БЕЗ обновления позиции consumer:

```lua
grpc_peek(subject, durable_name, batch_size) → messages[]
```

**Пример:**
```go
resp, err := conn.Call("grpc_peek", []interface{}{
    "orders",
    "consumer-1",
    5,
})

messages := resp[0].([]interface{})
// Позиция consumer не изменилась
```

**Использование:** Полезно для preview/inspection перед реальной обработкой.

---

### `grpc_ack` - Ручное подтверждение

Подтвердить обработку сообщений до указанного sequence (включительно):

```lua
grpc_ack(durable_name, subject, sequence) → success
```

**Пример:**
```go
resp, err := conn.Call("grpc_ack", []interface{}{
    "consumer-1",  // durable_name
    "orders",      // subject
    uint64(42),    // sequence - подтвердить до 42 включительно
})

success := resp[0].(bool)
```

**Логика:** Обновляет позицию только если новый sequence больше текущего.

---

### `get_subject_message_count` - Общее количество сообщений

```lua
get_subject_message_count(subject) → count
```

**Пример:**
```go
resp, err := conn.Call("get_subject_message_count", []interface{}{"orders"})
count := resp[0].(uint64)
```

---

## Маппинг форматов данных

### Message tuple → Proto Message

**Tarantool tuple формат:**
```
{
    [1] sequence     (uint64)
    [2] headers      (map)
    [3] object_name  (string)
    [4] subject      (string)
    [5] create_at    (uint64 unix timestamp)
}
```

**Proto Message формат:**
```protobuf
message Message {
  string subject = 1;
  uint64 sequence = 2;
  bytes data = 3;
  map<string, string> headers = 4;
  google.protobuf.Timestamp timestamp = 5;
}
```

**Конвертация в Go gRPC сервере:**
```go
func tarantoolTupleToProtoMessage(tuple []interface{}, minioClient *minio.Client) (*pb.Message, error) {
    sequence := tuple[0].(uint64)
    headers := tuple[1].(map[interface{}]interface{})
    objectName := tuple[2].(string)
    subject := tuple[3].(string)
    createAt := tuple[4].(uint64)

    // Загрузить данные из MinIO
    data, err := minioClient.GetObject(context.Background(), "bucket", objectName)
    if err != nil {
        return nil, err
    }

    // Конвертировать headers
    pbHeaders := make(map[string]string)
    for k, v := range headers {
        pbHeaders[k.(string)] = v.(string)
    }

    // Конвертировать timestamp
    timestamp := &timestamppb.Timestamp{
        Seconds: int64(createAt),
    }

    return &pb.Message{
        Subject:   subject,
        Sequence:  sequence,
        Data:      data,
        Headers:   pbHeaders,
        Timestamp: timestamp,
    }, nil
}
```

---

## Workflow: Полный цикл Publish → Fetch → Ack

### 1. Publish (IngressService)

```go
// 1. Загрузить данные в MinIO
objectName, err := minioClient.PutObject(ctx, "bucket", key, bytes.NewReader(data), ...)

// 2. Сохранить метаданные в Tarantool
resp, err := tarantoolConn.Call("grpc_publish", []interface{}{
    subject,
    objectName,
    headers,
})

result := resp[0].(map[interface{}]interface{})
sequence := result["sequence"].(uint64)

// 3. Вернуть sequence клиенту
return &pb.PublishResponse{
    Sequence:       sequence,
    StatusCode:     0,
    ResponderError: "",
}
```

### 2. Subscribe (EgressService) - опционально

```go
// Клиент подписывается на уведомления о новых сообщениях
// Периодически проверяем и отправляем нотификации
for {
    result := check_new_messages(subject, consumer_group)
    if result.has_new {
        stream.Send(&pb.Notification{
            Subject:  subject,
            Sequence: result.latest_sequence,
        })
    }
    time.Sleep(1 * time.Second)
}
```

### 3. Fetch (EgressService)

```go
// Получить batch сообщений
resp, err := tarantoolConn.Call("grpc_fetch", []interface{}{
    subject,
    durableName,
    batchSize,
    false, // не auto-ack
})

messages := resp[0].([]interface{})

// Конвертировать и отправить клиенту через stream
for _, tuple := range messages {
    protoMsg, err := tarantoolTupleToProtoMessage(tuple, minioClient)
    if err != nil {
        return err
    }

    if err := stream.Send(protoMsg); err != nil {
        return err
    }
}
```

### 4. Ack (после обработки клиентом)

```go
// Клиент обработал сообщения и подтверждает
lastSeq := messages[len(messages)-1][0].(uint64)

tarantoolConn.Call("grpc_ack", []interface{}{
    durableName,
    subject,
    lastSeq,
})
```

---

## Сравнительная таблица

| gRPC Method | Proto | Tarantool Function | Примечание |
|-------------|-------|-------------------|-----------|
| **IngressService.Publish** | `PublishRequest` → `PublishResponse` | `grpc_publish(subject, object_name, headers)` | Возвращает `{sequence, status_code, error_message}` |
| **EgressService.GetLastSequence** | `GetLastSequenceRequest` → `GetLastSequenceResponse` | `grpc_get_last_sequence(subject)` | Возвращает `{last_sequence}` |
| **EgressService.Fetch** | `FetchRequest` → `stream Message` | `grpc_fetch(subject, durable_name, batch_size, auto_ack)` | Auto-ack опционален |
| **EgressService.Subscribe** | `SubscribeRequest` → `stream Notification` | `check_new_messages(subject, consumer_group)` | Polling-based, вызывается периодически |
| - | - | `grpc_ack(durable_name, subject, sequence)` | Ручное подтверждение после Fetch |
| - | - | `grpc_peek(subject, durable_name, batch_size)` | Предпросмотр без ack |

---

## Итоговая оценка

✅ **Функций достаточно** для реализации обоих gRPC сервисов:

### IngressService
- ✅ `Publish` - полностью покрыт через `grpc_publish`

### EgressService
- ✅ `GetLastSequence` - полностью покрыт через `grpc_get_last_sequence`
- ✅ `Fetch` - полностью покрыт через `grpc_fetch` с поддержкой manual/auto ack
- ✅ `Subscribe` - реализуется polling через `check_new_messages`

### Дополнительно
- ✅ `grpc_ack` - ручное подтверждение
- ✅ `grpc_peek` - предпросмотр без изменения позиции
- ✅ `get_new_messages_count` - счетчик новых сообщений
- ✅ `get_subject_message_count` - общий счетчик

Все функции протестированы и готовы к использованию в gRPC сервисах.
