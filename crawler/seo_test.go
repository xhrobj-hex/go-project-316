package crawler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// TestAnalyze_ReportsSEOBlockForExistingTags проверяет, что crawler
// извлекает SEO-поля из существующих title, meta description и h1.
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
			want := mockedBaseURL

			if got != want {
				t.Fatalf("got request URL %q, want %q", got, want)
			}

			return newTestResponse(rq, http.StatusOK, "200 OK", htmlBody), nil
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

// TestAnalyze_ReportsEmptySEOBlockWhenTagsAreMissing проверяет, что crawler
// возвращает пустые SEO-поля, если на странице нет title, description и h1.
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
			got := rq.URL.String()
			want := mockedBaseURL

			if got != want {
				t.Fatalf("got request URL %q, want %q", got, want)
			}

			return newTestResponse(rq, http.StatusOK, "200 OK", htmlBody), nil
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

// TestAnalyze_DecodesHTMLEntitiesInSEOText проверяет,
// что crawler декодирует HTML-сущности в SEO-тексте.
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
			got := rq.URL.String()
			want := mockedBaseURL

			if got != want {
				t.Fatalf("got request URL %q, want %q", got, want)
			}

			return newTestResponse(rq, http.StatusOK, "200 OK", htmlBody), nil
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
