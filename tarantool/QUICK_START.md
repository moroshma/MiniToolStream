# Quick Start Guide - Tarantool для MiniToolStream

Быстрый старт для работы с Tarantool в проекте MiniToolStream.

## Запуск

### Docker Compose (локально)

```bash
# Запустить
docker-compose up -d

# Проверить логи
docker-compose logs -f tarantool

# Остановить
docker-compose down
```

### Kubernetes (Minikube)

```bash
# Развернуть
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/statefulset.yaml
kubectl apply -f k8s/service.yaml

# Проверить статус
kubectl get pods -n minitoolstream

# Port-forward для доступа
kubectl port-forward -n minitoolstream svc/tarantool-service 3301:3301
```

## Подключение

### Go клиент

```go
import "github.com/tarantool/go-tarantool/v2"

ctx := context.Background()
conn, err := tarantool.Connect(ctx, tarantool.NetDialer{
    Address:  "localhost:3301",
    User:     "minitoolstream",
    Password: "changeme",
}, tarantool.Opts{})
```

## Использование API

### 1. Публикация сообщения (для IngressService)

```go
resp, err := conn.Call("grpc_publish", []interface{}{
    "orders",                    // subject
    "minio/orders/12345.json",   // object_name (путь в MinIO)
    map[string]interface{}{      // headers
        "content-type": "application/json",
        "source": "api-gateway",
    },
})

result := resp[0].(map[interface{}]interface{})
sequence := result["sequence"].(uint64)
statusCode := result["status_code"].(int64)
```

### 2. Получение последнего sequence (для EgressService)

```go
resp, err := conn.Call("grpc_get_last_sequence", []interface{}{"orders"})
result := resp[0].(map[interface{}]interface{})
lastSeq := result["last_sequence"].(uint64)
```

### 3. Fetch сообщений с ручным подтверждением

```go
// Получить batch сообщений
resp, err := conn.Call("grpc_fetch", []interface{}{
    "orders",        // subject
    "consumer-1",    // durable_name
    10,              // batch_size
    false,           // auto_ack = false (ручное подтверждение)
})

messages := resp[0].([]interface{})

// Обработать сообщения
for _, m := range messages {
    msg := m.([]interface{})
    sequence := msg[0].(uint64)
    headers := msg[1].(map[interface{}]interface{})
    objectName := msg[2].(string)
    subject := msg[3].(string)
    createAt := msg[4].(uint64)

    // Загрузить данные из MinIO
    data := minioClient.GetObject(objectName)

    // Обработать
    processMessage(data)
}

// Подтвердить обработку
if len(messages) > 0 {
    lastMsg := messages[len(messages)-1].([]interface{})
    lastSeq := lastMsg[0].(uint64)

    conn.Call("grpc_ack", []interface{}{
        "consumer-1",
        "orders",
        lastSeq,
    })
}
```

### 4. Subscribe: проверка новых сообщений

```go
// В цикле проверять наличие новых сообщений
ticker := time.NewTicker(1 * time.Second)
for range ticker.C {
    resp, err := conn.Call("check_new_messages", []interface{}{
        "orders",       // subject
        "consumer-1",   // consumer_group
    })

    result := resp[0].(map[interface{}]interface{})
    hasNew := result["has_new"].(bool)
    latestSeq := result["latest_sequence"].(uint64)

    if hasNew {
        fmt.Printf("New messages available! Latest sequence: %d\n", latestSeq)
        // Отправить notification клиенту через gRPC stream
    }
}
```

### 5. Peek: предпросмотр без изменения позиции

```go
resp, err := conn.Call("grpc_peek", []interface{}{
    "orders",
    "consumer-1",
    5,  // batch_size
})

messages := resp[0].([]interface{})
// Позиция consumer не изменилась
```

## Типичные сценарии

### Сценарий 1: Ingress сервис (публикация)

