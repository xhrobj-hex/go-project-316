package crawler

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"
)

// TestAnalyze_AnalyzesPagesInParallelAccordingToConcurrency проверяет, что
// Concurrency применяется к анализу страниц, а не только к проверке ссылок.
func TestAnalyze_AnalyzesPagesInParallelAccordingToConcurrency(t *testing.T) {
	const (
		mockedRootURL  = mockedBaseURL + "/index.html"
		mockedPage1URL = mockedBaseURL + "/page-1.html"
		mockedPage2URL = mockedBaseURL + "/page-2.html"
	)

	rootBody := `
		<html>
			<body>
				<a href="/page-1.html">page 1</a>
				<a href="/page-2.html">page 2</a>
			</body>
		</html>
	`

	pageBody := `
		<html>
			<body>
				<p>page</p>
			</body>
		</html>
	`

	var requestsMu sync.Mutex
	requestsByURL := make(map[string]int)
	activePageRequests := 0
	maxActivePageRequests := 0

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			requestedURL := rq.URL.String()

			switch requestedURL {
			case mockedRootURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", rootBody), nil

			case mockedPage1URL, mockedPage2URL:
				requestsMu.Lock()
				requestsByURL[requestedURL]++
				requestNumber := requestsByURL[requestedURL]
				isPageAnalysisRequest := requestNumber > 1

				if isPageAnalysisRequest {
					activePageRequests++
					if activePageRequests > maxActivePageRequests {
						maxActivePageRequests = activePageRequests
					}
				}
				requestsMu.Unlock()

				if isPageAnalysisRequest {
					time.Sleep(50 * time.Millisecond)

					requestsMu.Lock()
					activePageRequests--
					requestsMu.Unlock()
				}

				return newTestResponse(rq, http.StatusOK, "200 OK", pageBody), nil

			default:
				t.Fatalf("got unexpected request URL %q", requestedURL)
				return nil, nil
			}
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:         mockedRootURL,
		Depth:       2,
		Concurrency: 2,
		HTTPClient:  client,
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

	requestsMu.Lock()
	got := maxActivePageRequests
	requestsMu.Unlock()

	want := 2
	if got < want {
		t.Fatalf("got max active page requests %d, want at least %d", got, want)
	}
}
