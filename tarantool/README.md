# Tarantool 2.11 для MiniToolStream

Этот каталог содержит конфигурацию и манифесты для развертывания Tarantool 2.11 в Kubernetes (Minikube) и локально через Docker Compose.

## Версия

**Tarantool 2.11** - стабильная LTS версия, оптимальная для standalone развертывания.

Для production с требованиями репликации и HA рекомендуется использовать Tarantool 3.x + Tarantool Kubernetes Operator. Подробнее см. [TARANTOOL_3_ISSUES.md](TARANTOOL_3_ISSUES.md).

## Структура проекта

```
tarantool/
├── init.lua                          # Скрипт инициализации Tarantool
├── docker-compose.yml                # Docker Compose для локальной разработки
├── test_new_schema.go                # Go тесты для новой схемы
├── test_persistence.sh               # Bash скрипт для тестирования persistence
├── k8s/                              # Kubernetes манифесты
│   ├── namespace.yaml                # Namespace minitoolstream
│   ├── configmap.yaml                # ConfigMap с init.lua
│   ├── statefulset.yaml              # StatefulSet (1 pod)
│   ├── statefulset-multi-pods.yaml   # StatefulSet (3 пода)
│   └── service.yaml                  # Services для доступа
└── docs/                             # Документация
    ├── README.md                     # Этот файл
    ├── SCHEMA.md                     # Подробное описание схемы
    ├── QUICKSTART_MACOS.md           # Быстрый старт на macOS
    ├── MINIKUBE_GUIDE.md             # Гайд по развертыванию в Minikube
    ├── MULTI_POD_DEPLOYMENT.md       # Multi-pod развертывание
    ├── DEPLOYMENT_STATUS.md          # Текущий статус развертывания
    └── TARANTOOL_3_ISSUES.md         # Проблемы с Tarantool 3.x
```

## Схема данных

### Space 1: message

Хранит метаданные сообщений:

| Поле | Тип | Описание |
|------|-----|----------|
| `sequence` | unsigned | Глобальный уникальный номер сообщения (PK) |
| `headers` | map | Метаданные сообщения (произвольные поля) |
| `object_name` | string | Путь к объекту в MinIO/S3 |
| `subject` | string | Канал/топик сообщения |
| `create_at` | unsigned | Unix timestamp создания (для TTL cleanup) |

**Индексы:**
- `primary`: `sequence` (unique, TREE)
- `subject`: `subject` (non-unique, TREE)
- `subject_sequence`: `(subject, sequence)` (unique, TREE) - для range queries
- `create_at`: `create_at` (non-unique, TREE) - для cleanup по TTL

### Space 2: consumers

Хранит позицию чтения для каждого consumer:

| Поле | Тип | Описание |
|------|-----|----------|
| `durable_name` | string | Имя consumer group (часть PK) |
| `subject` | string | Канал подписки (часть PK) |
| `last_sequence` | unsigned | Последний прочитанный sequence |

**Индексы:**
- `primary`: `(durable_name, subject)` (unique, TREE)
- `subject`: `subject` (non-unique, TREE)

Подробное описание схемы: [SCHEMA.md](SCHEMA.md)

## API функции

### Базовые функции

#### Публикация сообщений

```lua
-- Опубликовать сообщение
-- Возвращает: sequence (unsigned)
publish_message(subject, object_name, headers)

-- Пример:
local seq = publish_message("orders", "minio/orders/123", {content_type = "json", size = 1024})
```

#### Получение сообщений

```lua
-- Получить сообщение по sequence
-- Возвращает: tuple или nil
get_message_by_sequence(sequence)

-- Получить сообщения по subject
-- Возвращает: array of tuples
get_messages_by_subject(subject, start_sequence, limit)

-- Получить последний sequence для subject
-- Возвращает: unsigned (0 если сообщений нет)
get_latest_sequence_for_subject(subject)
```

#### Управление consumers

```lua
-- Обновить позицию consumer
-- Возвращает: true
update_consumer_position(durable_name, subject, last_sequence)

-- Получить позицию consumer
-- Возвращает: unsigned (0 если consumer не найден)
get_consumer_position(durable_name, subject)

-- Получить всех consumers для subject
-- Возвращает: array of tuples
get_consumers_by_subject(subject)
```

### Функции для gRPC API

Специальные функции для поддержки gRPC сервисов (IngressService и EgressService):

