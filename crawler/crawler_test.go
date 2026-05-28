package crawler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(rq *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(rq *http.Request) (*http.Response, error) {
	return f(rq)
}

func TestAnalyze_UsesProvidedHTTPClient(t *testing.T) {
	called := false

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			called = true

			got := rq.URL.String()
			want := "https://example.com"
			if got != want {
				t.Fatalf("got URL %q, want %q", got, want)
			}

			rs := &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("ok")),
				Request:    rq,
			}

			return rs, nil
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        "https://example.com",
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
		want := "https://example.com"
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
		want := "https://example.com"
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

func TestAnalyze_NetworkErrorReturnsReport(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			return nil, errors.New("network is down")
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        "https://example.com",
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
					rs := &http.Response{
						StatusCode: tt.statusCode,
						Status:     tt.status,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader("error")),
						Request:    rq,
					}

					return rs, nil
				}),
			}

			result, err := Analyze(context.Background(), Options{
				URL:        "https://example.com",
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

func TestAnalyze_TimeoutReturnsErrorReport(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        "https://example.com",
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
