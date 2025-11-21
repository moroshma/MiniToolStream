# Изменения в MiniToolStream

## Дата: 2024-11-20

### Изменения в Tarantool (tarantool/init.lua)

#### 1. Автоматическая генерация `object_name`

**Функции обновлены:**

- `publish_message_msgpack(subject, data_msgpack, headers)`
  - ✅ Удален параметр `object_name` (был опциональным)
  - ✅ Теперь автоматически генерирует `object_name` как `{{subject}}_{{sequence}}`
  - Пример: `orders_12345`

- `grpc_publish_msgpack(subject, data_msgpack, headers)`
  - ✅ Автоматически генерирует `object_name` как `{{subject}}_{{sequence}}`
  - ✅ Возвращает `object_name` в ответе:
    ```lua
    {
        sequence = uint64,
        object_name = string,     -- NEW!
        status_code = int64,
        error_message = string
    }
    ```

**Преимущества:**
- Единообразные имена объектов
- Не нужно передавать `object_name` из клиента
- Упрощение API
- Имя можно использовать для интеграции с MinIO

---

### Изменения в Go библиотеке (MiniToolStreamIngress/)

#### 1. Обновлен `PublishResponse` (types.go)

```go
type PublishResponse struct {
    Sequence       uint64 // Присвоенный sequence number
    ObjectName     string // NEW! Автоматически сгенерированное имя
    StatusCode     int64
    ResponderError string
}
```

#### 2. Удален метод `PublishWithObjectName` (publisher.go)

**Было:**
- `Publish()` - MessagePack inline
- `PublishRaw()` - MessagePack raw
- `PublishWithObjectName()` - MinIO mode ❌ УДАЛЕНО

**Стало:**
- `Publish()` - MessagePack inline, возвращает auto-generated `ObjectName`
- `PublishRaw()` - MessagePack raw, возвращает auto-generated `ObjectName`

#### 3. Обновлены примеры (example/main.go)

```go
resp, err := publisher.Publish(req)

fmt.Printf("Sequence: %d\n", resp.Sequence)
fmt.Printf("ObjectName: %s\n", resp.ObjectName)  // NEW!
// Output: ObjectName: orders_12345
```

---

## Как использовать

### Базовое использование

```go
req := &ingress.PublishRequest{
    Subject: "orders",
    Data:    []byte("message data"),
    Headers: map[string]string{"content-type": "application/json"},
}

resp, err := publisher.Publish(req)

// resp.Sequence = 12345
// resp.ObjectName = "orders_12345"  (автоматически)
```

### Интеграция с MinIO (опционально)

```go
// 1. Опубликовать сообщение
resp, _ := publisher.Publish(req)

// 2. Использовать ObjectName для загрузки в MinIO
err := minioClient.PutObject(
    ctx,
    "bucket",
    resp.ObjectName,  // используем сгенерированное имя
    largeData,
    -1,
)

// 3. При чтении сообщения ObjectName доступен
message := fetchMessage(resp.Sequence)
data := minioClient.GetObject(ctx, "bucket", message.ObjectName)
```

---

## Миграция со старой версии

### Если использовали `Publish()` или `PublishRaw()`

**Никаких изменений не требуется!** Просто получите дополнительное поле:

```go
resp, err := publisher.Publish(req)
// Теперь доступно: resp.ObjectName
```

### Если использовали `PublishWithObjectName()`

**До:**
```go
// 1. Загрузить в MinIO
objectName, _ := minioClient.PutObject(ctx, "bucket", "my-key", data, ...)

// 2. Опубликовать
resp, _ := publisher.PublishWithObjectName("orders", objectName, headers)
```

**После:**
```go
// 1. Опубликовать (ObjectName генерируется автоматически)
resp, _ := publisher.Publish(&ingress.PublishRequest{
    Subject: "orders",
    Data:    data,  // или PublishRaw для больших данных
    Headers: headers,
})

// 2. Использовать сгенерированный ObjectName для MinIO
minioClient.PutObject(ctx, "bucket", resp.ObjectName, largeData, ...)
```

---

## Обратная совместимость

### ✅ Совместимо:
- Код, использующий `Publish()` работает без изменений
- Код, использующий `PublishRaw()` работает без изменений
- Старые данные в Tarantool остаются валидными

### ❌ Несовместимо:
- Метод `PublishWithObjectName()` удален
- Нужно обновить код, если использовали этот метод (см. миграцию выше)

---

## Файлы изменены

### Tarantool
- `tarantool/init.lua` - функции `publish_message_msgpack` и `grpc_publish_msgpack`

### Go библиотека
- `MiniToolStreamIngress/types.go` - добавлено поле `ObjectName` в `PublishResponse`
- `MiniToolStreamIngress/publisher.go` - удален метод `PublishWithObjectName`, обновлена обработка ответа
- `MiniToolStreamIngress/README.md` - обновлена документация
- `MiniToolStreamIngress/LIBRARY_STRUCTURE.md` - обновлена техническая документация
- `MiniToolStreamIngress/example/main.go` - обновлены примеры

---

## Тестирование

### Сборка библиотеки

```bash
cd MiniToolStreamIngress
go build -v .
```

✅ Успешно

### Сборка примера

```bash
cd MiniToolStreamIngress/example
go build -v .
```

✅ Успешно

### Запуск примера (требует запущенный Tarantool)

```bash
cd tarantool
docker-compose up -d

cd ../MiniToolStreamIngress/example
./example
```

Ожидаемый вывод:
```
✅ Connected to Tarantool

📤 Example 1: Publishing a simple message
   Published message:
     - Sequence: 1
     - ObjectName: orders_1

📤 Example 2: Publishing structured data
   Published order:
     - Sequence: 2
     - ObjectName: orders_2

📤 Example 3: Publishing multiple messages
   Message #1: sequence=3, object_name=test_3
   Message #2: sequence=4, object_name=test_4
   Message #3: sequence=5, object_name=test_5
   Message #4: sequence=6, object_name=test_6
   Message #5: sequence=7, object_name=test_7

✅ All examples completed successfully!
```

---

## Преимущества новой архитектуры

1. **Простота API** - не нужно генерировать и передавать `object_name`
2. **Единообразие** - все объекты следуют одному паттерну именования
3. **Гибкость** - можно использовать inline MessagePack или MinIO
4. **Прозрачность** - клиент сразу получает имя объекта в ответе
5. **Меньше кода** - удален лишний метод `PublishWithObjectName`

---

## Следующие шаги

Изменения готовы к использованию. Рекомендуется:

1. Протестировать с реальными данными
2. Обновить клиентский код, если использовался `PublishWithObjectName`
3. Обновить документацию других сервисов (Egress, если есть)
