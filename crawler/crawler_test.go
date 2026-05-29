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

func TestAnalyze_ReportsBrokenLinksFromHTMLPage(t *testing.T) {
	htmlBody := `
		<!doctype html>
		<html>
			<head>
				<link rel="stylesheet" href="/assets/app.css">
				<link rel="stylesheet" href="/assets/ghost.css">
			</head>
			<body>
				<a href="/blog/post.html">working link</a>
				<img src="/images/missing.png">
				<a href="">empty link</a>
				<a href="#content">anchor link</a>
				<a href="mailto:team@example.com">email link</a>
				<a href="javascript:void(0)">javascript link</a>
			</body>
		</html>
	`

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			switch rq.URL.String() {
			case "http://simple.test/blog/index.html":
				return newTestResponse(rq, http.StatusOK, "200 OK", htmlBody), nil
			case "http://simple.test/assets/app.css":
				return newTestResponse(rq, http.StatusOK, "200 OK", "ok"), nil
			case "http://simple.test/blog/post.html":
				return newTestResponse(rq, http.StatusOK, "200 OK", "ok"), nil
			case "http://simple.test/assets/ghost.css":
				return newTestResponse(rq, http.StatusNotFound, "404 Not Found", "not found"), nil
			case "http://simple.test/images/missing.png":
				return nil, errors.New("network is down")
			default:
				t.Fatalf("got unexpected request URL %q", rq.URL.String())
				return nil, nil
			}
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        "http://simple.test/blog/index.html",
		Depth:      2,
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
		got := page.URL
		want := "http://simple.test/blog/index.html"
		if got != want {
			t.Fatalf("got page URL %q, want %q", got, want)
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
		got := page.DiscoveredAt == ""
		want := false
		if got != want {
			t.Fatalf("got empty discovered_at %t, want %t", got, want)
		}
	}

	{
		got := len(page.BrokenLinks)
		want := 2
		if got != want {
			t.Fatalf("got broken links len %d, want %d", got, want)
		}
	}

	{
		got := page.BrokenLinks[0].URL
		want := "http://simple.test/assets/ghost.css"
		if got != want {
			t.Fatalf("got broken link URL %q, want %q", got, want)
		}
	}

	{
		got := page.BrokenLinks[0].StatusCode
		want := http.StatusNotFound
		if got != want {
			t.Fatalf("got broken link status code %d, want %d", got, want)
		}
	}

	{
		got := page.BrokenLinks[0].Error
		want := ""
		if got != want {
			t.Fatalf("got broken link error %q, want %q", got, want)
		}
	}

	{
		got := page.BrokenLinks[1].URL
		want := "http://simple.test/images/missing.png"
		if got != want {
			t.Fatalf("got broken link URL %q, want %q", got, want)
		}
	}

	{
		got := page.BrokenLinks[1].StatusCode
		want := 0
		if got != want {
			t.Fatalf("got broken link status code %d, want %d", got, want)
		}
	}

	{
		got := page.BrokenLinks[1].Error
		want := "network is down"
		if !strings.Contains(got, want) {
			t.Fatalf("got broken link error %q, want it to contain %q", got, want)
		}
	}
}

func newTestResponse(rq *http.Request, statusCode int, status string, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    rq,
	}
}

func TestAnalyze_ReportsSEOBlockForExistingTags(t *testing.T) {
	htmlBody := `
		<!doctype html>
		<html>
			<head>
				<title> Example &amp; Test </title>
				<meta name="description" content=" Basic &amp; useful page ">
			</head>
			<body>
				<h1>Main header</h1>
			</body>
		</html>
	`

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			got := rq.URL.String()
			want := "http://example.test"

			if got != want {
				t.Fatalf("got request URL %q, want %q", got, want)
			}

			return newTestResponse(rq, http.StatusOK, "200 OK", htmlBody), nil
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        "http://example.test",
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

	page := report.Pages[0]

	{
		got := page.SEO.HasTitle
		want := true

		if got != want {
			t.Fatalf("got has_title %t, want %t", got, want)
		}
	}

	{
		got := page.SEO.Title
		want := "Example & Test"

		if got != want {
			t.Fatalf("got title %q, want %q", got, want)
		}
	}

	{
		got := page.SEO.HasDescription
		want := true

		if got != want {
			t.Fatalf("got has_description %t, want %t", got, want)
		}
	}

	{
		got := page.SEO.Description
		want := "Basic & useful page"

		if got != want {
			t.Fatalf("got description %q, want %q", got, want)
		}
	}

	{
		got := page.SEO.HasH1
		want := true

		if got != want {
			t.Fatalf("got has_h1 %t, want %t", got, want)
		}
	}
}

func TestAnalyze_ReportsEmptySEOBlockWhenTagsAreMissing(t *testing.T) {
	htmlBody := `
		<!doctype html>
		<html>
			<head></head>
			<body>
				<p>Page without SEO tags</p>
			</body>
		</html>
	`

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			return newTestResponse(rq, http.StatusOK, "200 OK", htmlBody), nil
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        "http://example.test",
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

	page := report.Pages[0]

	{
		got := page.SEO.HasTitle
		want := false

		if got != want {
			t.Fatalf("got has_title %t, want %t", got, want)
		}
	}

	{
		got := page.SEO.Title
		want := ""

		if got != want {
			t.Fatalf("got title %q, want %q", got, want)
		}
	}

	{
		got := page.SEO.HasDescription
		want := false

		if got != want {
			t.Fatalf("got has_description %t, want %t", got, want)
		}
	}

	{
		got := page.SEO.Description
		want := ""

		if got != want {
			t.Fatalf("got description %q, want %q", got, want)
		}
	}

	{
		got := page.SEO.HasH1
		want := false

		if got != want {
			t.Fatalf("got has_h1 %t, want %t", got, want)
		}
	}
}

func TestAnalyze_DecodesHTMLEntitiesInSEOText(t *testing.T) {
	htmlBody := `
		<!doctype html>
		<html>
			<head>
				<title>Fish &amp; Chips</title>
				<meta name="description" content="Tom &amp; Jerry">
			</head>
			<body></body>
		</html>
	`

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			return newTestResponse(rq, http.StatusOK, "200 OK", htmlBody), nil
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        "http://example.test",
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

	page := report.Pages[0]

	{
		got := page.SEO.Title
		want := "Fish & Chips"

		if got != want {
			t.Fatalf("got title %q, want %q", got, want)
		}
	}

	{
		got := page.SEO.Description
		want := "Tom & Jerry"

		if got != want {
			t.Fatalf("got description %q, want %q", got, want)
		}
	}
}
