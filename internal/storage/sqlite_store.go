package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/wallentx/jobscout/internal/domain"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

const healthCacheSourceVersion = "v3"

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.configure(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.ensureMigrations(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) configure() error {
	stmts := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA journal_mode = WAL;",
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("sqlite configure failed: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) ensureMigrations() error {
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at_epoch INTEGER NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	migrations := []struct {
		version int
		sql     string
	}{
		{
			version: 1,
			sql: `
				CREATE TABLE IF NOT EXISTS jobs (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					company TEXT NOT NULL,
					title TEXT NOT NULL,
					remote TEXT NOT NULL,
					compensation TEXT NOT NULL,
					source TEXT NOT NULL,
					apply_url TEXT NOT NULL,
					why_matches_json TEXT NOT NULL DEFAULT '[]',
					status TEXT NOT NULL,
					date_added INTEGER NOT NULL DEFAULT 0,
					description TEXT NOT NULL DEFAULT '',
					UNIQUE(company, title)
				);

				CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
				CREATE INDEX IF NOT EXISTS idx_jobs_date_added ON jobs(date_added DESC);
				CREATE INDEX IF NOT EXISTS idx_jobs_company ON jobs(company);
				CREATE INDEX IF NOT EXISTS idx_jobs_title ON jobs(title);

				CREATE TABLE IF NOT EXISTS health_cache (
					company_key TEXT PRIMARY KEY,
					company_display TEXT NOT NULL,
					payload_json TEXT NOT NULL,
					score INTEGER NOT NULL DEFAULT 0,
					fetched_at_epoch INTEGER NOT NULL,
					source_version TEXT NOT NULL DEFAULT 'v1'
				);

				CREATE INDEX IF NOT EXISTS idx_health_cache_fetched_at ON health_cache(fetched_at_epoch DESC);
			`,
		},
		{
			version: 2,
			sql: `
				ALTER TABLE jobs ADD COLUMN company_website TEXT NOT NULL DEFAULT '';
				ALTER TABLE jobs ADD COLUMN company_summary TEXT NOT NULL DEFAULT '';
			`,
		},
		{
			version: 3,
			sql: `
				ALTER TABLE jobs ADD COLUMN company_industry TEXT NOT NULL DEFAULT '';
			`,
		},
		{
			version: 4,
			sql: `
				ALTER TABLE jobs ADD COLUMN company_identity_json TEXT NOT NULL DEFAULT '';
			`,
		},
		{
			version: 5,
			sql: `
				CREATE TABLE IF NOT EXISTS company_identities (
					company_key TEXT PRIMARY KEY,
					display_name TEXT NOT NULL,
					website TEXT NOT NULL DEFAULT '',
					summary TEXT NOT NULL DEFAULT '',
					industry TEXT NOT NULL DEFAULT '',
					identity_evidence_json TEXT NOT NULL DEFAULT '',
					source_version TEXT NOT NULL DEFAULT '',
					created_at_epoch INTEGER NOT NULL,
					updated_at_epoch INTEGER NOT NULL
				);

				CREATE TABLE IF NOT EXISTS company_identity_keys (
					lookup_key TEXT PRIMARY KEY,
					company_key TEXT NOT NULL,
					key_type TEXT NOT NULL,
					key_value TEXT NOT NULL,
					FOREIGN KEY(company_key) REFERENCES company_identities(company_key) ON DELETE CASCADE
				);

				CREATE INDEX IF NOT EXISTS idx_company_identity_keys_company_key ON company_identity_keys(company_key);
				CREATE INDEX IF NOT EXISTS idx_company_identity_keys_type_value ON company_identity_keys(key_type, key_value);
			`,
		},
	}

	for _, migration := range migrations {
		applied, err := s.hasMigration(migration.version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", migration.version, err)
		}

		if _, err := tx.Exec(migration.sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", migration.version, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at_epoch) VALUES(?, ?)`, migration.version, time.Now().Unix()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", migration.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", migration.version, err)
		}
	}

	return nil
}

func (s *SQLiteStore) hasMigration(version int) (bool, error) {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM schema_migrations WHERE version = ?`, version).Scan(&count); err != nil {
		return false, fmt.Errorf("check migration %d: %w", version, err)
	}
	return count > 0, nil
}

func (s *SQLiteStore) LoadJobs() ([]Job, error) {
	rows, err := s.db.Query(`
		SELECT
			company,
			company_website,
			company_summary,
			company_industry,
			company_identity_json,
			title,
			remote,
			compensation,
			source,
			apply_url,
			why_matches_json,
			status,
			date_added,
			description
		FROM jobs
		ORDER BY date_added DESC, id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("load jobs: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var jobs []Job
	for rows.Next() {
		var job Job
		var whyMatchesJSON string
		var companyIdentityJSON string
		if err := rows.Scan(
			&job.Company,
			&job.CompanyWebsite,
			&job.CompanySummary,
			&job.CompanyIndustry,
			&companyIdentityJSON,
			&job.Title,
			&job.Remote,
			&job.Compensation,
			&job.Source,
			&job.ApplyURL,
			&whyMatchesJSON,
			&job.Status,
			&job.DateAdded,
			&job.Description,
		); err != nil {
			return nil, fmt.Errorf("scan job row: %w", err)
		}
		if err := json.Unmarshal([]byte(whyMatchesJSON), &job.WhyMatches); err != nil {
			job.WhyMatches = nil
		}
		if strings.TrimSpace(companyIdentityJSON) != "" {
			if err := json.Unmarshal([]byte(companyIdentityJSON), &job.CompanyIdentity); err != nil {
				job.CompanyIdentity = nil
			}
		}
		job.DateDiscovered = formatUnixDate(job.DateAdded)
		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs: %w", err)
	}

	return jobs, nil
}

func (s *SQLiteStore) SaveJobs(jobs []Job) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin save jobs: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM jobs`); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("clear jobs: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO jobs (
			company,
			company_website,
			company_summary,
			company_industry,
			company_identity_json,
			title,
			remote,
			compensation,
			source,
			apply_url,
			why_matches_json,
			status,
			date_added,
			description
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare save jobs: %w", err)
	}
	defer func() {
		_ = stmt.Close()
	}()

	for _, job := range jobs {
		whyMatchesJSON, err := json.Marshal(job.WhyMatches)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("marshal why_matches for %s - %s: %w", job.Company, job.Title, err)
		}
		companyIdentityJSON := ""
		if job.CompanyIdentity != nil {
			identityBytes, err := json.Marshal(job.CompanyIdentity)
			if err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("marshal company_identity for %s - %s: %w", job.Company, job.Title, err)
			}
			companyIdentityJSON = string(identityBytes)
		}

		dateAdded := job.DateAdded
		if dateAdded <= 0 {
			dateAdded = unixFromDateString(job.DateDiscovered)
		}

		if _, err := stmt.Exec(
			job.Company,
			job.CompanyWebsite,
			job.CompanySummary,
			job.CompanyIndustry,
			companyIdentityJSON,
			job.Title,
			job.Remote,
			job.Compensation,
			job.Source,
			job.ApplyURL,
			string(whyMatchesJSON),
			job.Status,
			dateAdded,
			job.Description,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert job %s - %s: %w", job.Company, job.Title, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit save jobs: %w", err)
	}
	return nil
}

func (s *SQLiteStore) LoadHealthCache() (HealthCache, error) {
	rows, err := s.db.Query(`
		SELECT company_key, company_display, payload_json, fetched_at_epoch, source_version
		FROM health_cache
	`)
	if err != nil {
		return nil, fmt.Errorf("load health cache: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	cache := make(HealthCache)
	for rows.Next() {
		var key string
		var display string
		var payloadJSON string
		var fetchedAtEpoch int64
		var sourceVersion string

		if err := rows.Scan(&key, &display, &payloadJSON, &fetchedAtEpoch, &sourceVersion); err != nil {
			return nil, fmt.Errorf("scan health cache row: %w", err)
		}
		if sourceVersion != healthCacheSourceVersion {
			continue
		}

		var result CompanyHealthResult
		if err := json.Unmarshal([]byte(payloadJSON), &result); err != nil {
			continue
		}

		cacheKey := strings.TrimSpace(key)
		if cacheKey == "" {
			cacheKey = strings.TrimSpace(display)
		}
		entry := HealthCacheEntry{
			Result:    &result,
			Timestamp: time.Unix(fetchedAtEpoch, 0),
		}
		cache[cacheKey] = entry
		if displayKey := strings.TrimSpace(display); displayKey != "" && displayKey != cacheKey {
			cache[displayKey] = entry
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate health cache: %w", err)
	}

	return cache, nil
}

func (s *SQLiteStore) SaveHealthCache(cache HealthCache) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin save health cache: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM health_cache`); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("clear health cache: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO health_cache (
			company_key,
			company_display,
			payload_json,
			score,
			fetched_at_epoch,
			source_version
		) VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare save health cache: %w", err)
	}
	defer func() {
		_ = stmt.Close()
	}()

	for company, entry := range cache {
		if entry.Result == nil {
			continue
		}

		payload, err := json.Marshal(entry.Result)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("marshal health payload for %s: %w", company, err)
		}

		fetchedAt := entry.Timestamp.Unix()
		if fetchedAt <= 0 {
			fetchedAt = time.Now().Unix()
		}

		if _, err := stmt.Exec(
			normalizeCompanyKey(company),
			company,
			string(payload),
			entry.Result.Score,
			fetchedAt,
			healthCacheSourceVersion,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert health cache row for %s: %w", company, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit save health cache: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetHealth(company string) (*CompanyHealthResult, time.Time, error) {
	row := s.db.QueryRow(`
		SELECT payload_json, fetched_at_epoch, source_version
		FROM health_cache
		WHERE company_key = ?
	`, normalizeCompanyKey(company))

	var payloadJSON string
	var fetchedAtEpoch int64
	var sourceVersion string
	if err := row.Scan(&payloadJSON, &fetchedAtEpoch, &sourceVersion); err != nil {
		if err == sql.ErrNoRows {
			return nil, time.Time{}, nil
		}
		return nil, time.Time{}, fmt.Errorf("get health cache row for %s: %w", company, err)
	}
	if sourceVersion != healthCacheSourceVersion {
		return nil, time.Time{}, nil
	}

	var result CompanyHealthResult
	if err := json.Unmarshal([]byte(payloadJSON), &result); err != nil {
		return nil, time.Time{}, fmt.Errorf("decode health cache payload for %s: %w", company, err)
	}

	return &result, time.Unix(fetchedAtEpoch, 0), nil
}

func (s *SQLiteStore) SetHealth(company string, result *CompanyHealthResult, fetchedAt time.Time) error {
	if result == nil {
		return nil
	}

	payload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal health payload for %s: %w", company, err)
	}

	epoch := fetchedAt.Unix()
	if epoch <= 0 {
		epoch = time.Now().Unix()
	}

	_, err = s.db.Exec(`
		INSERT INTO health_cache (
			company_key,
			company_display,
			payload_json,
			score,
			fetched_at_epoch,
			source_version
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(company_key) DO UPDATE SET
			company_display = excluded.company_display,
			payload_json = excluded.payload_json,
			score = excluded.score,
			fetched_at_epoch = excluded.fetched_at_epoch,
			source_version = excluded.source_version
	`, normalizeCompanyKey(company), company, string(payload), result.Score, epoch, healthCacheSourceVersion)
	if err != nil {
		return fmt.Errorf("upsert health cache row for %s: %w", company, err)
	}
	return nil
}

