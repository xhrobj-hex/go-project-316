package crawler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// TestAnalyze_ReportsBrokenLinksFromHTMLPage проверяет, что crawler добавляет
// в отчет только недоступные ссылки и игнорирует пустые, якорные и неподдерживаемые ссылки.
func TestAnalyze_ReportsBrokenLinksFromHTMLPage(t *testing.T) {
	const (
		mockedPageURL         = mockedBaseURL + "/blog/index.html"
		mockedAppCSSURL       = mockedBaseURL + "/assets/app.css"
		mockedGhostCSSURL     = mockedBaseURL + "/assets/ghost.css"
		mockedBlogPostURL     = mockedBaseURL + "/blog/post.html"
		mockedMissingImageURL = mockedBaseURL + "/images/missing.png"
	)

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
				<a href="mailto:team@crawler.test">email link</a>
				<a href="javascript:void(0)">javascript link</a>
			</body>
		</html>
	`

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			switch rq.URL.String() {
			case mockedPageURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", htmlBody), nil
			case mockedAppCSSURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", "ok"), nil
			case mockedBlogPostURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", "ok"), nil
			case mockedGhostCSSURL:
				return newTestResponse(rq, http.StatusNotFound, "404 Not Found", "not found"), nil
			case mockedMissingImageURL:
				return nil, errors.New("network is down")
			default:
				t.Fatalf("got unexpected request URL %q", rq.URL.String())
				return nil, nil
			}
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        mockedPageURL,
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
		want := mockedPageURL
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
		want := mockedGhostCSSURL
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
		want := mockedMissingImageURL
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
