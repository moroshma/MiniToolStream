# Benchmark Setup Guide: MiniToolStream vs Kafka

## Что создано

Я создал полноценную систему для нагрузочного тестирования и сравнения MiniToolStream с Apache Kafka.

### Структура проекта

```
benchmarks/
├── README.md                          # Общая документация
├── SETUP_GUIDE.md                     # Данный файл
├── docker-compose.kafka.yml           # Kafka инфраструктура
│
├── minitoolstream/                    # Benchmarks для MiniToolStream
│   ├── cmd/
│   │   ├── bench-small/               # Benchmark для 10KB файлов
│   │   │   └── main.go
│   │   └── bench-large/               # Benchmark для 1GB файлов
│   │       └── main.go
│   ├── pkg/
│   │   └── metrics/                   # Сбор и анализ метрик
│   │       ├── collector.go
│   │       └── docker_monitor.go
│   └── configs/
│       ├── small-files.yaml           # Конфигурация для 10KB
│       └── large-files.yaml           # Конфигурация для 1GB
│
├── kafka/                             # Benchmarks для Kafka
│   ├── cmd/
│   │   ├── bench-small/               # Benchmark для 10KB файлов
│   │   │   └── main.go
│   │   └── bench-chunked/             # Для больших файлов (chunking)
│   ├── pkg/
│   │   └── metrics/                   # Сбор метрик (копия)
│   └── configs/
│       ├── small-files.yaml
│       └── large-files.yaml
│
├── comparative/                       # Сравнительный анализ
│   └── cmd/
│       └── analyze/                   # Генерация сравнительных отчетов
│           └── main.go
│
├── tools/
│   └── scripts/
│       ├── run-all-benchmarks.sh      # Автоматический запуск всех тестов
│       └── generate-test-files.sh     # Генерация тестовых данных
│
└── results/                           # Результаты тестов (JSON)
    ├── minitoolstream/
    └── kafka/
```

## Компоненты системы

### 1. Система сбора метрик (`pkg/metrics/`)

**collector.go** - Центральный коллектор метрик:
- Throughput (msg/sec, MB/sec)
- Latency (p50, p95, p99, min, max, avg)
- Resources (CPU, Memory, Disk I/O, Network)
- Error statistics
- JSON export результатов

**docker_monitor.go** - Мониторинг ресурсов через Docker:
- Автоматический сбор метрик контейнеров
- CPU, Memory, Network, Disk I/O
- Поддержка мультиконтейнерного мониторинга

### 2. MiniToolStream Benchmarks

**bench-small** - Тест маленьких файлов (10KB):
- Настраиваемое количество producers/consumers
- Rate limiting (target RPS)
- Параллельная отправка сообщений
- Детальные метрики latency

**bench-large** - Тест больших файлов (1GB):
- Генерация данных по частям (100MB chunks)
- Оптимизация использования памяти
- Измерение throughput в MB/s

### 3. Kafka Benchmarks

**bench-small** - Тест маленьких файлов (10KB):
- Использует IBM Sarama клиент
- Поддержка compression (gzip, snappy, lz4, zstd)
- Настраиваемые acks и timeout
- Автоматическое создание topics

**bench-chunked** - Для больших файлов:
- Разбивка на 10MB chunks (Kafka limitation)
- Simulation 1GB передачи

### 4. Comparative Analysis

**analyze** - Генератор сравнительных отчетов:
- Загрузка JSON результатов обеих систем
- Markdown таблицы сравнения
- Определение "победителя" по каждой метрике
- Детальные отчеты

## Быстрый старт

### Шаг 1: Запуск инфраструктуры

```bash
# 1. Запустите Docker Desktop (если еще не запущен)
open -a Docker

# 2. Запустите MiniToolStream (из корня проекта)
cd /Users/moroshma/go/DiplomaThesis/MiniToolStream
docker-compose up -d

# 3. Запустите Kafka
cd benchmarks
docker-compose -f docker-compose.kafka.yml up -d

# Подождите ~30 секунд пока Kafka полностью запустится
```

### Шаг 2: Запуск тестов MiniToolStream

#### Тест маленьких файлов (10KB)

```bash
cd minitoolstream/cmd/bench-small

# Инициализация (первый раз)
go mod init github.com/moroshma/benchmarks/minitoolstream/bench-small
go mod tidy

# Запуск теста
go run main.go -config=../../configs/small-files.yaml
```

#### Тест больших файлов (1GB)

```bash
cd ../bench-large

# Инициализация
go mod init github.com/moroshma/benchmarks/minitoolstream/bench-large
go mod tidy

# Запуск теста (ВНИМАНИЕ: занимает много времени!)
go run main.go -config=../../configs/large-files.yaml
```

### Шаг 3: Запуск тестов Kafka

#### Тест маленьких файлов (10KB)

```bash
cd ../../../kafka/cmd/bench-small

# Инициализация
go mod init github.com/moroshma/benchmarks/kafka/bench-small
go mod tidy

# Запуск теста
go run main.go -config=../../configs/small-files.yaml
```

### Шаг 4: Анализ результатов

```bash
cd ../../../comparative/cmd/analyze

# Инициализация
go mod init github.com/moroshma/benchmarks/comparative/analyze
go mod tidy

# Генерация отчета
go run main.go \
  -mts=../../results/minitoolstream \
  -kafka=../../results/kafka \
  -output=../../results/comparison-report.md

# Просмотр отчета
cat ../../results/comparison-report.md
```

