package storage

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/wallentx/jobscout/internal/domain"
)

func TestSQLiteStoreMigratesCompanyIdentitySchema(t *testing.T) {
	store := newTestSQLiteStore(t)

	for _, table := range []string{"company_identities", "company_identity_keys"} {
		var name string
		if err := store.db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name); err != nil {
			t.Fatalf("missing table %s: %v", table, err)
		}
	}

	var count int
	if err := store.db.QueryRow(`SELECT COUNT(1) FROM schema_migrations WHERE version = 5`).Scan(&count); err != nil {
		t.Fatalf("query migration version 5: %v", err)
	}
	if count != 1 {
		t.Fatalf("migration version 5 count = %d; want 1", count)
	}
}

func TestSQLiteStoreUpsertAndGetCompanyIdentityByName(t *testing.T) {
	store := newTestSQLiteStore(t)
	now := time.Unix(1710000000, 0)

	identity := domain.CompanyIdentity{
		Key:              "acme",
		DisplayName:      "Acme",
		Website:          "https://www.acme.example",
		Summary:          "Acme builds deployment tooling.",
		Industry:         "Developer Tools",
		IdentityEvidence: json.RawMessage(`{"website":{"source":"company_profile"}}`),
		SourceVersion:    "identity-v1",
		CreatedAt:        now,
		UpdatedAt:        now.Add(time.Hour),
		NameAliases:      []string{"Acme Inc."},
	}
	if err := store.UpsertCompanyIdentity(context.Background(), identity); err != nil {
		t.Fatalf("UpsertCompanyIdentity() error = %v", err)
	}

	got, err := store.GetCompanyIdentity(context.Background(), "acme inc.", "")
	if err != nil {
		t.Fatalf("GetCompanyIdentity() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetCompanyIdentity() = nil; want identity")
	}
	if got.Key != "acme" || got.DisplayName != "Acme" || got.Website != "https://www.acme.example" {
		t.Fatalf("GetCompanyIdentity() = %#v; want Acme identity", got)
	}
	if !got.CreatedAt.Equal(now) || !got.UpdatedAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("timestamps = (%v, %v); want (%v, %v)", got.CreatedAt, got.UpdatedAt, now, now.Add(time.Hour))
	}
}

func TestSQLiteStoreGetsCompanyIdentityByDomainAlias(t *testing.T) {
	store := newTestSQLiteStore(t)

	if err := store.UpsertCompanyIdentity(context.Background(), domain.CompanyIdentity{
		DisplayName:   "OpenAI",
		Website:       "https://openai.com/careers",
		Summary:       "OpenAI builds AI systems.",
		Industry:      "Artificial Intelligence",
		SourceVersion: "identity-v1",
		DomainAliases: []string{"www.openai.com", "openai.com"},
	}); err != nil {
		t.Fatalf("UpsertCompanyIdentity() error = %v", err)
	}

	got, err := store.GetCompanyIdentity(context.Background(), "", "https://www.openai.com/about")
	if err != nil {
		t.Fatalf("GetCompanyIdentity() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetCompanyIdentity() = nil; want identity")
	}
	if got.DisplayName != "OpenAI" {
		t.Fatalf("DisplayName = %q; want OpenAI", got.DisplayName)
	}
}

func TestSQLiteStoreCompanyIdentityEvidenceRoundTrip(t *testing.T) {
	store := newTestSQLiteStore(t)
	evidence := json.RawMessage(`{"website":{"value":"https://stripe.com","confidence":"high"},"industry":{"value":"Payments","provisional":true}}`)

	if err := store.UpsertCompanyIdentity(context.Background(), domain.CompanyIdentity{
		DisplayName:      "Stripe",
		Website:          "https://stripe.com",
		Summary:          "Stripe builds financial infrastructure.",
		Industry:         "Payments",
		IdentityEvidence: evidence,
		SourceVersion:    "identity-v1",
	}); err != nil {
		t.Fatalf("UpsertCompanyIdentity() error = %v", err)
	}

	got, err := store.GetCompanyIdentity(context.Background(), "Stripe", "")
	if err != nil {
		t.Fatalf("GetCompanyIdentity() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetCompanyIdentity() = nil; want identity")
	}
	if string(got.IdentityEvidence) != string(evidence) {
		t.Fatalf("IdentityEvidence = %s; want %s", got.IdentityEvidence, evidence)
	}
}

func newTestSQLiteStore(t *testing.T) *SQLiteStore {
	t.Helper()

	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "jobscout.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}
