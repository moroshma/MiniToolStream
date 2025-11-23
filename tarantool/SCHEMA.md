# Схема данных Tarantool для MiniToolStream

## Обзор

MiniToolStream использует две основные таблицы (spaces) в Tarantool для хранения метаданных сообщений и состояния потребителей.

## Space 1: `message`

Хранит метаданные о каждом сообщении в потоке.

### Структура

| Поле | Тип | Описание |
|------|-----|----------|
| `sequence` | `unsigned` (uint64) | Уникальный, монотонно возрастающий номер сообщения. **Первичный ключ (PK)**. Глобальный счетчик для всех subject. |
| `headers` | `map<string, string>` | Карта с заголовками (метаданными) сообщения. Может содержать content-type, source, correlation-id и т.д. |
| `object_name` | `string` | Имя/ключ объекта в S3-хранилище (MinIO), где лежит тело сообщения. |
| `subject` | `string` | Тема (канал), к которой относится сообщение. Аналог topic в Kafka. |
| `create_at` | `unsigned` | Время создания сообщения в формате Unix timestamp. Используется для TTL. |

### Индексы

| Имя индекса | Тип | Поля | Уникальный | Назначение |
|-------------|------|------|------------|------------|
| `primary` | TREE | `sequence` | ✅ Да | Первичный ключ для прямого доступа по sequence |
| `subject` | TREE | `subject` | ❌ Нет | Поиск всех сообщений по теме |
| `subject_sequence` | TREE | `subject, sequence` | ✅ Да | Диапазонные запросы по теме, упорядоченные по sequence |
| `create_at` | TREE | `create_at` | ❌ Нет | Очистка старых сообщений по TTL |

### Пример данных

```lua
{
    sequence = 12345,
    headers = {
        ["content-type"] = "application/json",
        ["source"] = "order-service",
        ["correlation-id"] = "abc-123"
    },
    object_name = "minio/orders/2024/11/18/12345.json",
    subject = "orders",
    create_at = 1700320800
}
```

---

## Space 2: `consumers`

Хранит состояние (позицию чтения) для каждого долговечного потребителя (durable consumer).

### Структура

| Поле | Тип | Описание |
|------|-----|----------|
| `durable_name` | `string` | Уникальное имя группы потребителей. Часть **композитного первичного ключа (PK)**. |
| `subject` | `string` | Тема, на которую подписан потребитель. Часть **композитного первичного ключа (PK)** и имеет **вторичный TREE-индекс**. |
| `last_sequence` | `unsigned` (uint64) | Номер последнего сообщения (`sequence`), которое было прочитано этим потребителем. |

### Индексы

| Имя индекса | Тип | Поля | Уникальный | Назначение |
|-------------|------|------|------------|------------|
| `primary` | TREE | `durable_name, subject` | ✅ Да | Первичный ключ для получения позиции конкретного потребителя |
| `subject` | TREE | `subject` | ❌ Нет | Поиск всех потребителей конкретной темы |

### Пример данных

```lua
{
    durable_name = "order-processor-v1",
    subject = "orders",
    last_sequence = 12340
}
```

---

## API Функции

### Публикация сообщений

#### `publish_message(subject, object_name, headers)`

Публикует новое сообщение и возвращает присвоенный sequence.

**Параметры:**
- `subject` (string) - название темы/канала
- `object_name` (string) - ключ объекта в MinIO
- `headers` (table/map) - карта заголовков (необязательно)

**Возвращает:** `sequence` (uint64)

**Пример:**
```lua
local seq = publish_message("orders", "minio/orders/123.json", {
    ["content-type"] = "application/json",
    ["source"] = "api-gateway"
})
-- seq = 1
```

### Чтение сообщений

#### `get_message_by_sequence(sequence)`

Получает сообщение по его глобальному sequence.

**Параметры:**
- `sequence` (uint64) - номер сообщения

**Возвращает:** tuple или nil

**Пример:**
```lua
local msg = get_message_by_sequence(12345)
-- msg = {12345, {...}, "minio/orders/123.json", "orders", 1700320800}
```

