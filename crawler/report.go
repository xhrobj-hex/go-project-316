package crawler

const (
	// PageStatusOK означает, что страница успешно загружена и проанализирована.
	PageStatusOK = "ok"

	// PageStatusError означает, что страницу не удалось успешно загрузить или обработать.
	PageStatusError = "error"
)

const (
	// AssetTypeImage обозначает ассет изображения, найденный в теге img.
	AssetTypeImage = "image"

	// AssetTypeScript обозначает JavaScript-ассет, найденный в теге script.
	AssetTypeScript = "script"

	// AssetTypeStyle обозначает CSS-ассет, найденный в link rel="stylesheet".
	AssetTypeStyle = "style"

	// AssetTypeOther обозначает ассет другого типа.
	AssetTypeOther = "other"
)

// BrokenLink описывает ссылку на недоступный ресурс.
type BrokenLink struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	Error      string `json:"error"`
}

// Asset описывает внешний ресурс страницы: изображение, скрипт, стиль или другой ассет.
type Asset struct {
	URL        string `json:"url"`
	Type       string `json:"type"`
	StatusCode int    `json:"status_code"`
	SizeBytes  int64  `json:"size_bytes"`
	Error      string `json:"error,omitempty"`
}

// SEOReport содержит SEO-данные, извлеченные со страницы.
type SEOReport struct {
	HasTitle       bool   `json:"has_title"`
	Title          string `json:"title"`
	HasDescription bool   `json:"has_description"`
	Description    string `json:"description"`
	HasH1          bool   `json:"has_h1"`
}

// PageReport описывает результат анализа одной страницы.
type PageReport struct {
	URL          string       `json:"url"`
	Depth        int          `json:"depth"`
	HTTPStatus   int          `json:"http_status"`
	Status       string       `json:"status"`
	Error        string       `json:"error,omitempty"`
	SEO          SEOReport    `json:"seo"`
	BrokenLinks  []BrokenLink `json:"broken_links"`
	Assets       []Asset      `json:"assets"`
	DiscoveredAt string       `json:"discovered_at"`
}

// Report описывает итоговый отчет по результатам обхода сайта.
type Report struct {
	RootURL     string       `json:"root_url"`
	Depth       int          `json:"depth"`
	GeneratedAt string       `json:"generated_at"`
	Pages       []PageReport `json:"pages"`
}
