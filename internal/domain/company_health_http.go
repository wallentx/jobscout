package domain

import "github.com/wallentx/jobscout/internal/netutil"

func doHTTPGet(urlStr string, headers map[string]string) ([]byte, error) {
	return netutil.HTTPGet(urlStr, requestTimeout, companyHealthUserAgent(), headers)
}

// httpGet performs an HTTP GET request with timeout and user agent.
var httpGet = defaultHTTPGet

func defaultHTTPGet(urlStr string) ([]byte, error) {
	return doHTTPGet(urlStr, map[string]string{"Accept": "application/json,text/html,*/*"})
}