func (s *SQLiteStore) DeleteHealth(company string) error {
	key := normalizeCompanyKey(company)
	if key == "" {
		return nil
	}
	if _, err := s.db.Exec(`
		DELETE FROM health_cache
		WHERE company_key = ?
			OR lower(trim(company_display)) = ?
	`, key, key); err != nil {
		return fmt.Errorf("delete health cache row for %s: %w", company, err)
	}
	return nil
}

func (s *SQLiteStore) ClearHealthCache() error {
	if _, err := s.db.Exec(`DELETE FROM health_cache`); err != nil {
		return fmt.Errorf("clear health cache: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpsertCompanyIdentity(ctx context.Context, identity domain.CompanyIdentity) error {
	identity.DisplayName = strings.TrimSpace(identity.DisplayName)
	identity.Website = strings.TrimSpace(identity.Website)
	identity.Summary = strings.TrimSpace(identity.Summary)
	identity.Industry = strings.TrimSpace(identity.Industry)
	identity.SourceVersion = strings.TrimSpace(identity.SourceVersion)
	identity.Key = normalizeCompanyIdentityKey(identity.Key)
	if identity.Key == "" {
		identity.Key = companyIdentityKeyFor(identity.DisplayName, identity.Website)
	}
	if identity.Key == "" {
		return fmt.Errorf("company identity requires display name or website")
	}
	if identity.DisplayName == "" {
		identity.DisplayName = identity.Key
	}

	now := time.Now()
	if identity.CreatedAt.IsZero() {
		identity.CreatedAt = now
	}
	if identity.UpdatedAt.IsZero() {
		identity.UpdatedAt = now
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin upsert company identity: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO company_identities (
			company_key,
			display_name,
			website,
			summary,
			industry,
			identity_evidence_json,
			source_version,
			created_at_epoch,
			updated_at_epoch
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(company_key) DO UPDATE SET
			display_name = excluded.display_name,
			website = excluded.website,
			summary = excluded.summary,
			industry = excluded.industry,
			identity_evidence_json = excluded.identity_evidence_json,
			source_version = excluded.source_version,
			updated_at_epoch = excluded.updated_at_epoch
	`, identity.Key, identity.DisplayName, identity.Website, identity.Summary, identity.Industry, string(identity.IdentityEvidence), identity.SourceVersion, identity.CreatedAt.Unix(), identity.UpdatedAt.Unix()); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("upsert company identity %s: %w", identity.Key, err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM company_identity_keys WHERE company_key = ?`, identity.Key); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("clear company identity aliases for %s: %w", identity.Key, err)
	}

	aliases := companyIdentityLookupKeys(identity)
	for _, alias := range aliases {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO company_identity_keys (
				lookup_key,
				company_key,
				key_type,
				key_value
			) VALUES (?, ?, ?, ?)
			ON CONFLICT(lookup_key) DO UPDATE SET
				company_key = excluded.company_key,
				key_type = excluded.key_type,
				key_value = excluded.key_value
		`, alias.lookupKey, identity.Key, alias.keyType, alias.keyValue); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("upsert company identity alias %s: %w", alias.lookupKey, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit upsert company identity: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetCompanyIdentity(ctx context.Context, companyName string, websiteOrDomain string) (*domain.CompanyIdentity, error) {
	for _, lookupKey := range companyIdentityLookupKeysForQuery(companyName, websiteOrDomain) {
		identity, err := s.getCompanyIdentityByLookupKey(ctx, lookupKey)
		if err != nil {
			return nil, err
		}
		if identity != nil {
			return identity, nil
		}
	}
	return nil, nil
}

func (s *SQLiteStore) getCompanyIdentityByLookupKey(ctx context.Context, lookupKey string) (*domain.CompanyIdentity, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			i.company_key,
			i.display_name,
			i.website,
			i.summary,
			i.industry,
			i.identity_evidence_json,
			i.source_version,
			i.created_at_epoch,
			i.updated_at_epoch
		FROM company_identity_keys k
		JOIN company_identities i ON i.company_key = k.company_key
		WHERE k.lookup_key = ?
	`, lookupKey)

	identity, err := scanCompanyIdentity(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get company identity for %s: %w", lookupKey, err)
	}
	if err := s.loadCompanyIdentityAliases(ctx, identity); err != nil {
		return nil, err
	}
	return identity, nil
}

func scanCompanyIdentity(row *sql.Row) (*domain.CompanyIdentity, error) {
	var identity domain.CompanyIdentity
	var evidenceJSON string
	var createdAtEpoch int64
	var updatedAtEpoch int64
	if err := row.Scan(
		&identity.Key,
		&identity.DisplayName,
		&identity.Website,
		&identity.Summary,
		&identity.Industry,
		&evidenceJSON,
		&identity.SourceVersion,
		&createdAtEpoch,
		&updatedAtEpoch,
	); err != nil {
		return nil, err
	}
	if evidenceJSON != "" {
		identity.IdentityEvidence = json.RawMessage(evidenceJSON)
	}
	identity.CreatedAt = time.Unix(createdAtEpoch, 0)
	identity.UpdatedAt = time.Unix(updatedAtEpoch, 0)
	return &identity, nil
}

func (s *SQLiteStore) loadCompanyIdentityAliases(ctx context.Context, identity *domain.CompanyIdentity) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT key_type, key_value
		FROM company_identity_keys
		WHERE company_key = ?
		ORDER BY key_type, key_value
	`, identity.Key)
	if err != nil {
		return fmt.Errorf("load company identity aliases for %s: %w", identity.Key, err)
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var keyType string
		var keyValue string
		if err := rows.Scan(&keyType, &keyValue); err != nil {
			return fmt.Errorf("scan company identity alias for %s: %w", identity.Key, err)
		}
		switch keyType {
		case "name":
			identity.NameAliases = append(identity.NameAliases, keyValue)
		case "domain":
			identity.DomainAliases = append(identity.DomainAliases, keyValue)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate company identity aliases for %s: %w", identity.Key, err)
	}
	return nil
}

func normalizeCompanyKey(company string) string {
	return strings.ToLower(strings.TrimSpace(company))
}

type companyIdentityAlias struct {
	lookupKey string
	keyType   string
	keyValue  string
}

func companyIdentityLookupKeys(identity domain.CompanyIdentity) []companyIdentityAlias {
	var aliases []companyIdentityAlias
	seen := make(map[string]bool)
	addName := func(name string) {
		normalized := normalizeCompanyNameAlias(name)
		if normalized == "" {
			return
		}
		lookupKey := "name:" + normalized
		if seen[lookupKey] {
			return
		}
		seen[lookupKey] = true
		aliases = append(aliases, companyIdentityAlias{
			lookupKey: lookupKey,
			keyType:   "name",
			keyValue:  normalized,
		})
	}
	addDomain := func(domain string) {
		normalized := normalizeCompanyDomainAlias(domain)
		if normalized == "" {
			return
		}
		lookupKey := "domain:" + normalized
		if seen[lookupKey] {
			return
		}
		seen[lookupKey] = true
		aliases = append(aliases, companyIdentityAlias{
			lookupKey: lookupKey,
			keyType:   "domain",
			keyValue:  normalized,
		})
	}

	addName(identity.DisplayName)
	for _, alias := range identity.NameAliases {
		addName(alias)
	}
	addDomain(identity.Website)
	for _, alias := range identity.DomainAliases {
		addDomain(alias)
	}
	return aliases
}

func companyIdentityLookupKeysForQuery(companyName string, websiteOrDomain string) []string {
	var lookupKeys []string
	seen := make(map[string]bool)
	add := func(lookupKey string) {
		if lookupKey == "" || seen[lookupKey] {
			return
		}
		seen[lookupKey] = true
		lookupKeys = append(lookupKeys, lookupKey)
	}
	if domain := normalizeCompanyDomainAlias(websiteOrDomain); domain != "" {
		add("domain:" + domain)
	}
	if name := normalizeCompanyNameAlias(companyName); name != "" {
		add("name:" + name)
	}
	return lookupKeys
}

func companyIdentityKeyFor(companyName string, websiteOrDomain string) string {
	if domain := normalizeCompanyDomainAlias(websiteOrDomain); domain != "" {
		return "domain:" + domain
	}
	if name := normalizeCompanyNameAlias(companyName); name != "" {
		return "name:" + name
	}
	return ""
}

func normalizeCompanyIdentityKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func normalizeCompanyNameAlias(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), " "))
}

func normalizeCompanyDomainAlias(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := parsed.Hostname()
	host = strings.TrimPrefix(strings.ToLower(host), "www.")
	return strings.TrimSpace(host)
}