```go
func PublishMessage(subject string, data []byte, headers map[string]string) (uint64, error) {
    // 1. Загрузить data в MinIO
    objectName, err := minioClient.PutObject(ctx, "bucket", key, bytes.NewReader(data), ...)
    if err != nil {
        return 0, err
    }

    // 2. Сохранить метаданные в Tarantool
    resp, err := tarantoolConn.Call("grpc_publish", []interface{}{
        subject,
        objectName,
        headers,
    })
    if err != nil {
        return 0, err
    }

    result := resp[0].(map[interface{}]interface{})
    sequence := result["sequence"].(uint64)
    statusCode := result["status_code"].(int64)

    if statusCode != 0 {
        return 0, fmt.Errorf("publish failed: %v", result["error_message"])
    }

    return sequence, nil
}
```

### Сценарий 2: Egress сервис (чтение)

```go
func FetchMessages(subject, durableName string, batchSize int) ([]*Message, error) {
    // 1. Fetch batch из Tarantool
    resp, err := tarantoolConn.Call("grpc_fetch", []interface{}{
        subject,
        durableName,
        batchSize,
        false, // manual ack
    })
    if err != nil {
        return nil, err
    }

    tuples := resp[0].([]interface{})
    messages := make([]*Message, 0, len(tuples))

    // 2. Для каждого tuple загрузить данные из MinIO
    for _, t := range tuples {
        tuple := t.([]interface{})
        sequence := tuple[0].(uint64)
        headers := tuple[1].(map[interface{}]interface{})
        objectName := tuple[2].(string)

        // Загрузить data из MinIO
        data, err := minioClient.GetObject(ctx, "bucket", objectName)
        if err != nil {
            return nil, err
        }

        messages = append(messages, &Message{
            Sequence: sequence,
            Subject:  subject,
            Data:     data,
            Headers:  convertHeaders(headers),
        })
    }

    // 3. После успешной обработки - подтвердить
    if len(tuples) > 0 {
        lastSeq := tuples[len(tuples)-1].([]interface{})[0].(uint64)
        tarantoolConn.Call("grpc_ack", []interface{}{
            durableName,
            subject,
            lastSeq,
        })
    }

    return messages, nil
}
```

### Сценарий 3: Subscribe stream

```go
func Subscribe(subject, consumerGroup string, stream pb.EgressService_SubscribeServer) error {
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()

    lastNotifiedSeq := uint64(0)

    for {
        select {
        case <-stream.Context().Done():
            return nil

        case <-ticker.C:
            resp, err := tarantoolConn.Call("check_new_messages", []interface{}{
                subject,
                consumerGroup,
            })
            if err != nil {
                return err
            }

            result := resp[0].(map[interface{}]interface{})
            hasNew := result["has_new"].(bool)
            latestSeq := result["latest_sequence"].(uint64)

            if hasNew && latestSeq > lastNotifiedSeq {
                notification := &pb.Notification{
                    Subject:  subject,
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

## Тестирование

### Базовые функции

```bash
go run test_new_schema.go
```

### gRPC функции

```bash
go run test_grpc_functions.go
```

### Персистентность

```bash
./test_persistence_simple.sh
```

## Полезные команды

### Docker Compose

```bash
# Логи
docker-compose logs -f tarantool

# Перезапуск
docker-compose restart

# Консоль Tarantool
docker exec -it minitoolstream-tarantool tarantoolctl connect /var/run/tarantool/tarantool.sock
```

### Kubernetes

```bash
# Статус подов
kubectl get pods -n minitoolstream

# Логи
kubectl logs -f tarantool-0 -n minitoolstream

# Консоль Tarantool
kubectl exec -it tarantool-0 -n minitoolstream -- tarantoolctl connect localhost:3301

# Перезапуск
kubectl rollout restart statefulset/tarantool -n minitoolstream
```

## Следующие шаги

1. Интегрировать с MinIO для хранения payload
2. Реализовать IngressService (gRPC) для публикации
3. Реализовать EgressService (gRPC) для чтения
4. Добавить Cleaner для TTL cleanup

## Справочная информация

- [README.md](README.md) - Полная документация
- [SCHEMA.md](SCHEMA.md) - Схема данных
- [GRPC_API_MAPPING.md](GRPC_API_MAPPING.md) - Маппинг gRPC → Tarantool
- [Tarantool Docs](https://www.tarantool.io/en/doc/latest/)
