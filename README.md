# 🕷️ Парсер сайтов

## Описание

`hexlet-go-crawler` - CLI-утилита для анализа структуры сайта и формирования JSON-отчёта.

Утилита загружает стартовую страницу, обходит внутренние ссылки в пределах заданной глубины и собирает по каждой странице:

- HTTP-статус;
- статус обработки;
- список битых ссылок;
- SEO-данные;
- время обнаружения страницы.

## Возможности

### Глубина обхода

Флаг `--depth` задаёт количество уровней обхода, включая стартовую страницу.

Стартовая страница всегда имеет `depth: 0`.

Примеры:

- `--depth=1` - только стартовая страница;
- `--depth=2` - стартовая страница и её внутренние дочерние страницы;
- `--depth=3` - стартовая страница, дочерние страницы и следующий уровень.

В отчёт попадают только страницы исходного хоста. Внешние ссылки не обходятся как страницы, но могут проверяться как обычные ссылки и попадать в `broken_links`, если недоступны.

### Скорость обхода

Crawler поддерживает ограничение скорости HTTP-запросов, чтобы не отправлять слишком много запросов подряд.

Флаг `--delay` задаёт фиксированную задержку между соседними HTTP-запросами:

```bash
go run ./cmd/hexlet-go-crawler --delay=200ms https://example.com
```

Флаг `--rps` задаёт целевое количество HTTP-запросов в секунду:

```bash
go run ./cmd/hexlet-go-crawler --rps=5 https://example.com
```

Если одновременно указаны `--delay` и `--rps`, приоритет имеет `--rps`.

Например:

- `--rps=1` - примерно один HTTP-запрос в секунду;
- `--rps=5` - примерно пять HTTP-запросов в секунду;
- `--delay=500ms` - пауза 500 миллисекунд между соседними HTTP-запросами.

Ограничение применяется ко всем HTTP-запросам crawler'а: к загрузке страниц и к проверке ссылок.

### Повторные попытки

Флаг `--retries` задает максимальное количество повторных попыток для временно неудачных HTTP-запросов.

Например:

```bash
go run ./cmd/hexlet-go-crawler --retries=2 https://example.com
```

В этом случае crawler может выполнить до 3 обращений к одному ресурсу: первая попытка + 2 повтора.

Повтор выполняется только для временных проблем:

- ошибки сети или соединения, например недоступный сервер, разрыв соединения или таймаут;
- `429 Too Many Requests`;
- HTTP-статусов `5xx`.

Статусы вроде `404 Not Found` не повторяются, потому что обычно не означают временную перегрузку сервера.

Между повторными попытками crawler делает паузу. Если задан `--delay` или `--rps`, пауза между запросами достигается за счет общего ограничения скорости. Если ограничение скорости не задано, используется небольшая задержка по умолчанию.

Подробнее о временных серверных ошибках: [Поведение клиентов при временных ошибках](https://datatracker.ietf.org/doc/html/rfc7231#section-6.6).

## Команды

### Сборка

```bash
make build
```

Команда собирает исполняемый файл:

```bash
bin/hexlet-go-crawler
```

### Запуск тестов

```bash
make test
```

### Запуск линтера

```bash
make lint
```

### Запуск crawler

```bash
make run URL=https://example.com
```

В этой команде `URL` - это переменная Makefile, переданная в `make` из командной строки.

Makefile подставляет её в команду запуска:

```bash
go run ./cmd/hexlet-go-crawler https://example.com
```

То есть само Go-приложение получает адрес сайта как обычный аргумент командной строки, а не читает переменную окружения `URL`.

Запуск с параметрами можно выполнить напрямую:

```bash
go run ./cmd/hexlet-go-crawler --depth=2 https://example.com
```

После сборки:

```bash
bin/hexlet-go-crawler --depth=2 https://example.com
```

### Справка

Если запустить команду без URL:

```bash
make run
```

будет показана справка по CLI.

Также справку можно вызвать напрямую:

```bash
go run ./cmd/hexlet-go-crawler --help
```

После сборки:

```bash
bin/hexlet-go-crawler --help
```

## Пример отчёта

```json
{
 "root_url": "https://example.com",
 "depth": 2,
 "generated_at": "2026-05-29T15:01:47Z",
 "pages": [
  {
   "url": "https://example.com",
   "depth": 0,
   "http_status": 200,
   "status": "ok",
   "error": "",
   "broken_links": [
    {
     "url": "https://iana.org/domains/example",
     "error": "Get \"https://iana.org/domains/example\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)"
    }
   ],
   "discovered_at": "2026-05-29T15:01:47Z",
   "seo": {
    "has_title": true,
    "title": "Example Domain",
    "has_description": false,
    "description": "",
    "has_h1": true
   }
  }
 ]
}
```

## Архитектура

Основная точка входа в crawler находится в пакете `code/crawler`:

```go
func Analyze(ctx context.Context, opts Options) ([]byte, error)
```

HTTP-клиент передаётся через `Options.HTTPClient`, поэтому сетевой код можно тестировать без реальных HTTP-запросов.

## Статусы

### Hexlet tests and linter status

[![Actions Status](https://github.com/xhrobj-hex/go-project-316/actions/workflows/hexlet-check.yml/badge.svg)](https://github.com/xhrobj-hex/go-project-316/actions)

### Project CI

[![(-_-) GO CI](https://github.com/xhrobj-hex/go-project-316/actions/workflows/go-ci.yml/badge.svg)](https://github.com/xhrobj-hex/go-project-316/actions/workflows/go-ci.yml)
