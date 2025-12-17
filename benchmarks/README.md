# Benchmarks: MiniToolStream vs Kafka

Комплексная система нагрузочного тестирования для сравнения производительности MiniToolStream и Apache Kafka.

## Архитектура

```
benchmarks/
├── minitoolstream/          # Benchmarks для MiniToolStream
│   ├── cmd/
│   │   ├── bench-small/     # 10KB файлы
│   │   └── bench-large/     # 1GB файлы
│   ├── pkg/
│   │   ├── metrics/         # Сбор метрик
│   │   └── reporter/        # Генерация отчетов
│   └── configs/             # Конфигурации тестов
├── kafka/                   # Benchmarks для Kafka
│   ├── cmd/
│   │   ├── bench-small/     # 10KB файлы
│   │   └── bench-chunked/   # Chunked для больших файлов
│   ├── pkg/
│   │   ├── metrics/         # Сбор метрик
│   │   └── reporter/        # Генерация отчетов
│   └── configs/             # Конфигурации тестов
├── comparative/             # Сравнительный анализ
│   └── cmd/
│       ├── analyze/         # Анализатор результатов
│       └── visualize/       # Генератор графиков
├── tools/
│   ├── scripts/             # Automation scripts
│   └── ghz/                 # ghz конфигурации
├── data/
│   └── test-files/          # Тестовые данные
└── results/                 # Результаты тестов
    ├── minitoolstream/
    └── kafka/
```

## Быстрый старт

### 1. Запуск инфраструктуры

```bash
# Запуск MiniToolStream инфраструктуры
cd ..
docker-compose up -d

# Запуск Kafka инфраструктуры
cd benchmarks
docker-compose -f docker-compose.kafka.yml up -d
```

### 2. Генерация тестовых данных

```bash
cd tools/scripts
./generate-test-files.sh
```

### 3. Запуск бенчмарков

```bash
# MiniToolStream - маленькие файлы
cd minitoolstream/cmd/bench-small
go run main.go -config ../../configs/small-files.yaml

# MiniToolStream - большие файлы
cd ../bench-large
go run main.go -config ../../configs/large-files.yaml

# Kafka - маленькие файлы
cd ../../../kafka/cmd/bench-small
go run main.go -config ../../configs/small-files.yaml

# Kafka - большие файлы (chunked)
cd ../bench-chunked
go run main.go -config ../../configs/large-files.yaml
```

### 4. Анализ результатов

```bash
cd comparative/cmd/analyze
go run main.go -mts ../../results/minitoolstream/ -kafka ../../results/kafka/
```

## Метрики

### Собираемые метрики:

- **Throughput**: messages/sec, MB/sec
- **Latency**: p50, p95, p99, min, max
- **Resources**: CPU%, Memory MB, Disk I/O, Network I/O
- **Errors**: count, rate

### Сценарии тестирования:

#### Маленькие файлы (10 KB)
- Producers: 1, 10, 50, 100
- Consumers: 1, 5, 10
- Target RPS: 100, 500, 1000, 2000
- Duration: 5 minutes

#### Большие файлы (1 GB)
- Producers: 1, 3, 5
- Concurrent transfers: 1, 3, 5
- Total files: 10
- Измеряем время передачи

## Инструменты

- **Custom Go Benchmarks**: Основные измерения
- **ghz**: Quick gRPC тесты
- **Prometheus**: Real-time метрики
- **docker stats**: Resource monitoring

## Результаты

Результаты сохраняются в `results/` в формате JSON:

```json
{
  "system": "minitoolstream",
  "test_name": "small-files-10kb",
  "timestamp": "2025-12-09T...",
  "throughput": {
    "msg_per_sec": 1234.56,
    "mb_per_sec": 12.34
  },
  "latency": {
    "p50_ms": 25.3,
    "p95_ms": 48.7,
    "p99_ms": 89.2
  },
  "resources": {
    "cpu_percent": 45.2,
    "memory_mb": 256.8
  }
}
```

## Отчеты

Сравнительные отчеты генерируются в формате:
- Markdown с таблицами
- HTML с графиками
- CSV для дальнейшего анализа
