package crawler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// TestAnalyze_DepthOneReportsOnlyRootPage проверяет, что при depth = 1
// crawler добавляет в отчет только стартовую страницу.
func TestAnalyze_DepthOneReportsOnlyRootPage(t *testing.T) {
	const mockedAboutURL = mockedBaseURL + "/about.html"

	htmlBody := `<a href="/about.html">about</a>`

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			switch rq.URL.String() {
			case mockedBaseURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", htmlBody), nil
			case mockedAboutURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", `<title>About</title>`), nil
			default:
				t.Fatalf("got unexpected request URL %q", rq.URL.String())
				return nil, nil
			}
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

	{
		got := report.Pages[0].URL
		want := mockedBaseURL
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

// TestAnalyze_DepthTwoReportsInternalPagesOnly проверяет, что при depth = 2
// crawler обходит внутренние страницы и не добавляет внешние страницы в очередь обхода.
func TestAnalyze_DepthTwoReportsInternalPagesOnly(t *testing.T) {
	const (
		mockedAboutURL    = mockedBaseURL + "/about.html"
		mockedContactsURL = mockedBaseURL + "/contacts.html"
		mockedExternalURL = "https://external.crawler.test/page.html"
	)

	rootBody := `
		<a href="/about.html">about</a>
		<a href="/contacts.html">contacts</a>
		<a href="https://external.crawler.test/page.html">external</a>
	`

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			switch rq.URL.String() {
			case mockedBaseURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", rootBody), nil
			case mockedAboutURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", `<title>About</title><h1>About</h1>`), nil
			case mockedContactsURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", `<title>Contacts</title>`), nil
			case mockedExternalURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", `<title>External</title>`), nil
			default:
				t.Fatalf("got unexpected request URL %q", rq.URL.String())
				return nil, nil
			}
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        mockedBaseURL,
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

	rootPage := findPageByURL(t, report.Pages, mockedBaseURL)
	{
		got := rootPage.Depth
		want := 0
		if got != want {
			t.Fatalf("got root page depth %d, want %d", got, want)
		}
	}

	aboutPage := findPageByURL(t, report.Pages, mockedAboutURL)
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

	contactsPage := findPageByURL(t, report.Pages, mockedContactsURL)
	{
		got := contactsPage.Depth
		want := 1
		if got != want {
			t.Fatalf("got contacts page depth %d, want %d", got, want)
		}
	}
}

// TestAnalyze_DoesNotReportDuplicateInternalPages проверяет, что ссылки,
// отличающиеся только fragment-частью, не добавляют дубль страницы в отчет.
func TestAnalyze_DoesNotReportDuplicateInternalPages(t *testing.T) {
	const mockedAboutURL = mockedBaseURL + "/about.html"

	rootBody := `
		<a href="/about.html">about</a>
		<a href="/about.html#top">about again</a>
	`

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			switch rq.URL.String() {
			case mockedBaseURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", rootBody), nil
			case mockedAboutURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", `<title>About</title>`), nil
			default:
				t.Fatalf("got unexpected request URL %q", rq.URL.String())
				return nil, nil
			}
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        mockedBaseURL,
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

// TestAnalyze_DoesNotReportRootPageTwiceWhenHomeLinkHasSlash проверяет,
// что стартовая страница без "/" и ссылка на "/" считаются одной страницей.
func TestAnalyze_DoesNotReportRootPageTwiceWhenHomeLinkHasSlash(t *testing.T) {
	const mockedHomeURL = mockedBaseURL + "/"

	htmlBody := `<a href="/">home</a>`

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			switch rq.URL.String() {
			case mockedBaseURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", htmlBody), nil
			case mockedHomeURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", htmlBody), nil
			default:
				t.Fatalf("got unexpected request URL %q", rq.URL.String())
				return nil, nil
			}
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        mockedBaseURL,
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
		want := mockedBaseURL
		if got != want {
			t.Fatalf("got page URL %q, want %q", got, want)
		}
	}
}

// TestAnalyze_NormalizesNonPositiveDepthInReport проверяет, что неположительная
// глубина обхода нормализуется до минимального значения 1.
func TestAnalyze_NormalizesNonPositiveDepthInReport(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			return newTestResponse(rq, http.StatusOK, "200 OK", "ok"), nil
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        mockedBaseURL,
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
