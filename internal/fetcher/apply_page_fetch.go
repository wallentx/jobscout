package fetcher

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func fetchApplyPageHTML(ctx context.Context, applyURL string) (string, error) {
	rawHTML, _, err := fetchApplyPage(ctx, applyURL)
	return rawHTML, err
}

func fetchApplyPage(ctx context.Context, applyURL string) (string, string, error) {
	if registry := currentURLVisitRun(); registry != nil {
		return registry.FetchApplyPage(ctx, applyURL)
	}
	return fetchApplyPageDirect(ctx, applyURL)
}

func fetchApplyPageDirect(ctx context.Context, applyURL string) (string, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(applyURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", "", fmt.Errorf("invalid apply URL")
	}

	// Indeed heavily blocks non-browser static scraping. Skip it to save time
	// and fall straight through to browser search or other fallbacks.
	if isIndeedHost(parsed.Hostname()) {
		return "", "", fmt.Errorf("HTTP 403: Skipping static fetch for Indeed URL to avoid scraper block")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", "", err
	}
	setBrowserLikeRequestHeaders(req)
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "", "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", "", err
	}
	finalURL := parsed.String()
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return string(data), finalURL, nil
}

func probeURL(ctx context.Context, client *http.Client, method string, applyURL string) (int, error) {
	if registry := currentURLVisitRun(); registry != nil {
		return registry.ProbeURL(ctx, client, method, applyURL)
	}
	return probeURLDirect(ctx, client, method, applyURL)
}

func probeURLDirect(ctx context.Context, client *http.Client, method string, applyURL string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, applyURL, nil)
	if err != nil {
		return 0, err
	}
	setBrowserLikeRequestHeaders(req)

	if method == http.MethodGet {
		req.Header.Set("Range", "bytes=0-0")
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	return resp.StatusCode, nil
}

func setBrowserLikeRequestHeaders(req *http.Request) {
	if req == nil {
		return
	}
	req.Header.Set("User-Agent", browserLikeUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
}

func resolveURL(baseURL string, href string) string {
	href = strings.TrimSpace(html.UnescapeString(href))
	if href == "" {
		return ""
	}
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	return base.ResolveReference(ref).String()
}
