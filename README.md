# MiniToolStream

Легковесный message streaming сервис с метаданными в Tarantool и данными в MinIO.

## Архитектура

```
┌─────────────────────┐
│   gRPC Clients      │
│  (publisher_client) │
└──────────┬──────────┘
           │ gRPC
           ▼
┌─────────────────────┐      ┌──────────────┐
│ MiniToolStreamIngress│─────▶│  Tarantool   │
│   (gRPC Server)      │      │ (metadata)   │
└──────────┬──────────┘      └──────────────┘
           │
           ▼
      ┌─────────┐
      │  MinIO  │
      │ (data)  │
      └─────────┘
```

## Компоненты

- **MiniToolStreamIngress** - gRPC сервер для приема сообщений
- **Tarantool** - In-memory БД для метаданных сообщений
- **MinIO** - S3-совместимое хранилище для данных сообщений
- **model** - Protobuf определения gRPC API

## Быстрый старт

### 1. Запуск инфраструктуры

```bash
# Запуск Tarantool и MinIO
docker-compose up -d

# Проверка статуса
docker-compose ps
```

Сервисы:
- **Tarantool**: `localhost:3301`
- **MinIO API**: `localhost:9000`
- **MinIO Console**: http://localhost:9001 (админка)
  - Логин: `minioadmin`
  - Пароль: `minioadmin`

### 2. Запуск gRPC сервера

```bash
cd MiniToolStreamIngress/cmd/app
go build -o ingress-app .
./ingress-app
```

Сервер будет слушать на `localhost:50051`

### 3. Тестирование

```bash
cd example/publisher_client
go build -o publisher_client .
./publisher_client -subject terminator.diff -image tst.jpeg
```

## Структура проекта

```
MiniToolStream/
├── docker-compose.yml              # Инфраструктура (Tarantool + MinIO)
├── tarantool/
│   └── init.lua                    # Схема и функции Tarantool
├── model/
│   └── publish.proto               # gRPC API определения
├── MiniToolStreamIngress/
│   ├── cmd/server/                 # gRPC сервер
│   └── internal/                   # Внутренние пакеты
└── example/
    └── publisher_client/           # Пример клиента
```

## Docker сервисы

### Tarantool
```bash
# Подключение к консоли
docker-compose exec tarantool console

# Просмотр сообщений
box.space.message:select()

# Статистика
box.space.message:count()
```

### MinIO

**Веб-интерфейс**: http://localhost:9001
- Логин: `minioadmin`
- Пароль: `minioadmin`

**CLI**:
```bash
# Установка mc (MinIO Client)
brew install minio/stable/mc  # macOS
# или скачайте с https://min.io/docs/minio/linux/reference/minio-mc.html

# Конфигурация
mc alias set local http://localhost:9000 minioadmin minioadmin

# Список объектов
mc ls local/minitoolstream

# Скачать объект
mc cp local/minitoolstream/terminator.diff_38 ./
```

## Управление

### Запуск всех сервисов
```bash
docker-compose up -d
```

### Остановка
```bash
docker-compose down
```

### Просмотр логов
```bash
# Все сервисы
docker-compose logs -f

# Конкретный сервис
docker-compose logs -f tarantool
docker-compose logs -f minio
```

### Очистка данных
```bash
# Остановка и удаление volumes
docker-compose down -v
```

### Перезапуск сервиса
```bash
docker-compose restart tarantool
docker-compose restart minio
```

## Разработка

### Обновление Tarantool схемы

1. Отредактируйте `tarantool/init.lua`
2. Перезапустите:
```bash
docker-compose restart tarantool
# или для полного сброса данных
docker-compose down -v
docker-compose up -d
```

### Обновление gRPC API

1. Отредактируйте `model/publish.proto`
2. Регенерируйте код:
```bash
cd model
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       publish.proto
```
3. Пересоберите сервер и клиентов

## Порты

| Сервис | Порт | Описание |
|--------|------|----------|
| Tarantool | 3301 | Binary protocol |
| MinIO API | 9000 | S3-совместимый API |
| MinIO Console | 9001 | Веб-админка |
| Ingress gRPC | 50051 | gRPC API |

## Troubleshooting

### Tarantool не запускается
```bash
# Проверка логов
docker-compose logs tarantool

# Проверка синтаксиса init.lua
docker-compose exec tarantool tarantool -l /opt/tarantool/app.lua
```

### MinIO недоступен
```bash
# Проверка статуса
docker-compose ps minio

# Проверка healthcheck
docker inspect minitoolstream-minio | grep -A 10 Health
```

### gRPC сервер не подключается к Tarantool
```bash
# Проверка доступности
telnet localhost 3301

# Проверка через Docker
docker-compose exec tarantool tarantoolctl connect /var/run/tarantool/tarantool.sock
```

## Документация

- [MiniToolStreamIngress](MiniToolStreamIngress/README.md) - gRPC сервер
- [Tarantool Schema](tarantool/SCHEMA.md) - Схема БД
- [Publisher Client Example](example/publisher_client/README.md) - Пример клиента

## TODO

- [ ] Добавить сохранение data в MinIO
- [ ] Добавить Egress API для чтения сообщений
- [ ] Добавить мониторинг (Prometheus + Grafana)
- [ ] Добавить TLS для gRPC
- [ ] Добавить аутентификацию
