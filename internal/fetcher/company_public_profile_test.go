package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestExtractPublicProfileIndustryFromText(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{
			text: "Acme | LinkedIn Industry Software Development Company size 51-200 employees Headquarters Austin, TX",
			want: "Software Development",
		},
		{
			text: "Acme Reviews | Glassdoor Industry: Enterprise Software & Network Solutions Revenue $10M",
			want: "Enterprise Software & Network Solutions",
		},
		{
			text: "Acme Careers Jobs at Acme Industry jobs Search remote software roles",
			want: "",
		},
	}

	for _, tt := range tests {
		if got := extractPublicProfileIndustryFromText(tt.text); got != tt.want {
			t.Fatalf("extractPublicProfileIndustryFromText(%q) = %q; want %q", tt.text, got, tt.want)
		}
	}
}

func TestEnrichJobsFromPublicProfileIndustryUsesSearchSnippet(t *testing.T) {
	searchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
<html><body>
  <a href="https://www.linkedin.com/company/acme/">Acme | LinkedIn</a>
  <span>Industry Software Development Company size 51-200 employees Headquarters Austin, Texas</span>
</body></html>`))
	}))
	defer searchServer.Close()

	restorePublicProfileTestHooks()
	publicProfileSearchURLFunc = func(query string) string {
		return searchServer.URL + "/search?q=" + url.QueryEscape(query)
	}
	fetchPublicProfileHTML = fetchApplyPage
	defer restorePublicProfileTestHooks()

	jobs := []Job{{
		Company:        "Acme",
		CompanyWebsite: "https://www.acme.com",
		ApplyURL:       "https://jobs.acme.com/staff-platform-engineer",
		Status:         "Unopened",
	}}

	jobs = enrichJobsFromPublicProfileIndustryWithProgress(context.Background(), jobs, nil)

	if jobs[0].CompanyIndustry != "Software Development" {
		t.Fatalf("CompanyIndustry = %q; want Software Development", jobs[0].CompanyIndustry)
	}
	if jobs[0].CompanyIdentity == nil || jobs[0].CompanyIdentity.Industry == nil || jobs[0].CompanyIdentity.Industry.Source != "public_profile_linkedin" {
		t.Fatalf("CompanyIdentity.Industry = %#v; want public_profile_linkedin evidence", jobs[0].CompanyIdentity)
	}
}

func TestEnrichJobsFromPublicProfileIndustryFetchesProfilePage(t *testing.T) {
	profileHTML := `
<html><body>
  <h1>Acme</h1>
  <dl><dt>Industry</dt><dd>Developer Tools</dd></dl>
  <span>Company size 51-200 employees</span>
</body></html>`

	searchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><a href="https://www.indeed.com/cmp/acme">Acme company profile | Indeed</a></body></html>`))
	}))
	defer searchServer.Close()

	restorePublicProfileTestHooks()
	publicProfileSearchURLFunc = func(query string) string {
		return searchServer.URL + "/search?q=" + url.QueryEscape(query)
	}
	fetchPublicProfileHTML = func(ctx context.Context, pageURL string) (string, string, error) {
		if strings.Contains(pageURL, "indeed.com/cmp/acme") {
			return profileHTML, pageURL, nil
		}
		return fetchApplyPage(ctx, pageURL)
	}
	defer restorePublicProfileTestHooks()

	jobs := []Job{{
		Company:        "Acme",
		CompanyWebsite: "https://www.acme.com",
		ApplyURL:       "https://jobs.acme.com/staff-platform-engineer",
		Status:         "Unopened",
	}}

	jobs = enrichJobsFromPublicProfileIndustryWithProgress(context.Background(), jobs, nil)

	if jobs[0].CompanyIndustry != "Developer Tools" {
		t.Fatalf("CompanyIndustry = %q; want Developer Tools", jobs[0].CompanyIndustry)
	}
	if jobs[0].CompanyIdentity == nil || jobs[0].CompanyIdentity.Industry == nil || jobs[0].CompanyIdentity.Industry.Source != "public_profile_indeed" {
		t.Fatalf("CompanyIdentity.Industry = %#v; want public_profile_indeed evidence", jobs[0].CompanyIdentity)
	}
}

