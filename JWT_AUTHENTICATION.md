# JWT Authentication для MiniToolStream

Этот документ описывает как настроить и использовать JWT аутентификацию для MiniToolStream.

## Архитектура

MiniToolStream поддерживает JWT (JSON Web Token) аутентификацию с использованием HashiCorp Vault для хранения ключей и токенов.

### Компоненты

1. **JWT Manager** (`pkg/auth/jwt.go`) - Управление генерацией и валидацией JWT токенов
2. **gRPC Interceptors** - Server и Client interceptors для автоматической аутентификации
3. **Vault Integration** - Хранение RSA ключей и токенов в Vault
4. **CLI Tool** (`tools/jwt-gen`) - Утилита для генерации токенов

## Быстрый старт

### 1. Генерация RSA ключей

Первым делом нужно сгенерировать RSA ключи и сохранить их в Vault:

```bash
cd tools/jwt-gen
go run main.go \
  -vault-addr=$VAULT_ADDR \
  -vault-token=$VAULT_TOKEN \
  -generate-keys
```

Ключи будут сохранены в Vault по пути `secret/data/minitoolstream/jwt`.

### 2. Генерация JWT токена

Сгенерируйте JWT токен для клиента:

```bash
go run main.go \
  -vault-addr=$VAULT_ADDR \
  -vault-token=$VAULT_TOKEN \
  -client="publisher-client-1" \
  -subjects="images.*,logs.*" \
  -permissions="publish,subscribe,fetch" \
  -duration=24h
```

Параметры:
- `-client` - Уникальный ID клиента (обязательно)
- `-subjects` - Разрешенные subjects через запятую. Поддерживаются wildcards (`*`, `images.*`, etc.)
- `-permissions` - Разрешения: `publish`, `subscribe`, `fetch`, или `*` для всех
- `-duration` - Срок действия токена (по умолчанию 24h)

### 3. Сохранение токена в Vault (опционально)

Сохраните сгенерированный токен в Vault для использования клиентами:

```bash
vault kv put secret/minitoolstream/tokens/publisher-client-1 token="<JWT_TOKEN>"
```

### 4. Настройка серверов

#### MiniToolStreamIngress

Добавьте в `k8s/configmap.yaml` или переменные окружения:

```yaml
auth:
  enabled: true
  jwt_vault_path: "secret/data/minitoolstream/jwt"
  jwt_issuer: "minitoolstream"
  require_auth: true  # false для опциональной аутентификации
```

Переменные окружения:
```bash
AUTH_ENABLED=true
JWT_VAULT_PATH=secret/data/minitoolstream/jwt
JWT_ISSUER=minitoolstream
REQUIRE_AUTH=true
```

#### MiniToolStreamEgress

Аналогичная конфигурация как для Ingress.

### 5. Использование в клиентах

#### Publisher Client

**С токеном напрямую:**

```go
pub, err := minitoolstream_connector.NewPublisherBuilder(serverAddr).
    WithJWTToken("eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...").
    Build()
```

**С токеном из Vault:**

```yaml
# config.yaml
client:
  server_address: "localhost:50051"
  jwt_vault_path: "secret/data/minitoolstream/tokens/publisher-client-1"

vault:
  enabled: true
  address: "http://localhost:8200"
  token: "${VAULT_TOKEN}"
```

Или через переменные окружения:
```bash
JWT_TOKEN="eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9..."
# или
JWT_VAULT_PATH="secret/data/minitoolstream/tokens/publisher-client-1"
```

#### Subscriber Client

```go
sub, err := minitoolstream_connector.NewSubscriberBuilder(serverAddr).
    WithJWTToken("eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...").
    WithDurableName("my-subscriber").
    Build()
```

## Permissions и Subject Patterns

### Permissions

- `publish` - Разрешает публикацию сообщений
- `subscribe` - Разрешает подписку на уведомления
- `fetch` - Разрешает получение сообщений
- `*` - Все разрешения

### Subject Patterns

Поддерживаются wildcard паттерны:

- `*` - Все subjects
- `images.*` - Все subjects начинающиеся с `images.` (например `images.jpeg`, `images.png`)
- `logs.system.*` - Все subjects начинающиеся с `logs.system.`
- `exact.match` - Точное совпадение

**Примеры:**

```bash
# Полный доступ
-subjects="*" -permissions="*"

# Только публикация изображений
-subjects="images.*" -permissions="publish"

# Чтение логов и метрик
-subjects="logs.*,metrics.*" -permissions="subscribe,fetch"

# Специфические subjects
-subjects="users.created,users.deleted" -permissions="subscribe"
```

## Deployment в Kubernetes

### 1. Создайте Secrets для Vault

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: vault-token
  namespace: minitoolstream
type: Opaque
stringData:
  token: ${VAULT_TOKEN}
```

### 2. Обновите ConfigMaps

Для Ingress (`MiniToolStreamIngress/k8s/configmap.yaml`):

```yaml
auth:
  enabled: true
  jwt_vault_path: "secret/data/minitoolstream/jwt"
  jwt_issuer: "minitoolstream"
  require_auth: true
