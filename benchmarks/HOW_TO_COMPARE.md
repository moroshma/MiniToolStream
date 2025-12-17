# Как сравнить MiniToolStream и Kafka

## Быстрый старт (5 минут)

### Шаг 1: Запустите Kafka

```bash
cd /Users/moroshma/go/DiplomaThesis/MiniToolStream/benchmarks

# Запустите Kafka и Zookeeper
docker-compose -f docker-compose.kafka.yml up -d

# Подождите 30 секунд пока Kafka запустится
sleep 30

# Проверьте статус
docker ps | grep kafka
```

### Шаг 2: Запустите оба бенчмарка одновременно

Откройте **два терминала**:

**Терминал 1 - MiniToolStream:**
```bash
cd /Users/moroshma/go/DiplomaThesis/MiniToolStream/benchmarks/minitoolstream/cmd/bench-small
go run main.go -config=../../configs/small-files.yaml
```

**Терминал 2 - Kafka:**
```bash
cd /Users/moroshma/go/DiplomaThesis/MiniToolStream/benchmarks/kafka/cmd/bench-small
go run main.go -config=../../configs/small-files.yaml
```

### Шаг 3: Откройте Grafana

```bash
# В браузере откройте:
open http://localhost:3000

# Логин: admin
# Пароль: admin

# Перейдите в: Dashboards → Benchmarks → "MiniToolStream vs Kafka Benchmark Comparison"
```

## Что вы увидите в Grafana

### Графики с реальными данными:

1. **Throughput: Messages/sec**
   - 🟢 Зеленая линия = MiniToolStream
   - 🟠 Оранжевая линия = Kafka
   - Показывает количество сообщений в секунду

2. **Throughput: MB/sec**
   - Пропускная способность в мегабайтах

3. **Latency: P95 и P99**
   - Задержка обработки сообщений
   - P95 = 95% сообщений обработаны быстрее этого времени
   - P99 = 99% сообщений обработаны быстрее этого времени

4. **CPU Usage** - Загрузка процессора системы
5. **Memory Usage** - Использование памяти
6. **Error Rate** - Количество ошибок (должно быть 0)

## Конфигурация тестов

### Small Files Test (10KB messages)
- **Файл:** `configs/small-files.yaml`
- **Размер сообщения:** 10 KB
- **Всего сообщений:** 10,000
- **Producers:** 10
- **Target RPS:** 1,000
- **Длительность:** ~2 минуты

### Large Files Test (1GB messages)
- **Файл:** `configs/large-files.yaml`
- **Размер сообщения:** 1 GB
- **Всего сообщений:** 100
- **Producers:** 5
- **Длительность:** ~15-20 минут

## Запуск large files теста

```bash
# MiniToolStream
cd /Users/moroshma/go/DiplomaThesis/MiniToolStream/benchmarks/minitoolstream/cmd/bench-large
go run main.go -config=../../configs/large-files.yaml

# Kafka
cd /Users/moroshma/go/DiplomaThesis/MiniToolStream/benchmarks/kafka/cmd/bench-large
go run main.go -config=../../configs/large-files.yaml
```

## Очистка метрик

Если хотите начать с чистого листа:

```bash
# Очистить метрики из Pushgateway
curl -X PUT http://localhost:9091/api/v1/admin/wipe

# Перезапустить мониторинг
docker-compose -f docker-compose.monitoring.yml restart
```

## Остановка всего

```bash
# Остановить Kafka
docker-compose -f docker-compose.kafka.yml down

# Остановить мониторинг
docker-compose -f docker-compose.monitoring.yml down

# Остановить MiniToolStream (если запущен)
cd /Users/moroshma/go/DiplomaThesis/MiniToolStream
docker-compose down
```

## Troubleshooting

### Grafana показывает "No data"
1. Проверьте, что бенчмарки запущены: `ps aux | grep "go run"`
2. Проверьте метрики: `curl http://localhost:9091/metrics | grep benchmark`
3. Обновите страницу в Grafana (F5)
4. Измените временной диапазон на "Last 5 minutes"

### Kafka не запускается
```bash
# Проверьте логи
docker logs benchmark-kafka
docker logs benchmark-zookeeper

# Пересоздайте контейнеры
docker-compose -f docker-compose.kafka.yml down -v
docker-compose -f docker-compose.kafka.yml up -d
```

### MiniToolStream недоступен
```bash
# Проверьте статус
docker ps | grep minitoolstream

# Проверьте Ingress/Egress
curl http://localhost:50051/health
curl http://localhost:50052/health
```

## Полезные ссылки

- **Grafana:** http://localhost:3000
- **Prometheus:** http://localhost:9090
- **Pushgateway:** http://localhost:9091
- **Kafka UI:** http://localhost:8080 (когда Kafka запущен)

## Результаты

Результаты бенчмарков сохраняются в:
- **MiniToolStream:** `benchmarks/results/minitoolstream/`
- **Kafka:** `benchmarks/results/kafka/`

Формат: JSON и CSV файлы с timestamp.
