package fetcher

import (
	"time"

	"github.com/wallentx/jobscout/internal/netutil"
)

const requestTimeout = 20 * time.Second

var doHTTPGet = defaultDoHTTPGet
var httpGet = defaultHTTPGet

func defaultDoHTTPGet(urlStr string, headers map[string]string) ([]byte, error) {
	return netutil.HTTPGet(urlStr, requestTimeout, jobscoutUserAgent, headers)
}

func defaultHTTPGet(urlStr string) ([]byte, error) {
	return doHTTPGet(urlStr, map[string]string{"Accept": "application/json,text/html,*/*"})
}