#### `get_messages_by_subject(subject, start_sequence, limit)`

Получает диапазон сообщений из указанной темы.

**Параметры:**
- `subject` (string) - название темы
- `start_sequence` (uint64) - начальный sequence (включительно)
- `limit` (number) - максимальное количество сообщений

**Возвращает:** array of tuples

**Пример:**
```lua
local messages = get_messages_by_subject("orders", 12340, 10)
-- Вернет до 10 сообщений из темы "orders" начиная с sequence 12340
```

#### `get_latest_sequence_for_subject(subject)`

Получает последний sequence для указанной темы.

**Параметры:**
- `subject` (string) - название темы

**Возвращает:** `sequence` (uint64) или 0 если нет сообщений

**Пример:**
```lua
local latest = get_latest_sequence_for_subject("orders")
-- latest = 12345
```

### Управление потребителями

#### `update_consumer_position(durable_name, subject, last_sequence)`

Обновляет или создает позицию потребителя.

**Параметры:**
- `durable_name` (string) - имя группы потребителей
- `subject` (string) - название темы
- `last_sequence` (uint64) - последний прочитанный sequence

**Возвращает:** true

**Пример:**
```lua
update_consumer_position("order-processor-v1", "orders", 12345)
```

#### `get_consumer_position(durable_name, subject)`

Получает текущую позицию потребителя.

**Параметры:**
- `durable_name` (string) - имя группы потребителей
- `subject` (string) - название темы

**Возвращает:** `last_sequence` (uint64) или 0

**Пример:**
```lua
local pos = get_consumer_position("order-processor-v1", "orders")
-- pos = 12345
```

#### `get_consumers_by_subject(subject)`

Получает всех потребителей конкретной темы.

**Параметры:**
- `subject` (string) - название темы

**Возвращает:** array of tuples

**Пример:**
```lua
local consumers = get_consumers_by_subject("orders")
-- consumers = {
--   {"order-processor-v1", "orders", 12345},
--   {"analytics-service", "orders", 12000}
-- }
```

### Очистка данных

#### `delete_old_messages(ttl_seconds)`

Удаляет сообщения старше указанного TTL.

**Параметры:**
- `ttl_seconds` (number) - время жизни в секундах

**Возвращает:** `deleted_count`, `deleted_messages_array`

**Пример:**
```lua
local count, deleted = delete_old_messages(86400) -- 24 часа
-- count = 150
-- deleted = {
--   {sequence = 100, subject = "orders", object_name = "minio/orders/100.json"},
--   {sequence = 101, subject = "orders", object_name = "minio/orders/101.json"},
--   ...
-- }
```

---

## Паттерны использования

### Паттерн 1: Публикация сообщения

```lua
-- 1. Сохранить payload в MinIO
local object_key = minio_client:upload(payload, "bucket/key")

-- 2. Сохранить метаданные в Tarantool
local sequence = publish_message("orders", object_key, {
    ["content-type"] = "application/json",
    ["size"] = tostring(#payload)
})

-- 3. Вернуть sequence клиенту
return sequence
```

### Паттерн 2: Чтение новых сообщений (Pull модель)

```lua
local consumer_name = "order-processor-v1"
local subject = "orders"

-- 1. Получить текущую позицию потребителя
local current_pos = get_consumer_position(consumer_name, subject)

-- 2. Получить новые сообщения
local messages = get_messages_by_subject(subject, current_pos + 1, 100)

-- 3. Обработать сообщения
for _, msg in ipairs(messages) do
    local seq = msg[1]
    local object_name = msg[3]

    -- Загрузить payload из MinIO
    local payload = minio_client:download(object_name)

    -- Обработать
    process_message(payload)

    -- Обновить позицию
    update_consumer_position(consumer_name, subject, seq)
end
```

### Паттерн 3: Периодическая очистка

