package crawler

const (
	PageStatusOK    = "ok"
	PageStatusError = "error"
)

// Step 1: Пример короткого отчета до появления SEO-метрик — он уже позволяет проверить,
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
	URL          string       `json:"url"`
	Depth        int          `json:"depth"`
	HTTPStatus   int          `json:"http_status"`
	Status       string       `json:"status"`
	Error        string       `json:"error"`
	BrokenLinks  []BrokenLink `json:"broken_links"`
	DiscoveredAt string       `json:"discovered_at"` // ???: в примере step 3 есть поле "discovered_at", но другой инфы про него пока нет
}

// Step 3: Пример битых ссылок:

// {
//   "root_url": "http://simple.test",
//   "depth": 2,
//   "generated_at": "2025-11-26T14:10:00Z",
//   "pages": [
//     {
//       "url": "http://simple.test/blog/index.html",
//       "depth": 1,
//       "http_status": 200,
//       "status": "ok",
//       "broken_links": [
//         {
//           "url": "http://simple.test/assets/ghost.css",
//           "status_code": 404
//         },
//         {
//           "url": "https://cdn.simple.test/app.js",
//           "error": "Get \"https://cdn.simple.test/app.js\": dial tcp: lookup cdn.simple.test: no such host"
//         }
//       ],
//       "discovered_at": "2025-11-26T14:10:01Z"
//     }
//   ]
// }

type BrokenLink struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code,omitempty"`
	Error      string `json:"error,omitempty"`
}