```

Для Egress (`MiniToolStreamEgress/k8s/configmap.yaml`):

```yaml
auth:
  enabled: true
  jwt_vault_path: "secret/data/minitoolstream/jwt"
  jwt_issuer: "minitoolstream"
  require_auth: true
```

### 3. Обновите Deployments

В переменных окружения убедитесь что Vault включен:

```yaml
env:
  - name: VAULT_ENABLED
    value: "true"
  - name: VAULT_ADDR
    value: "http://vault.minitoolstream.svc.cluster.local:8200"
  - name: VAULT_TOKEN
    valueFrom:
      secretKeyRef:
        name: vault-token
        key: token
  - name: AUTH_ENABLED
    value: "true"
```

## Тестирование

### 1. Тест без аутентификации (должен упасть)

```bash
./publisher_client -subject "test.hello" -data "Hello World"
```

Ожидается: `rpc error: code = Unauthenticated desc = missing authorization header`

### 2. Тест с валидным токеном

```bash
export JWT_TOKEN="<ваш_токен>"
./publisher_client -subject "test.hello" -data "Hello World"
```

Ожидается: Успешная публикация

### 3. Тест с неправильными permissions

Создайте токен только с `subscribe` permission и попробуйте опубликовать:

```bash
go run tools/jwt-gen/main.go \
  -vault-addr=$VAULT_ADDR \
  -vault-token=$VAULT_TOKEN \
  -client="test-client" \
  -subjects="*" \
  -permissions="subscribe"

export JWT_TOKEN="<токен_только_для_subscribe>"
./publisher_client -subject "test.hello" -data "Hello World"
```

Ожидается: `rpc error: code = PermissionDenied desc = access denied`

### 4. Тест с неправильным subject

```bash
go run tools/jwt-gen/main.go \
  -vault-addr=$VAULT_ADDR \
  -vault-token=$VAULT_TOKEN \
  -client="test-client" \
  -subjects="images.*" \
  -permissions="publish"

export JWT_TOKEN="<токен_только_для_images>"
./publisher_client -subject "logs.error" -data "Error log"
```

Ожидается: `rpc error: code = PermissionDenied desc = access denied to subject logs.error`

## Troubleshooting

### JWT Manager не может загрузить ключи из Vault

Проверьте:
1. Vault доступен: `vault status`
2. Путь правильный: `vault kv get secret/minitoolstream/jwt`
3. Токен Vault имеет правильные permissions

### Клиент получает "invalid token"

Проверьте:
1. Токен не истек (проверьте `exp` claim)
2. Issuer совпадает в токене и сервере
3. RSA ключи одинаковые на сервере и при генерации токена

### Access denied несмотря на правильный токен

Проверьте:
1. Subject pattern в токене покрывает запрашиваемый subject
2. Permission включает требуемое действие (`publish`, `subscribe`, или `fetch`)

## Безопасность

### Best Practices

1. **Короткий срок жизни токенов**: Используйте `-duration=1h` или меньше для продакшена
2. **Ротация ключей**: Периодически генерируйте новые RSA ключи
3. **Минимальные permissions**: Давайте только необходимые permissions
4. **Ограничение subjects**: Используйте специфические паттерны вместо `*`
5. **Vault Policies**: Настройте правильные Vault policies для доступа к ключам и токенам
6. **TLS**: Используйте TLS для gRPC соединений в продакшене

### Пример минимальных прав

Для publisher только изображений:
```bash
-client="image-publisher" \
-subjects="images.jpeg,images.png" \
-permissions="publish" \
-duration=1h
```

Для subscriber только логов:
```bash
-client="log-subscriber" \
-subjects="logs.*" \
-permissions="subscribe,fetch" \
-duration=1h
```

## Опциональная аутентификация

Если установить `require_auth: false`, сервер будет:
- Принимать запросы с JWT токеном (и валидировать его)
- Принимать запросы без токена
- Логировать все аутентифицированные и неаутентифицированные запросы

Это полезно для постепенной миграции на JWT без breaking changes.

## API Reference

### JWTManager

```go
// Создание с ключами из Vault
jwtManager, err := auth.NewJWTManagerFromVault(ctx, vaultClient, vaultPath, issuer)

// Генерация токена
token, err := jwtManager.GenerateToken(clientID, allowedSubjects, permissions, duration)

// Валидация токена
claims, err := jwtManager.ValidateToken(tokenString)

// Сохранение ключей в Vault
err := jwtManager.SaveKeysToVault(ctx, vaultClient)
```

### Claims

```go
type Claims struct {
    ClientID        string   // Уникальный ID клиента
    AllowedSubjects []string // Паттерны разрешенных subjects
    Permissions     []string // Разрешения: publish, subscribe, fetch
    jwt.RegisteredClaims
}

// Проверка доступа
err := claims.ValidatePublishAccess("images.jpeg")
err := claims.ValidateSubscribeAccess("logs.system")
err := claims.ValidateFetchAccess("metrics.cpu")
```

## Миграция с неаутентифицированной системы

1. Включите `require_auth: false` на серверах
2. Начните выдавать JWT токены клиентам
3. Обновите клиенты для использования токенов
4. Мониторьте логи на неаутентифицированные запросы
5. Когда все клиенты обновлены, установите `require_auth: true`
