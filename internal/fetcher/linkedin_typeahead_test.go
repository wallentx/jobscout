package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestRefreshLinkedInCriteriaHintsResolvesGeoID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("typeaheadType"); got != "GEO" {
			t.Fatalf("typeaheadType = %q; want GEO", got)
		}
		if got := r.URL.Query().Get("query"); got != "Austin, TX" {
			t.Fatalf("query = %q; want Austin, TX", got)
		}
		_, _ = w.Write([]byte(`[
			{"id":"90000064","type":"GEO","displayName":"Austin, Texas Metropolitan Area"},
			{"id":"111","type":"GEO","displayName":"Other Place"},
			{"id":"104472865","type":"GEO","displayName":"Austin, Texas, United States"}
		]`))
	}))
	defer server.Close()
	restore := replaceLinkedInTypeaheadURLForTest(server.URL)
	defer restore()
	restoreCache := replaceLinkedInTypeaheadCacheForTest(t)
	defer restoreCache()

	criteria := &CriteriaConfig{}
	criteria.Candidate.City = "Austin"
	criteria.Candidate.State = "TX"

	if err := refreshLinkedInCriteriaHints(context.Background(), criteria); err != nil {
		t.Fatalf("refreshLinkedInCriteriaHints() error = %v", err)
	}
	if got := linkedInCachedGeoID("Austin, TX"); got != "104472865" {
		t.Fatalf("linkedInCachedGeoID(%q) = %q; want 104472865", "Austin, TX", got)
	}
}

func TestRefreshLinkedInCriteriaHintsUsesCachedGeoID(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	restore := replaceLinkedInTypeaheadURLForTest(server.URL)
	defer restore()
	restoreCache := replaceLinkedInTypeaheadCacheForTest(t)
	defer restoreCache()
	if err := saveLinkedInTypeaheadCacheEntry("geo", "Austin, TX", linkedInTypeaheadHit{
		Type:        "GEO",
		ID:          "104472865",
		DisplayName: "Austin, Texas, United States",
	}); err != nil {
		t.Fatalf("saveLinkedInTypeaheadCacheEntry() error = %v", err)
	}

	criteria := &CriteriaConfig{}
	criteria.Candidate.City = "Austin"
	criteria.Candidate.State = "TX"

	if err := refreshLinkedInCriteriaHints(context.Background(), criteria); err != nil {
		t.Fatalf("refreshLinkedInCriteriaHints() error = %v", err)
	}
	if requests != 0 {
		t.Fatalf("LinkedIn typeahead requests = %d; want 0 for fresh cache", requests)
	}
}

func TestRefreshLinkedInCriteriaHintsResolvesTitleSuggestions(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Query().Get("typeaheadType")+":"+r.URL.Query().Get("query"))
		switch r.URL.Query().Get("typeaheadType") {
		case "GEO":
			_, _ = w.Write([]byte(`[
				{"id":"104472865","type":"GEO","displayName":"Austin, Texas, United States"}
			]`))
		case "JOB_TITLE":
			_, _ = w.Write([]byte(`[
				{"id":"602","type":"SKILL","displayName":"Software Development"},
				{"id":"39","type":"TITLE","displayName":"Senior Software Engineer"}
			]`))
		default:
			t.Fatalf("typeaheadType = %q; want GEO or JOB_TITLE", r.URL.Query().Get("typeaheadType"))
		}
	}))
	defer server.Close()
	restore := replaceLinkedInTypeaheadURLForTest(server.URL)
	defer restore()
	restoreCache := replaceLinkedInTypeaheadCacheForTest(t)
	defer restoreCache()

	criteria := &CriteriaConfig{}
	criteria.Candidate.City = "Austin"
	criteria.Candidate.State = "TX"
	criteria.Filters.TitleRequires = []string{"Sr."}
	criteria.Filters.TitleIncludes = []string{"Software Eng"}

	if err := refreshLinkedInCriteriaHints(context.Background(), criteria); err != nil {
		t.Fatalf("refreshLinkedInCriteriaHints() error = %v", err)
	}
	if got := linkedInCachedTitle("Senior Software Engineer"); got != "Senior Software Engineer" {
		t.Fatalf("linkedInCachedTitle() = %q; want Senior Software Engineer", got)
	}
	if len(requests) != 2 {
		t.Fatalf("LinkedIn typeahead requests = %#v; want geo and title", requests)
	}
}

func TestRefreshLinkedInCriteriaHintsResolvesTitleSuggestionsWithoutLocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("typeaheadType"); got != "JOB_TITLE" {
			t.Fatalf("typeaheadType = %q; want JOB_TITLE", got)
		}
		if got := r.URL.Query().Get("query"); got != "Senior Frontend Engineer" {
			t.Fatalf("query = %q; want Senior Frontend Engineer", got)
		}
		_, _ = w.Write([]byte(`[
			{"id":"1578","type":"SKILL","displayName":"Front-End Development"},
			{"id":"17265","type":"TITLE","displayName":"Senior Frontend Developer"}
		]`))
	}))
	defer server.Close()
	restore := replaceLinkedInTypeaheadURLForTest(server.URL)
	defer restore()
	restoreCache := replaceLinkedInTypeaheadCacheForTest(t)
	defer restoreCache()

	criteria := &CriteriaConfig{}
	criteria.Filters.TitleRequires = []string{"Senior"}
	criteria.Filters.TitleIncludes = []string{"Frontend"}
	criteria.RoleFamilies = []RoleFamilyID{RoleFrontendEngineering}

	if err := refreshLinkedInCriteriaHints(context.Background(), criteria); err != nil {
		t.Fatalf("refreshLinkedInCriteriaHints() error = %v", err)
	}
	if got := linkedInCachedTitle("Senior Frontend Engineer"); got != "Senior Frontend Developer" {
		t.Fatalf("linkedInCachedTitle() = %q; want Senior Frontend Developer", got)
	}
}

func TestRefreshLinkedInCriteriaHintsReturnsErrorForFailedLookup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusTooManyRequests)
	}))
	defer server.Close()
	restore := replaceLinkedInTypeaheadURLForTest(server.URL)
	defer restore()
	restoreCache := replaceLinkedInTypeaheadCacheForTest(t)
	defer restoreCache()

	criteria := &CriteriaConfig{}
	criteria.Candidate.City = "Austin"
	criteria.Candidate.State = "TX"

	err := refreshLinkedInCriteriaHints(context.Background(), criteria)
	if err == nil {
		t.Fatal("refreshLinkedInCriteriaHints() error = nil; want error")
	}
	if !strings.Contains(err.Error(), "HTTP 429") {
		t.Fatalf("refreshLinkedInCriteriaHints() error = %q; want HTTP 429", err)
	}
}

func replaceLinkedInTypeaheadURLForTest(value string) func() {
	previous := linkedInTypeaheadURL
	linkedInTypeaheadURL = value
	return func() {
		linkedInTypeaheadURL = previous
	}
}

func replaceLinkedInTypeaheadCacheForTest(t *testing.T) func() {
	t.Helper()
	previous := runtimeLinkedInTypeaheadCachePath
	runtimeLinkedInTypeaheadCachePath = filepath.Join(t.TempDir(), "linkedin_cache.json")
	return func() {
		runtimeLinkedInTypeaheadCachePath = previous
	}
}
