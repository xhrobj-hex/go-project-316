package crawler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const mockedBaseURL = "http://crawler.test"

type roundTripFunc func(rq *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(rq *http.Request) (*http.Response, error) {
	return f(rq)
}

func findPageByURL(t *testing.T, pages []PageReport, pageURL string) PageReport {
	t.Helper()

	for _, page := range pages {
		if page.URL == pageURL {
			return page
		}
	}

	t.Fatalf("got no page with URL %q", pageURL)
	return PageReport{}
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

func TestAnalyze_DepthOneReportsOnlyRootPage(t *testing.T) {
	htmlBody := `<a href="/about.html">about</a>`

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			switch rq.URL.String() {
			case "http://simple.test":
				return newTestResponse(rq, http.StatusOK, "200 OK", htmlBody), nil
			case "http://simple.test/about.html":
				return newTestResponse(rq, http.StatusOK, "200 OK", `<title>About</title>`), nil
			default:
				t.Fatalf("got unexpected request URL %q", rq.URL.String())
				return nil, nil
			}
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        "http://simple.test",
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

	{
		got := report.Pages[0].URL
		want := "http://simple.test"
		if got != want {
			t.Fatalf("got page URL %q, want %q", got, want)
		}
	}

	{
		got := report.Pages[0].Depth
		want := 0
		if got != want {
			t.Fatalf("got page depth %d, want %d", got, want)
		}
	}
}

func TestAnalyze_DepthTwoReportsInternalPagesOnly(t *testing.T) {
	rootBody := `
		<a href="/about.html">about</a>
		<a href="/contacts.html">contacts</a>
		<a href="https://external.test/page.html">external</a>
	`

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			switch rq.URL.String() {
			case "http://simple.test":
				return newTestResponse(rq, http.StatusOK, "200 OK", rootBody), nil
			case "http://simple.test/about.html":
				return newTestResponse(rq, http.StatusOK, "200 OK", `<title>About</title><h1>About</h1>`), nil
			case "http://simple.test/contacts.html":
				return newTestResponse(rq, http.StatusOK, "200 OK", `<title>Contacts</title>`), nil
			case "https://external.test/page.html":
				return newTestResponse(rq, http.StatusOK, "200 OK", `<title>External</title>`), nil
			default:
				t.Fatalf("got unexpected request URL %q", rq.URL.String())
				return nil, nil
			}
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        "http://simple.test",
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
		want := 3
		if got != want {
			t.Fatalf("got pages len %d, want %d", got, want)
		}
	}

	rootPage := findPageByURL(t, report.Pages, "http://simple.test")
	{
		got := rootPage.Depth
		want := 0
		if got != want {
			t.Fatalf("got root page depth %d, want %d", got, want)
		}
	}

	aboutPage := findPageByURL(t, report.Pages, "http://simple.test/about.html")
	{
		got := aboutPage.Depth
		want := 1
		if got != want {
			t.Fatalf("got about page depth %d, want %d", got, want)
		}
	}

	{
		got := aboutPage.SEO.Title
		want := "About"
		if got != want {
			t.Fatalf("got about page title %q, want %q", got, want)
		}
	}

	contactsPage := findPageByURL(t, report.Pages, "http://simple.test/contacts.html")
	{
		got := contactsPage.Depth
		want := 1
		if got != want {
			t.Fatalf("got contacts page depth %d, want %d", got, want)
		}
	}
}

func TestAnalyze_DoesNotReportDuplicateInternalPages(t *testing.T) {
	rootBody := `
		<a href="/about.html">about</a>
		<a href="/about.html#top">about again</a>
	`

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			switch rq.URL.String() {
			case "http://simple.test":
				return newTestResponse(rq, http.StatusOK, "200 OK", rootBody), nil
			case "http://simple.test/about.html":
				return newTestResponse(rq, http.StatusOK, "200 OK", `<title>About</title>`), nil
			default:
				t.Fatalf("got unexpected request URL %q", rq.URL.String())
				return nil, nil
			}
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        "http://simple.test",
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
		want := 2
		if got != want {
			t.Fatalf("got pages len %d, want %d", got, want)
		}
	}
}

func TestAnalyze_DoesNotReportRootPageTwiceWhenHomeLinkHasSlash(t *testing.T) {
	htmlBody := `<a href="/">home</a>`

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			switch rq.URL.String() {
			case "http://simple.test":
				return newTestResponse(rq, http.StatusOK, "200 OK", htmlBody), nil
			case "http://simple.test/":
				return newTestResponse(rq, http.StatusOK, "200 OK", "ok"), nil
			default:
				t.Fatalf("got unexpected request URL %q", rq.URL.String())
				return nil, nil
			}
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        "http://simple.test",
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

	{
		got := report.Pages[0].URL
		want := "http://simple.test"
		if got != want {
			t.Fatalf("got page URL %q, want %q", got, want)
		}
	}
}