```lua
-- IngressService.Publish (MinIO mode)
grpc_publish(subject, object_name, headers)
-- Возвращает: {sequence, status_code, error_message}

-- IngressService.Publish (MessagePack inline mode) 🆕
grpc_publish_msgpack(subject, data_msgpack, headers)
-- Возвращает: {sequence, status_code, error_message}
-- data_msgpack: бинарные MessagePack данные

-- EgressService.GetLastSequence
grpc_get_last_sequence(subject)
-- Возвращает: {last_sequence}

-- EgressService.Fetch (standard)
grpc_fetch(subject, durable_name, batch_size, auto_ack)
-- Возвращает: array of message tuples

-- EgressService.Fetch (MessagePack mode) 🆕
grpc_fetch_msgpack(subject, durable_name, batch_size, auto_ack)
-- Возвращает: array of message tables (structured)
-- auto_ack: true - автоматически обновить позицию consumer

-- EgressService.Subscribe (поддержка)
check_new_messages(subject, consumer_group)
-- Возвращает: {has_new, latest_sequence, consumer_position, new_count}

-- Ручное подтверждение (acknowledge)
grpc_ack(durable_name, subject, sequence)
-- Возвращает: boolean (success)

-- Предпросмотр без изменения позиции
grpc_peek(subject, durable_name, batch_size)
-- Возвращает: array of message tuples

-- Счетчик новых сообщений
get_new_messages_count(subject, durable_name, since_sequence)
-- Возвращает: unsigned (count)
```

**📦 MessagePack Support:**
- Храните данные прямо в Tarantool (для сообщений < 1MB)
- Компактнее JSON на 30-50%
- Быстрая сериализация/десериализация
- См. [MSGPACK_SUPPORT.md](MSGPACK_SUPPORT.md) для деталей

**Подробнее:** См. [GRPC_API_MAPPING.md](GRPC_API_MAPPING.md) для полного маппинга gRPC методов на функции Tarantool.

### Cleanup (TTL)

```lua
-- Удалить старые сообщения
-- Возвращает: deleted_count, array of deleted_messages
delete_old_messages(ttl_seconds)

-- Пример: удалить сообщения старше 7 дней
local count, deleted = delete_old_messages(7 * 24 * 60 * 60)
```

## Быстрый старт

### 1. Docker Compose (для локальной разработки)

```bash
# Запустить Tarantool
docker-compose up -d

# Проверить логи
docker-compose logs -f tarantool

# Остановить
docker-compose down
```

### 2. Kubernetes / Minikube (для тестирования репликации)

```bash
# Запустить Minikube
minikube start --memory=4096 --cpus=2

# Развернуть Tarantool с репликацией
make deploy

# Проверить статус
make status

# Открыть Dashboard
minikube dashboard
```

Подробные инструкции: [QUICKSTART_MACOS.md](QUICKSTART_MACOS.md)

## Разработка и редактирование

### Редактирование init.lua

**Важно:** Теперь код НЕ дублируется в ConfigMap!

1. **Редактируйте файлы напрямую:**
   - `init-master.lua` - для master пода
   - `init-replica.lua` - для replica подов

2. **Примените изменения:**
```bash
make update
```

Это автоматически:
- Пересоздаст ConfigMap из локальных файлов
- Перезапустит поды

3. **Проверьте:**
```bash
make status
make logs-master
```

Подробнее: [WORKFLOW.md](WORKFLOW.md)

## Текущее развертывание

**Статус:** 3 пода запущены в Minikube

```
NAME          STATUS    IP            NODE
tarantool-0   Running   10.244.0.16   minikube
tarantool-1   Running   10.244.0.17   minikube
tarantool-2   Running   10.244.0.18   minikube
```

**Хранилище:** 3 × 5Gi PVC (15Gi total)

Подробности: [DEPLOYMENT_STATUS.md](DEPLOYMENT_STATUS.md)

## Тестирование

### Go клиент

```bash
# Запустить port-forward (в отдельном терминале)
kubectl port-forward -n minitoolstream svc/tarantool-external 3301:3301

# Запустить тесты
go run test_new_schema.go
```

### Bash скрипт

```bash
chmod +x test_persistence.sh
./test_persistence.sh
```

### Консоль Tarantool

```bash
# Docker Compose
docker exec -it minitoolstream-tarantool tarantoolctl connect localhost:3301

# Kubernetes
kubectl exec -it tarantool-0 -n minitoolstream -- tarantoolctl connect localhost:3301
```

В консоли:

```lua
-- Опубликовать сообщение
seq = publish_message("test", "minio/test/1", {type = "test"})

-- Получить сообщение
get_message_by_sequence(seq)

-- Получить все сообщения по subject
get_messages_by_subject("test", 0, 10)

-- Обновить позицию consumer
update_consumer_position("consumer-1", "test", seq)

-- Получить позицию
get_consumer_position("consumer-1", "test")
```

## Конфигурация

### Persistence (WAL)

```lua
box.cfg {
    memtx_memory = 1024 * 1024 * 1024,  -- 1GB
    wal_mode = 'write',                  -- fsync после каждой записи
    wal_dir_rescan_delay = 2,
    log_level = 5
}
```

**WAL включен** - все данные сохраняются на диск и восстанавливаются после перезапуска.

### Ресурсы (Kubernetes)

```yaml
resources:
  requests:
    memory: "512Mi"
    cpu: "250m"
  limits:
    memory: "1Gi"
    cpu: "500m"
```

### Пользователи

**Admin:**
- User: `admin`
- Password: `secret` (⚠️ изменить для production!)

**Application:**
- User: `minitoolstream`
- Password: `changeme` (⚠️ изменить для production!)

## Масштабирование

### Увеличить количество подов

