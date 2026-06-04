package crawler

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestAnalyze_ChecksLinksInParallelAccordingToConcurrency(t *testing.T) {
	const (
		mockedPageURL  = mockedBaseURL + "/index.html"
		mockedLink1URL = mockedBaseURL + "/link-1"
		mockedLink2URL = mockedBaseURL + "/link-2"
		mockedLink3URL = mockedBaseURL + "/link-3"
	)

	htmlBody := `
		<html>
			<body>
				<a href="/link-1">link 1</a>
				<a href="/link-2">link 2</a>
				<a href="/link-3">link 3</a>
			</body>
		</html>
	`

	var requestsMu sync.Mutex
	activeRequests := 0
	maxActiveRequests := 0

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			requestedURL := rq.URL.String()

			switch requestedURL {
			case mockedPageURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", htmlBody), nil

			case mockedLink1URL, mockedLink2URL, mockedLink3URL:
				requestsMu.Lock()
				activeRequests++
				if activeRequests > maxActiveRequests {
					maxActiveRequests = activeRequests
				}
				requestsMu.Unlock()

				time.Sleep(50 * time.Millisecond)

				requestsMu.Lock()
				activeRequests--
				requestsMu.Unlock()

				return newTestResponse(rq, http.StatusOK, "200 OK", "ok"), nil

			default:
				t.Fatalf("got unexpected request URL %q", requestedURL)
				return nil, nil
			}
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:         mockedPageURL,
		Depth:       1,
		Concurrency: 3,
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
		want := 1
		if got != want {
			t.Fatalf("got pages len %d, want %d", got, want)
		}
	}

	{
		got := len(report.Pages[0].BrokenLinks)
		want := 0
		if got != want {
			t.Fatalf("got broken links len %d, want %d", got, want)
		}
	}

	requestsMu.Lock()
	got := maxActiveRequests
	requestsMu.Unlock()

	want := 2
	if got < want {
		t.Fatalf("got max active requests %d, want at least %d", got, want)
	}
}
