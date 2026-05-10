package fetcher

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/wallentx/jobscout/internal/domain"
)

type CompanyIdentityRecord struct {
	Website  string
	Summary  string
	Industry string
	Identity *domain.JobIdentityMetadata
}

type PersistentCompanyIdentityStore interface {
	GetCompanyIdentity(ctx context.Context, company string, website string) (*domain.CompanyIdentity, error)
	UpsertCompanyIdentity(ctx context.Context, identity domain.CompanyIdentity) error
}

type CompanyIdentityCache struct {
	mu      sync.RWMutex
	records map[string]CompanyIdentityRecord
}

func NewCompanyIdentityCache() *CompanyIdentityCache {
	return &CompanyIdentityCache{
		records: make(map[string]CompanyIdentityRecord),
	}
}

func (c *CompanyIdentityCache) Get(company string) (CompanyIdentityRecord, bool) {
	if strings.TrimSpace(company) == "" {
		return CompanyIdentityRecord{}, false
	}
	key := companyIdentityCacheKey(company)

	c.mu.RLock()
	defer c.mu.RUnlock()
	record, ok := c.records[key]
	return record, ok
}

func (c *CompanyIdentityCache) Set(company string, record CompanyIdentityRecord) {
	if strings.TrimSpace(company) == "" {
		return
	}
	key := companyIdentityCacheKey(company)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.records[key] = record
}

func (c *CompanyIdentityCache) IsIdentityComplete(job Job) bool {
	return !jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) &&
		!jobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company)
}

func companyIdentityCacheKey(company string) string {
	return Slugify(CleanCompanyName(company))
}

func CloneJobIdentityMetadata(m *domain.JobIdentityMetadata) *domain.JobIdentityMetadata {
	if m == nil {
		return nil
	}
	clone := &domain.JobIdentityMetadata{}
	if m.Website != nil {
		w := *m.Website
		clone.Website = &w
	}
	if m.Summary != nil {
		s := *m.Summary
		clone.Summary = &s
	}
	if m.Industry != nil {
		i := *m.Industry
		clone.Industry = &i
	}
	return clone
}

func ApplyCachedIdentity(job *Job, record CompanyIdentityRecord) {
	if job == nil {
		return
	}
	if jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) && record.Website != "" && trustedRecordEvidence(record.Identity, "website", record.Website) {
		job.CompanyWebsite = record.Website
		setCachedIdentityEvidence(job, "website", record.Website, copiedEvidence(record.Identity, "website"))
	}

	if jobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) && record.Summary != "" && trustedRecordEvidence(record.Identity, "summary", record.Summary) {
		job.CompanySummary = record.Summary
		setCachedIdentityEvidence(job, "summary", record.Summary, copiedEvidence(record.Identity, "summary"))
	}

	if jobCompanyIndustryNeedsEnrichment(*job) && record.Industry != "" && trustedRecordEvidence(record.Identity, "industry", record.Industry) {
		job.CompanyIndustry = record.Industry
		setCachedIdentityEvidence(job, "industry", record.Industry, copiedEvidence(record.Identity, "industry"))
	}
}

func seedCompanyIdentityFromTrustedJobs(jobs []Job, cache *CompanyIdentityCache) int {
	copied := propagateSameCompanyIdentity(jobs)
	if cache == nil {
		return copied
	}
	for i := range jobs {
		if !cache.IsIdentityComplete(jobs[i]) {
			continue
		}
		cache.Set(jobs[i].Company, CompanyIdentityRecord{
			Website:  jobs[i].CompanyWebsite,
			Summary:  jobs[i].CompanySummary,
			Industry: jobs[i].CompanyIndustry,
			Identity: CloneJobIdentityMetadata(jobs[i].CompanyIdentity),
		})
	}
	return copied
}

