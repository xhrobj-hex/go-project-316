package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

type crawlItem struct {
	url   string
	depth int
}

type crawlJob struct {
	index int
	item  crawlItem
}

type crawlPageResult struct {
	index int
	item  crawlItem
	page  PageReport
	body  []byte
}

type assetCandidate struct {
	url       string
	assetType string
}

type resourceInfo struct {
	statusCode int
	sizeBytes  int64
	error      string
}

type resourceCache struct {
	mu    sync.Mutex
	items map[string]resourceInfo
}

const defaultRetryDelay = time.Millisecond * 100

// Analyze запускает обход сайта и возвращает JSON-отчёт.
//
// Функция начинает обход с opts.URL, загружает страницы до заданной глубины,
// собирает SEO-данные, проверяет ссылки на недоступные ресурсы и формирует
// итоговый отчёт в формате JSON.
//
// Обход можно отменить через ctx. Перед каждым HTTP-запросом применяется
// ограничение скорости из opts.Delay или opts.RPS, если оно задано.
//
// Для работы функции обязательны opts.URL и opts.HTTPClient.
func Analyze(ctx context.Context, opts Options) ([]byte, error) {
	if opts.URL == "" {
		return nil, errors.New("url is required")
	}

	if opts.HTTPClient == nil {
		return nil, errors.New("http client is required")
	}

	generatedAt := reportTime(opts).Format(time.RFC3339)
	maxDepth := normalizeDepth(opts.Depth)
	limiter := newRequestLimiter(opts)
	cache := newResourceCache()

	currentItems := []crawlItem{
		{
			url:   opts.URL,
			depth: 0,
		},
	}

	seen := map[string]struct{}{
		pageKey(opts.URL): {},
	}

	pages := make([]PageReport, 0)

	for len(currentItems) > 0 {
		if ctx.Err() != nil {
			break
		}

		layerResults := analyzePages(ctx, opts, limiter, generatedAt, cache, currentItems)
		nextItems := make([]crawlItem, 0)

		for _, result := range layerResults {
			pages = append(pages, result.page)

			if result.page.Status != PageStatusOK {
				continue
			}

			nextDepth := result.item.depth + 1
			if nextDepth >= maxDepth {
				continue
			}

			for _, link := range extractInternalPageLinks(opts.URL, result.page.URL, result.body) {
				key := pageKey(link)
				if _, exists := seen[key]; exists {
					continue
				}

				seen[key] = struct{}{}

				nextItems = append(nextItems, crawlItem{
					url:   link,
					depth: nextDepth,
				})
			}
		}

		currentItems = nextItems
	}

	return buildReport(opts, maxDepth, pages, generatedAt)
}

func analyzePages(
	ctx context.Context,
	opts Options,
	limiter *requestLimiter,
	generatedAt string,
	cache *resourceCache,
	items []crawlItem,
) []crawlPageResult {
	if len(items) == 0 {
		return nil
	}

	workers := normalizeConcurrency(opts.Concurrency)
	if workers > len(items) {
		workers = len(items)
	}

	jobs := make(chan crawlJob, len(items))
	results := make(chan crawlPageResult, len(items))

	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for job := range jobs {
				page, body := analyzePage(
					ctx,
					opts,
					limiter,
					job.item.url,
					job.item.depth,
					generatedAt,
					cache,
				)

				results <- crawlPageResult{
					index: job.index,
					item:  job.item,
					page:  page,
					body:  body,
				}
			}
		}()
	}

	for index, item := range items {
		jobs <- crawlJob{
			index: index,
			item:  item,
		}
	}
	close(jobs)

	wg.Wait()
	close(results)

	layerResults := make([]crawlPageResult, len(items))
	for result := range results {
		layerResults[result.index] = result
	}

	return layerResults
}

func newResourceCache() *resourceCache {
	return &resourceCache{
		items: make(map[string]resourceInfo),
	}
}

