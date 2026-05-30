package crawler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"
)

// TestAnalyze_RetriesNetworkErrorAndUsesLastAttempt проверяет, что crawler
// повторяет запрос при ошибке соединения и использует успешный ответ последней попытки.
func TestAnalyze_RetriesNetworkErrorAndUsesLastAttempt(t *testing.T) {
	requestsCount := 0

	mockedClient := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			requestsCount++

			got := rq.URL.String()
			want := mockedBaseURL
			if got != want {
				t.Fatalf("got request URL %q, want %q", got, want)
			}

			if requestsCount == 1 {
				return nil, errors.New("network is down")
			}

			return newTestResponse(rq, http.StatusOK, "200 OK", "<title>OK</title>"), nil
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        mockedBaseURL,
		Depth:      1,
		Retries:    2,
		Delay:      time.Nanosecond,
		HTTPClient: mockedClient,
	})
	if err != nil {
		t.Fatalf("got error %v, want nil", err)
	}

	{
		got := requestsCount
		want := 2

		if got != want {
			t.Fatalf("got requests count %d, want %d", got, want)
		}
	}

	var report Report
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("got unmarshal error %v, want nil", err)
	}

	page := report.Pages[0]

	{
		got := page.HTTPStatus
		want := http.StatusOK

		if got != want {
			t.Fatalf("got HTTP status %d, want %d", got, want)
		}
	}

	{
		got := page.Status
		want := PageStatusOK

		if got != want {
			t.Fatalf("got page status %q, want %q", got, want)
		}
	}

	{
		got := page.Error
		want := ""

		if got != want {
			t.Fatalf("got page error %q, want %q", got, want)
		}
	}
}

// TestAnalyze_RetriesTemporaryHTTPStatusAndUsesLastAttempt проверяет, что crawler
// повторяет запросы при временных HTTP-статусах и использует успешный ответ последней попытки.
func TestAnalyze_RetriesTemporaryHTTPStatusAndUsesLastAttempt(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		status     string
	}{
		{
			name:       "too many requests",
			statusCode: http.StatusTooManyRequests,
			status:     "429 Too Many Requests",
		},
		{
			name:       "internal server error",
			statusCode: http.StatusInternalServerError,
			status:     "500 Internal Server Error",
		},
		{
			name:       "service unavailable",
			statusCode: http.StatusServiceUnavailable,
			status:     "503 Service Unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestsCount := 0

			mockedClient := &http.Client{
				Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
					requestsCount++

					got := rq.URL.String()
					want := mockedBaseURL
					if got != want {
						t.Fatalf("got request URL %q, want %q", got, want)
					}

					if requestsCount == 1 {
						return newTestResponse(rq, tt.statusCode, tt.status, "try later"), nil
					}

					return newTestResponse(rq, http.StatusOK, "200 OK", "<title>OK</title>"), nil
				}),
			}

			result, err := Analyze(context.Background(), Options{
				URL:        mockedBaseURL,
				Depth:      1,
				Retries:    2,
				Delay:      time.Nanosecond,
				HTTPClient: mockedClient,
			})
			if err != nil {
				t.Fatalf("got error %v, want nil", err)
			}

			{
				got := requestsCount
				want := 2

				if got != want {
					t.Fatalf("got requests count %d, want %d", got, want)
				}
			}

			var report Report
			if err := json.Unmarshal(result, &report); err != nil {
				t.Fatalf("got unmarshal error %v, want nil", err)
			}

			page := report.Pages[0]

			{
				got := page.HTTPStatus
				want := http.StatusOK

				if got != want {
					t.Fatalf("got HTTP status %d, want %d", got, want)
				}
			}

			{
				got := page.Status
				want := PageStatusOK

				if got != want {
					t.Fatalf("got page status %q, want %q", got, want)
				}
			}
		})
	}
}

