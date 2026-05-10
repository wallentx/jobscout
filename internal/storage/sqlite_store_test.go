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
