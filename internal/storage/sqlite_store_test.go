package storage

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStoreRoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "jobscout.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	jobs := []Job{
		{
			Company:         "Acme",
			CompanyWebsite:  "https://www.acme.example",
			CompanySummary:  "Acme builds deployment tooling.",
			CompanyIndustry: "Developer Tools",
			CompanyIdentity: &JobIdentityMetadata{
				Website: &JobIdentityEvidence{
					Value:      "https://www.acme.example",
					Source:     "company_profile",
					URL:        "https://jobs.example/acme/company",
					Confidence: "high",
				},
				Industry: &JobIdentityEvidence{
					Value:       "Developer Tools",
					Source:      "company_summary_inference",
					Confidence:  "low",
					Provisional: true,
				},
			},
			Title:        "Staff Engineer",
			Remote:       "Remote",
			Compensation: "$200,000",
			Source:       "test",
			ApplyURL:     "https://example.com/jobs/1",
			WhyMatches:   []string{"keyword"},
			Status:       "Unopened",
			DateAdded:    time.Now().Unix(),
			Description:  "desc",
		},
	}

	if err := store.SaveJobs(jobs); err != nil {
		t.Fatalf("SaveJobs() error = %v", err)
	}
	loadedJobs, err := store.LoadJobs()
	if err != nil {
		t.Fatalf("LoadJobs() error = %v", err)
	}
	if len(loadedJobs) != 1 {
		t.Fatalf("LoadJobs() len = %d; want 1", len(loadedJobs))
	}
	if loadedJobs[0].Company != "Acme" || loadedJobs[0].Title != "Staff Engineer" {
		t.Fatalf("LoadJobs()[0] = %#v; want Acme/Staff Engineer", loadedJobs[0])
	}
	if loadedJobs[0].CompanyWebsite != "https://www.acme.example" {
		t.Fatalf("LoadJobs()[0].CompanyWebsite = %q; want company website", loadedJobs[0].CompanyWebsite)
	}
	if loadedJobs[0].CompanySummary != "Acme builds deployment tooling." {
		t.Fatalf("LoadJobs()[0].CompanySummary = %q; want company summary", loadedJobs[0].CompanySummary)
	}
	if loadedJobs[0].CompanyIndustry != "Developer Tools" {
		t.Fatalf("LoadJobs()[0].CompanyIndustry = %q; want company industry", loadedJobs[0].CompanyIndustry)
	}
	if loadedJobs[0].CompanyIdentity == nil || loadedJobs[0].CompanyIdentity.Industry == nil || !loadedJobs[0].CompanyIdentity.Industry.Provisional {
		t.Fatalf("LoadJobs()[0].CompanyIdentity = %#v; want provisional industry evidence", loadedJobs[0].CompanyIdentity)
	}

	cache := HealthCache{
		"Acme": {
			Result: &CompanyHealthResult{
				Company: "Acme",
				Score:   77,
			},
			Timestamp: time.Now().Add(-2 * time.Hour),
		},
	}

	if err := store.SaveHealthCache(cache); err != nil {
		t.Fatalf("SaveHealthCache() error = %v", err)
	}
	loadedCache, err := store.LoadHealthCache()
	if err != nil {
		t.Fatalf("LoadHealthCache() error = %v", err)
	}
	entry, ok := loadedCache["Acme"]
	if !ok {
		t.Fatalf("LoadHealthCache() missing Acme key: %#v", loadedCache)
	}
	if entry.Result == nil || entry.Result.Score != 77 {
		t.Fatalf("LoadHealthCache()[Acme] = %#v; want score 77", entry)
	}
}

func TestSQLiteStoreSkipsStaleHealthCacheSourceVersion(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "jobscout.db")

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	_, err = store.db.Exec(`
		INSERT INTO health_cache (
			company_key,
			company_display,
			payload_json,
			score,
			fetched_at_epoch,
			source_version
		) VALUES (?, ?, ?, ?, ?, ?)
	`, "domain:openai.com", "domain:openai.com", `{"company":"OpenAI","score":57}`, 57, time.Now().Unix(), "v1")
	if err != nil {
		t.Fatalf("insert stale health cache row: %v", err)
	}

	loadedCache, err := store.LoadHealthCache()
	if err != nil {
		t.Fatalf("LoadHealthCache() error = %v", err)
	}
	if _, ok := loadedCache["domain:openai.com"]; ok {
		t.Fatalf("LoadHealthCache() loaded stale source_version row: %#v", loadedCache)
	}

	result, fetchedAt, err := store.GetHealth("domain:openai.com")
	if err != nil {
		t.Fatalf("GetHealth() error = %v", err)
	}
	if result != nil || !fetchedAt.IsZero() {
		t.Fatalf("GetHealth() = (%#v, %v), want stale row skipped", result, fetchedAt)
	}
}

func TestSQLiteStoreDeleteHealth(t *testing.T) {
	store := newTestSQLiteStore(t)
	fetchedAt := time.Now().Add(-time.Hour)

	if err := store.SetHealth("Acme", &CompanyHealthResult{
		Company: "Acme",
		Score:   72,
	}, fetchedAt); err != nil {
		t.Fatalf("SetHealth(%q) error = %v", "Acme", err)
	}
	if err := store.SetHealth("domain:acme.example", &CompanyHealthResult{
		Company: "Acme",
		Score:   88,
	}, fetchedAt); err != nil {
		t.Fatalf("SetHealth(%q) error = %v", "domain:acme.example", err)
	}

	if err := store.DeleteHealth("Acme"); err != nil {
		t.Fatalf("DeleteHealth(%q) error = %v", "Acme", err)
	}
	assertNoSQLiteHealth(t, store, "Acme")
	assertSQLiteHealthScore(t, store, "domain:acme.example", 88)

	if err := store.DeleteHealth("domain:acme.example"); err != nil {
		t.Fatalf("DeleteHealth(%q) error = %v", "domain:acme.example", err)
	}
	assertNoSQLiteHealth(t, store, "domain:acme.example")

	loadedCache, err := store.LoadHealthCache()
	if err != nil {
		t.Fatalf("LoadHealthCache() error = %v", err)
	}
	if len(loadedCache) != 0 {
		t.Fatalf("LoadHealthCache() = %#v, want empty after DeleteHealth calls", loadedCache)
	}
}

func assertSQLiteHealthScore(t *testing.T, store *SQLiteStore, company string, want int) {
	t.Helper()

	result, fetchedAt, err := store.GetHealth(company)
	if err != nil {
		t.Fatalf("GetHealth(%q) error = %v", company, err)
	}
	if result == nil || result.Score != want {
		t.Fatalf("GetHealth(%q) = (%#v, %v), want score %d", company, result, fetchedAt, want)
	}
}

func assertNoSQLiteHealth(t *testing.T, store *SQLiteStore, company string) {
	t.Helper()

	result, fetchedAt, err := store.GetHealth(company)
	if err != nil {
		t.Fatalf("GetHealth(%q) error = %v", company, err)
	}
	if result != nil || !fetchedAt.IsZero() {
		t.Fatalf("GetHealth(%q) = (%#v, %v), want no health row", company, result, fetchedAt)
	}

	loadedCache, err := store.LoadHealthCache()
	if err != nil {
		t.Fatalf("LoadHealthCache() error = %v", err)
	}
	if _, ok := loadedCache[company]; ok {
		t.Fatalf("LoadHealthCache()[%q] exists after DeleteHealth: %#v", company, loadedCache)
	}
}
