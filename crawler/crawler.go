package crawler

import (
	"context"
	"errors"
)

// Основная точка входа в crawler — функция:
//
// func Analyze(ctx context.Context, opts Options) ([]byte, error)
//
// Она принимает context.Context, чтобы можно было отменять обход, и структуру Options с параметрами запуска
// (URL, Depth, Retries, Delay, Timeout, UserAgent, Concurrency, IndentJSON, HTTPClient).
//
// Функция возвращает JSON-отчет в виде байтового слайса и ошибку.

func Analyze(ctx context.Context, opts Options) ([]byte, error) {
	if opts.URL == "" {
		return nil, errors.New("url is required")
	}

	return nil, nil
}
