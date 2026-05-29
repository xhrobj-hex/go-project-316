package crawler

import (
	"net/http"
	"time"
)

// Options хранит параметры запуска анализатора сайта.
//
// URL и HTTPClient обязательны. Delay и RPS управляют скоростью HTTP-запросов;
// если указаны оба параметра, приоритет имеет RPS.
type Options struct {
	// URL задаёт стартовый адрес для обхода сайта.
	URL string

	// Depth задаёт максимальную глубину обхода.
	Depth int

	// Retries задаёт количество повторных попыток HTTP-запроса.
	Retries int

	// Delay задаёт фиксированную паузу между соседними HTTP-запросами.
	Delay time.Duration

	// RPS задаёт целевое количество HTTP-запросов в секунду.
	RPS int

	// Timeout задаёт максимальное время ожидания HTTP-запроса.
	Timeout time.Duration

	// UserAgent задаёт значение заголовка User-Agent для HTTP-запросов.
	UserAgent string

	// Concurrency задаёт максимальное количество параллельных обработчиков.
	Concurrency int

	// IndentJSON включает форматированный JSON-отчёт.
	IndentJSON bool

	// HTTPClient выполняет HTTP-запросы во время обхода сайта.
	HTTPClient *http.Client
}