func (cache *resourceCache) get(rawURL string) (resourceInfo, bool) {
	if cache == nil {
		return resourceInfo{}, false
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	info, exists := cache.items[rawURL]

	return info, exists
}

func (cache *resourceCache) set(rawURL string, info resourceInfo) resourceInfo {
	if cache == nil {
		return info
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	if storedInfo, exists := cache.items[rawURL]; exists {
		return storedInfo
	}

	cache.items[rawURL] = info

	return info
}

func normalizeDepth(depth int) int {
	if depth < 1 {
		return 1
	}

	return depth
}

func normalizeConcurrency(concurrency int) int {
	if concurrency < 1 {
		return 1
	}

	return concurrency
}

func reportTime(opts Options) time.Time {
	if opts.Now != nil {
		return opts.Now().UTC()
	}

	return time.Now().UTC()
}

func pageKey(rawPageURL string) string {
	parsedURL, err := url.Parse(rawPageURL)
	if err != nil {
		return rawPageURL
	}

	parsedURL.Scheme = strings.ToLower(parsedURL.Scheme)
	parsedURL.Host = strings.ToLower(parsedURL.Host)
	parsedURL.Fragment = ""

	if parsedURL.Path == "" {
		parsedURL.Path = "/"
	}

	if parsedURL.Path != "/" {
		parsedURL.Path = strings.TrimRight(parsedURL.Path, "/")
		if parsedURL.Path == "" {
			parsedURL.Path = "/"
		}
	}

	return parsedURL.String()
}

func analyzePage(
	ctx context.Context,
	opts Options,
	limiter *requestLimiter,
	pageURL string,
	depth int,
	discoveredAt string,
	cache *resourceCache,
) (PageReport, []byte) {
	page := PageReport{
		URL:          pageURL,
		Depth:        depth,
		DiscoveredAt: discoveredAt,
	}

	rs, err := getWithRetries(ctx, opts, limiter, pageURL)
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
		page.Assets = findAssets(ctx, opts, limiter, page.URL, body, cache)
		page.BrokenLinks = findBrokenLinks(ctx, opts, limiter, page.URL, body)
	} else {
		page.Status = PageStatusError
		page.Error = rs.Status
	}

	return page, body
}

func getWithRetries(
	ctx context.Context,
	opts Options,
	limiter *requestLimiter,
	rawURL string,
) (*http.Response, error) {
	return requestWithRetries(ctx, opts, limiter, http.MethodGet, rawURL)
}

func headWithRetries(
	ctx context.Context,
	opts Options,
	limiter *requestLimiter,
	rawURL string,
) (*http.Response, error) {
	return requestWithRetries(ctx, opts, limiter, http.MethodHead, rawURL)
}

func requestWithRetries(
	ctx context.Context,
	opts Options,
	limiter *requestLimiter,
	method string,
	rawURL string,
) (*http.Response, error) {
	retries := normalizeRetries(opts.Retries)

	for attempt := 0; ; attempt++ {
		rs, err := request(ctx, opts, limiter, method, rawURL)
		if !shouldRetry(ctx, rs, err) || attempt >= retries {
			return rs, err
		}

		closeResponseBody(rs)

		if err := waitBeforeRetry(ctx, retryDelay(opts)); err != nil {
			return nil, err
		}
	}
}

func request(
	ctx context.Context,
	opts Options,
	limiter *requestLimiter,
	method string,
	rawURL string,
) (*http.Response, error) {
	rq, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}

	if opts.UserAgent != "" {
		rq.Header.Set("User-Agent", opts.UserAgent)
	}

	if err := limiter.Wait(ctx); err != nil {
		return nil, err
	}

	return opts.HTTPClient.Do(rq)
}

func normalizeRetries(retries int) int {
	if retries < 0 {
		return 0
	}

	return retries
}

func shouldRetry(ctx context.Context, rs *http.Response, err error) bool {
	if ctx.Err() != nil {
		return false
	}

	if err != nil {
		return true
	}

	if rs == nil {
		return false
	}

	return rs.StatusCode == http.StatusTooManyRequests ||
		rs.StatusCode >= http.StatusInternalServerError
}