// TestAnalyze_DoesNotRetryPermanentClientError проверяет, что crawler
// не повторяет запрос при постоянной клиентской ошибке вроде 404.
func TestAnalyze_DoesNotRetryPermanentClientError(t *testing.T) {
	requestsCount := 0

	mockedClient := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			requestsCount++

			return newTestResponse(rq, http.StatusNotFound, "404 Not Found", "not found"), nil
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        mockedBaseURL,
		Depth:      1,
		Retries:    2,
		Delay:      time.Nanosecond,
		HTTPClient: mockedClient,
	})
	if err != nil {
		t.Fatalf("got error %v, want nil", err)
	}

	{
		got := requestsCount
		want := 1

		if got != want {
			t.Fatalf("got requests count %d, want %d", got, want)
		}
	}

	var report Report
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("got unmarshal error %v, want nil", err)
	}

	page := report.Pages[0]

	{
		got := page.HTTPStatus
		want := http.StatusNotFound

		if got != want {
			t.Fatalf("got HTTP status %d, want %d", got, want)
		}
	}

	{
		got := page.Status
		want := PageStatusError

		if got != want {
			t.Fatalf("got page status %q, want %q", got, want)
		}
	}
}

// TestAnalyze_StopsAfterRetryLimit проверяет, что crawler не делает больше запросов,
// чем первая попытка плюс заданное количество повторов.
func TestAnalyze_StopsAfterRetryLimit(t *testing.T) {
	requestsCount := 0

	mockedClient := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			requestsCount++

			return newTestResponse(rq, http.StatusServiceUnavailable, "503 Service Unavailable", "try later"), nil
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        mockedBaseURL,
		Depth:      1,
		Retries:    2,
		Delay:      time.Nanosecond,
		HTTPClient: mockedClient,
	})
	if err != nil {
		t.Fatalf("got error %v, want nil", err)
	}

	{
		got := requestsCount
		want := 3

		if got != want {
			t.Fatalf("got requests count %d, want %d", got, want)
		}
	}

	var report Report
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("got unmarshal error %v, want nil", err)
	}

	page := report.Pages[0]

	{
		got := page.HTTPStatus
		want := http.StatusServiceUnavailable

		if got != want {
			t.Fatalf("got HTTP status %d, want %d", got, want)
		}
	}

	{
		got := page.Status
		want := PageStatusError

		if got != want {
			t.Fatalf("got page status %q, want %q", got, want)
		}
	}
}

// TestAnalyze_BrokenLinkUsesLastRetryAttempt проверяет, что проверка битых ссылок
// тоже использует retry-логику и не помечает ссылку битой после успешного повтора.
func TestAnalyze_BrokenLinkUsesLastRetryAttempt(t *testing.T) {
	const mockedLinkURL = mockedBaseURL + "/flaky.css"

	linkRequestsCount := 0
	htmlBody := `<a href="/flaky.css">flaky css</a>`

	mockedClient := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			switch rq.URL.String() {
			case mockedBaseURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", htmlBody), nil
			case mockedLinkURL:
				linkRequestsCount++

				if linkRequestsCount == 1 {
					return newTestResponse(rq, http.StatusServiceUnavailable, "503 Service Unavailable", "try later"), nil
				}

				return newTestResponse(rq, http.StatusOK, "200 OK", "ok"), nil
			default:
				t.Fatalf("got unexpected request URL %q", rq.URL.String())
				return nil, nil
			}
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        mockedBaseURL,
		Depth:      1,
		Retries:    2,
		Delay:      time.Nanosecond,
		HTTPClient: mockedClient,
	})
	if err != nil {
		t.Fatalf("got error %v, want nil", err)
	}

	{
		got := linkRequestsCount
		want := 2

		if got != want {
			t.Fatalf("got link requests count %d, want %d", got, want)
		}
	}

	var report Report
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("got unmarshal error %v, want nil", err)
	}

	page := report.Pages[0]

	{
		got := len(page.BrokenLinks)
		want := 0

		if got != want {
			t.Fatalf("got broken links len %d, want %d", got, want)
		}
	}
}
