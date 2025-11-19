# MessagePack Support для MiniToolStream

Документация по использованию MessagePack формата для хранения данных в Tarantool.

## Обзор

MessagePack - это эффективный бинарный формат сериализации данных, который:
- ✅ Компактнее JSON (экономия ~30-50% места)
- ✅ Быстрее сериализуется/десериализуется
- ✅ Сохраняет типы данных
- ✅ Поддерживается многими языками программирования
- ✅ Не требует отдельного хранилища MinIO для небольших сообщений

## Обновленная схема

### Space: message

Добавлено новое поле `data_msgpack` для хранения бинарных данных:

```lua
{
    sequence      (unsigned)  -- PK, уникальный номер
    headers       (map)        -- метаданные
    object_name   (string)     -- путь в MinIO (опционально)
    subject       (string)     -- топик
    create_at     (unsigned)   -- timestamp
    data_msgpack  (scalar)     -- MessagePack данные (nullable)
}
```

**Два режима работы:**

1. **Inline mode** - данные хранятся прямо в Tarantool (для сообщений < 1MB):
   - `data_msgpack` содержит MessagePack данные
   - `object_name` пустое

2. **MinIO mode** - для больших payloads (> 1MB):
   - `object_name` содержит путь в MinIO
   - `data_msgpack` = null

## API Функции

### Публикация с MessagePack

#### `grpc_publish_msgpack(subject, data_msgpack, headers)`

Публикует сообщение с MessagePack данными.

**Параметры:**
- `subject` (string) - название топика
- `data_msgpack` (binary) - MessagePack закодированные данные
- `headers` (map) - метаданные (опционально)

**Возвращает:**
```lua
{
    sequence = uint64,
    status_code = 0|1,  -- 0 = success, 1 = error
    error_message = string|nil
}
```

**Пример (Go):**
```go
import "github.com/vmihailenco/msgpack/v5"

// Структура сообщения
type Order struct {
    OrderID string  `msgpack:"order_id"`
    Amount  float64 `msgpack:"amount"`
    Items   []string `msgpack:"items"`
}

// Создать и сериализовать
order := Order{
    OrderID: "ORD-123",
    Amount:  99.99,
    Items:   []string{"laptop", "mouse"},
}

data, _ := msgpack.Marshal(order)

// Опубликовать
resp, _ := conn.Call("grpc_publish_msgpack", []interface{}{
    "orders",
    data,
    map[string]interface{}{
        "content-type": "application/x-msgpack",
    },
})

result := resp[0].(map[interface{}]interface{})
sequence := result["sequence"].(uint64)
```

#### `publish_message_msgpack(subject, data_msgpack, headers, object_name)`

Низкоуровневая функция для публикации с опциональным `object_name`.

**Параметры:**
- `subject` (string)
- `data_msgpack` (binary)
- `headers` (map)
- `object_name` (string, optional) - для гибридного режима

### Чтение с MessagePack

#### `grpc_fetch_msgpack(subject, durable_name, batch_size, auto_ack)`

Получает batch сообщений в виде структурированных таблиц.

**Параметры:**
- `subject` (string)
- `durable_name` (string)
- `batch_size` (number)
- `auto_ack` (boolean)

**Возвращает:** Array of messages:
```lua
{
    sequence = uint64,
    headers = map,
    object_name = string,
    subject = string,
    create_at = uint64,
    data_msgpack = binary  -- MessagePack данные
}
```

**Пример (Go):**
```go
resp, _ := conn.Call("grpc_fetch_msgpack", []interface{}{
    "orders",
    "consumer-1",
    10,
    false, // manual ack
})

messages := resp[0].([]interface{})

for _, m := range messages {
    msg := m.(map[interface{}]interface{})

    sequence := msg["sequence"].(uint64)
    dataMsgpack := msg["data_msgpack"].([]byte)

    // Десериализовать
    var order Order
    msgpack.Unmarshal(dataMsgpack, &order)

    // Обработать
    processOrder(order)
}

// Подтвердить
if len(messages) > 0 {
    lastSeq := messages[len(messages)-1].(map[interface{}]interface{})["sequence"].(uint64)
    conn.Call("grpc_ack", []interface{}{"consumer-1", "orders", lastSeq})
}
```

