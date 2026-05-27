package crawler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
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

	if opts.HTTPClient == nil {
		return nil, errors.New("http client is required")
	}

	page := PageReport{
		URL:   opts.URL,
		Depth: 0,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, opts.URL, nil)
	if err != nil {
		page.Status = PageStatusError
		page.Error = err.Error()

		return buildReport(opts, page)
	}

	if opts.UserAgent != "" {
		req.Header.Set("User-Agent", opts.UserAgent)
	}

	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		page.Status = PageStatusError
		page.Error = err.Error()

		return buildReport(opts, page)
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, resp.Body)

	page.HTTPStatus = resp.StatusCode

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusBadRequest {
		page.Status = PageStatusOK
	} else {
		page.Status = PageStatusError
		page.Error = resp.Status
	}

	return buildReport(opts, page)
}

func buildReport(opts Options, page PageReport) ([]byte, error) {
	report := Report{
		RootURL:     opts.URL,
		Depth:       opts.Depth,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Pages:       []PageReport{page},
	}

	if opts.IndentJSON {
		return json.MarshalIndent(report, "", "  ")
	}

	return json.Marshal(report)
}