## Собираемые метрики

### Throughput
- **Messages/sec**: Количество сообщений в секунду
- **MB/sec**: Пропускная способность в мегабайтах

### Latency
- **Min**: Минимальная задержка
- **Avg**: Средняя задержка
- **P50**: 50-й перцентиль
- **P95**: 95-й перцентиль (SLA metric)
- **P99**: 99-й перцентиль
- **Max**: Максимальная задержка

### Resources
- **CPU %**: Утилизация процессора
- **Memory MB**: Использование памяти
- **Disk I/O**: Чтение/запись на диск
- **Network I/O**: Входящий/исходящий трафик

### Errors
- **Error Count**: Количество ошибок
- **Error Rate**: Процент ошибочных запросов

## Конфигурация тестов

### Параметры small-files.yaml

```yaml
test:
  message_size: 10240       # 10KB
  total_messages: 10000     # Общее количество
  num_producers: 10         # Количество producers
  num_consumers: 5          # Количество consumers
  target_rps: 1000          # Целевой RPS
  duration: "5m"            # Максимальная длительность
  warmup: "10s"             # Warmup период
```

### Параметры large-files.yaml

```yaml
test:
  message_size: 1073741824  # 1GB
  total_messages: 10        # Только 10 файлов
  num_producers: 3          # Меньше producers
  num_consumers: 1          # Один consumer
  target_rps: 0             # Без rate limiting
  duration: "30m"           # Дольше timeout
```

## Автоматизация

### Запуск всех тестов одной командой

```bash
cd tools/scripts
./run-all-benchmarks.sh
```

Этот скрипт:
1. Проверяет инфраструктуру
2. Запускает MiniToolStream small files
3. Запускает Kafka small files
4. Запускает MiniToolStream large files
5. Генерирует итоговый отчет

## Примеры результатов

### JSON формат

```json
{
  "system": "minitoolstream",
  "test_name": "small-files-10kb",
  "timestamp": "2025-12-09T15:00:00Z",
  "throughput": {
    "msg_per_sec": 1234.56,
    "mb_per_sec": 12.34
  },
  "latency": {
    "p50_ms": 25300000,
    "p95_ms": 48700000,
    "p99_ms": 89200000
  },
  "resources": {
    "cpu_percent": 45.2,
    "memory_mb": 256.8
  },
  "errors": {
    "error_count": 0,
    "error_rate": 0
  }
}
```

### Markdown отчет

```markdown
| Metric | MiniToolStream | Kafka | Winner |
|--------|---------------|-------|--------|
| **Throughput (msg/s)** | 1234.56 | 980.23 | MTS |
| **Latency P95** | 48.7ms | 55.3ms | MTS |
| **CPU Usage** | 45.2% | 52.1% | MTS |
| **Memory Usage** | 256.8 MB | 312.4 MB | MTS |
```

## Troubleshooting

### Kafka не запускается

```bash
# Проверьте логи
docker-compose -f docker-compose.kafka.yml logs kafka

# Перезапустите
docker-compose -f docker-compose.kafka.yml down
docker-compose -f docker-compose.kafka.yml up -d
```

### Docker stats не работает

```bash
# Проверьте Docker daemon
docker info

# Проверьте права доступа
docker ps
```

### Go модули не резолвятся

```bash
# В каждом cmd директории:
go mod init <module-name>
go mod tidy
```

### Мониторинг не собирает метрики

Убедитесь что имена контейнеров в `configs/*.yaml` совпадают с реальными:

```bash
docker ps --format "{{.Names}}"
```

## Следующие шаги

1. **Запустите quick test**: Уменьшите `total_messages` до 100 для быстрой проверки
2. **Настройте параметры**: Подберите оптимальные значения RPS, producers
3. **Запустите полные тесты**: 10000 сообщений для статистической значимости
4. **Проанализируйте**: Используйте comparative analysis
5. **Визуализация**: Добавьте Grafana для real-time мониторинга (опционально)

## Ожидаемые преимущества MiniToolStream

На основе архитектуры ожидаем:

### Маленькие файлы (10KB)
- **Сопоставимая производительность** с Kafka
- **Меньше памяти** (Tarantool in-memory vs Kafka page cache)
- **Проще масштабирование** (stateless Ingress/Egress)

### Большие файлы (1GB)
- **Огромное преимущество**: Kafka требует chunking
- **Прямая поддержка**: MinIO S3-compatible storage
- **Простота использования**: Один API call vs множество chunks

## Производительность

Целевые метрики (из ТЗ):

| Метрика | Target | MiniToolStream | Kafka |
|---------|--------|----------------|-------|
| Throughput (10KB) | >= 1000 RPS | ? | ? |
| Latency P95 (10KB) | < 50ms | ? | ? |
| Message size limit | >= 1GB | ✅ Unlimited | ❌ 100MB max |
| Setup complexity | Low | ✅ Medium | ❌ High |

## Заключение

Эта система предоставляет:

✅ **Комплексное тестирование**: Small и large files
✅ **Автоматизация**: Скрипты для запуска и анализа
✅ **Детальные метрики**: Latency, throughput, resources
✅ **Сравнительный анализ**: Side-by-side comparison
✅ **Расширяемость**: Легко добавить новые тесты

Теперь у вас есть полный инструментарий для объективного сравнения MiniToolStream и Kafka!
