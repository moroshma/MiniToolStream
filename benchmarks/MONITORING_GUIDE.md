# Monitoring Guide: Real-time Benchmark Visualization

Подробное руководство по настройке и использованию системы мониторинга для бенчмарков MiniToolStream vs Kafka.

## Архитектура системы мониторинга

```
┌──────────────────────────────────────────────────────────────┐
│                  Benchmark Infrastructure                    │
│                                                              │
│  ┌────────────────┐              ┌────────────────┐        │
│  │ MiniToolStream │              │     Kafka      │        │
│  │  Benchmarks    │              │  Benchmarks    │        │
│  │                │              │                │        │
│  │ • bench-small  │              │ • bench-small  │        │
│  │ • bench-large  │              │ • bench-chunked│        │
│  └────────┬───────┘              └────────┬───────┘        │
│           │                               │                 │
│           │ Push metrics every 5s         │                 │
│           │                               │                 │
│           └──────────┬────────────────────┘                 │
│                      │                                       │
│           ┌──────────▼──────────┐                           │
│           │    Pushgateway      │                           │
│           │   localhost:9091    │                           │
│           └──────────┬──────────┘                           │
│                      │                                       │
│                      │ Scrape every 15s                      │
│                      │                                       │
│           ┌──────────▼──────────┐                           │
│           │    Prometheus       │ ◄──── cAdvisor (Docker)   │
│           │   localhost:9090    │ ◄──── Node Exporter       │
│           │  (TSDB Storage)     │                           │
│           └──────────┬──────────┘                           │
│                      │                                       │
│                      │ Query PromQL                          │
│                      │                                       │
│           ┌──────────▼──────────┐                           │
│           │      Grafana        │                           │
│           │   localhost:3000    │                           │
│           │  (Visualization)    │                           │
│           └─────────────────────┘                           │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

## Быстрый старт

### Шаг 1: Запуск мониторинга

```bash
cd /Users/moroshma/go/DiplomaThesis/MiniToolStream/benchmarks

# Запуск всего стека
docker-compose -f docker-compose.monitoring.yml up -d

# Проверка статуса
docker-compose -f docker-compose.monitoring.yml ps
```

### Шаг 2: Открыть Grafana

1. Откройте браузер: http://localhost:3000
2. Войдите: `admin` / `admin`
3. Перейдите в Dashboards → Benchmarks → "MiniToolStream vs Kafka"

### Шаг 3: Запуск бенчмарка с мониторингом

```bash
cd minitoolstream/cmd/bench-small
go run main.go -config=../../configs/small-files.yaml

# В консоли увидите:
# Prometheus push enabled: http://localhost:9091 (interval: 5s)
```

### Шаг 4: Наблюдение в реальном времени

В Grafana вы увидите обновление графиков каждые 5 секунд!

## Компоненты

### Prometheus (localhost:9090)
Time-series база данных для хранения метрик

### Pushgateway (localhost:9091)
Прием push-метрик от бенчмарков

### Grafana (localhost:3000)
Визуализация метрик
- Username: `admin`
- Password: `admin`

### cAdvisor (localhost:8081)
Мониторинг Docker контейнеров

### Node Exporter (localhost:9100)
Системные метрики хоста

## Экспортируемые метрики

### benchmark_messages_total
Counter: Общее количество обработанных сообщений

### benchmark_bytes_total
Counter: Общий объем данных в байтах

### benchmark_latency_seconds
Histogram: Latency distribution (P50, P95, P99)

### benchmark_errors_total
Counter: Количество ошибок

## Grafana Dashboard

### Панели:
1. 📊 Throughput: Messages/sec
2. 🚀 Throughput: MB/sec
3. ⚡ Latency Distribution (P50, P95, P99)
4. 💻 CPU Usage
5. 💾 Memory Usage
6. ❌ Error Rate
7. 📈 Total Messages Processed

Dashboard автоматически обновляется каждые 5 секунд!

## Полный цикл тестирования

```bash
# 1. Запустить мониторинг
docker-compose -f docker-compose.monitoring.yml up -d

# 2. Открыть Grafana
open http://localhost:3000

# 3. Запустить MiniToolStream benchmark
cd minitoolstream/cmd/bench-small
go run main.go -config=../../configs/small-files.yaml

# 4. В другом терминале запустить Kafka benchmark
cd ../../../kafka/cmd/bench-small
go run main.go -config=../../configs/small-files.yaml

# 5. Сравнить результаты в Grafana в реальном времени!
```

## Troubleshooting

### Метрики не появляются в Grafana?

**Проверка 1**: Pushgateway получает метрики?
```bash
curl http://localhost:9091/metrics | grep benchmark_messages_total
```

**Проверка 2**: Prometheus scrape-ит Pushgateway?
```bash
open http://localhost:9090/targets
# pushgateway должен быть UP
```

**Проверка 3**: Grafana подключен к Prometheus?
```bash
# Grafana → Configuration → Data Sources → Prometheus → Test
```

**Решение**: Перезапустить стек
```bash
docker-compose -f docker-compose.monitoring.yml down
docker-compose -f docker-compose.monitoring.yml up -d
```

### Dashboard не загружается?

```bash
# Перезапустить Grafana
docker-compose -f docker-compose.monitoring.yml restart grafana
sleep 10  # Подождать auto-provisioning
```

## Продвинутые запросы PromQL

### Сравнение throughput

```promql
# Разница: MiniToolStream - Kafka
rate(benchmark_messages_total{system="minitoolstream"}[1m])
-
rate(benchmark_messages_total{system="kafka"}[1m])
```

### Latency comparison

```promql
# P95 latency разница
histogram_quantile(0.95,
  rate(benchmark_latency_seconds_bucket{system="minitoolstream"}[1m])
)
-
histogram_quantile(0.95,
  rate(benchmark_latency_seconds_bucket{system="kafka"}[1m])
)
```

## Экспорт данных

### Экспорт из Prometheus

```bash
# Query API
curl -G http://localhost:9090/api/v1/query \
  --data-urlencode 'query=rate(benchmark_messages_total[1m])' \
  | jq '.'
```

### Экспорт dashboard как PNG

В Grafana: Dashboard → Share → Link → Direct link rendered image

## Cleanup после тестов

```bash
# Удалить метрики
curl -X DELETE http://localhost:9091/metrics

# Остановить мониторинг
docker-compose -f docker-compose.monitoring.yml down
```

## Заключение

Система предоставляет:
- ✅ Real-time визуализацию бенчмарков
- ✅ Автоматическое сравнение MiniToolStream vs Kafka
- ✅ Детальные метрики: latency, throughput, resources
- ✅ Готовые дашборды для презентаций
- ✅ Экспорт данных для научной работы