```lua
-- Запускать по расписанию (например, каждые 6 часов)
local ttl = 7 * 24 * 60 * 60  -- 7 дней

local count, deleted = delete_old_messages(ttl)

-- Удалить объекты из MinIO
for _, msg_info in ipairs(deleted) do
    minio_client:delete(msg_info.object_name)
end

print(string.format("Deleted %d old messages", count))
```

---

## Особенности реализации

### Глобальный sequence

Sequence является **глобальным** счетчиком для всех тем. Это упрощает:
- Обеспечение уникальности
- Атомарность инкремента
- Реализацию "читать всё" сценариев

**Инициализация:**
```lua
local global_sequence = 0

box.once('init_global_sequence', function()
    local max_seq = box.space.message.index.primary:max()
    if max_seq ~= nil then
        global_sequence = max_seq[1]
    end
end)
```

**Инкремент (thread-safe в Tarantool):**
```lua
function get_next_sequence()
    global_sequence = global_sequence + 1
    return global_sequence
end
```

### Композитный ключ в consumers

Использование `(durable_name, subject)` как составного ключа позволяет:
- Одной группе потребителей подписываться на разные темы
- Иметь множество групп для одной темы
- Эффективно искать по обоим измерениям

---

## Примеры запросов из Go

### Публикация

```go
resp, err := conn.Call("publish_message", []interface{}{
    "orders",                    // subject
    "minio/orders/123.json",     // object_name
    map[string]interface{}{      // headers
        "content-type": "application/json",
        "source": "api-gateway",
    },
})
sequence := resp[0].(uint64)
```

### Чтение

```go
resp, err := conn.Call("get_messages_by_subject", []interface{}{
    "orders",       // subject
    uint64(12340),  // start_sequence
    100,            // limit
})
messages := resp[0].([]interface{})
```

### Управление позицией

```go
// Обновить позицию
conn.Call("update_consumer_position", []interface{}{
    "processor-v1",  // durable_name
    "orders",        // subject
    uint64(12345),   // last_sequence
})

// Получить позицию
resp, _ := conn.Call("get_consumer_position", []interface{}{
    "processor-v1",
    "orders",
})
position := resp[0].(uint64)
```

---

## Миграция со старой схемы

Если у вас была старая схема с `messages` и `sequences`, выполните:

```bash
# 1. Остановить все сервисы
docker-compose down

# 2. Удалить старые данные
docker volume rm tarantool_tarantool-data

# 3. Запустить с новой схемой
docker-compose up -d
```

Или для Kubernetes:

```bash
kubectl delete -f tarantool/k8s/
kubectl delete pvc tarantool-data-tarantool-0 -n minitoolstream_connector
kubectl apply -f tarantool/k8s/
```

---

## Производительность

### Примерные характеристики:

| Операция | Сложность | Примерная скорость |
|----------|-----------|-------------------|
| `publish_message` | O(log N) | ~50,000 ops/sec |
| `get_message_by_sequence` | O(log N) | ~100,000 ops/sec |
| `get_messages_by_subject` | O(log N + K) | Зависит от K (limit) |
| `update_consumer_position` | O(log N) | ~80,000 ops/sec |
| `get_consumer_position` | O(log N) | ~100,000 ops/sec |

**Примечание:** Скорость зависит от hardware, размера данных, наличия WAL и других факторов.

---

## Мониторинг

### Размер данных

```lua
-- Количество сообщений
box.space.message:count()

-- Количество сообщений по теме (приблизительно)
box.space.message.index.subject:count("orders")

-- Количество потребителей
box.space.consumers:count()
```

### Информация о последних sequence

```lua
-- Глобальный последний sequence
local max = box.space.message.index.primary:max()
if max then
    print("Latest global sequence:", max[1])
end

-- Последний sequence для темы
get_latest_sequence_for_subject("orders")
```

---

## Дополнительные ресурсы

- [Tarantool Data Model](https://www.tarantool.io/en/doc/latest/book/box/data_model/)
- [Tarantool Indexes](https://www.tarantool.io/en/doc/latest/book/box/indexes/)
- [Tarantool Lua Tutorial](https://www.tarantool.io/en/doc/latest/tutorials/lua_tutorials/)
