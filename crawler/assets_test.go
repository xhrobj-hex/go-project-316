package crawler

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
)

// TestAnalyze_ReportsAssetsFromHTMLPage проверяет, что crawler добавляет
// в отчет изображения, скрипты и стили с типом, статусом, размером и ошибкой.
func TestAnalyze_ReportsAssetsFromHTMLPage(t *testing.T) {
	const (
		mockedPageURL = mockedBaseURL + "/index.html"
		mockedLogoURL = mockedBaseURL + "/static/logo.png"
		mockedJSURL   = mockedBaseURL + "/static/app.js"
		mockedCSSURL  = mockedBaseURL + "/static/app.css"
	)

	logoBody := "image-body"
	jsBody := "console.log('ok')"
	cssBody := "body{}"

	htmlBody := `
		<!doctype html>
		<html>
		<head>
			<link rel="stylesheet" href="/static/app.css">
			<script src="/static/app.js"></script>
		</head>
		<body>
			<img src="/static/logo.png">
		</body>
		</html>
	`

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			switch rq.URL.String() {
			case mockedPageURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", htmlBody), nil
			case mockedCSSURL:
				rs := newTestResponse(rq, http.StatusOK, "200 OK", cssBody)
				rs.ContentLength = int64(len(cssBody))
				return rs, nil
			case mockedJSURL:
				rs := newTestResponse(rq, http.StatusOK, "200 OK", jsBody)
				rs.ContentLength = int64(len(jsBody))
				return rs, nil
			case mockedLogoURL:
				rs := newTestResponse(rq, http.StatusOK, "200 OK", logoBody)
				rs.ContentLength = int64(len(logoBody))
				return rs, nil
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

	page := report.Pages[0]

	{
		got := len(page.Assets)
		want := 3
		if got != want {
			t.Fatalf("got assets len %d, want %d", got, want)
		}
	}

	{
		got := page.Assets[0]
		want := Asset{
			URL:        mockedCSSURL,
			Type:       AssetTypeStyle,
			StatusCode: http.StatusOK,
			SizeBytes:  int64(len(cssBody)),
			Error:      "",
		}
		if got != want {
			t.Fatalf("got asset %+v, want %+v", got, want)
		}
	}

	{
		got := page.Assets[1]
		want := Asset{
			URL:        mockedJSURL,
			Type:       AssetTypeScript,
			StatusCode: http.StatusOK,
			SizeBytes:  int64(len(jsBody)),
			Error:      "",
		}
		if got != want {
			t.Fatalf("got asset %+v, want %+v", got, want)
		}
	}

	{
		got := page.Assets[2]
		want := Asset{
			URL:        mockedLogoURL,
			Type:       AssetTypeImage,
			StatusCode: http.StatusOK,
			SizeBytes:  int64(len(logoBody)),
			Error:      "",
		}
		if got != want {
			t.Fatalf("got asset %+v, want %+v", got, want)
		}
	}
}

// TestAnalyze_RequestsSameAssetOnceAcrossPages проверяет, что один и тот же
// ассет на разных страницах запрашивается только один раз.
func TestAnalyze_RequestsSameAssetOnceAcrossPages(t *testing.T) {
	const (
		mockedRootURL  = mockedBaseURL + "/index.html"
		mockedAboutURL = mockedBaseURL + "/about.html"
		mockedLogoURL  = mockedBaseURL + "/static/logo.png"
	)

	logoBody := "logo"

	rootBody := `
		<!doctype html>
		<html>
		<body>
			<img src="/static/logo.png">
			<a href="/about.html">about</a>
		</body>
		</html>
	`

	aboutBody := `
		<!doctype html>
		<html>
		<body>
			<img src="/static/logo.png">
		</body>
		</html>
	`

	requestCounts := make(map[string]int)
	var requestCountsMu sync.Mutex

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			requestedURL := rq.URL.String()

			requestCountsMu.Lock()
			requestCounts[requestedURL]++
			requestCountsMu.Unlock()

			switch requestedURL {
			case mockedRootURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", rootBody), nil
			case mockedAboutURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", aboutBody), nil
			case mockedLogoURL:
				rs := newTestResponse(rq, http.StatusOK, "200 OK", logoBody)
				rs.ContentLength = int64(len(logoBody))
				return rs, nil
			default:
				t.Fatalf("got unexpected request URL %q", requestedURL)
				return nil, nil
			}
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        mockedRootURL,
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

	rootPage := findPageByURL(t, report.Pages, mockedRootURL)
	aboutPage := findPageByURL(t, report.Pages, mockedAboutURL)

	{
		got := len(rootPage.Assets)
		want := 1
		if got != want {
			t.Fatalf("got root page assets len %d, want %d", got, want)
		}
	}

	{
		got := len(aboutPage.Assets)
		want := 1
		if got != want {
			t.Fatalf("got about page assets len %d, want %d", got, want)
		}
	}

	{
		got := rootPage.Assets[0]
		want := aboutPage.Assets[0]
		if got != want {
			t.Fatalf("got about page asset %+v, want %+v", got, want)
		}
	}

	{
		requestCountsMu.Lock()
		got := requestCounts[mockedLogoURL]
		requestCountsMu.Unlock()

		want := 1
		if got != want {
			t.Fatalf("got logo requests count %d, want %d", got, want)
		}
	}
}

// TestAnalyze_UsesBodySizeWhenContentLengthIsMissing проверяет, что при
// отсутствующем Content-Length размер ассета считается по телу ответа.
func TestAnalyze_UsesBodySizeWhenContentLengthIsMissing(t *testing.T) {
	const (
		mockedPageURL = mockedBaseURL + "/index.html"
		mockedJSURL   = mockedBaseURL + "/static/app.js"
	)

	jsBody := "hello"

	htmlBody := `
		<!doctype html>
		<html>
		<body>
			<script src="/static/app.js"></script>
		</body>
		</html>
	`

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			switch rq.URL.String() {
			case mockedPageURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", htmlBody), nil
			case mockedJSURL:
				rs := newTestResponse(rq, http.StatusOK, "200 OK", jsBody)
				rs.ContentLength = -1
				return rs, nil
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

	page := report.Pages[0]

	{
		got := len(page.Assets)
		want := 1
		if got != want {
			t.Fatalf("got assets len %d, want %d", got, want)
		}
	}

	{
		got := page.Assets[0].SizeBytes
		want := int64(len(jsBody))
		if got != want {
			t.Fatalf("got asset size bytes %d, want %d", got, want)
		}
	}
}

// TestAnalyze_ReportsFailedAsset проверяет, что ассет с HTTP-ошибкой
// попадает в отчет со статусом и текстом ошибки.
func TestAnalyze_ReportsFailedAsset(t *testing.T) {
	const (
		mockedPageURL  = mockedBaseURL + "/index.html"
		mockedImageURL = mockedBaseURL + "/images/missing.png"
	)

	imageBody := "not found"

	htmlBody := `
		<!doctype html>
		<html>
		<body>
			<img src="/images/missing.png">
		</body>
		</html>
	`

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			switch rq.URL.String() {
			case mockedPageURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", htmlBody), nil
			case mockedImageURL:
				rs := newTestResponse(rq, http.StatusNotFound, "404 Not Found", imageBody)
				rs.ContentLength = int64(len(imageBody))
				return rs, nil
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

	page := report.Pages[0]

	{
		got := len(page.Assets)
		want := 1
		if got != want {
			t.Fatalf("got assets len %d, want %d", got, want)
		}
	}

	asset := page.Assets[0]

	{
		got := asset.URL
		want := mockedImageURL
		if got != want {
			t.Fatalf("got asset URL %q, want %q", got, want)
		}
	}

	{
		got := asset.Type
		want := AssetTypeImage
		if got != want {
			t.Fatalf("got asset type %q, want %q", got, want)
		}
	}

	{
		got := asset.StatusCode
		want := http.StatusNotFound
		if got != want {
			t.Fatalf("got asset status code %d, want %d", got, want)
		}
	}

	{
		got := asset.SizeBytes
		want := int64(len(imageBody))
		if got != want {
			t.Fatalf("got asset size bytes %d, want %d", got, want)
		}
	}

	{
		got := page.Assets[0].Error
		want := "Not Found"
		if got != want {
			t.Fatalf("got asset error %q, want %q", got, want)
		}
	}
}

// TestAnalyze_ReportsFaviconAsOtherAsset проверяет, что favicon из link rel="icon"
// попадает в отчет как ассет типа other.
func TestAnalyze_ReportsFaviconAsOtherAsset(t *testing.T) {
	const (
		mockedPageURL    = mockedBaseURL + "/index.html"
		mockedFaviconURL = mockedBaseURL + "/favicon.ico"
	)

	faviconBody := "ico"

	htmlBody := `
		<!doctype html>
		<html>
		<head>
			<link rel="icon" href="/favicon.ico">
		</head>
		<body></body>
		</html>
	`

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			switch rq.URL.String() {
			case mockedPageURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", htmlBody), nil
			case mockedFaviconURL:
				rs := newTestResponse(rq, http.StatusOK, "200 OK", faviconBody)
				rs.ContentLength = int64(len(faviconBody))
				return rs, nil
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

	page := report.Pages[0]

	{
		got := len(page.Assets)
		want := 1
		if got != want {
			t.Fatalf("got assets len %d, want %d", got, want)
		}
	}

	{
		got := page.Assets[0]
		want := Asset{
			URL:        mockedFaviconURL,
			Type:       AssetTypeOther,
			StatusCode: http.StatusOK,
			SizeBytes:  int64(len(faviconBody)),
			Error:      "",
		}
		if got != want {
			t.Fatalf("got asset %+v, want %+v", got, want)
		}
	}
}
