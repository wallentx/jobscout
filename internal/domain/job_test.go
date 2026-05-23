package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJobJSONIncludesCompanyMetadata(t *testing.T) {
	input := []byte(`{
		"company": "Acme",
		"company_website": "https://www.acme.example",
		"company_summary": "Acme builds deployment tooling.",
		"company_industry": "Developer Tools",
		"title": "Staff Engineer",
		"remote": "Remote",
		"compensation": "$200,000",
		"apply_url": "https://jobs.example/acme",
		"description": "Platform role"
	}`)

	var job Job
	if err := json.Unmarshal(input, &job); err != nil {
		t.Fatalf("json.Unmarshal(Job) error = %v", err)
	}
	if job.CompanyWebsite != "https://www.acme.example" {
		t.Fatalf("Job.CompanyWebsite = %q; want company website", job.CompanyWebsite)
	}
	if job.CompanySummary != "Acme builds deployment tooling." {
		t.Fatalf("Job.CompanySummary = %q; want company summary", job.CompanySummary)
	}
	if job.CompanyIndustry != "Developer Tools" {
		t.Fatalf("Job.CompanyIndustry = %q; want company industry", job.CompanyIndustry)
	}

	encoded, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("json.Marshal(Job) error = %v", err)
	}
	out := string(encoded)
	for _, want := range []string{`"company_website":"https://www.acme.example"`, `"company_summary":"Acme builds deployment tooling."`, `"company_industry":"Developer Tools"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("json.Marshal(Job) = %s; want %s", out, want)
		}
	}
}

func TestJobJSONPreservesCompanyIdentityEvidence(t *testing.T) {
	input := []byte(`{
		"company": "Acme",
		"company_identity": {
			"website": {
				"value": "https://www.acme.example",
				"source": "company_profile",
				"url": "https://jobs.example/acme/company",
				"confidence": "high"
			},
			"industry": {
				"value": "Developer Tools",
				"source": "company_summary_inference",
				"confidence": "low",
				"provisional": true
			}
		},
		"title": "Staff Engineer",
		"remote": "Remote",
		"compensation": "$200,000",
		"apply_url": "https://jobs.example/acme"
	}`)

	var job Job
	if err := json.Unmarshal(input, &job); err != nil {
		t.Fatalf("json.Unmarshal(Job) error = %v", err)
	}
	if job.CompanyWebsite != "https://www.acme.example" {
		t.Fatalf("Job.CompanyWebsite = %q; want metadata website value", job.CompanyWebsite)
	}
	if job.CompanyIndustry != "Developer Tools" {
		t.Fatalf("Job.CompanyIndustry = %q; want metadata industry value", job.CompanyIndustry)
	}
	if job.CompanyIdentity == nil || job.CompanyIdentity.Industry == nil || !job.CompanyIdentity.Industry.Provisional {
		t.Fatalf("Job.CompanyIdentity = %#v; want provisional industry evidence", job.CompanyIdentity)
	}

	encoded, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("json.Marshal(Job) error = %v", err)
	}
	out := string(encoded)
	for _, want := range []string{`"company_identity"`, `"source":"company_profile"`, `"provisional":true`} {
		if !strings.Contains(out, want) {
			t.Fatalf("json.Marshal(Job) = %s; want %s", out, want)
		}
	}
}

func TestJobJSONAcceptsSimpleCompanyIdentityMap(t *testing.T) {
	input := []byte(`{
		"company": "Acme",
		"company_identity": {
			"website": "https://www.acme.example",
			"summary": "Acme builds deployment tooling.",
			"industry": "Developer Tools"
		},
		"title": "Staff Engineer",
		"remote": "Remote",
		"compensation": "$200,000",
		"apply_url": "https://jobs.example/acme"
	}`)

	var job Job
	if err := json.Unmarshal(input, &job); err != nil {
		t.Fatalf("json.Unmarshal(Job) error = %v", err)
	}
	if job.CompanyWebsite != "https://www.acme.example" {
		t.Fatalf("Job.CompanyWebsite = %q; want simple identity website", job.CompanyWebsite)
	}
	if job.CompanySummary != "Acme builds deployment tooling." {
		t.Fatalf("Job.CompanySummary = %q; want simple identity summary", job.CompanySummary)
	}
	if job.CompanyIndustry != "Developer Tools" {
		t.Fatalf("Job.CompanyIndustry = %q; want simple identity industry", job.CompanyIndustry)
	}
	if job.CompanyIdentity == nil || job.CompanyIdentity.Website == nil || job.CompanyIdentity.Website.Value != "https://www.acme.example" {
		t.Fatalf("Job.CompanyIdentity = %#v; want simple identity evidence", job.CompanyIdentity)
	}
}

func TestJobJSONPreservesOpportunisticMetadata(t *testing.T) {
	input := []byte(`{
		"company": "Acme",
		"title": "Staff Engineer",
		"remote": "Remote",
		"compensation": "$200,000",
		"apply_url": "https://builtin.com/job/staff-engineer/123",
		"metadata": {
			"source": {
				"posting_url": "https://builtin.com/job/staff-engineer/123",
				"external_apply_url": "https://acme.example/careers/123",
				"company_profile_url": "https://builtin.com/company/acme",
				"date_posted": "2026-05-01",
				"valid_through": "2026-06-01",
				"employment_type": "FULL_TIME",
				"locations": ["Austin, TX", "Remote"],
				"industries": ["Fintech", "Payments"],
				"skills": ["Go", "Kubernetes"]
			},
			"company": {
				"industries": ["Fintech", "Payments"],
				"employee_range": "501-1,000 employees",
				"estimated_employees": 750,
				"founded_year": 2018,
				"headquarters": "Austin, TX",
				"revenue": "$100M-$500M",
				"ticker": "ACME",
				"exchange": "NYSE",
				"public": true
			}
		}
	}`)

	var job Job
	if err := json.Unmarshal(input, &job); err != nil {
		t.Fatalf("json.Unmarshal(Job) error = %v", err)
	}
	if job.Metadata == nil || job.Metadata.Source == nil || job.Metadata.Company == nil {
		t.Fatalf("Job.Metadata = %#v; want source and company metadata", job.Metadata)
	}
	if got := job.Metadata.Source.ExternalApplyURL; got != "https://acme.example/careers/123" {
		t.Fatalf("ExternalApplyURL = %q; want external apply URL", got)
	}
	if got := strings.Join(job.Metadata.Source.Skills, ","); got != "Go,Kubernetes" {
		t.Fatalf("Source.Skills = %#v; want Go,Kubernetes", job.Metadata.Source.Skills)
	}
	if job.Metadata.Company.EstimatedEmployees == nil || *job.Metadata.Company.EstimatedEmployees != 750 {
		t.Fatalf("EstimatedEmployees = %#v; want 750", job.Metadata.Company.EstimatedEmployees)
	}
	if job.Metadata.Company.Public == nil || !*job.Metadata.Company.Public {
		t.Fatalf("Public = %#v; want true", job.Metadata.Company.Public)
	}

	encoded, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("json.Marshal(Job) error = %v", err)
	}
	out := string(encoded)
	for _, want := range []string{`"metadata"`, `"external_apply_url":"https://acme.example/careers/123"`, `"estimated_employees":750`, `"public":true`} {
		if !strings.Contains(out, want) {
			t.Fatalf("json.Marshal(Job) = %s; want %s", out, want)
		}
	}
}

func TestLooksLikeCompanyWebsiteRejectsDeepLinkAndTrackingHosts(t *testing.T) {
	tests := []string{
		"https://sofi.app.link/open",
		"https://link.rippling.com/careers",
		"https://branch.io/example",
		"https://example.onelink.me/open",
		"https://track.example.com/candidate",
		"https://click.example.com/company",
		"https://bit.ly/example",
	}

	for _, candidate := range tests {
		if LooksLikeCompanyWebsite(candidate, "") {
			t.Fatalf("LooksLikeCompanyWebsite(%q, \"\") = true, want false", candidate)
		}
	}
}