#### `get_message_by_sequence_decoded(sequence)`

Получает одно сообщение в виде структурированной таблицы.

**Параметры:**
- `sequence` (uint64)

**Возвращает:**
```lua
{
    sequence = uint64,
    headers = map,
    object_name = string,
    subject = string,
    create_at = uint64,
    data_msgpack = binary
}
```

## Сравнение с MinIO режимом

| Параметр | MessagePack (inline) | MinIO режим |
|----------|---------------------|-------------|
| **Размер данных** | < 1MB | > 1MB |
| **Латентность** | Низкая (1 запрос) | Выше (2 запроса) |
| **Хранилище** | Tarantool RAM + диск | MinIO S3 |
| **Консистентность** | Атомарная | Eventual |
| **Бэкапы** | Вместе с Tarantool | Отдельно MinIO |
| **Использование** | Метаданные, события, команды | Файлы, изображения, большие JSON |

## Workflow

### 1. Публикация (IngressService)

```go
func PublishMessage(subject string, data interface{}) (uint64, error) {
    // Сериализовать в MessagePack
    msgpackData, err := msgpack.Marshal(data)
    if err != nil {
        return 0, err
    }

    // Проверить размер
    if len(msgpackData) > 1*1024*1024 { // > 1MB
        // Использовать MinIO
        objectName, _ := minioClient.PutObject(ctx, "bucket", key, bytes.NewReader(msgpackData), ...)

        resp, _ := tarantoolConn.Call("grpc_publish", []interface{}{
            subject,
            objectName,
            map[string]interface{}{"size": len(msgpackData)},
        })
    } else {
        // Inline в Tarantool
        resp, _ := tarantoolConn.Call("grpc_publish_msgpack", []interface{}{
            subject,
            msgpackData,
            map[string]interface{}{"content-type": "application/x-msgpack"},
        })
    }

    result := resp[0].(map[interface{}]interface{})
    return result["sequence"].(uint64), nil
}
```

### 2. Чтение (EgressService)

```go
func FetchMessages(subject, durableName string, batchSize int) ([]*Message, error) {
    // Fetch из Tarantool
    resp, _ := tarantoolConn.Call("grpc_fetch_msgpack", []interface{}{
        subject,
        durableName,
        batchSize,
        false, // manual ack
    })

    msgs := resp[0].([]interface{})
    result := make([]*Message, 0, len(msgs))

    for _, m := range msgs {
        msg := m.(map[interface{}]interface{})

        sequence := msg["sequence"].(uint64)
        dataMsgpack := msg["data_msgpack"]
        objectName := msg["object_name"].(string)

        var payload []byte

        if dataMsgpack != nil && len(dataMsgpack.([]byte)) > 0 {
            // Inline mode - данные прямо в Tarantool
            payload = dataMsgpack.([]byte)
        } else if objectName != "" {
            // MinIO mode - загрузить из S3
            payload, _ = minioClient.GetObject(ctx, "bucket", objectName)
        }

        // Десериализовать MessagePack
        var data map[string]interface{}
        msgpack.Unmarshal(payload, &data)

        result = append(result, &Message{
            Sequence: sequence,
            Subject:  subject,
            Data:     data,
        })
    }

    // Подтвердить после обработки
    if len(msgs) > 0 {
        lastSeq := msgs[len(msgs)-1].(map[interface{}]interface{})["sequence"].(uint64)
        tarantoolConn.Call("grpc_ack", []interface{}{durableName, subject, lastSeq})
    }

    return result, nil
}
```

## Преимущества MessagePack

### 1. Компактность

**JSON:**
```json
{"order_id":"ORD-12345","user_id":42,"amount":199.99,"items":["laptop","mouse","keyboard"]}
```
Размер: ~94 байта

**MessagePack:**
```
(binary data)
```
Размер: ~65 байт (-31%)

### 2. Типы данных

MessagePack сохраняет типы:
- Integers (int8, int16, int32, int64)
- Floats (float32, float64)
- Strings
- Binary data
- Arrays
- Maps
- Boolean
- Nil

JSON конвертирует все числа в float64.

### 3. Скорость

Бенчмарк (Go):
- JSON Marshal: ~300 ns/op
- MessagePack Marshal: ~200 ns/op (на 33% быстрее)