```bash
kubectl scale statefulset tarantool --replicas=5 -n minitoolstream
```

### Уменьшить

```bash
kubectl scale statefulset tarantool --replicas=1 -n minitoolstream
```

## Мониторинг

### Логи

```bash
# Конкретный pod
kubectl logs -f tarantool-0 -n minitoolstream

# Все поды
kubectl logs -l app=tarantool -n minitoolstream
```

### Метрики

```bash
# Использование ресурсов
kubectl top pods -n minitoolstream

# События
kubectl get events -n minitoolstream --sort-by='.lastTimestamp'
```

### Dashboard

```bash
minikube dashboard
```

Навигация: Namespace: `minitoolstream` → Workloads → Pods

## Важные команды

### Пересоздание с сохранением данных

```bash
kubectl delete statefulset tarantool -n minitoolstream
kubectl apply -f k8s/statefulset-multi-pods.yaml
```

PVC останутся, данные сохранятся.

### Полное удаление

```bash
kubectl delete -f k8s/
kubectl delete pvc --all -n minitoolstream
```

⚠️ **Все данные будут потеряны!**

### Обновление конфигурации

```bash
# Изменить init.lua в configmap.yaml
kubectl apply -f k8s/configmap.yaml

# Перезапустить поды
kubectl rollout restart statefulset/tarantool -n minitoolstream
```

## Архитектура развертывания

### Текущая (3 независимых пода)

```
┌─────────────────────────────────────────┐
│      Minikube Node (1 машина)          │
│                                         │
│  ┌──────────┐  ┌──────────┐            │
│  │tarantool-0│  │tarantool-1│          │
│  │PVC: 5Gi  │  │PVC: 5Gi  │            │
│  └──────────┘  └──────────┘            │
│                                         │
│  ┌──────────┐                          │
│  │tarantool-2│                         │
│  │PVC: 5Gi  │                          │
│  └──────────┘                          │
└─────────────────────────────────────────┘
```

**Особенности:**
- ✅ Каждый pod независим (свои данные)
- ✅ Простая настройка
- ❌ Нет репликации между подами
- ❌ Нет автоматического failover

**Подходит для:**
- Разработки и тестирования
- MVP и proof-of-concept
- Независимых инстансов

## Production рекомендации

Для production с высокой доступностью:

### Вариант 1: Tarantool 3.x + Operator

```bash
# Установить Tarantool Operator
kubectl apply -f https://github.com/tarantool/tarantool-operator/releases/latest/download/tarantool-operator.yaml

# Создать кластер
kubectl apply -f tarantool-cluster.yaml
```

**Преимущества:**
- ✅ Автоматическая репликация
- ✅ Automatic failover
- ✅ Управление топологией
- ✅ Встроенный мониторинг

Подробнее: https://github.com/tarantool/tarantool-operator

### Вариант 2: Tarantool Cartridge

Для sharding и сложной топологии.

Подробнее: https://www.tarantool.io/en/doc/latest/book/cartridge/

## Следующие шаги

Для полной реализации MiniToolStream:

1. **MinIO** - хранилище для payload сообщений
2. **MiniToolStreamIngress** - gRPC сервис публикации
3. **MiniToolStreamEgress** - gRPC сервис потребления
4. **Cleaner** - сервис для TTL cleanup
5. **HashiCorp Vault** - управление секретами

## Документация

### Внутренняя

- [SCHEMA.md](SCHEMA.md) - Подробное описание схемы данных
- [GRPC_API_MAPPING.md](GRPC_API_MAPPING.md) - Маппинг gRPC методов на функции Tarantool
- [MSGPACK_SUPPORT.md](MSGPACK_SUPPORT.md) - Поддержка MessagePack формата
- [QUICK_START.md](QUICK_START.md) - Быстрый старт

### Внешняя

- [Tarantool Documentation](https://www.tarantool.io/en/doc/latest/)
- [Go Tarantool Driver](https://github.com/tarantool/go-tarantool)
- [Tarantool Kubernetes Operator](https://github.com/tarantool/tarantool-operator)

## Troubleshooting

### Pod не запускается

```bash
# Проверить события
kubectl describe pod tarantool-0 -n minitoolstream

# Проверить логи
kubectl logs tarantool-0 -n minitoolstream
```

### PVC в статусе Pending

```bash
# Проверить storage provisioner
kubectl get storageclass

# Включить provisioner в Minikube
minikube addons enable storage-provisioner
```

### Ошибка подключения

```bash
# Проверить что pod запущен
kubectl get pods -n minitoolstream

# Проверить port-forward
kubectl port-forward -n minitoolstream svc/tarantool-external 3301:3301

# Проверить логи
kubectl logs -f tarantool-0 -n minitoolstream
```

### Данные не сохраняются

```bash
# Проверить PVC
kubectl get pvc -n minitoolstream

# Проверить WAL в логах
kubectl logs tarantool-0 -n minitoolstream | grep -i wal
```

---

**Tarantool 2.11 готов к использованию! 🚀**

Все 3 пода работают, данные персистентны, схема настроена, API протестировано.