func TestAnalyze_NormalizesNonPositiveDepthInReport(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			return newTestResponse(rq, http.StatusOK, "200 OK", "ok"), nil
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        "http://simple.test",
		Depth:      0,
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
		got := report.Depth
		want := 1
		if got != want {
			t.Fatalf("got report depth %d, want %d", got, want)
		}
	}

	{
		got := len(report.Pages)
		want := 1
		if got != want {
			t.Fatalf("got pages len %d, want %d", got, want)
		}
	}
}

func TestRequestLimiter_WaitsBetweenRequests(t *testing.T) {
	var virtualNow time.Duration

	limiter := newRequestLimiterWithWait(Options{
		Delay: 200 * time.Millisecond,
	}, func(ctx context.Context, delay time.Duration) error {
		virtualNow += delay
		return nil
	})

	starts := make([]time.Duration, 0, 4)

	for range 4 {
		if err := limiter.Wait(context.Background()); err != nil {
			t.Fatalf("got wait error %v, want nil", err)
		}

		starts = append(starts, virtualNow)
	}

	for i := 1; i < len(starts); i++ {
		got := starts[i] - starts[i-1]
		want := 200 * time.Millisecond

		if got < want {
			t.Fatalf("got interval %s, want at least %s", got, want)
		}
	}
}

func TestRequestLimiter_RPSOverridesDelay(t *testing.T) {
	waits := make([]time.Duration, 0)

	limiter := newRequestLimiterWithWait(Options{
		Delay: time.Second,
		RPS:   5,
	}, func(ctx context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		return nil
	})

	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("got first wait error %v, want nil", err)
	}

	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("got second wait error %v, want nil", err)
	}

	{
		got := len(waits)
		want := 1

		if got != want {
			t.Fatalf("got waits len %d, want %d", got, want)
		}
	}

	{
		got := waits[0]
		want := 200 * time.Millisecond

		if got != want {
			t.Fatalf("got wait duration %s, want %s", got, want)
		}
	}
}

func TestRequestLimiter_NoLimitDoesNotWait(t *testing.T) {
	called := false

	limiter := newRequestLimiterWithWait(Options{}, func(ctx context.Context, delay time.Duration) error {
		called = true
		return nil
	})

	for range 3 {
		if err := limiter.Wait(context.Background()); err != nil {
			t.Fatalf("got wait error %v, want nil", err)
		}
	}

	{
		got := called
		want := false

		if got != want {
			t.Fatalf("got wait called %t, want %t", got, want)
		}
	}
}

func TestRequestLimiter_ContextCancelStopsWaiting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	waitStarted := make(chan struct{})

	limiter := newRequestLimiterWithWait(Options{
		Delay: time.Hour,
	}, func(ctx context.Context, delay time.Duration) error {
		close(waitStarted)
		<-ctx.Done()

		return ctx.Err()
	})

	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("got first wait error %v, want nil", err)
	}

	errCh := make(chan error, 1)

	go func() {
		errCh <- limiter.Wait(ctx)
	}()

	<-waitStarted
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got wait error %v, want %v", err, context.Canceled)
		}
	case <-time.After(time.Second):
		t.Fatal("got hanging wait, want context cancellation to stop it")
	}
}

func TestAnalyze_LimitedAndUnlimitedSpeedProcessSamePages(t *testing.T) {
	tests := []struct {
		name  string
		delay time.Duration
	}{
		{
			name:  "unlimited",
			delay: 0,
		},
		{
			name:  "limited",
			delay: time.Nanosecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{
				Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
					pages := map[string]string{
						"http://example.com":       `<html><body><a href="/about">About</a></body></html>`,
						"http://example.com/about": `<html><body>About</body></html>`,
					}

					body, ok := pages[rq.URL.String()]
					if !ok {
						rs := &http.Response{
							StatusCode: http.StatusNotFound,
							Status:     "404 Not Found",
							Header:     make(http.Header),
							Body:       io.NopCloser(strings.NewReader("not found")),
							Request:    rq,
						}

						return rs, nil
					}

					rs := &http.Response{
						StatusCode: http.StatusOK,
						Status:     "200 OK",
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(body)),
						Request:    rq,
					}

					return rs, nil
				}),
			}

			result, err := Analyze(context.Background(), Options{
				URL:        "http://example.com",
				Depth:      2,
				Delay:      tt.delay,
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
				want := 2

				if got != want {
					t.Fatalf("got pages len %d, want %d", got, want)
				}
			}

			for _, page := range report.Pages {
				got := page.Status
				want := PageStatusOK

				if got != want {
					t.Fatalf("got page %q status %q, want %q", page.URL, got, want)
				}
			}
		})
	}
}

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
