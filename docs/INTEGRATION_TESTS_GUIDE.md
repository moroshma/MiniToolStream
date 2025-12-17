# Руководство по запуску интеграционных тестов

**Дата:** 17 декабря 2025

---

## Архитектура интеграционных тестов

```
tests/integration/
├── auth_test.go              # Тесты JWT авторизации и permissions
└── error_scenarios_test.go   # Тесты сценариев ошибок
```

### Зависимости

Интеграционные тесты требуют запущенной инфраструктуры:
- **Vault** (localhost:8200) - JWT токены
- **Tarantool** (localhost:3301) - база данных метаданных
- **MinIO** (localhost:9000) - объектное хранилище
- **Ingress Service** (localhost:50051) - gRPC сервис публикации
- **Egress Service** (localhost:50052) - gRPC сервис чтения

---

## Шаг 1: Запуск инфраструктуры

### Вариант A: Docker Compose (рекомендуется)

```bash
cd /Users/moroshma/go/DiplomaThesis/MiniToolStream

# Запуск всех сервисов
docker-compose up -d

# Проверка статуса
docker-compose ps

# Ожидаемый вывод:
# NAME                              STATUS
# minitoolstream-egress             Up
# minitoolstream-ingress            Up
# minitoolstream_connector-minio    Up (healthy)
# minitoolstream_connector-tarantool Up (healthy)
# minitoolstream_connector-vault    Up (healthy)
```

### Проверка готовности сервисов

```bash
# Vault
curl http://localhost:8200/v1/sys/health

# MinIO
curl http://localhost:9000/minio/health/live

# Tarantool (если установлен tarantoolctl)
echo "box.info.status" | tarantoolctl connect localhost:3301

# Ingress Service
grpcurl -plaintext localhost:50051 list

# Egress Service
grpcurl -plaintext localhost:50052 list
```

---

## Шаг 2: Запуск интеграционных тестов

### Запуск всех интеграционных тестов

```bash
cd /Users/moroshma/go/DiplomaThesis/MiniToolStream/tests/integration

# Запуск всех тестов
go test -v

# Запуск с подробным выводом
go test -v -count=1
```

### Запуск отдельных тестов

**Тест 1: JWT авторизация**
```bash
go test -v -run TestAuthJWTValidation
```

Покрываемые сценарии:
- ✅ Валидный токен принимается
- ✅ Невалидная подпись отклоняется
- ✅ Истекший токен отклоняется
- ✅ Отсутствие токена отклоняется

**Тест 2: Проверка permissions**
```bash
go test -v -run TestAuthPermissions
```

Покрываемые сценарии:
- ✅ Publish без permission "publish" отклоняется
- ✅ Доступ к недозволенному subject отклоняется

**Тест 3: Сценарии ошибок**
```bash
go test -v -run TestErrorScenarios
```

Покрываемые сценарии:
- ✅ Connection timeout
- ✅ Invalid server address
- ✅ Invalid request data
- ✅ Empty subject rejection
- ✅ Large data handling

---

## Шаг 3: Просмотр результатов

### Ожидаемый вывод успешных тестов

```bash
=== RUN   TestAuthJWTValidation
================================================================================
ИНТЕГРАЦИОННЫЙ ТЕСТ 1: JWT VALIDATION (Проверка валидации токенов)
================================================================================

▶ Подтест 1.1: Валидный токен должен быть принят
  ✓ Токен сгенерирован для client_id: test-client-valid
  ✓ Разрешенные subjects: test.*
  ✓ Permissions: publish, subscribe, fetch
  ✓ Publish запрос успешен
  ✓ Sequence получен: 1
--- PASS: TestAuthJWTValidation/Valid_Token_Success (0.05s)

▶ Подтест 1.2: Невалидная подпись должна быть отклонена
  ✓ Publish с невалидным токеном отклонен
  ✓ Код ошибки: Unauthenticated
--- PASS: TestAuthJWTValidation/Invalid_Signature (0.02s)

...

PASS
ok      integration     2.134s
```

### Логи сервисов

```bash
# Просмотр логов всех сервисов
docker-compose logs -f

# Только Ingress
docker-compose logs -f ingress

# Только Egress
docker-compose logs -f egress

# Только Vault
docker-compose logs -f vault
```

---

## Шаг 4: Остановка инфраструктуры

```bash
# Остановка всех сервисов
docker-compose down

# Остановка с удалением volumes (полная очистка)
docker-compose down -v
```

