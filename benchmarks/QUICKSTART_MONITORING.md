# Quick Start: Grafana Monitoring для бенчмарков

## За 3 минуты до визуализации

### 1. Запустить мониторинг (30 сек)

```bash
cd /Users/moroshma/go/DiplomaThesis/MiniToolStream/benchmarks

# Запуск всего стека
docker-compose -f docker-compose.monitoring.yml up -d

# Ждём 10 секунд
sleep 10
```

### 2. Открыть Grafana (10 сек)

```bash
# В браузере
open http://localhost:3000

# Логин: admin
# Пароль: admin
```

### 3. Перейти к dashboard (10 сек)

```
Dashboards → Benchmarks → "MiniToolStream vs Kafka - Real-time Benchmark Comparison"
```

### 4. Запустить бенчмарк (2 мин)

```bash
cd minitoolstream/cmd/bench-small

# Запуск с Prometheus экспортом
go run main.go -config=../../configs/small-files.yaml
```

### 5. Смотреть в реальном времени! 🎉

Графики обновляются каждые 5 секунд:
- 📊 Throughput (msg/s, MB/s)
- ⚡ Latency (P50, P95, P99)
- 💻 CPU Usage
- 💾 Memory Usage

## Что вы увидите

### Real-time графики:

1. **Throughput Messages/sec** - зелёная линия растёт до ~1000 msg/s
2. **Throughput MB/sec** - пропускная способность в реальном времени
3. **Latency Distribution** - P50, P95, P99 latency (должно быть < 50ms)
4. **CPU Usage** - gauge показывает загрузку процессора
5. **Memory Usage** - потребление памяти контейнерами
6. **Error Rate** - количество ошибок (должно быть 0)
7. **Total Messages** - счётчик обработанных сообщений

### Цветовая схема:
- 🟢 **Зелёный** = MiniToolStream
- 🟠 **Оранжевый** = Kafka

## Сравнение с Kafka

Для side-by-side сравнения:

```bash
# В одном терминале: MiniToolStream
cd minitoolstream/cmd/bench-small
go run main.go -config=../../configs/small-files.yaml

# В другом терминале: Kafka
cd kafka/cmd/bench-small
go run main.go -config=../../configs/small-files.yaml
```

Grafana покажет обе линии на одном графике!

## Остановка

```bash
cd /Users/moroshma/go/DiplomaThesis/MiniToolStream/benchmarks

# Остановить мониторинг
docker-compose -f docker-compose.monitoring.yml down

# Очистить метрики (опционально)
curl -X DELETE http://localhost:9091/metrics
```

## Troubleshooting за 30 секунд

### Графики пустые?

```bash
# 1. Проверить Pushgateway
curl http://localhost:9091/metrics | grep benchmark

# 2. Проверить Prometheus targets
open http://localhost:9090/targets

# 3. Перезапустить стек
docker-compose -f docker-compose.monitoring.yml restart
```

### Dashboard не загружается?

```bash
# Перезапустить Grafana
docker-compose -f docker-compose.monitoring.yml restart grafana
sleep 10
```

## Полезные ссылки

- Grafana: http://localhost:3000
- Prometheus: http://localhost:9090
- Pushgateway: http://localhost:9091
- MONITORING_GUIDE.md - детальная документация

## Конфигурация

Файл: `configs/small-files.yaml`

```yaml
prometheus:
  enabled: true                          # Включить экспорт
  pushgateway_url: "http://localhost:9091"
  push_interval: "5s"                    # Обновление каждые 5 сек
  instance: "benchmark-1"
```

Всё! Теперь у вас есть real-time мониторинг бенчмарков как у профессионалов! 🚀
