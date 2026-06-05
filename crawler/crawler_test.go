package crawler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// TestAnalyze_UsesProvidedHTTPClient проверяет, что Analyze использует
// переданный HTTP-клиент и формирует успешный отчет по ответу клиента.
func TestAnalyze_UsesProvidedHTTPClient(t *testing.T) {
	called := false

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			called = true

			got := rq.URL.String()
			want := mockedBaseURL
			if got != want {
				t.Fatalf("got URL %q, want %q", got, want)
			}

			return newTestResponse(rq, http.StatusOK, "200 OK", "ok"), nil
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        mockedBaseURL,
		Depth:      1,
		HTTPClient: client,
	})
	if err != nil {
		t.Fatalf("got error %v, want nil", err)
	}

	{
		got := called
		want := true
		if got != want {
			t.Fatalf("got HTTP client called %t, want %t", got, want)
		}
	}

	var report Report
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("got unmarshal error %v, want nil", err)
	}

	{
		got := report.RootURL
		want := mockedBaseURL
		if got != want {
			t.Fatalf("got root URL %q, want %q", got, want)
		}
	}

	{
		got := report.Depth
		want := 1
		if got != want {
			t.Fatalf("got depth %d, want %d", got, want)
		}
	}

	{
		got := len(report.Pages)
		want := 1
		if got != want {
			t.Fatalf("got pages len %d, want %d", got, want)
		}
	}

	page := report.Pages[0]

	{
		got := page.URL
		want := mockedBaseURL
		if got != want {
			t.Fatalf("got page URL %q, want %q", got, want)
		}
	}

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

// TestAnalyze_NetworkErrorReturnsReport проверяет, что ошибка соединения
// не прерывает Analyze, а попадает в отчёт как ошибка страницы.
func TestAnalyze_NetworkErrorReturnsReport(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			return nil, errors.New("network is down")
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        mockedBaseURL,
		Depth:      1,
		HTTPClient: client,
	})
	if err != nil {
		t.Fatalf("got error %v, want nil", err)
	}

	var report Report
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("got unmarshal error %v, want nil", err)
	}

	{
		got := len(report.Pages)
		want := 1
		if got != want {
			t.Fatalf("got pages len %d, want %d", got, want)
		}
	}

	page := report.Pages[0]

	{
		got := page.HTTPStatus
		want := 0
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

	{
		got := page.Error
		want := "network is down"
		if !strings.Contains(got, want) {
			t.Fatalf("got page error %q, want it to contain %q", got, want)
		}
	}
}

// TestAnalyze_HTTPErrorStatusReturnsErrorReport проверяет, что HTTP-статусы
// ошибок сохраняются в отчете как ошибка страницы.
func TestAnalyze_HTTPErrorStatusReturnsErrorReport(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		status     string
	}{
		{
			name:       "not found",
			statusCode: http.StatusNotFound,
			status:     "404 Not Found",
		},
		{
			name:       "internal server error",
			statusCode: http.StatusInternalServerError,
			status:     "500 Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{
				Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
					return newTestResponse(rq, tt.statusCode, tt.status, "error"), nil
				}),
			}

			result, err := Analyze(context.Background(), Options{
				URL:        mockedBaseURL,
				Depth:      1,
				HTTPClient: client,
			})
			if err != nil {
				t.Fatalf("got error %v, want nil", err)
			}

			var report Report
			if err := json.Unmarshal(result, &report); err != nil {
				t.Fatalf("got unmarshal error %v, want nil", err)
			}

			{
				got := len(report.Pages)
				want := 1
				if got != want {
					t.Fatalf("got pages len %d, want %d", got, want)
				}
			}

			page := report.Pages[0]

			{
				got := page.HTTPStatus
				want := tt.statusCode
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

			{
				got := page.Error
				want := tt.status
				if !strings.Contains(got, want) {
					t.Fatalf("got page error %q, want it to contain %q", got, want)
				}
			}
		})
	}
}

// TestAnalyze_TimeoutReturnsErrorReport проверяет, что таймаут запроса
// не прерывает Analyze, а сохраняется в отчете как ошибка страницы.
func TestAnalyze_TimeoutReturnsErrorReport(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        mockedBaseURL,
		Depth:      1,
		HTTPClient: client,
	})
	if err != nil {
		t.Fatalf("got error %v, want nil", err)
	}

	var report Report
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("got unmarshal error %v, want nil", err)
	}

	{
		got := len(report.Pages)
		want := 1
		if got != want {
			t.Fatalf("got pages len %d, want %d", got, want)
		}
	}

	page := report.Pages[0]

	{
		got := page.HTTPStatus
		want := 0
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

	{
		got := page.Error
		want := context.DeadlineExceeded.Error()
		if !strings.Contains(got, want) {
			t.Fatalf("got page error %q, want it to contain %q", got, want)
		}
	}
}