func propagateSameCompanyIdentity(jobs []Job) int {
	type trustedRecord struct {
		record       CompanyIdentityRecord
		websiteRank  int
		summaryRank  int
		industryRank int
	}

	records := make(map[string]trustedRecord)
	for _, job := range jobs {
		key := companyIdentityCacheKey(job.Company)
		if key == "" {
			continue
		}
		record, websiteRank, summaryRank, industryRank := trustedIdentityRecordFromJob(job)
		if websiteRank == 0 && summaryRank == 0 && industryRank == 0 {
			continue
		}
		existing := records[key]
		if websiteRank > existing.websiteRank {
			existing.record.Website = record.Website
			existing.websiteRank = websiteRank
			if existing.record.Identity == nil {
				existing.record.Identity = &domain.JobIdentityMetadata{}
			}
			existing.record.Identity.Website = copiedEvidence(record.Identity, "website")
		}
		if summaryRank > existing.summaryRank {
			existing.record.Summary = record.Summary
			existing.summaryRank = summaryRank
			if existing.record.Identity == nil {
				existing.record.Identity = &domain.JobIdentityMetadata{}
			}
			existing.record.Identity.Summary = copiedEvidence(record.Identity, "summary")
		}
		if industryRank > existing.industryRank {
			existing.record.Industry = record.Industry
			existing.industryRank = industryRank
			if existing.record.Identity == nil {
				existing.record.Identity = &domain.JobIdentityMetadata{}
			}
			existing.record.Identity.Industry = copiedEvidence(record.Identity, "industry")
		}
		records[key] = existing
	}

	copied := 0
	for i := range jobs {
		key := companyIdentityCacheKey(jobs[i].Company)
		if key == "" {
			continue
		}
		record, ok := records[key]
		if !ok {
			continue
		}
		copied += applySameCompanyIdentity(&jobs[i], record.record)
	}
	return copied
}

func trustedIdentityRecordFromJob(job Job) (CompanyIdentityRecord, int, int, int) {
	var record CompanyIdentityRecord
	var identity domain.JobIdentityMetadata
	websiteRank := 0
	summaryRank := 0
	industryRank := 0
	if !jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) && trustedIdentityEvidence(job.CompanyIdentity, "website", job.CompanyWebsite) {
		record.Website = job.CompanyWebsite
		w := *job.CompanyIdentity.Website
		identity.Website = &w
		websiteRank = identityEvidenceRank(&w)
	}
	if !jobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) && trustedIdentityEvidence(job.CompanyIdentity, "summary", job.CompanySummary) {
		record.Summary = job.CompanySummary
		s := *job.CompanyIdentity.Summary
		identity.Summary = &s
		summaryRank = identityEvidenceRank(&s)
	}
	if strings.TrimSpace(job.CompanyIndustry) != "" && looksLikeCompanyIndustry(job.CompanyIndustry) && trustedIdentityEvidence(job.CompanyIdentity, "industry", job.CompanyIndustry) {
		record.Industry = job.CompanyIndustry
		i := *job.CompanyIdentity.Industry
		identity.Industry = &i
		industryRank = identityEvidenceRank(&i)
	}
	if identity.Website != nil || identity.Summary != nil || identity.Industry != nil {
		record.Identity = &identity
	}
	return record, websiteRank, summaryRank, industryRank
}

func trustedCompanyIdentityRecordFromJob(job Job) (CompanyIdentityRecord, bool) {
	record, websiteRank, summaryRank, industryRank := trustedIdentityRecordFromJob(job)
	return record, websiteRank > 0 || summaryRank > 0 || industryRank > 0
}

func companyIdentityRecordFromPersistent(identity *domain.CompanyIdentity) CompanyIdentityRecord {
	if identity == nil {
		return CompanyIdentityRecord{}
	}
	record := CompanyIdentityRecord{
		Website:  strings.TrimSpace(identity.Website),
		Summary:  strings.TrimSpace(identity.Summary),
		Industry: strings.TrimSpace(identity.Industry),
	}
	if len(identity.IdentityEvidence) > 0 {
		var evidence domain.JobIdentityMetadata
		if err := json.Unmarshal(identity.IdentityEvidence, &evidence); err == nil {
			record.Identity = &evidence
		}
	}
	return record
}

