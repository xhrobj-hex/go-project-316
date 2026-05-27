# 🕷️ Парсер сайтов

## Описание

`hexlet-go-crawler` - CLI-утилита для анализа структуры сайта и формирования JSON-отчёта.

На первом шаге проект выполняет один HTTP-запрос к указанному URL и выводит черновой JSON-отчёт с базовой информацией о странице.

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
  "depth": 10,
  "generated_at": "2026-05-27T16:23:27Z",
  "pages": [
    {
      "url": "https://example.com",
      "depth": 0,
      "http_status": 200,
      "status": "ok",
      "error": ""
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
