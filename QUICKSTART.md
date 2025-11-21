# Quick Start Guide

## 1. Запуск всех сервисов

```bash
# Из корня проекта
docker-compose up -d

# Проверка
docker-compose ps
```

## 2. Доступ к сервисам

### MinIO Console (Админка)
- **URL**: http://localhost:9001
- **Логин**: `minioadmin`
- **Пароль**: `minioadmin`

### MinIO API
- **URL**: http://localhost:9000
- **Bucket**: `minitoolstream`

### Tarantool
- **Host**: `localhost:3301`
- **User**: `minitoolstream`
- **Password**: `changeme`

### gRPC Server
- **Host**: `localhost:50051`

## 3. Запуск gRPC сервера

```bash
cd MiniToolStreamIngress/cmd/app
go build -o ingress-app .
./ingress-app
```

## 4. Тест публикации

```bash
cd example/publisher_client
go build -o publisher_client .
./publisher_client
```

## 5. Просмотр данных

### Tarantool
```bash
docker-compose exec tarantool console
```
```lua
-- Все сообщения
box.space.message:select()

-- Количество
box.space.message:count()

-- По subject
box.space.message.index.subject:select('terminator.diff')
```

### MinIO
Откройте http://localhost:9001 и войдите с credentials выше.

## 6. Остановка

```bash
docker-compose down
```

## 7. Полная очистка

```bash
docker-compose down -v  # Удаляет volumes с данными
```
