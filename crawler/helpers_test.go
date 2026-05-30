package crawler

import (
	"io"
	"net/http"
	"strings"
	"testing"
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

func newTestResponse(rq *http.Request, statusCode int, status string, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    rq,
	}
}