---

## Troubleshooting

### Проблема: Тесты пропускаются (SKIP)

**Причина:** Сервисы не доступны

**Решение:**
```bash
# Проверить запущены ли сервисы
docker-compose ps

# Перезапустить инфраструктуру
docker-compose restart

# Просмотреть логи для диагностики
docker-compose logs
```

### Проблема: Connection refused

**Причина:** Сервисы еще не готовы

**Решение:**
```bash
# Подождать пока все healthcheck'и пройдут
docker-compose ps

# Проверить порты
netstat -an | grep -E "3301|8200|9000|50051|50052"

# Перезапустить конкретный сервис
docker-compose restart ingress
```

### Проблема: JWT validation error

**Причина:** Vault не инициализирован или секреты не загружены

**Решение:**
```bash
# Проверить Vault
export VAULT_ADDR=http://localhost:8200
export VAULT_TOKEN=dev-root-token

vault kv get secret/minitoolstream/jwt

# Если секрета нет, создать его
vault kv put secret/minitoolstream/jwt \
    private_key="$(cat vault/jwt_private.pem)" \
    public_key="$(cat vault/jwt_public.pem)"
```

### Проблема: Tarantool connection failed

**Причина:** Схема не инициализирована

**Решение:**
```bash
# Пересоздать Tarantool контейнер
docker-compose stop tarantool
docker volume rm minitoolstream_tarantool-data
docker-compose up -d tarantool

# Проверить логи инициализации
docker-compose logs tarantool
```

---

## Структура тестов

### auth_test.go (8 подтестов)

| Тест | Описание | Ожидаемый результат |
|------|----------|---------------------|
| **Valid_Token_Success** | Валидный JWT токен | ✅ 200 OK |
| **Invalid_Signature** | Невалидная подпись токена | ✅ Unauthenticated |
| **Expired_Token** | Истекший токен | ✅ Unauthenticated |
| **Missing_Token** | Отсутствие токена | ✅ Unauthenticated |
| **Publish_Permission_Denied** | Нет permission "publish" | ✅ PermissionDenied |
| **Subject_Access_Denied** | Доступ к недозволенному subject | ✅ PermissionDenied |
| **Subscribe_Permission_Check** | Проверка permission "subscribe" | ✅ PermissionDenied |
| **Fetch_Permission_Check** | Проверка permission "fetch" | ✅ PermissionDenied |

### error_scenarios_test.go (6 подтестов)

| Тест | Описание | Ожидаемый результат |
|------|----------|---------------------|
| **Invalid_Server_Address** | Подключение к несуществующему серверу | ✅ Connection error |
| **Connection_Timeout** | Таймаут подключения | ✅ Deadline exceeded |
| **Empty_Subject_Validation** | Пустой subject в Publish | ✅ InvalidArgument |
| **Invalid_Request_Data** | Nil message в Publish | ✅ InvalidArgument |
| **Large_Data_Handling** | Очень большой payload (10MB) | ✅ Success или ResourceExhausted |
| **Context_Cancellation** | Отмена операции через context | ✅ Canceled |

---

## Переменные окружения для тестов

Можно переопределить адреса сервисов:

```bash
# Установить переменные
export INGRESS_ADDR=localhost:50051
export EGRESS_ADDR=localhost:50052
export VAULT_ADDR=http://localhost:8200
export VAULT_TOKEN=dev-root-token

# Запустить тесты
go test -v
```

---

## CI/CD Integration

### GitHub Actions

```yaml
name: Integration Tests

on: [push, pull_request]

jobs:
  integration-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.24'
      
      - name: Start infrastructure
        run: docker-compose up -d
      
      - name: Wait for services
        run: |
          timeout 60 bash -c 'until docker-compose ps | grep healthy; do sleep 1; done'
      
      - name: Run integration tests
        working-directory: tests/integration
        run: go test -v -count=1
      
      - name: Stop infrastructure
        if: always()
        run: docker-compose down -v
```

---

## Метрики и покрытие

### Запуск с покрытием кода

```bash
go test -v -cover -coverprofile=coverage.out

# Просмотр покрытия
go tool cover -html=coverage.out
```

### Benchmark тесты

```bash
go test -v -bench=. -benchmem
```

---

**Автор:** Claude Code  
**Версия:** 1.0  
**Дата обновления:** 17.12.2025

