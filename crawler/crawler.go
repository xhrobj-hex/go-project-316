package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type crawlItem struct {
	url   string
	depth int
}

// Step1: Основная точка входа в crawler — функция:
//
// func Analyze(ctx context.Context, opts Options) ([]byte, error)
//
// Она принимает context.Context, чтобы можно было отменять обход, и структуру Options с параметрами запуска
// (URL, Depth, Retries, Delay, Timeout, UserAgent, Concurrency, IndentJSON, HTTPClient).
//
// Функция возвращает JSON-отчет в виде байтового слайса и ошибку.

func Analyze(ctx context.Context, opts Options) ([]byte, error) {
	if opts.URL == "" {
		return nil, errors.New("url is required")
	}

	if opts.HTTPClient == nil {
		return nil, errors.New("http client is required")
	}

	generatedAt := time.Now().UTC().Format(time.RFC3339)

	maxDepth := opts.Depth
	if maxDepth < 1 {
		maxDepth = 1
	}

	queue := []crawlItem{
		{
			url:   opts.URL,
			depth: 0,
		},
	}

	seen := map[string]struct{}{
		opts.URL: {},
	}

	pages := make([]PageReport, 0)

	for len(queue) > 0 {
		if ctx.Err() != nil {
			break
		}

		item := queue[0]
		queue = queue[1:]

		page, body := analyzePage(ctx, opts, item.url, item.depth, generatedAt)
		pages = append(pages, page)

		if page.Status != PageStatusOK {
			continue
		}

		nextDepth := item.depth + 1
		if nextDepth >= maxDepth {
			continue
		}

		for _, link := range extractInternalPageLinks(opts.URL, page.URL, body) {
			if _, exists := seen[link]; exists {
				continue
			}

			seen[link] = struct{}{}
			queue = append(queue, crawlItem{
				url:   link,
				depth: nextDepth,
			})
		}
	}

	return buildReport(opts, pages, generatedAt)
}

func analyzePage(ctx context.Context, opts Options, pageURL string, depth int, discoveredAt string) (PageReport, []byte) {
	page := PageReport{
		URL:          pageURL,
		Depth:        depth,
		BrokenLinks:  []BrokenLink{},
		DiscoveredAt: discoveredAt,
	}

	rq, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		page.Status = PageStatusError
		page.Error = err.Error()
		return page, nil
	}

	if opts.UserAgent != "" {
		rq.Header.Set("User-Agent", opts.UserAgent)
	}

	rs, err := opts.HTTPClient.Do(rq)
	if err != nil {
		page.Status = PageStatusError
		page.Error = err.Error()
		return page, nil
	}
	defer func() {
		_ = rs.Body.Close()
	}()

	body, err := io.ReadAll(rs.Body)
	if err != nil {
		page.HTTPStatus = rs.StatusCode
		page.Status = PageStatusError
		page.Error = err.Error()
		return page, nil
	}

	page.HTTPStatus = rs.StatusCode
	page.SEO = extractSEO(body)

	if rs.StatusCode >= http.StatusOK && rs.StatusCode < http.StatusBadRequest {
		page.Status = PageStatusOK
		page.BrokenLinks = findBrokenLinks(ctx, opts, page.URL, body)
	} else {
		page.Status = PageStatusError
		page.Error = rs.Status
	}

	return page, body
}

func buildReport(opts Options, pages []PageReport, generatedAt string) ([]byte, error) {
	report := Report{
		RootURL:     opts.URL,
		Depth:       opts.Depth,
		GeneratedAt: generatedAt,
		Pages:       pages,
	}

	if opts.IndentJSON {
		return json.MarshalIndent(report, "", " ")
	}

	return json.Marshal(report)
}

func findBrokenLinks(ctx context.Context, opts Options, pageURL string, body []byte) []BrokenLink {
	links := extractLinks(pageURL, body)

	brokenLinks := make([]BrokenLink, 0)

	for _, link := range links {
		brokenLink, ok := checkBrokenLink(ctx, opts, link)
		if ok {
			brokenLinks = append(brokenLinks, brokenLink)
		}
	}

	return brokenLinks
}

func extractLinks(pageURL string, body []byte) []string {
	baseURL, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}

	document, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil
	}

	links := make([]string, 0)
	seen := make(map[string]struct{})

	var walk func(node *html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			for _, attr := range node.Attr {
				key := strings.ToLower(attr.Key)
				if key != "href" && key != "src" {
					continue
				}

				link, ok := normalizeLink(baseURL, attr.Val)
				if !ok {
					continue
				}

				if _, exists := seen[link]; exists {
					continue
				}

				seen[link] = struct{}{}
				links = append(links, link)
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	walk(document)

	return links
}

func normalizeLink(baseURL *url.URL, rawLink string) (string, bool) {
	rawLink = strings.TrimSpace(rawLink)
	if rawLink == "" || strings.HasPrefix(rawLink, "#") {
		return "", false
	}

	parsedLink, err := url.Parse(rawLink)
	if err != nil {
		return "", false
	}

	if parsedLink.Scheme != "" && !isSupportedScheme(parsedLink.Scheme) {
		return "", false
	}

	resolvedLink := baseURL.ResolveReference(parsedLink)
	if !isSupportedScheme(resolvedLink.Scheme) || resolvedLink.Host == "" {
		return "", false
	}

	resolvedLink.Fragment = ""

	return resolvedLink.String(), true
}

func isSupportedScheme(scheme string) bool {
	return scheme == "http" || scheme == "https"
}

func checkBrokenLink(ctx context.Context, opts Options, link string) (BrokenLink, bool) {
	rq, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return BrokenLink{
			URL:   link,
			Error: err.Error(),
		}, true
	}

	if opts.UserAgent != "" {
		rq.Header.Set("User-Agent", opts.UserAgent)
	}

	rs, err := opts.HTTPClient.Do(rq)
	if err != nil {
		return BrokenLink{
			URL:   link,
			Error: err.Error(),
		}, true
	}
	defer func() {
		_ = rs.Body.Close()
	}()

	_, _ = io.Copy(io.Discard, rs.Body)

	if rs.StatusCode >= http.StatusBadRequest {
		return BrokenLink{
			URL:        link,
			StatusCode: rs.StatusCode,
		}, true
	}

	return BrokenLink{}, false
}

func extractInternalPageLinks(rootPageURL string, pageURL string, body []byte) []string {
	rootURL, err := url.Parse(rootPageURL)
	if err != nil {
		return nil
	}

	baseURL, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}

	document, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil
	}

	links := make([]string, 0)
	seen := make(map[string]struct{})

	var walk func(node *html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "a") {
			for _, attr := range node.Attr {
				if !strings.EqualFold(attr.Key, "href") {
					continue
				}

				link, ok := normalizeLink(baseURL, attr.Val)
				if !ok {
					continue
				}

				parsedLink, err := url.Parse(link)
				if err != nil {
					continue
				}

				if !strings.EqualFold(parsedLink.Host, rootURL.Host) {
					continue
				}

				if _, exists := seen[link]; exists {
					continue
				}

				seen[link] = struct{}{}
				links = append(links, link)
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	walk(document)

	return links
}