func retryDelay(opts Options) time.Duration {
	if opts.Delay > 0 || opts.RPS > 0 {
		return 0
	}

	return defaultRetryDelay
}

func waitBeforeRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func closeResponseBody(rs *http.Response) {
	if rs == nil || rs.Body == nil {
		return
	}

	_, _ = io.Copy(io.Discard, rs.Body)
	_ = rs.Body.Close()
}

func buildReport(opts Options, depth int, pages []PageReport, generatedAt string) ([]byte, error) {
	report := Report{
		RootURL:     opts.URL,
		Depth:       depth,
		GeneratedAt: generatedAt,
		Pages:       pages,
	}

	if opts.IndentJSON {
		return json.MarshalIndent(report, "", "  ")
	}

	return json.Marshal(report)
}

func findBrokenLinks(
	ctx context.Context,
	opts Options,
	limiter *requestLimiter,
	pageURL string,
	body []byte,
) []BrokenLink {
	links := extractInternalPageLinks(opts.URL, pageURL, body)
	brokenLinks := make([]BrokenLink, 0)

	for _, link := range links {
		if ctx.Err() != nil {
			break
		}

		brokenLink, ok := checkBrokenLink(ctx, opts, limiter, link)
		if ok {
			brokenLinks = append(brokenLinks, brokenLink)
		}
	}

	return brokenLinks
}

func findAssets(
	ctx context.Context,
	opts Options,
	limiter *requestLimiter,
	pageURL string,
	body []byte,
	cache *resourceCache,
) []Asset {
	candidates := extractAssets(pageURL, body)
	assets := make([]Asset, 0, len(candidates))

	for _, candidate := range candidates {
		if ctx.Err() != nil {
			break
		}

		info := getResourceInfo(ctx, opts, limiter, candidate.url, cache)

		assets = append(assets, Asset{
			URL:        candidate.url,
			Type:       candidate.assetType,
			StatusCode: info.statusCode,
			SizeBytes:  info.sizeBytes,
			Error:      info.error,
		})
	}

	return assets
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

func checkBrokenLink(
	ctx context.Context,
	opts Options,
	limiter *requestLimiter,
	link string,
) (BrokenLink, bool) {
	info := getBrokenLinkInfo(ctx, opts, limiter, link)

	if info.statusCode >= http.StatusBadRequest {
		return BrokenLink{
			URL:        link,
			StatusCode: info.statusCode,
			Error:      http.StatusText(info.statusCode),
		}, true
	}

	if info.error != "" {
		return BrokenLink{
			URL:   link,
			Error: info.error,
		}, true
	}

	return BrokenLink{}, false
}

func getBrokenLinkInfo(
	ctx context.Context,
	opts Options,
	limiter *requestLimiter,
	rawURL string,
) resourceInfo {
	rs, err := headWithRetries(ctx, opts, limiter, rawURL)
	if err != nil {
		return resourceInfo{
			error: err.Error(),
		}
	}

	if rs.StatusCode == http.StatusMethodNotAllowed {
		closeResponseBody(rs)

		return fetchResourceInfo(ctx, opts, limiter, rawURL)
	}

	closeResponseBody(rs)

	info := resourceInfo{
		statusCode: rs.StatusCode,
	}

	if rs.StatusCode >= http.StatusBadRequest {
		info.error = http.StatusText(rs.StatusCode)
	}

	return info
}

func getResourceInfo(
	ctx context.Context,
	opts Options,
	limiter *requestLimiter,
	rawURL string,
	cache *resourceCache,
) resourceInfo {
	if info, exists := cache.get(rawURL); exists {
		return info
	}

	info := fetchResourceInfo(ctx, opts, limiter, rawURL)

	return cache.set(rawURL, info)
}

func fetchResourceInfo(
	ctx context.Context,
	opts Options,
	limiter *requestLimiter,
	rawURL string,
) resourceInfo {
	rs, err := getWithRetries(ctx, opts, limiter, rawURL)
	if err != nil {
		return resourceInfo{
			error: err.Error(),
		}
	}
	defer func() {
		_ = rs.Body.Close()
	}()

	info := resourceInfo{
		statusCode: rs.StatusCode,
	}

	body, err := io.ReadAll(rs.Body)
	if err != nil {
		info.error = err.Error()

		return info
	}

	info.sizeBytes = responseSize(rs, body)

	if rs.StatusCode >= http.StatusBadRequest {
		info.error = http.StatusText(rs.StatusCode)
	}

	return info
}

func responseSize(rs *http.Response, body []byte) int64 {
	if rs.ContentLength >= 0 {
		return rs.ContentLength
	}

	return int64(len(body))
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

	seen := map[string]struct{}{
		pageKey(pageURL): {},
	}

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

				if !strings.EqualFold(parsedLink.Scheme, rootURL.Scheme) ||
					!strings.EqualFold(parsedLink.Host, rootURL.Host) {
					continue
				}

				key := pageKey(link)
				if _, exists := seen[key]; exists {
					continue
				}

				seen[key] = struct{}{}
				links = append(links, link)
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	walk(document)

	sort.Strings(links)

	return links
}

func extractAssets(pageURL string, body []byte) []assetCandidate {
	baseURL, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}

	document, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil
	}

	assets := make([]assetCandidate, 0)
	seen := make(map[string]struct{})

	var walk func(node *html.Node)

	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			assetType, rawURL, ok := assetFromNode(node)
			if ok {
				link, ok := normalizeLink(baseURL, rawURL)
				if ok {
					if _, exists := seen[link]; !exists {
						seen[link] = struct{}{}

						assets = append(assets, assetCandidate{
							url:       link,
							assetType: assetType,
						})
					}
				}
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	walk(document)

	sort.SliceStable(assets, func(i, j int) bool {
		leftRank := assetTypeRank(assets[i].assetType)
		rightRank := assetTypeRank(assets[j].assetType)
		if leftRank != rightRank {
			return leftRank < rightRank
		}

		return assets[i].url < assets[j].url
	})

	return assets
}

