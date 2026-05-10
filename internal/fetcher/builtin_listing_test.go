package fetcher

import (
	"strings"
	"testing"

	"github.com/wallentx/jobscout/internal/domain"
)

func TestExtractBuiltInJobsFromListingParsesExpandedCards(t *testing.T) {
	rawHTML := `<html><body>
		<div id="job-card-7934986" data-id="job-card">
			<a href="/company/cadence-care" data-id="company-title"><span>Cadence (cadence.care)</span></a>
			<h2><a href="/job/staff-devops-engineer/7934986?utm=ignored" data-id="job-card-title">Staff DevOps Engineer</a></h2>
			<span>Reposted 17 Days Ago</span>
			<span>Remote</span>
			<span>United States</span>
			<div id="drop-data-7934986" class="collapse">
				<span>200K-260K Annually</span>
				<span>Senior level</span>
				<div class="mb-md fs-xs fw-bold">Artificial Intelligence &#x2022; Healthtech &#x2022; Machine Learning &#x2022; Software &#x2022; Telehealth</div>
				<div class="fs-sm fw-regular mb-md text-gray-04">The Staff DevOps Engineer will lead cloud infrastructure design and improve DevOps practices.</div>
				<span class="fs-xs fw-bold text-uppercase text-gray-04">Top Skills:</span>
				<span class="fs-xs text-gray-04 mx-sm">AWS</span>
				<span class="fs-xs text-gray-04 mx-sm">Kubernetes</span>
				<span class="fs-xs text-gray-04 mx-sm">Terraform</span>
			</div>
		</div>
		<div id="job-card-6462163" data-id="job-card">
			<a href="/company/sailpoint" data-id="company-title"><span>SailPoint</span></a>
			<h2><a href="/job/senior-staff-devops-engineer/6462163" data-id="job-card-title">Senior Staff DevOps Engineer - AWS Infrastructure</a></h2>
			<span>Remote or Hybrid</span>
			<span>United States</span>
			<div id="drop-data-6462163" class="collapse">
				<span>Senior level</span>
				<div class="mb-md fs-xs fw-bold">Artificial Intelligence &#x2022; Cloud &#x2022; Security &#x2022; Software</div>
				<div class="fs-sm fw-regular mb-md text-gray-04">Develop and implement architecture patterns and automate deployments.</div>
			</div>
		</div>
	</body></html>`
	criteria := &CriteriaConfig{}
	criteria.Filters.WorkSettings.Remote = true
	criteria.Filters.TitleRequires = []string{"Staff"}
	criteria.Filters.TitleIncludes = []string{"DevOps Engineer"}

	jobs, filtered, cardCount := extractBuiltInJobsFromListing(rawHTML, "https://builtin.com/jobs/remote?search=Staff+DevOps+Engineer", "Site Search: Built In", criteria)

	if cardCount != 2 {
		t.Fatalf("extractBuiltInJobsFromListing() cardCount = %d; want 2", cardCount)
	}
	if len(filtered) != 0 {
		t.Fatalf("extractBuiltInJobsFromListing() filtered = %#v; want none", filtered)
	}
	if len(jobs) != 2 {
		t.Fatalf("extractBuiltInJobsFromListing() len = %d; want 2 (%#v)", len(jobs), jobs)
	}
	first := jobs[0]
	if first.Company != "Cadence (cadence.care)" {
		t.Fatalf("jobs[0].Company = %q; want Cadence (cadence.care)", first.Company)
	}
	if first.CompanyWebsite != "https://cadence.care" {
		t.Fatalf("jobs[0].CompanyWebsite = %q; want https://cadence.care", first.CompanyWebsite)
	}
	if first.CompanyIdentity == nil || first.CompanyIdentity.Website == nil || first.CompanyIdentity.Website.URL != "https://builtin.com/company/cadence-care" {
		t.Fatalf("jobs[0].CompanyIdentity.Website = %#v; want Built In company profile evidence URL", first.CompanyIdentity)
	}
	if first.ApplyURL != "https://builtin.com/job/staff-devops-engineer/7934986" {
		t.Fatalf("jobs[0].ApplyURL = %q; want canonical Built In job URL", first.ApplyURL)
	}
	if first.Remote != "Remote" {
		t.Fatalf("jobs[0].Remote = %q; want Remote", first.Remote)
	}
	if first.Compensation != "200K-260K Annually" {
		t.Fatalf("jobs[0].Compensation = %q; want 200K-260K Annually", first.Compensation)
	}
	if first.CompanyIndustry != "Artificial Intelligence" {
		t.Fatalf("jobs[0].CompanyIndustry = %q; want Artificial Intelligence", first.CompanyIndustry)
	}
	if first.CompanyIdentity == nil || first.CompanyIdentity.Industry == nil || first.CompanyIdentity.Industry.URL != "https://builtin.com/company/cadence-care" {
		t.Fatalf("jobs[0].CompanyIdentity.Industry = %#v; want Built In company profile evidence URL", first.CompanyIdentity)
	}
	if !strings.Contains(first.Description, "Top skills: AWS, Kubernetes, Terraform") {
		t.Fatalf("jobs[0].Description = %q; want top skills included", first.Description)
	}
	if jobs[1].Remote != "Remote or Hybrid" {
		t.Fatalf("jobs[1].Remote = %q; want Remote or Hybrid", jobs[1].Remote)
	}
}

func TestBuiltInCompanyProfileURLFromJobUsesCardEvidence(t *testing.T) {
	job := Job{
		Company:  "Cadence",
		Title:    "Staff DevOps Engineer",
		ApplyURL: "https://builtin.com/job/staff-devops-engineer/7934986",
		CompanyIdentity: &domain.JobIdentityMetadata{
			Industry: &JobIdentityEvidence{
				Value:  "Healthtech",
				Source: "builtin_card_industry",
				URL:    "https://builtin.com/company/cadence-care?utm=ignored",
			},
		},
	}

	got := builtInCompanyProfileURLFromJob(job)
	if got != "https://builtin.com/company/cadence-care" {
		t.Fatalf("builtInCompanyProfileURLFromJob() = %q; want canonical Built In company profile URL", got)
	}
}

func TestBuiltInCompanyProfileURLFromJobRejectsNonBuiltInJob(t *testing.T) {
	job := Job{
		Company:  "Cadence",
		Title:    "Staff DevOps Engineer",
		ApplyURL: "https://example.com/jobs/7934986",
		CompanyIdentity: &domain.JobIdentityMetadata{
			Industry: &JobIdentityEvidence{
				Value:  "Healthtech",
				Source: "builtin_card_industry",
				URL:    "https://builtin.com/company/cadence-care",
			},
		},
	}

	if got := builtInCompanyProfileURLFromJob(job); got != "" {
		t.Fatalf("builtInCompanyProfileURLFromJob() = %q; want empty for non-Built In job", got)
	}
}

func TestBuiltInWorkSettingLabel(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{text: "Remote", want: "Remote"},
		{text: "Fully Remote", want: "Remote"},
		{text: "Remote or Hybrid", want: "Remote or Hybrid"},
		{text: "In-Office or Remote", want: "In-Office or Remote"},
		{text: "Hybrid", want: "Hybrid"},
		{text: "United States", want: ""},
	}

	for _, tt := range tests {
		if got := builtInWorkSettingLabel(tt.text); got != tt.want {
			t.Fatalf("builtInWorkSettingLabel(%q) = %q; want %q", tt.text, got, tt.want)
		}
	}
}
