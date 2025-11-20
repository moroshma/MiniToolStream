# Publisher Client Example

Простой клиент для публикации изображений через MiniToolStreamIngress.

## Использование

```bash
# Сборка
go build -o publisher_client

# Запуск с дефолтными параметрами (публикует tst.jpeg в канал terminator.diff)
./publisher_client

# Запуск с кастомными параметрами
./publisher_client -app localhost:50051 -image /path/to/image.jpg -subject my.channel
```

## Параметры

- `-server` - адрес gRPC сервера MiniToolStreamIngress (по умолчанию: `localhost:50051`)
- `-image` - путь к файлу изображения (по умолчанию: `tst.jpeg`)
- `-subject` - название канала/subject (по умолчанию: `terminator.diff`)

## Что делает клиент

1. Читает файл изображения
2. Подключается к MiniToolStreamIngress gRPC серверу
3. Отправляет изображение с метаданными:
   - `content-type: image/jpeg`
   - `filename: имя файла`
   - `timestamp: текущее время`
4. Получает в ответ sequence и object_name
5. Выводит информацию о публикации

## Примечание

На данном этапе MiniToolStreamIngress сохраняет только метаданные в Tarantool.
Данные изображения (поле `data`) передаются через gRPC, но их нужно будет загружать в MinIO отдельно, используя `object_name` как ключ.
