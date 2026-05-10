package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestURLVisitRegistryReusesStaticHTMLFetches(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>Acme builds deployment tools.</body></html>`))
	}))
	defer server.Close()

	registry := beginURLVisitRun()
	t.Cleanup(func() { setURLVisitRun(nil) })

	firstHTML, firstFinalURL, firstErr := registry.FetchApplyPage(context.Background(), server.URL+"/jobs/123?utm_source=test")
	secondHTML, secondFinalURL, secondErr := registry.FetchApplyPage(context.Background(), server.URL+"/jobs/123")

	if firstErr != nil || secondErr != nil {
		t.Fatalf("FetchApplyPage errors = %v, %v; want nil", firstErr, secondErr)
	}
	if firstHTML == "" || secondHTML != firstHTML {
		t.Fatalf("cached HTML mismatch: first=%q second=%q", firstHTML, secondHTML)
	}
	if firstFinalURL == "" || secondFinalURL == "" {
		t.Fatalf("final URLs = %q, %q; want populated", firstFinalURL, secondFinalURL)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("server requests = %d; want 1", got)
	}
}

func TestURLVisitRegistryReusesProbeResults(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	registry := beginURLVisitRun()
	t.Cleanup(func() { setURLVisitRun(nil) })

	client := &http.Client{Timeout: time.Second}
	firstStatus, firstErr := registry.ProbeURL(context.Background(), client, http.MethodHead, server.URL+"/jobs/123?utm_source=test")
	secondStatus, secondErr := registry.ProbeURL(context.Background(), client, http.MethodHead, server.URL+"/jobs/123")

	if firstErr != nil || secondErr != nil {
		t.Fatalf("ProbeURL errors = %v, %v; want nil", firstErr, secondErr)
	}
	if firstStatus != http.StatusOK || secondStatus != http.StatusOK {
		t.Fatalf("statuses = %d, %d; want 200", firstStatus, secondStatus)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("server requests = %d; want 1", got)
	}
}
