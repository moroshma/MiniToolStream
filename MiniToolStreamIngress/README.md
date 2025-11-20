# MiniToolStream Ingress

gRPC сервер для приема и обработки сообщений в MiniToolStream. Сохраняет метаданные в Tarantool.

## Описание

MiniToolStreamIngress - это gRPC сервер, который:
- Принимает запросы на публикацию сообщений через gRPC API
- Сохраняет метаданные (subject, headers, sequence, object_name) в Tarantool
- Автоматически генерирует уникальный object_name для каждого сообщения в формате `{{subject}}_{{sequence}}`
- Возвращает клиенту sequence number и object_name

## Архитектура
 
```
example/publisher_client     →  gRPC API  →  MiniToolStreamIngress  →  Tarantool
      (клиент)                                    (сервер)              (метаданные)
```

На данном этапе:
- ✅ Метаданные сохраняются в Tarantool
- ⏳ Данные (data) будут сохраняться в MinIO (планируется)

## Структура проекта

```
MiniToolStreamIngress/
├── cmd/
│   └── server/
│       └── main.go           # Точка входа gRPC сервера
├── internal/
│   ├── server/
│   │   └── server.go         # Реализация gRPC IngressService
│   └── tarantool/
│       └── client.go         # Клиент для работы с Tarantool
├── go.mod
└── README.md
```

## Сборка и запуск

### Предварительные требования

1. **Tarantool** должен быть запущен:
```bash
cd ../tarantool
docker-compose up -d
```

### Сборка

```bash
cd cmd/app
go build -o ingress-app .
```

### Запуск

```bash
# С дефолтными параметрами
./ingress-app

# С кастомными параметрами
./ingress-app \
  -port 50051 \
  -tarantool-addr localhost:3301 \
  -tarantool-user minitoolstream \
  -tarantool-password changeme
```

### Параметры командной строки

- `-port` - порт для gRPC сервера (по умолчанию: `50051`)
- `-tarantool-addr` - адрес Tarantool (по умолчанию: `localhost:3301`)
- `-tarantool-user` - пользователь Tarantool (по умолчанию: `minitoolstream`)
- `-tarantool-password` - пароль Tarantool (по умолчанию: `changeme`)

## API

### Publish RPC

Публикует сообщение в указанный subject.

**Request:**
```protobuf
message PublishRequest {
  string subject = 1;                    // Название канала/топика
  bytes data = 2;                        // Данные сообщения (будут в MinIO)
  map<string, string> headers = 3;       // Метаданные
}
```

**Response:**
```protobuf
message PublishResponse {
  uint64 sequence = 1;                   // Уникальный номер сообщения
  string object_name = 2;                // Имя объекта (subject_sequence)
  int64 status_code = 3;                 // 0 = success, 1 = error
  string error_message = 4;              // Сообщение об ошибке
}
```

## Примеры использования

### Тестовый клиент

См. [example/publisher_client](../example/publisher_client/)

```bash
cd ../example/publisher_client
./publisher_client -subject terminator.diff -image tst.jpeg
```

### Go клиент

```go
package main

import (
    "context"
    "log"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    pb "github.com/moroshma/MiniToolStream/model"
)

func main() {
    // Подключение к серверу
    conn, err := grpc.NewClient("localhost:50051",
        grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    client := pb.NewIngressServiceClient(conn)

    // Публикация сообщения
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    resp, err := client.Publish(ctx, &pb.PublishRequest{
        Subject: "my.channel",
        Data:    []byte("message data"),
        Headers: map[string]string{
            "content-type": "text/plain",
            "source":       "my-app",
        },
    })

    if err != nil {
        log.Fatal(err)
    }

    if resp.StatusCode != 0 {
        log.Fatalf("Error: %s", resp.ErrorMessage)
    }

    log.Printf("Published: sequence=%d, object_name=%s",
        resp.Sequence, resp.ObjectName)
}
```

## Логика работы

1. Клиент отправляет `PublishRequest` с subject, data и headers
2. Сервер:
   - Валидирует запрос (subject не пустой)
   - Добавляет размер data в headers (`data-size`)
   - Вызывает Tarantool функцию `publish_message(subject, headers)`
3. Tarantool:
   - Генерирует уникальный sequence number
   - Создает object_name как `{{subject}}_{{sequence}}`
   - Сохраняет метаданные в space `message`
   - Возвращает sequence
4. Сервер:
   - Формирует object_name
   - Возвращает клиенту `PublishResponse`

## Разработка

### Внутренние пакеты

#### `internal/tarantool`
Клиент для работы с Tarantool:
- Подключение и управление соединением
- Вызов Lua функций через Call17 API
- Публикация сообщений

#### `internal/server`
Реализация gRPC сервиса:
- Обработка Publish RPC
- Валидация запросов
- Формирование ответов

### Добавление новых RPC методов

1. Обновите proto файл в `../model/publish.proto`
2. Перегенерируйте Go код: `cd ../model && make generate`
3. Добавьте реализацию в `internal/server/server.go`
4. Обновите Tarantool функции в `../tarantool/init.lua` если нужно

## Следующие шаги

- [ ] Добавить интеграцию с MinIO для хранения данных
- [ ] Добавить Egress API для чтения сообщений
- [ ] Добавить метрики и мониторинг
- [ ] Добавить graceful shutdown
- [ ] Добавить rate limiting

## Связанные компоненты

- [model](../model/) - Protobuf определения и сгенерированный код
- [tarantool](../tarantool/) - Конфигурация и схема Tarantool
- [example/publisher_client](../example/publisher_client/) - Пример клиента
