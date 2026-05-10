package fetcher

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
)

var urlVisitRunMu sync.Mutex
var urlVisitRun *urlVisitRegistry

type urlVisitRegistry struct {
	mu      sync.Mutex
	html    map[string]*urlHTMLVisit
	probes  map[string]*urlProbeVisit
	fetches int
	hits    int
}

type urlHTMLVisit struct {
	done     chan struct{}
	rawHTML  string
	finalURL string
	err      error
}

type urlProbeVisit struct {
	done   chan struct{}
	status int
	err    error
}

func newURLVisitRegistry() *urlVisitRegistry {
	return &urlVisitRegistry{
		html:   make(map[string]*urlHTMLVisit),
		probes: make(map[string]*urlProbeVisit),
	}
}

func beginURLVisitRun() *urlVisitRegistry {
	registry := newURLVisitRegistry()
	setURLVisitRun(registry)
	return registry
}

func currentURLVisitRun() *urlVisitRegistry {
	urlVisitRunMu.Lock()
	defer urlVisitRunMu.Unlock()
	return urlVisitRun
}

func setURLVisitRun(registry *urlVisitRegistry) {
	urlVisitRunMu.Lock()
	urlVisitRun = registry
	urlVisitRunMu.Unlock()
}

func ensureURLVisitRun() (*urlVisitRegistry, bool) {
	if registry := currentURLVisitRun(); registry != nil {
		return registry, false
	}
	return beginURLVisitRun(), true
}

func clearURLVisitRun(registry *urlVisitRegistry) {
	urlVisitRunMu.Lock()
	if urlVisitRun == registry {
		urlVisitRun = nil
	}
	urlVisitRunMu.Unlock()
}

func (r *urlVisitRegistry) Stats() (int, int, int, int) {
	if r == nil {
		return 0, 0, 0, 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.fetches, r.hits, len(r.html), len(r.probes)
}

func (r *urlVisitRegistry) FetchApplyPage(ctx context.Context, applyURL string) (string, string, error) {
	if r == nil {
		return fetchApplyPageDirect(ctx, applyURL)
	}
	key := "html:" + canonicalURLVisitKey(applyURL)
	if key == "html:" {
		return fetchApplyPageDirect(ctx, applyURL)
	}

	r.mu.Lock()
	if entry, ok := r.html[key]; ok {
		r.hits++
		r.mu.Unlock()
		select {
		case <-entry.done:
			logDebug("url visit registry: hit kind=html url=%q err=%t", key, entry.err != nil)
			return entry.rawHTML, entry.finalURL, entry.err
		case <-ctx.Done():
			return "", "", ctx.Err()
		}
	}
	entry := &urlHTMLVisit{done: make(chan struct{})}
	r.html[key] = entry
	r.fetches++
	r.mu.Unlock()

	rawHTML, finalURL, err := fetchApplyPageDirect(ctx, applyURL)

	r.mu.Lock()
	entry.rawHTML = rawHTML
	entry.finalURL = finalURL
	entry.err = err
	if shouldForgetURLVisitError(err) {
		delete(r.html, key)
	}
	close(entry.done)
	r.mu.Unlock()
	if err == nil {
		logDebug("url visit registry: stored kind=html url=%q final_url=%q bytes=%d", key, finalURL, len(rawHTML))
	}
	return rawHTML, finalURL, err
}

func (r *urlVisitRegistry) ProbeURL(ctx context.Context, client *http.Client, method string, applyURL string) (int, error) {
	if r == nil {
		return probeURLDirect(ctx, client, method, applyURL)
	}
	key := "probe:" + strings.ToUpper(strings.TrimSpace(method)) + ":" + canonicalURLVisitKey(applyURL)
	if strings.HasSuffix(key, ":") {
		return probeURLDirect(ctx, client, method, applyURL)
	}

	r.mu.Lock()
	if entry, ok := r.probes[key]; ok {
		r.hits++
		r.mu.Unlock()
		select {
		case <-entry.done:
			logDebug("url visit registry: hit kind=probe url=%q status=%d err=%t", key, entry.status, entry.err != nil)
			return entry.status, entry.err
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
	entry := &urlProbeVisit{done: make(chan struct{})}
	r.probes[key] = entry
	r.fetches++
	r.mu.Unlock()

	status, err := probeURLDirect(ctx, client, method, applyURL)

	r.mu.Lock()
	entry.status = status
	entry.err = err
	if shouldForgetURLVisitError(err) {
		delete(r.probes, key)
	}
	close(entry.done)
	r.mu.Unlock()
	logDebug("url visit registry: stored kind=probe url=%q status=%d err=%t", key, status, err != nil)
	return status, err
}

func shouldForgetURLVisitError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func canonicalURLVisitKey(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return ""
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	query := parsed.Query()
	for key := range query {
		if isTrackingQueryParam(key) {
			query.Del(key)
		}
	}
	parsed.RawQuery = sortedQueryString(query)
	return strings.TrimRight(parsed.String(), "?")
}

func sortedQueryString(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		vals := append([]string(nil), values[key]...)
		sort.Strings(vals)
		for _, value := range vals {
			if builder.Len() > 0 {
				builder.WriteByte('&')
			}
			builder.WriteString(url.QueryEscape(key))
			builder.WriteByte('=')
			builder.WriteString(url.QueryEscape(value))
		}
	}
	return builder.String()
}

func isTrackingQueryParam(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	return strings.HasPrefix(key, "utm_") ||
		key == "fbclid" ||
		key == "gclid" ||
		key == "msclkid" ||
		key == "mc_cid" ||
		key == "mc_eid"
}
