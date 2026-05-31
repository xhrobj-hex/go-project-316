package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestAnalyze_ReturnsExpectedJSONContract(t *testing.T) {
	result := analyzeJSONContractFixture(t, false)

	want := `{"root_url":"https://example.com","depth":1,"generated_at":"2024-06-01T12:34:56Z","pages":[{"url":"https://example.com","depth":0,"http_status":200,"status":"ok","error":"","seo":{"has_title":true,"title":"Example title","has_description":true,"description":"Example description","has_h1":true},"broken_links":[{"url":"https://example.com/missing","status_code":404,"error":"Not Found"}],"assets":[{"url":"https://example.com/static/logo.png","type":"image","status_code":200,"size_bytes":12345,"error":""}],"discovered_at":"2024-06-01T12:34:56Z"}]}`

	if string(result) != want {
		t.Fatalf("got JSON:\n%s\nwant:\n%s", result, want)
	}
}

func TestAnalyze_IndentJSONChangesOnlyFormatting(t *testing.T) {
	compact := analyzeJSONContractFixture(t, false)
	indented := analyzeJSONContractFixture(t, true)

	if bytes.Equal(compact, indented) {
		t.Fatal("got identical JSON, want formatting to differ")
	}

	var normalized bytes.Buffer
	if err := json.Compact(&normalized, indented); err != nil {
		t.Fatalf("got compact error %v, want nil", err)
	}

	if normalized.String() != string(compact) {
		t.Fatalf("got compacted indented JSON:\n%s\nwant:\n%s", normalized.String(), compact)
	}
}

func analyzeJSONContractFixture(t *testing.T, indentJSON bool) []byte {
	t.Helper()

	const (
		rootURL    = "https://example.com"
		missingURL = "https://example.com/missing"
		logoURL    = "https://example.com/static/logo.png"
	)

	fixedTime := time.Date(2024, time.June, 1, 12, 34, 56, 0, time.UTC)

	htmlBody := `<!doctype html>
<html>
<head>
<title>Example title</title>
<meta name="description" content="Example description">
</head>
<body>
<h1>Hello</h1>
<a href="/missing">missing</a>
<img src="/static/logo.png" alt="Logo">
</body>
</html>`

	client := &http.Client{
		Transport: roundTripFunc(func(rq *http.Request) (*http.Response, error) {
			switch rq.URL.String() {
			case rootURL:
				return newTestResponse(rq, http.StatusOK, "200 OK", htmlBody), nil
			case missingURL:
				return newTestResponse(rq, http.StatusNotFound, "404 Not Found", "not found"), nil
			case logoURL:
				rs := newTestResponse(rq, http.StatusOK, "200 OK", "logo")
				rs.ContentLength = 12345

				return rs, nil
			default:
				t.Fatalf("got unexpected request URL %q", rq.URL.String())
				return nil, nil
			}
		}),
	}

	result, err := Analyze(context.Background(), Options{
		URL:        rootURL,
		Depth:      1,
		IndentJSON: indentJSON,
		HTTPClient: client,
		Now: func() time.Time {
			return fixedTime
		},
	})
	if err != nil {
		t.Fatalf("got error %v, want nil", err)
	}

	return result
}
