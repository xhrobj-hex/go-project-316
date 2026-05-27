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

	got := called
	want := true
	if got != want {
		t.Fatalf("got HTTP client called %t, want %t", got, want)
	}

	var report Report
	if err := json.Unmarshal(result, &report); err != nil {
		t.Fatalf("got unmarshal error %v, want nil", err)
	}

	gotRootURL := report.RootURL
	wantRootURL := "https://example.com"
	if gotRootURL != wantRootURL {
		t.Fatalf("got root URL %q, want %q", gotRootURL, wantRootURL)
	}

	gotDepth := report.Depth
	wantDepth := 1
	if gotDepth != wantDepth {
		t.Fatalf("got depth %d, want %d", gotDepth, wantDepth)
	}

	gotPagesLen := len(report.Pages)
	wantPagesLen := 1
	if gotPagesLen != wantPagesLen {
		t.Fatalf("got pages len %d, want %d", gotPagesLen, wantPagesLen)
	}

	page := report.Pages[0]

	gotPageURL := page.URL
	wantPageURL := "https://example.com"
	if gotPageURL != wantPageURL {
		t.Fatalf("got page URL %q, want %q", gotPageURL, wantPageURL)
	}

	gotHTTPStatus := page.HTTPStatus
	wantHTTPStatus := http.StatusOK
	if gotHTTPStatus != wantHTTPStatus {
		t.Fatalf("got HTTP status %d, want %d", gotHTTPStatus, wantHTTPStatus)
	}

	gotStatus := page.Status
	wantStatus := PageStatusOK
	if gotStatus != wantStatus {
		t.Fatalf("got page status %q, want %q", gotStatus, wantStatus)
	}

	gotError := page.Error
	wantError := ""
	if gotError != wantError {
		t.Fatalf("got page error %q, want %q", gotError, wantError)
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

	gotPagesLen := len(report.Pages)
	wantPagesLen := 1
	if gotPagesLen != wantPagesLen {
		t.Fatalf("got pages len %d, want %d", gotPagesLen, wantPagesLen)
	}

	page := report.Pages[0]

	gotHTTPStatus := page.HTTPStatus
	wantHTTPStatus := 0
	if gotHTTPStatus != wantHTTPStatus {
		t.Fatalf("got HTTP status %d, want %d", gotHTTPStatus, wantHTTPStatus)
	}

	gotStatus := page.Status
	wantStatus := PageStatusError
	if gotStatus != wantStatus {
		t.Fatalf("got page status %q, want %q", gotStatus, wantStatus)
	}

	gotError := page.Error
	wantError := "network is down"
	if !strings.Contains(gotError, wantError) {
		t.Fatalf("got page error %q, want it to contain %q", gotError, wantError)
	}
}