### 4. Кросс-язычность

MessagePack поддерживается:
- Go: `github.com/vmihailenco/msgpack`
- Python: `msgpack-python`
- JavaScript: `msgpack-lite`
- Rust: `rmp-serde`
- Java: `msgpack-java`
- C/C++: `msgpack-c`
- Ruby, PHP, Perl, и др.

## Ограничения

1. **Максимальный размер inline данных**: ~1 MB
   - Для больших данных используйте MinIO mode
   - Tarantool хранит данные в RAM (memtx)

2. **Нечитаемость**: MessagePack бинарный формат
   - Для отладки используйте десериализацию
   - Или храните в JSON для dev-окружения

3. **Версионирование**: При изменении структуры данных
   - Используйте headers для версии схемы
   - Или prefix в subject: `orders.v2`

## Совместимость

### Обратная совместимость

Старые функции продолжают работать:
- `publish_message(subject, object_name, headers)`
- `grpc_publish(subject, object_name, headers)`
- `grpc_fetch(subject, durable_name, batch_size, auto_ack)`

Они записывают `data_msgpack = null` и используют `object_name`.

### Миграция

Для миграции существующих сообщений:

```lua
-- Скрипт миграции (пример)
for _, msg in box.space.message:pairs() do
    if msg[6] == nil and msg[3] ~= "" then  -- нет msgpack, есть object_name
        -- Опционально: загрузить из MinIO и сконвертировать
        -- Но обычно оставляют как есть
    end
end
```

## Тестирование

```bash
# Простой тест
go run test_msgpack_simple.go

# Полный тест всех функций
go run test_msgpack.go
```

## Мониторинг

### Размер данных в памяти

```lua
-- Общий размер space
box.space.message:bsize()

-- Количество сообщений с inline данными
local count = 0
for _, msg in box.space.message:pairs() do
    if msg[6] ~= nil then
        count = count + 1
    end
end
print("Messages with inline data:", count)
```

### Статистика размеров

```go
resp, _ := conn.Call("box.space.message:bsize", []interface{}{})
totalBytes := resp[0].(uint64)
fmt.Printf("Total space size: %d MB\n", totalBytes/(1024*1024))
```

## Рекомендации

### Когда использовать MessagePack inline:

- ✅ Метаданные и события (< 100 KB)
- ✅ Команды и RPC вызовы
- ✅ Малые JSON документы
- ✅ Логи и метрики
- ✅ Когда нужна низкая латентность

### Когда использовать MinIO:

- ✅ Файлы и изображения
- ✅ Видео и аудио
- ✅ Большие JSON/XML (> 1 MB)
- ✅ Бинарные blob'ы
- ✅ Когда нужен отдельный lifecycle

## Примеры структур

### Event

```go
type Event struct {
    Type      string                 `msgpack:"type"`
    Timestamp int64                  `msgpack:"timestamp"`
    UserID    int                    `msgpack:"user_id"`
    Data      map[string]interface{} `msgpack:"data"`
}
```

### Command

```go
type Command struct {
    Command   string   `msgpack:"command"`
    Args      []string `msgpack:"args"`
    Timeout   int      `msgpack:"timeout"`
}
```

### Order

```go
type Order struct {
    OrderID     string           `msgpack:"order_id"`
    UserID      int              `msgpack:"user_id"`
    TotalAmount float64          `msgpack:"total_amount"`
    Items       []OrderItem      `msgpack:"items"`
    Status      string           `msgpack:"status"`
    CreatedAt   time.Time        `msgpack:"created_at"`
}

type OrderItem struct {
    ProductID string  `msgpack:"product_id"`
    Quantity  int     `msgpack:"quantity"`
    Price     float64 `msgpack:"price"`
}
```

## Заключение

MessagePack поддержка добавляет гибкость в MiniToolStream:

- 🚀 Быстрая публикация для малых сообщений (без MinIO round-trip)
- 💾 Эффективное использование памяти
- 🔄 Обратная совместимость со старым API
- 🌐 Кросс-язычная сериализация
- ⚡ Низкая латентность для inline данных

Используйте MessagePack для большинства случаев, MinIO для больших файлов.
