package fetcher

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

const requestTimeout = 20 * time.Second

var doHTTPGet = defaultDoHTTPGet
var httpGet = defaultHTTPGet

func defaultDoHTTPGet(urlStr string, headers map[string]string) ([]byte, error) {
	client := &http.Client{Timeout: requestTimeout}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", jobscoutUserAgent)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func defaultHTTPGet(urlStr string) ([]byte, error) {
	return doHTTPGet(urlStr, map[string]string{"Accept": "application/json,text/html,*/*"})
}