func assetTypeRank(assetType string) int {
	switch assetType {
	case AssetTypeImage:
		return 0
	case AssetTypeScript:
		return 1
	case AssetTypeStyle:
		return 2
	case AssetTypeOther:
		return 3
	default:
		return 4
	}
}

func assetFromNode(node *html.Node) (string, string, bool) {
	switch strings.ToLower(node.Data) {
	case "img":
		src, ok := attrValue(node, "src")
		src = strings.TrimSpace(src)

		return AssetTypeImage, src, ok && src != ""
	case "script":
		src, ok := attrValue(node, "src")
		src = strings.TrimSpace(src)

		return AssetTypeScript, src, ok && src != ""
	case "link":
		href, ok := attrValue(node, "href")
		href = strings.TrimSpace(href)
		if !ok || href == "" {
			return "", "", false
		}

		if hasRel(node, "stylesheet") {
			return AssetTypeStyle, href, true
		}

		if hasIconRel(node) {
			return AssetTypeOther, href, true
		}

		return "", "", false
	default:
		return "", "", false
	}
}

func hasRel(node *html.Node, value string) bool {
	rel, ok := attrValue(node, "rel")
	if !ok {
		return false
	}

	for _, part := range strings.Fields(strings.ToLower(rel)) {
		if part == value {
			return true
		}
	}

	return false
}

func hasIconRel(node *html.Node) bool {
	rel, ok := attrValue(node, "rel")
	if !ok {
		return false
	}

	for _, part := range strings.Fields(strings.ToLower(rel)) {
		if part == "icon" || strings.HasSuffix(part, "-icon") {
			return true
		}
	}

	return false
}
