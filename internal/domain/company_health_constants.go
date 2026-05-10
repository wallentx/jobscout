package domain

import (
	"time"
)

const (
	userAgent            = "jobscout (+https://github.com/wallentx/jobscout)"
	secTickersURL        = "https://www.sec.gov/files/company_tickers.json"
	secSubmissionsURL    = "https://data.sec.gov/submissions/CIK%s.json"
	wikiTitleSearchURL   = "https://en.wikipedia.org/w/rest.php/v1/search/title"
	wikiSummaryURL       = "https://en.wikipedia.org/api/rest_v1/page/summary/%s"
	wikidataEntityURL    = "https://www.wikidata.org/wiki/Special:EntityData/%s.json"
	googleNewsRSSURL     = "https://news.google.com/rss/search?q=%s&hl=en-US&gl=US&ceid=US:en"
	yahooFinanceChartURL = "https://query1.finance.yahoo.com/v8/finance/chart/%s?range=1y&interval=1d"
	hnSearchURL          = "https://hn.algolia.com/api/v1/search?query=%s&tags=story&hitsPerPage=15"
	secDocumentURL       = "https://www.sec.gov/Archives/edgar/data/%s/%s/%s"
	requestTimeout       = 20 * time.Second
)
