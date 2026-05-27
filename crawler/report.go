package crawler

const (
	PageStatusOK    = "ok"
	PageStatusError = "error"
)

// Пример короткого отчета до появления SEO-метрик — он уже позволяет проверить,
// что HTTP-запросы к сайту выполняются успешно:

// {
//   "root_url": "https://example.com",
//   "depth": 1,
//   "generated_at": "2024-05-18T12:34:56Z",
//   "pages": [
//     {
//       "url": "https://example.com",
//       "depth": 0,
//       "http_status": 200,
//       "status": "ok",
//       "error": ""
//     }
//   ]
// }

type Report struct {
	RootURL     string       `json:"root_url"`
	Depth       int          `json:"depth"`
	GeneratedAt string       `json:"generated_at"`
	Pages       []PageReport `json:"pages"`
}

type PageReport struct {
	URL        string `json:"url"`
	Depth      int    `json:"depth"`
	HTTPStatus int    `json:"http_status"`
	Status     string `json:"status"`
	Error      string `json:"error"`
}