func persistentCompanyIdentityFromRecord(company string, record CompanyIdentityRecord) (domain.CompanyIdentity, bool) {
	if strings.TrimSpace(company) == "" {
		return domain.CompanyIdentity{}, false
	}
	identity := domain.CompanyIdentity{
		DisplayName:   strings.TrimSpace(company),
		Website:       strings.TrimSpace(record.Website),
		Summary:       strings.TrimSpace(record.Summary),
		Industry:      strings.TrimSpace(record.Industry),
		SourceVersion: "identity-v1",
		NameAliases:   []string{strings.TrimSpace(company)},
	}
	if record.Identity != nil {
		if evidence, err := json.Marshal(record.Identity); err == nil {
			identity.IdentityEvidence = evidence
		}
	}
	return identity, identity.Website != "" || identity.Summary != "" || identity.Industry != "" || len(identity.IdentityEvidence) > 0
}

func trustedIdentityEvidence(identity *domain.JobIdentityMetadata, field string, value string) bool {
	if identity == nil || strings.TrimSpace(value) == "" {
		return false
	}
	var evidence *domain.JobIdentityEvidence
	switch field {
	case "website":
		evidence = identity.Website
	case "summary":
		evidence = identity.Summary
	case "industry":
		evidence = identity.Industry
	}
	return identityEvidenceRank(evidence) > 0 && strings.EqualFold(strings.TrimSpace(evidence.Value), strings.TrimSpace(value))
}

func trustedRecordEvidence(identity *domain.JobIdentityMetadata, field string, value string) bool {
	return trustedIdentityEvidence(identity, field, value)
}

func identityEvidenceRank(evidence *domain.JobIdentityEvidence) int {
	if evidence == nil || evidence.Provisional {
		return 0
	}
	switch strings.ToLower(strings.TrimSpace(evidence.Confidence)) {
	case "high":
		return 2
	case "medium":
		return 1
	default:
		return 0
	}
}

func applySameCompanyIdentity(job *Job, record CompanyIdentityRecord) int {
	if job == nil {
		return 0
	}
	copied := 0
	if jobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) && record.Website != "" {
		job.CompanyWebsite = record.Website
		setCopiedSameCompanyEvidence(job, "website", record.Website, copiedEvidence(record.Identity, "website"))
		copied++
	}
	if jobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) && record.Summary != "" {
		job.CompanySummary = record.Summary
		setCopiedSameCompanyEvidence(job, "summary", record.Summary, copiedEvidence(record.Identity, "summary"))
		copied++
	}
	if jobCompanyIndustryNeedsEnrichment(*job) && record.Industry != "" {
		job.CompanyIndustry = record.Industry
		setCopiedSameCompanyEvidence(job, "industry", record.Industry, copiedEvidence(record.Identity, "industry"))
		copied++
	}
	return copied
}

func copiedEvidence(identity *domain.JobIdentityMetadata, field string) *domain.JobIdentityEvidence {
	if identity == nil {
		return nil
	}
	switch field {
	case "website":
		return identity.Website
	case "summary":
		return identity.Summary
	case "industry":
		return identity.Industry
	default:
		return nil
	}
}

func setCachedIdentityEvidence(job *Job, field string, value string, original *domain.JobIdentityEvidence) {
	if original == nil {
		return
	}
	setJobIdentityEvidence(job, field, value, original.Source, original.URL, original.Confidence, original.Provisional, original.Reason)
}

func setCopiedSameCompanyEvidence(job *Job, field string, value string, original *domain.JobIdentityEvidence) {
	if original == nil {
		setJobIdentityEvidence(job, field, value, "same_company_identity_copy", "", "medium", false, "Copied from another same-company job with trusted identity evidence.")
		return
	}
	reason := "Copied from another same-company job with trusted identity evidence."
	if strings.TrimSpace(original.Source) != "" {
		reason = "Copied from another same-company job with trusted identity evidence from " + strings.TrimSpace(original.Source) + "."
	}
	setJobIdentityEvidence(job, field, value, "same_company_identity_copy", original.URL, original.Confidence, false, reason)
}
