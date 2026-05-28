package crawler

import (
	"net/http"
	"time"
)

// Step1: Options с параметрами запуска
// (URL, Depth, Retries, Delay, Timeout, UserAgent, Concurrency, IndentJSON, HTTPClient).

type Options struct {
	URL         string
	Depth       int
	Retries     int
	Delay       time.Duration
	Timeout     time.Duration
	UserAgent   string
	Concurrency int
	IndentJSON  bool
	HTTPClient  *http.Client
}