func TestEnrichJobsFromPublicProfileIndustryRequiresCompanyMatch(t *testing.T) {
	searchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
<html><body>
  <a href="https://www.linkedin.com/company/otherco/">OtherCo | LinkedIn</a>
  <span>Industry Healthcare Technology Company size 51-200 employees</span>
</body></html>`))
	}))
	defer searchServer.Close()

	restorePublicProfileTestHooks()
	publicProfileSearchURLFunc = func(query string) string {
		return searchServer.URL + "/search?q=" + url.QueryEscape(query)
	}
	fetchPublicProfileHTML = fetchApplyPage
	defer restorePublicProfileTestHooks()

	jobs := []Job{{
		Company:        "Acme",
		CompanyWebsite: "https://www.acme.com",
		ApplyURL:       "https://jobs.acme.com/staff-platform-engineer",
		Status:         "Unopened",
	}}

	jobs = enrichJobsFromPublicProfileIndustryWithProgress(context.Background(), jobs, nil)

	if jobs[0].CompanyIndustry != "" {
		t.Fatalf("CompanyIndustry = %q; want unrelated profile ignored", jobs[0].CompanyIndustry)
	}
}

func TestPublicProfileIndustryTargetsDedupeByCompanyDomain(t *testing.T) {
	jobs := []Job{
		{Company: "Acme", CompanyWebsite: "https://www.acme.com", ApplyURL: "https://jobs.acme.com/1", Status: "Unopened"},
		{Company: "Acme Inc.", CompanyWebsite: "https://acme.com/about", ApplyURL: "https://jobs.acme.com/2", Status: "Unopened"},
		{Company: "Other", CompanyWebsite: "https://other.example", ApplyURL: "https://jobs.other.example/1", Status: "Unopened"},
	}

	targets := publicProfileIndustryTargets(jobs)

	if len(targets) != 2 {
		t.Fatalf("publicProfileIndustryTargets(...) len = %d; want 2 (%#v)", len(targets), targets)
	}
	if len(targets[0].Indexes) != 2 {
		t.Fatalf("publicProfileIndustryTargets(...)[0].Indexes = %#v; want two Acme rows", targets[0].Indexes)
	}
}

func restorePublicProfileTestHooks() {
	publicProfileSearchURLFunc = companySearchURL
	fetchPublicProfileHTML = fetchApplyPage
}

func TestPublicProfileSourceRecognizesSupportedProfileHosts(t *testing.T) {
	tests := []struct {
		rawURL string
		want   string
	}{
		{rawURL: "https://www.linkedin.com/company/acme/", want: "linkedin"},
		{rawURL: "https://www.glassdoor.com/Overview/Working-at-Acme-EI_IE123.htm", want: "glassdoor"},
		{rawURL: "https://www.indeed.com/cmp/Acme", want: "indeed"},
		{rawURL: "https://www.linkedin.com/jobs/view/123", want: ""},
	}

	for _, tt := range tests {
		if got := publicProfileSource(tt.rawURL); got != tt.want {
			t.Fatalf("publicProfileSource(%q) = %q; want %q", tt.rawURL, got, tt.want)
		}
	}
}

func TestPublicProfileMatchesCompanyUsesDomainWhenCompanyTokenIsShort(t *testing.T) {
	job := Job{
		Company:        "H1",
		CompanyWebsite: "https://www.h1.co",
	}

	if !publicProfileMatchesCompany(job, "H1 company profile Industry Healthcare Technology", "https://www.linkedin.com/company/h1-co/") {
		t.Fatal("publicProfileMatchesCompany(...) = false; want true for short company name profile")
	}
	if publicProfileMatchesCompany(job, "Other company profile Industry Healthcare Technology", "https://www.linkedin.com/company/other/") {
		t.Fatal("publicProfileMatchesCompany(...) = true; want false for unrelated short-name profile")
	}
}

func TestCleanPublicProfileIndustryTrimsProfileMetadata(t *testing.T) {
	got := cleanPublicProfileIndustry("Software Development Company size 1,001-5,000 employees Headquarters New York")
	if !strings.EqualFold(got, "Software Development") {
		t.Fatalf("cleanPublicProfileIndustry(...) = %q; want Software Development", got)
	}
}
