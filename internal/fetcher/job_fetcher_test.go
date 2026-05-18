package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wallentx/jobscout/internal/domain"

	"github.com/mmcdole/gofeed"
	"github.com/tmc/langchaingo/llms"
)

type fakeLLMModel struct{}

func (fakeLLMModel) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	return nil, nil
}

func (fakeLLMModel) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return "", nil
}

func TestValidateJobURL(t *testing.T) {
	okServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer okServer.Close()

	notFoundServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer notFoundServer.Close()

	methodFallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer methodFallbackServer.Close()

	blockedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer blockedServer.Close()

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{name: "reachable", url: okServer.URL, want: true},
		{name: "head fallback", url: methodFallbackServer.URL, want: true},
		{name: "keep forbidden", url: blockedServer.URL, want: true},
		{name: "drop 404", url: notFoundServer.URL, want: false},
		{name: "drop malformed", url: "://bad url", want: false},
		{name: "drop empty", url: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := validateJobURL(context.Background(), tt.url)
			if got != tt.want {
				t.Fatalf("validateJobURL(%q) = %t; want %t", tt.url, got, tt.want)
			}
		})
	}
}

func TestVerifyJobPostingIgnoresLLMCompanyIdentity(t *testing.T) {
	okServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer okServer.Close()

	job := Job{
		Company:  "Unknown",
		Title:    "Platform Engineer",
		ApplyURL: okServer.URL,
		Source:   "llm:web_search",
	}

	got, reason := VerifyJobPosting(context.Background(), job)
	if !got {
		t.Fatalf("VerifyJobPosting(%#v) = false, %q; want true for reachable posting URL", job, reason)
	}
}

func TestIsKnownNonJobApplyURLAllowsAggregatorJobDetails(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "indeed search", raw: "https://www.indeed.com/jobs?q=software", want: true},
		{name: "indeed direct", raw: "https://www.indeed.com/viewjob?jk=abc123", want: false},
		{name: "linkedin search", raw: "https://www.linkedin.com/jobs/search?keywords=software", want: true},
		{name: "linkedin direct", raw: "https://www.linkedin.com/jobs/view/software-engineer-123", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isKnownNonJobApplyURL(tt.raw); got != tt.want {
				t.Fatalf("isKnownNonJobApplyURL(%q) = %t; want %t", tt.raw, got, tt.want)
			}
		})
	}
}

func TestValidateFetchedJobs(t *testing.T) {
	okServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer okServer.Close()

	notFoundServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer notFoundServer.Close()

	jobs := []Job{
		{Company: "KeepCo", Title: "Platform Engineer", ApplyURL: okServer.URL},
		{Company: "DropCo", Title: "SRE", ApplyURL: notFoundServer.URL},
		{Company: "BadCo", Title: "DevOps", ApplyURL: ""},
	}

	filtered, warnings := validateFetchedJobs(context.Background(), jobs)
	if len(filtered) != 1 {
		t.Fatalf("validateFetchedJobs() len = %d; want 1", len(filtered))
	}
	if len(warnings) != 2 {
		t.Fatalf("validateFetchedJobs() warnings len = %d; want 2", len(warnings))
	}
	if filtered[0].Company != "KeepCo" {
		t.Fatalf("validateFetchedJobs() kept %q; want KeepCo", filtered[0].Company)
	}
}

func TestValidateFetchedJobsRejectsKnownNonJobURLs(t *testing.T) {
	jobs := []Job{
		{Company: "BuiltIn", Title: "Listing Page", ApplyURL: "https://builtin.com/jobs/remote"},
		{Company: "BuiltIn", Title: "Regional Listing Page", ApplyURL: "https://www.builtinchicago.org/jobs/remote/dev-engineering/senior/search/site-reliability-engineer"},
		{Company: "Placeholder", Title: "Fake URL", ApplyURL: "https://builtin.com/company/acme/jobs/XXXXX"},
		{Company: "Glassdoor", Title: "Search Page", ApplyURL: "https://www.glassdoor.com/Job/remote-devops-technical-lead-jobs-SRCH_IL.0,6_IS11047_KO7,30.htm"},
		{Company: "ZipRecruiter", Title: "Search Page", ApplyURL: "https://www.ziprecruiter.com/jobs-search?search=Senior+DevOps+Engineer"},
		{Company: "Kube", Title: "Listing Page", ApplyURL: "https://kube.careers/kubernetes-jobs-in-usa"},
	}

	filtered, warnings := validateFetchedJobs(context.Background(), jobs)

	if len(filtered) != 0 {
		t.Fatalf("validateFetchedJobs() kept %#v, want none", filtered)
	}
	if got := len(warnings["not a direct job URL"]); got != len(jobs) {
		t.Fatalf("not a direct job URL warnings = %d, want %d: %#v", got, len(jobs), warnings)
	}
}

func TestValidateFetchedJobsRejectsLLMJobsWithoutCompanyIdentity(t *testing.T) {
	okServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer okServer.Close()

	jobs := []Job{
		{
			Company:        "Missing Identity",
			Title:          "Platform Engineer",
			ApplyURL:       okServer.URL,
			Source:         "LLM: llm_job_search",
			CompanyWebsite: "",
			CompanySummary: "",
		},
		{
			Company:        "Acme",
			Title:          "Platform Engineer",
			ApplyURL:       okServer.URL,
			Source:         "LLM: llm_job_search",
			CompanyWebsite: "https://acme.example",
			CompanySummary: "Acme builds deployment tooling for software teams.",
		},
		{
			Company:        "",
			Title:          "Frontend Engineer",
			ApplyURL:       okServer.URL,
			Source:         "LLM Web: provider web search",
			CompanyWebsite: "https://morsecorp.com",
			CompanySummary: "MORSE Corp is an employee-owned engineering services company.",
		},
	}

	filtered, warnings := validateFetchedJobs(context.Background(), jobs)

	if len(filtered) != 1 || filtered[0].Company != "Acme" {
		t.Fatalf("validateFetchedJobs() kept %#v, want only Acme", filtered)
	}
	if got := len(warnings["missing company identity"]); got != 2 {
		t.Fatalf("missing company identity warnings = %d, want 2: %#v", got, warnings)
	}
}

func TestFetchRSSFiltersBeforeApplyPageEnrichment(t *testing.T) {
	applyRequests := 0
	applyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		applyRequests++
		_, _ = w.Write([]byte(`<html><body><h1>Senior Manager</h1></body></html>`))
	}))
	defer applyServer.Close()

	rssServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <item>
      <title>Senior Manager at SlowCo</title>
      <link>` + applyServer.URL + `</link>
      <description>Remote leadership role</description>
    </item>
  </channel>
</rss>`))
	}))
	defer rssServer.Close()

	criteria := CriteriaConfig{}
	criteria.Filters.TitleExcludes = []string{"manager"}

	jobs, filtered, err := fetchRSS(context.Background(), RSSSource{Name: "RSSCo", URL: rssServer.URL}, &criteria)
	if err != nil {
		t.Fatalf("fetchRSS(...) error = %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("fetchRSS(...) jobs len = %d; want 0", len(jobs))
	}
	if got := countFilteredJobs(filtered); got != 1 {
		t.Fatalf("countFilteredJobs(filtered) = %d; want 1", got)
	}
	if applyRequests != 0 {
		t.Fatalf("apply page requests = %d; want 0 for pre-filtered RSS item", applyRequests)
	}
}

func TestFetchAllJobsDefersApplyPageEnrichmentUntilReviewAcceptance(t *testing.T) {
	applyGETRequests := 0
	applyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			applyGETRequests++
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><body><h1>Software Engineer</h1><a href="https://acme.example">Company website</a></body></html>`))
	}))
	defer applyServer.Close()

	rssServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <item>
      <title>Software Engineer at Acme</title>
      <link>` + applyServer.URL + `</link>
      <description>Remote platform role</description>
    </item>
  </channel>
</rss>`))
	}))
	defer rssServer.Close()

	appCfg := defaultAppConfig()
	appCfg.Sources.Enabled = true
	appCfg.Sources.BuiltinsEnabled = false
	appCfg.Sources.RSS.Enabled = true
	appCfg.Sources.RSS.Feeds = []RSSSource{{Name: "RSSCo", URL: rssServer.URL}}
	appCfg.Sources.APIs = nil
	appCfg.Sources.SiteSearch.Enabled = false
	appCfg.Sources.SiteSearch.Sites = nil
	appCfg.LLM.Enabled = false

	jobs, _, err := fetchAllJobs(context.Background(), &appCfg, nil, nil)
	if err != nil {
		t.Fatalf("fetchAllJobs(...) error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("fetchAllJobs(...) jobs len = %d, want 1", len(jobs))
	}
	if applyGETRequests != 0 {
		t.Fatalf("apply page GET requests = %d, want 0 before review acceptance", applyGETRequests)
	}
}

func TestFetchAllJobsDoesNotInitializeIdentityEnrichmentBeforeReviewAcceptance(t *testing.T) {
	prevInit := fetchAllJobsInitConfiguredLLM
	prevIdentity := fetchAllJobsEnrichJobIdentity
	initTasks := []string(nil)
	fetchAllJobsInitConfiguredLLM = func(ctx context.Context, appCfg *AppConfig, task string) (llms.Model, func(), error) {
		initTasks = append(initTasks, task)
		return fakeLLMModel{}, func() {}, nil
	}
	fetchAllJobsEnrichJobIdentity = func(ctx context.Context, llm llms.Model, job Job, page JobIdentityPage) (*JobIdentityEnrichment, LLMTokenUsage, error) {
		return nil, LLMTokenUsage{}, nil
	}
	t.Cleanup(func() {
		fetchAllJobsInitConfiguredLLM = prevInit
		fetchAllJobsEnrichJobIdentity = prevIdentity
	})

	applyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer applyServer.Close()

	rssServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <item>
      <title>Software Engineer at Acme</title>
      <link>` + applyServer.URL + `</link>
      <description>Remote platform role</description>
    </item>
  </channel>
</rss>`))
	}))
	defer rssServer.Close()

	appCfg := defaultAppConfig()
	appCfg.Sources.Enabled = true
	appCfg.Sources.BuiltinsEnabled = false
	appCfg.Sources.RSS.Enabled = true
	appCfg.Sources.RSS.Feeds = []RSSSource{{Name: "RSSCo", URL: rssServer.URL}}
	appCfg.Sources.APIs = nil
	appCfg.Sources.SiteSearch.Enabled = false
	appCfg.Sources.SiteSearch.Sites = nil
	appCfg.Sources.LLMWeb.Enabled = false
	appCfg.LLM.Enabled = true
	appCfg.LLM.JobSearch = false
	appCfg.LLM.JobFiltering = true

	if _, _, err := fetchAllJobs(context.Background(), &appCfg, nil, nil); err != nil {
		t.Fatalf("fetchAllJobs(...) error = %v", err)
	}
	if len(initTasks) != 0 {
		t.Fatalf("fetchAllJobs(...) initialized LLM tasks %#v, want none before review acceptance", initTasks)
	}
}

func TestFetchAllJobsLLMWebParseErrorIsSkipped(t *testing.T) {
	prevInit := fetchAllJobsInitConfiguredLLM
	prevSearch := fetchAllJobsExecuteLLMSearch
	prevWebSearch := fetchAllJobsExecuteLLMWebSearch
	fetchAllJobsInitConfiguredLLM = func(ctx context.Context, appCfg *AppConfig, task string) (llms.Model, func(), error) {
		if task != llmTaskJobSearch {
			t.Fatalf("fetchAllJobsInitConfiguredLLM task = %q, want %q", task, llmTaskJobSearch)
		}
		return fakeLLMModel{}, func() {}, nil
	}
	fetchAllJobsExecuteLLMWebSearch = nil
	fetchAllJobsExecuteLLMSearch = func(ctx context.Context, llm llms.Model, prompt string) ([]Job, error) {
		if !strings.Contains(prompt, "return exactly [] with no explanation") {
			t.Fatalf("LLM web prompt = %q; want no-web-search JSON fallback instruction", prompt)
		}
		return nil, assertError("failed to parse LLM JSON output: invalid character 'I' looking for beginning of value")
	}
	t.Cleanup(func() {
		fetchAllJobsInitConfiguredLLM = prevInit
		fetchAllJobsExecuteLLMSearch = prevSearch
		fetchAllJobsExecuteLLMWebSearch = prevWebSearch
	})

	appCfg := defaultAppConfig()
	appCfg.LLM.Enabled = true
	appCfg.LLM.JobSearch = false
	appCfg.LLM.JobFiltering = false
	appCfg.Sources.Enabled = false
	appCfg.Sources.RSS.Enabled = false
	appCfg.Sources.APIs = nil
	appCfg.Sources.SiteSearch.Enabled = false
	appCfg.Sources.SiteSearch.Sites = nil
	appCfg.Sources.LLMWeb.Enabled = true
	appCfg.Sources.LLMWeb.Targets = []string{"site:jobs.ashbyhq.com"}
	criteria := defaultCriteriaConfig()

	jobs, summary, err := fetchAllJobs(context.Background(), &appCfg, &criteria, nil)
	if err != nil {
		t.Fatalf("fetchAllJobs() error = %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("fetchAllJobs() jobs len = %d, want 0", len(jobs))
	}
	status := summary.Searches[FetchSearchLLMWeb]
	if !strings.HasPrefix(status, "enabled, but provider did not return JSON") {
		t.Fatalf("summary.Searches[%q] = %q; want non-JSON skip status", FetchSearchLLMWeb, status)
	}
	if searchStatusFailed(status) {
		t.Fatalf("searchStatusFailed(%q) = true, want false", status)
	}
}

func TestFetchAllJobsLLMWebUsesDedicatedRunner(t *testing.T) {
	applyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer applyServer.Close()

	prevInit := fetchAllJobsInitConfiguredLLM
	prevSearch := fetchAllJobsExecuteLLMSearch
	prevWebSearch := fetchAllJobsExecuteLLMWebSearch
	fetchAllJobsInitConfiguredLLM = nil
	fetchAllJobsExecuteLLMSearch = nil
	webSearchCalled := false
	fetchAllJobsExecuteLLMWebSearch = func(ctx context.Context, appCfg *AppConfig, prompt string) ([]Job, error) {
		webSearchCalled = true
		if !strings.Contains(prompt, "Search only these public-web queries") {
			t.Fatalf("dedicated llm_web prompt = %q; want web-query prompt", prompt)
		}
		return []Job{{
			Company:        "Acme",
			Title:          "Software Engineer",
			ApplyURL:       applyServer.URL,
			CompanyWebsite: "https://acme.example.com",
			CompanySummary: "Acme builds developer productivity software for engineering teams that ship cloud applications.",
			Source:         "provider web search",
		}}, nil
	}
	t.Cleanup(func() {
		fetchAllJobsInitConfiguredLLM = prevInit
		fetchAllJobsExecuteLLMSearch = prevSearch
		fetchAllJobsExecuteLLMWebSearch = prevWebSearch
	})

	appCfg := defaultAppConfig()
	appCfg.LLM.Enabled = true
	appCfg.LLM.JobSearch = false
	appCfg.LLM.JobFiltering = false
	appCfg.Sources.Enabled = false
	appCfg.Sources.RSS.Enabled = false
	appCfg.Sources.APIs = nil
	appCfg.Sources.SiteSearch.Enabled = false
	appCfg.Sources.SiteSearch.Sites = nil
	appCfg.Sources.LLMWeb.Enabled = true
	appCfg.Sources.LLMWeb.Targets = []string{"site:jobs.ashbyhq.com"}
	criteria := defaultCriteriaConfig()

	jobs, summary, err := fetchAllJobs(context.Background(), &appCfg, &criteria, nil)
	if err != nil {
		t.Fatalf("fetchAllJobs() error = %v", err)
	}
	if !webSearchCalled {
		t.Fatal("fetchAllJobs() did not call dedicated llm_web runner")
	}
	if got, want := len(jobs), 1; got != want {
		t.Fatalf("fetchAllJobs() jobs len = %d, want %d", got, want)
	}
	if status := summary.Searches[FetchSearchLLMWeb]; !strings.HasPrefix(status, "executed; found 1 results") {
		t.Fatalf("summary.Searches[%q] = %q; want executed status", FetchSearchLLMWeb, status)
	}
}

func TestFetchAllJobsLLMWebRepairsIdentityBeforeValidation(t *testing.T) {
	applyGETRequests := 0
	applyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodGet {
			return
		}
		applyGETRequests++
		_, _ = w.Write([]byte(`
<html>
  <body>
    <h1>Software Engineer</h1>
    <p>Acme builds developer productivity software for engineering teams shipping cloud applications.</p>
    <p>Industry: Developer Tools</p>
    <a href="https://www.acme.com">Company website</a>
  </body>
</html>`))
	}))
	defer applyServer.Close()

	prevInit := fetchAllJobsInitConfiguredLLM
	prevSearch := fetchAllJobsExecuteLLMSearch
	prevWebSearch := fetchAllJobsExecuteLLMWebSearch
	initCalled := false
	fetchAllJobsInitConfiguredLLM = func(ctx context.Context, appCfg *AppConfig, task string) (llms.Model, func(), error) {
		initCalled = true
		return fakeLLMModel{}, func() {}, nil
	}
	fetchAllJobsExecuteLLMSearch = nil
	fetchAllJobsExecuteLLMWebSearch = func(ctx context.Context, appCfg *AppConfig, prompt string) ([]Job, error) {
		return []Job{{
			Company:  "Acme",
			Title:    "Software Engineer",
			ApplyURL: applyServer.URL,
			Source:   "provider web search",
		}}, nil
	}
	t.Cleanup(func() {
		fetchAllJobsInitConfiguredLLM = prevInit
		fetchAllJobsExecuteLLMSearch = prevSearch
		fetchAllJobsExecuteLLMWebSearch = prevWebSearch
	})

	appCfg := defaultAppConfig()
	appCfg.LLM.Enabled = true
	appCfg.LLM.JobSearch = false
	appCfg.LLM.JobFiltering = false
	appCfg.Sources.Enabled = false
	appCfg.Sources.RSS.Enabled = false
	appCfg.Sources.APIs = nil
	appCfg.Sources.SiteSearch.Enabled = false
	appCfg.Sources.SiteSearch.Sites = nil
	appCfg.Sources.LLMWeb.Enabled = true
	appCfg.Sources.LLMWeb.Targets = []string{"site:jobs.ashbyhq.com"}
	criteria := defaultCriteriaConfig()

	jobs, summary, err := fetchAllJobs(context.Background(), &appCfg, &criteria, nil)
	if err != nil {
		t.Fatalf("fetchAllJobs() error = %v", err)
	}
	if got, want := len(jobs), 1; got != want {
		t.Fatalf("fetchAllJobs() jobs len = %d, want %d; rejected=%#v", got, want, summary.Rejected)
	}
	if applyGETRequests == 0 {
		t.Fatal("fetchAllJobs() did not fetch the apply page for llm_web identity repair")
	}
	if initCalled {
		t.Fatal("fetchAllJobs() initialized LLM identity fallback, want fast deterministic repair only")
	}
	if got, want := jobs[0].CompanyWebsite, "https://www.acme.com"; got != want {
		t.Fatalf("jobs[0].CompanyWebsite = %q, want %q", got, want)
	}
	if !strings.Contains(jobs[0].CompanySummary, "developer productivity software") {
		t.Fatalf("jobs[0].CompanySummary = %q, want apply-page company summary", jobs[0].CompanySummary)
	}
	if got, want := jobs[0].CompanyIndustry, "Developer Tools"; got != want {
		t.Fatalf("jobs[0].CompanyIndustry = %q, want %q", got, want)
	}
	if status := summary.Searches[FetchSearchLLMWeb]; !strings.HasPrefix(status, "executed; found 1 results") {
		t.Fatalf("summary.Searches[%q] = %q; want executed status", FetchSearchLLMWeb, status)
	}
}

func TestFetchAllJobsLLMWebFiltersBeforeIdentityRepair(t *testing.T) {
	applyGETRequests := make(map[string]int)
	applyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodGet {
			return
		}
		applyGETRequests[r.URL.Path]++
		_, _ = w.Write([]byte(`
<html>
  <body>
    <p>KeepCo builds deployment automation software for engineering teams.</p>
    <a href="https://www.keepco.com">Company website</a>
  </body>
</html>`))
	}))
	defer applyServer.Close()

	prevInit := fetchAllJobsInitConfiguredLLM
	prevSearch := fetchAllJobsExecuteLLMSearch
	prevWebSearch := fetchAllJobsExecuteLLMWebSearch
	fetchAllJobsInitConfiguredLLM = nil
	fetchAllJobsExecuteLLMSearch = nil
	fetchAllJobsExecuteLLMWebSearch = func(ctx context.Context, appCfg *AppConfig, prompt string) ([]Job, error) {
		return []Job{
			{
				Company:  "DropCo",
				Title:    "Engineering Manager",
				ApplyURL: applyServer.URL + "/drop",
				Source:   "provider web search",
			},
			{
				Company:  "KeepCo",
				Title:    "Software Engineer",
				ApplyURL: applyServer.URL + "/keep",
				Source:   "provider web search",
			},
		}, nil
	}
	t.Cleanup(func() {
		fetchAllJobsInitConfiguredLLM = prevInit
		fetchAllJobsExecuteLLMSearch = prevSearch
		fetchAllJobsExecuteLLMWebSearch = prevWebSearch
	})

	appCfg := defaultAppConfig()
	appCfg.LLM.Enabled = true
	appCfg.LLM.JobSearch = false
	appCfg.LLM.JobFiltering = false
	appCfg.Sources.Enabled = false
	appCfg.Sources.RSS.Enabled = false
	appCfg.Sources.APIs = nil
	appCfg.Sources.SiteSearch.Enabled = false
	appCfg.Sources.SiteSearch.Sites = nil
	appCfg.Sources.LLMWeb.Enabled = true
	appCfg.Sources.LLMWeb.Targets = []string{"site:jobs.ashbyhq.com"}
	criteria := defaultCriteriaConfig()
	criteria.Filters.TitleExcludes = []string{"manager"}

	jobs, summary, err := fetchAllJobs(context.Background(), &appCfg, &criteria, nil)
	if err != nil {
		t.Fatalf("fetchAllJobs() error = %v", err)
	}
	if got, want := len(jobs), 1; got != want {
		t.Fatalf("fetchAllJobs() jobs len = %d, want %d; filtered=%#v rejected=%#v", got, want, summary.Filtered, summary.Rejected)
	}
	if jobs[0].Company != "KeepCo" {
		t.Fatalf("fetchAllJobs()[0].Company = %q, want KeepCo", jobs[0].Company)
	}
	if got := applyGETRequests["/drop"]; got != 0 {
		t.Fatalf("apply GET requests for filtered job = %d, want 0", got)
	}
	if got := applyGETRequests["/keep"]; got == 0 {
		t.Fatal("apply GET requests for kept job = 0, want identity repair request")
	}
	if got, want := len(summary.Filtered["title excludes"]), 1; got != want {
		t.Fatalf("summary.Filtered[%q] len = %d, want %d", "title excludes", got, want)
	}
	if status := summary.Searches[FetchSearchLLMWeb]; !strings.Contains(status, "filtered 1") {
		t.Fatalf("summary.Searches[%q] = %q; want filtered count", FetchSearchLLMWeb, status)
	}
}

func TestJobNeedsApplyPageEnrichmentWhenCompanyMissing(t *testing.T) {
	job := Job{
		Company:         "",
		Title:           "Software Engineer",
		ApplyURL:        "https://job-boards.greenhouse.io/morsecorp/jobs/123456",
		CompanyWebsite:  "https://morsecorp.com",
		CompanySummary:  "MORSE Corp is an employee-owned engineering services company.",
		CompanyIndustry: "Engineering Services",
		Compensation:    "$120,000",
	}

	if !jobNeedsApplyPageEnrichment(job) {
		t.Fatalf("jobNeedsApplyPageEnrichment(%#v) = false, want true for missing company", job)
	}
}

func TestNormalizeRSSJobIdentityParsesCompanyFromAtSuffix(t *testing.T) {
	company, title := normalizeRSSJobIdentity(
		"Real Work From Anywhere Backend RSS",
		&gofeed.Item{Title: "Real Work From Anywhere Backend RSS - Senior Backend Engineer, Billing & Monetization at LiveKit"},
	)

	if company != "LiveKit" {
		t.Fatalf("company = %q; want LiveKit", company)
	}
	if title != "Senior Backend Engineer, Billing & Monetization" {
		t.Fatalf("title = %q; want parsed title without source prefix or company suffix", title)
	}
}

func TestNormalizeRSSJobIdentityParsesWeWorkRemotelyCompanyFromColon(t *testing.T) {
	company, title := normalizeRSSJobIdentity(
		"We Work Remotely Full-Stack RSS",
		&gofeed.Item{Title: "Booksy: Customer Support Advisor", Link: "https://weworkremotely.com/remote-jobs/booksy-customer-support-advisor"},
	)

	if company != "Booksy" {
		t.Fatalf("company = %q; want Booksy", company)
	}
	if title != "Customer Support Advisor" {
		t.Fatalf("title = %q; want parsed title after company prefix", title)
	}
}

func TestNormalizeRSSJobIdentityDoesNotGloballySplitNonJobColonTitles(t *testing.T) {
	company, title := normalizeRSSJobIdentity(
		"Generic RSS",
		&gofeed.Item{Title: "Booksy: Customer Support Advisor"},
	)

	if company != "Unknown" {
		t.Fatalf("company = %q; want Unknown for generic RSS colon title", company)
	}
	if title != "Booksy: Customer Support Advisor" {
		t.Fatalf("title = %q; want original title", title)
	}
}

func TestEnrichJobFromDescriptionExtractsCompanyMetadata(t *testing.T) {
	job := Job{
		Company:  "Cloudpepper",
		ApplyURL: "https://weworkremotely.com/remote-jobs/cloudpepper-senior-platform-devops-engineer",
		Description: `<img src="https://example.invalid/logo.gif" />
<p>
  <strong>Headquarters:</strong> Brussels, Belgium
  <br /><strong>URL:</strong> <a href="https://cloudpepper.io">https://cloudpepper.io</a>
</p>
<div>
  <p>At Cloudpepper, we build and operate the platform behind 10,000+ Odoo instances. Our control plane is built with Symfony/PHP.</p>
</div>
<p>Apply at <a href="https://cloudpepper.io/careers/platform-engineer/">https://cloudpepper.io/careers/platform-engineer/</a></p>
<p><strong>To apply:</strong> <a href="https://weworkremotely.com/remote-jobs/cloudpepper-senior-platform-devops-engineer">Apply</a></p>`,
	}

	enrichJobFromDescription(&job)

	if job.CompanyWebsite != "https://cloudpepper.io" {
		t.Fatalf("CompanyWebsite = %q; want https://cloudpepper.io", job.CompanyWebsite)
	}
	wantSummary := "At Cloudpepper, we build and operate the platform behind 10,000+ Odoo instances. Our control plane is built with Symfony/PHP."
	if job.CompanySummary != wantSummary {
		t.Fatalf("CompanySummary = %q; want %q", job.CompanySummary, wantSummary)
	}
	if job.ApplyURL != "https://cloudpepper.io/careers/platform-engineer/" {
		t.Fatalf("ApplyURL = %q; want direct company apply URL", job.ApplyURL)
	}
}

func TestEnrichJobFromHTMLExtractsApplyPageCompanyIdentity(t *testing.T) {
	job := Job{
		Company:      "Circle",
		Title:        "Senior Back-End Software Engineer, Infra",
		ApplyURL:     "https://www.realworkfromanywhere.com/jobs/senior-back-end-software-engineer-infra-circle-9182",
		Compensation: "Not listed",
	}
	rawHTML := `
<h1>Senior Back-End Software Engineer, Infra</h1>
<p>Circle is a global fintech firm focused on creating efficient, open financial infrastructure using stablecoins. They are the issuer of USDC and EURC, facilitating fast cross-border payments, programmable money, and financial products that bridge blockchain with traditional finance.</p>
<div>Industry: Financial Services / Crypto</div>
<a href="https://www.circle.com">Website</a>
<section><h2>About the job</h2><div>Salary $130,000 - $140,000 USD</div></section>`

	enrichJobFromHTML(&job, rawHTML, job.ApplyURL)

	if job.CompanyWebsite != "https://www.circle.com" {
		t.Fatalf("CompanyWebsite = %q; want https://www.circle.com", job.CompanyWebsite)
	}
	if job.CompanyIndustry != "Financial Services / Crypto" {
		t.Fatalf("CompanyIndustry = %q; want Financial Services / Crypto", job.CompanyIndustry)
	}
	if job.CompanyIdentity == nil || job.CompanyIdentity.Industry == nil {
		t.Fatalf("CompanyIdentity = %#v; want industry evidence", job.CompanyIdentity)
	}
	if job.CompanyIdentity.Industry.Provisional {
		t.Fatalf("CompanyIdentity.Industry.Provisional = true, want false for explicit industry label")
	}
	if job.CompanyIdentity.Industry.Source != "apply_page" {
		t.Fatalf("CompanyIdentity.Industry.Source = %q; want apply_page", job.CompanyIdentity.Industry.Source)
	}
	if job.Compensation != "$130,000 - $140,000 USD" {
		t.Fatalf("Compensation = %q; want salary range", job.Compensation)
	}
	if !strings.Contains(job.CompanySummary, "global fintech firm") {
		t.Fatalf("CompanySummary = %q; want factual Circle summary", job.CompanySummary)
	}
}

func TestEnrichJobFromHTMLExtractsYCombinatorCompanyIdentity(t *testing.T) {
	job := Job{
		Company:      "Unknown",
		Title:        "Senior Software Engineer",
		ApplyURL:     "https://www.ycombinator.com/companies/just-appraised/jobs/KmLmKMP-senior-software-engineer",
		Compensation: "Not listed",
	}
	rawHTML := `
<h2><a href="/companies/just-appraised">Just Appraised</a></h2>
<p>Just Appraised makes software for local governments.</p>
<h1>Senior Software Engineer</h1>
<div>$120K - $180K•US / CA / Remote (US; CA)</div>
<h2>About Just Appraised</h2>
<p>Our goal is to empower local government employees with tools that streamline local government tax assessors' workflows using AI.</p>
<a href="https://www.justappraised.com">Company website</a>`

	enrichJobFromHTML(&job, rawHTML, job.ApplyURL)

	if job.Company != "Just Appraised" {
		t.Fatalf("Company = %q; want Just Appraised", job.Company)
	}
	if job.CompanyWebsite != "https://www.justappraised.com" {
		t.Fatalf("CompanyWebsite = %q; want https://www.justappraised.com", job.CompanyWebsite)
	}
	if !strings.Contains(job.CompanySummary, "software for local governments") {
		t.Fatalf("CompanySummary = %q; want YC tagline summary", job.CompanySummary)
	}
	if job.CompanyIndustry != "Government Technology" {
		t.Fatalf("CompanyIndustry = %q; want Government Technology", job.CompanyIndustry)
	}
	if job.Compensation != "$120K - $180K" {
		t.Fatalf("Compensation = %q; want $120K - $180K", job.Compensation)
	}
}

func TestEnrichJobFromHTMLFallsBackToYCombinatorCompanySlug(t *testing.T) {
	job := Job{
		Company:  "Unknown",
		Title:    "Senior Software Engineer",
		ApplyURL: "https://www.ycombinator.com/companies/just-appraised/jobs/KmLmKMP-senior-software-engineer",
	}
	rawHTML := `<h1>Senior Software Engineer</h1>`

	enrichJobFromHTML(&job, rawHTML, job.ApplyURL)

	if job.Company != "Just Appraised" {
		t.Fatalf("Company = %q; want Just Appraised", job.Company)
	}
}

func TestEnrichJobFromHTMLExtractsGreenhouseLogoLink(t *testing.T) {
	job := Job{
		Company:  "Unknown",
		Title:    "Senior Software Engineer, DevOps",
		ApplyURL: "https://job-boards.greenhouse.io/muckrack/jobs/8523017002",
	}
	rawHTML := `
<main>
  <div class="image-container">
    <a href="http://muckrack.com" target="_blank" rel="noreferrer" class="logo">
      <img src="https://s2-recruiting.cdn.greenhouse.io/external_greenhouse_job_boards/logos/muckrack.png" alt="Muck Rack Logo">
    </a>
  </div>
  <div class="job__description body">
    <p>Muck Rack is the leading SaaS platform for public relations and communications professionals.</p>
  </div>
</main>`

	enrichJobFromHTML(&job, rawHTML, job.ApplyURL)

	if job.Company != "Muck Rack" {
		t.Fatalf("Company = %q; want Muck Rack", job.Company)
	}
	if job.CompanyWebsite != "http://muckrack.com" {
		t.Fatalf("CompanyWebsite = %q; want http://muckrack.com", job.CompanyWebsite)
	}
	if job.CompanyIndustry != "Media / Communications" {
		t.Fatalf("CompanyIndustry = %q; want Media / Communications", job.CompanyIndustry)
	}
}

func TestEnrichJobFromHTMLExtractsGreenhouseLogoAltOnly(t *testing.T) {
	job := Job{
		Company:  "Unknown",
		Title:    "Senior DevOps Engineer",
		ApplyURL: "https://job-boards.greenhouse.io/captivation/jobs/5200677008",
	}
	rawHTML := `
<main>
  <div class="image-container">
    <img src="https://s2-recruiting.cdn.greenhouse.io/external_greenhouse_job_boards/logos/captivation.png" alt="Captivation Software Logo">
  </div>
  <div class="job__description body">
    <p>Captivation Software is a government contractor building secure software for mission customers.</p>
  </div>
</main>`

	enrichJobFromHTML(&job, rawHTML, job.ApplyURL)

	if job.Company != "Captivation Software" {
		t.Fatalf("Company = %q; want Captivation Software", job.Company)
	}
	if job.CompanyWebsite != "" {
		t.Fatalf("CompanyWebsite = %q; want empty when Greenhouse logo has no external link", job.CompanyWebsite)
	}
	if job.CompanyIndustry != "Government Technology" {
		t.Fatalf("CompanyIndustry = %q; want Government Technology", job.CompanyIndustry)
	}
}

func TestEnrichJobFromHTMLExtractsGreenhouseCompanyFromTitle(t *testing.T) {
	job := Job{
		Company:  "",
		Title:    "Full Stack Software Engineer",
		ApplyURL: "https://job-boards.greenhouse.io/morsecorp/jobs/123456",
	}
	rawHTML := `
<html>
  <head><title>Job Application for Full Stack Software Engineer at MORSE Corp</title></head>
  <body>
    <div class="job__description body">
      <p>MORSE Corp is an employee-owned engineering services company building mission software.</p>
    </div>
  </body>
</html>`

	enrichJobFromHTML(&job, rawHTML, job.ApplyURL)

	if job.Company != "MORSE Corp" {
		t.Fatalf("Company = %q; want MORSE Corp", job.Company)
	}
	if !strings.Contains(job.CompanySummary, "employee-owned engineering services") {
		t.Fatalf("CompanySummary = %q; want MORSE Corp summary", job.CompanySummary)
	}
}

func TestEnrichJobFromHTMLExtractsCompanyFromSummary(t *testing.T) {
	job := Job{
		Company:  "",
		Title:    "Software Engineer",
		ApplyURL: "https://jobs.example/acme/software-engineer",
	}
	rawHTML := `
<html>
  <body>
    <p>Acme Robotics builds autonomous warehouse systems for logistics teams.</p>
    <a href="https://acmerobotics.example">Company website</a>
  </body>
</html>`

	enrichJobFromHTML(&job, rawHTML, job.ApplyURL)

	if job.Company != "Acme Robotics" {
		t.Fatalf("Company = %q; want Acme Robotics", job.Company)
	}
}

func TestEnrichJobFromHTMLMarksInferredIndustryProvisional(t *testing.T) {
	job := Job{
		Company:  "Circle",
		Title:    "Platform Engineer",
		ApplyURL: "https://jobs.example/circle/platform-engineer",
	}
	rawHTML := `
<h1>Platform Engineer</h1>
<p>Circle provides financial infrastructure for stablecoin payments and programmable money.</p>
<a href="https://www.circle.com">Website</a>`

	enrichJobFromHTML(&job, rawHTML, job.ApplyURL)

	if job.CompanyIndustry != "Financial Technology" {
		t.Fatalf("CompanyIndustry = %q; want Financial Technology", job.CompanyIndustry)
	}
	if job.CompanyIdentity == nil || job.CompanyIdentity.Industry == nil {
		t.Fatalf("CompanyIdentity = %#v; want industry evidence", job.CompanyIdentity)
	}
	if !job.CompanyIdentity.Industry.Provisional {
		t.Fatalf("CompanyIdentity.Industry.Provisional = false, want true for inferred industry")
	}
	if job.CompanyIdentity.Industry.Confidence != "low" {
		t.Fatalf("CompanyIdentity.Industry.Confidence = %q; want low", job.CompanyIdentity.Industry.Confidence)
	}
}

func TestInferCompanyIndustryRecognizesCommonProfileSummaries(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{
			text: "Anthropic is an AI safety and research company that builds reliable, interpretable, and steerable AI systems.",
			want: "Artificial Intelligence",
		},
		{
			text: "Agiloft is a global leader in contract lifecycle management software for legal operations teams.",
			want: "Legal Technology",
		},
		{
			text: "BNSF Railway operates one of North America's largest freight railroad networks.",
			want: "Transportation / Logistics",
		},
		{
			text: "Aurora is a company focused on self-driving freight technology.",
			want: "Autonomous Vehicle Technology",
		},
		{
			text: "Cardlytics uses purchase data and a consumer engagement platform to help brands reach bank customers.",
			want: "Advertising Technology",
		},
		{
			text: "Muck Rack is the leading SaaS platform for public relations and communications professionals.",
			want: "Media / Communications",
		},
	}

	for _, tt := range tests {
		if got := inferCompanyIndustry(tt.text); got != tt.want {
			t.Fatalf("inferCompanyIndustry(%q) = %q; want %q", tt.text, got, tt.want)
		}
	}
}

func TestEnrichJobIndustryFromExistingSummary(t *testing.T) {
	job := Job{
		Company:        "Anthropic",
		ApplyURL:       "https://jobs.example/anthropic/sre",
		CompanyWebsite: "https://www.anthropic.com",
		CompanySummary: "Anthropic is an AI safety and research company that builds reliable, interpretable, and steerable AI systems.",
	}

	enrichJobIndustryFromExistingSummary(&job)

	if job.CompanyIndustry != "Artificial Intelligence" {
		t.Fatalf("CompanyIndustry = %q; want Artificial Intelligence", job.CompanyIndustry)
	}
	if job.CompanyIdentity == nil || job.CompanyIdentity.Industry == nil {
		t.Fatalf("CompanyIdentity = %#v; want industry evidence", job.CompanyIdentity)
	}
	if job.CompanyIdentity.Industry.Source != "company_summary_inference" {
		t.Fatalf("CompanyIdentity.Industry.Source = %q; want company_summary_inference", job.CompanyIdentity.Industry.Source)
	}
	if !job.CompanyIdentity.Industry.Provisional {
		t.Fatalf("CompanyIdentity.Industry.Provisional = false; want true")
	}
}

func TestEnrichJobIndustryFromExistingSummaryDoesNotReplaceExistingIndustry(t *testing.T) {
	job := Job{
		Company:         "Aurora Innovation",
		ApplyURL:        "https://jobs.example/aurora/sre",
		CompanyWebsite:  "https://aurora.tech",
		CompanySummary:  "Aurora is a company focused on self-driving freight technology.",
		CompanyIndustry: "Autonomous Vehicle Technology",
		CompanyIdentity: &domain.JobIdentityMetadata{Industry: &domain.JobIdentityEvidence{
			Source:      "company_summary_inference",
			Provisional: true,
		}},
	}

	enrichJobIndustryFromExistingSummary(&job)

	if job.CompanyIndustry != "Autonomous Vehicle Technology" {
		t.Fatalf("CompanyIndustry = %q; want existing industry preserved", job.CompanyIndustry)
	}
}

func TestEnrichJobFromHTMLDoesNotInferIndustryFromBroadPageText(t *testing.T) {
	job := Job{
		Company:  "Agiloft",
		Title:    "Staff DevOps Engineer",
		ApplyURL: "https://builtin.com/job/staff-devops-engineer/8458870",
	}
	rawHTML := `
<h1>Staff DevOps Engineer</h1>
<p>Agiloft helps organizations manage the end-to-end process of proposing, negotiating, signing, and leveraging contracts.</p>
<p>Our customers include financial institutions, healthcare providers, and public-sector teams.</p>
<a href="https://www.agiloft.com">Website</a>`

	enrichJobFromHTML(&job, rawHTML, job.ApplyURL)

	if job.CompanyIndustry != "" {
		t.Fatalf("CompanyIndustry = %q; want no broad page-text industry inference", job.CompanyIndustry)
	}
}

func TestEnrichJobFromHTMLRejectsAssetAsCompanyWebsite(t *testing.T) {
	job := Job{
		Company:  "Circle",
		ApplyURL: "https://www.realworkfromanywhere.com/jobs/senior-site-reliability-engineer-circle-7225",
	}
	rawHTML := `
<link rel="apple-touch-icon" href="https://circle.so/apple-icon.png">
<link rel="preload" href="https://avatars.githubusercontent.com/u/67208791?s=200&amp;v=4" as="image">
<p>Check out our <a href="https://careers.circle.so/">Careers</a> page for more about working at Circle.</p>`

	enrichJobFromHTML(&job, rawHTML, job.ApplyURL)

	if job.CompanyWebsite != "https://circle.so" {
		t.Fatalf("CompanyWebsite = %q; want https://circle.so", job.CompanyWebsite)
	}
}

func TestNormalizeCompanyWebsiteURLStripsMarketingSubdomainsAndPaths(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{raw: "https://go.webflow.com/campaign", want: "https://webflow.com"},
		{raw: "https://discover.mongodb.com/sre", want: "https://mongodb.com"},
		{raw: "https://redirect.cyberhaven.io/login", want: "https://cyberhaven.io"},
		{raw: "https://jobs.twilio.com/careers", want: "https://twilio.com"},
		{raw: "https://pages2.wallarm.com/demo", want: "https://wallarm.com"},
		{raw: "https://forum.greenbone.net/", want: "https://greenbone.net"},
		{raw: "https://www.circle.com/about", want: "https://www.circle.com"},
	}

	for _, tt := range tests {
		if got := normalizeCompanyWebsiteURL(tt.raw); got != tt.want {
			t.Fatalf("normalizeCompanyWebsiteURL(%q) = %q; want %q", tt.raw, got, tt.want)
		}
	}
}

func TestSanitizeExistingJobIdentityClearsBadPayloadValues(t *testing.T) {
	tests := []struct {
		name string
		job  Job
	}{
		{
			name: "blocked utility host",
			job: Job{
				Company:         "Loft Orbital",
				CompanyWebsite:  "https://gmpg.org/xfn/11",
				CompanySummary:  "You will own cloud infrastructure and help us scale production systems.",
				CompanyIndustry: "leading 401",
			},
		},
		{
			name: "title-only directory match",
			job: Job{
				Company:        "Loft Orbital Solutions",
				CompanyWebsite: "https://marketplace.aviationweek.com/company/loft-orbital",
			},
		},
		{
			name: "blocked nested host",
			job: Job{
				Company:        "Netflix",
				CompanyWebsite: "https://apply.netflixhouse.com/careers",
			},
		},
		{
			name: "generic token host match",
			job: Job{
				Company:        "Nscon Network, Security & Consulting Gmbh",
				CompanyWebsite: "https://cybersecurityjobslist.com/company/nscon",
			},
		},
		{
			name: "video shortlink",
			job: Job{
				Company:        "Renesas Electronics",
				CompanyWebsite: "https://youtu.be/example",
			},
		},
		{
			name: "mobile deep link",
			job: Job{
				Company:        "SoFi",
				CompanyWebsite: "https://sofi.app.link/open",
			},
		},
		{
			name: "tracking link subdomain",
			job: Job{
				Company:        "Rippling",
				CompanyWebsite: "https://link.rippling.com/careers",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := tt.job
			sanitizeExistingJobIdentity(&job)

			if job.CompanyWebsite != "" {
				t.Fatalf("CompanyWebsite = %q; want bad URL cleared", job.CompanyWebsite)
			}
			if strings.TrimSpace(tt.job.CompanySummary) != "" && job.CompanySummary != "" {
				t.Fatalf("CompanySummary = %q; want job pitch cleared", job.CompanySummary)
			}
			if strings.TrimSpace(tt.job.CompanyIndustry) != "" && job.CompanyIndustry != "" {
				t.Fatalf("CompanyIndustry = %q; want invalid industry cleared", job.CompanyIndustry)
			}
		})
	}
}

func TestSanitizeExistingJobIdentityNormalizesMarketingWebsitePrefixes(t *testing.T) {
	tests := []struct {
		name string
		job  Job
		want string
	}{
		{
			name: "numbered pages prefix",
			job: Job{
				Company:        "Wallarm",
				CompanyWebsite: "https://pages2.wallarm.com/demo",
			},
			want: "https://wallarm.com",
		},
		{
			name: "forum prefix",
			job: Job{
				Company:        "Greenbone",
				CompanyWebsite: "https://forum.greenbone.net/",
			},
			want: "https://greenbone.net",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := tt.job
			sanitizeExistingJobIdentity(&job)

			if job.CompanyWebsite != tt.want {
				t.Fatalf("CompanyWebsite = %q; want %q", job.CompanyWebsite, tt.want)
			}
		})
	}
}

func TestSanitizeExistingJobIdentityClearsUnsupportedIndustryInference(t *testing.T) {
	job := Job{
		Company:         "Agiloft",
		CompanyWebsite:  "https://www.agiloft.com",
		CompanySummary:  "Agiloft helps organizations manage the end-to-end process of proposing, negotiating, signing, and leveraging contracts.",
		CompanyIndustry: "Healthcare Technology",
		CompanyIdentity: &domain.JobIdentityMetadata{Industry: &JobIdentityEvidence{
			Value:       "Healthcare Technology",
			Source:      "company_about_inference",
			URL:         "https://www.agiloft.com/about-us/",
			Confidence:  "low",
			Provisional: true,
		}},
	}

	sanitizeExistingJobIdentity(&job)

	if job.CompanyWebsite != "https://www.agiloft.com" {
		t.Fatalf("CompanyWebsite = %q; want valid website preserved", job.CompanyWebsite)
	}
	if job.CompanyIndustry != "" {
		t.Fatalf("CompanyIndustry = %q; want unsupported inferred industry cleared", job.CompanyIndustry)
	}
	if job.CompanyIdentity != nil && job.CompanyIdentity.Industry != nil {
		t.Fatalf("CompanyIdentity.Industry = %#v; want cleared", job.CompanyIdentity.Industry)
	}
}

func TestSanitizeExistingJobIdentityClearsBadSummariesAndIndustries(t *testing.T) {
	job := Job{
		Company:         "Veriff",
		CompanyWebsite:  "https://www.veriff.com",
		CompanySummary:  "We're sure there's lots more to know about Veriff, but we don't have all the info at the moment.",
		CompanyIndustry: "before",
		CompanyIdentity: &domain.JobIdentityMetadata{
			Summary: &JobIdentityEvidence{
				Value:  "We're sure there's lots more to know about Veriff, but we don't have all the info at the moment.",
				Source: "company_profile",
				URL:    "https://weworkremotely.com/company/veriff",
			},
			Industry: &JobIdentityEvidence{
				Value:  "before",
				Source: "company_homepage",
				URL:    "https://www.veriff.com/",
			},
		},
	}

	sanitizeExistingJobIdentity(&job)

	if job.CompanySummary != "" {
		t.Fatalf("CompanySummary = %q; want placeholder summary cleared", job.CompanySummary)
	}
	if job.CompanyIndustry != "" {
		t.Fatalf("CompanyIndustry = %q; want invalid industry cleared", job.CompanyIndustry)
	}
}

func TestSanitizeExistingJobIdentityClearsMismatchedBrowserSummary(t *testing.T) {
	job := Job{
		Company:        "Iceberg",
		CompanyWebsite: "https://www.icebergdata.com",
		CompanySummary: "ICEBERG has carved a path as an artist, producer, and engineer with raw emotional storytelling.",
		CompanyIdentity: &domain.JobIdentityMetadata{Summary: &JobIdentityEvidence{
			Value:      "ICEBERG has carved a path as an artist, producer, and engineer with raw emotional storytelling.",
			Source:     "browser_company_search",
			URL:        "https://www.icebergtheartist.com/",
			Confidence: "medium",
		}},
	}

	sanitizeExistingJobIdentity(&job)

	if job.CompanyWebsite != "https://www.icebergdata.com" {
		t.Fatalf("CompanyWebsite = %q; want valid website preserved", job.CompanyWebsite)
	}
	if job.CompanySummary != "" {
		t.Fatalf("CompanySummary = %q; want mismatched browser summary cleared", job.CompanySummary)
	}
	if job.CompanyIdentity != nil && job.CompanyIdentity.Summary != nil {
		t.Fatalf("CompanyIdentity.Summary = %#v; want cleared", job.CompanyIdentity.Summary)
	}
}

func TestSanitizeExistingJobIdentityPrefersFirstPartyApplyHost(t *testing.T) {
	job := Job{
		Company:        "Iceberg",
		ApplyURL:       "https://www.icebergdata.com/careers/",
		CompanyWebsite: "https://www.icebergtheartist.com",
		CompanyIdentity: &domain.JobIdentityMetadata{Website: &JobIdentityEvidence{
			Value:      "https://www.icebergtheartist.com",
			Source:     "browser_company_search",
			Confidence: "medium",
		}},
	}

	sanitizeExistingJobIdentity(&job)

	if job.CompanyWebsite != "https://www.icebergdata.com" {
		t.Fatalf("CompanyWebsite = %q; want first-party apply host", job.CompanyWebsite)
	}
	if job.CompanyIdentity == nil || job.CompanyIdentity.Website == nil || job.CompanyIdentity.Website.Source != "apply_url_host" {
		t.Fatalf("CompanyIdentity = %#v; want apply_url_host evidence", job.CompanyIdentity)
	}
}

func TestInferCompanyWebsiteFromApplyURLUsesFirstPartyHosts(t *testing.T) {
	tests := []struct {
		name string
		job  Job
		want string
	}{
		{
			name: "company careers page",
			job:  Job{Company: "Crunchyroll", ApplyURL: "https://www.crunchyroll.com/careers"},
			want: "https://www.crunchyroll.com",
		},
		{
			name: "jobs subdomain",
			job:  Job{Company: "Revvity", ApplyURL: "https://jobs.revvity.com/job/boston/principal-devops-engineer-ai-ml-remote-us/20539/87312750976"},
			want: "https://revvity.com",
		},
		{
			name: "shared ats excluded",
			job:  Job{Company: "NVIDIA", ApplyURL: "https://nvidia.wd5.myworkdayjobs.com/en-US/NVIDIAExternalCareerSite/job/example"},
			want: "",
		},
		{
			name: "job board excluded",
			job:  Job{Company: "Atria", ApplyURL: "https://www.glassdoor.com/Job/remote-devops-technical-lead-jobs-SRCH.htm"},
			want: "",
		},
		{
			name: "client role excluded",
			job:  Job{Company: "Motion Recruitment (Client Role)", ApplyURL: "https://motionrecruitment.com/tech-jobs/phoenix/direct-hire/example"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inferCompanyWebsiteFromApplyURL(tt.job); got != tt.want {
				t.Fatalf("inferCompanyWebsiteFromApplyURL(%#v) = %q; want %q", tt.job, got, tt.want)
			}
		})
	}
}

func TestEnrichJobFromHTMLPrefersJobPostingAboutSummary(t *testing.T) {
	job := Job{
		Company:        "Circle",
		CompanyWebsite: "https://avatars.githubusercontent.com/u/67208791?s=200&v=4",
		CompanySummary: "Circle is hiring a Senior Back-End Software Engineer, Infra. 100% worldwide remote. Live and work from anywhere in the world.",
	}
	rawHTML := `<meta name="description" content="Circle is hiring a Senior Back-End Software Engineer, Infra. 100% worldwide remote.">
<script type="application/ld+json">{"@context":"https://schema.org/","@type":"JobPosting","description":"<div><h3>About Us</h3><p>Circle is building the world's leading all-in-one platform for online communities and creator businesses.</p></div>","hiringOrganization":{"@type":"Organization","name":"Circle","logo":"https://circle.so/apple-icon.png"}}</script>
<a href="https://careers.circle.so/">Careers</a>`

	enrichJobFromHTML(&job, rawHTML, "https://www.realworkfromanywhere.com/jobs/example")

	if job.CompanyWebsite != "https://circle.so" {
		t.Fatalf("CompanyWebsite = %q; want https://circle.so", job.CompanyWebsite)
	}
	if !strings.Contains(job.CompanySummary, "online communities") {
		t.Fatalf("CompanySummary = %q; want JSON-LD company summary", job.CompanySummary)
	}
}

func TestExtractSourceCompanyProfileURL(t *testing.T) {
	tests := []struct {
		name    string
		pageURL string
		html    string
		want    string
	}{
		{
			name:    "we work remotely",
			pageURL: "https://weworkremotely.com/remote-jobs/engagedmd-staff-software-engineer",
			html:    `<a href="/company/engagedmd">View company</a>`,
			want:    "https://weworkremotely.com/company/engagedmd",
		},
		{
			name:    "real work from anywhere",
			pageURL: "https://www.realworkfromanywhere.com/jobs/senior-back-end-software-engineer-infra-circle-9182",
			html:    `<a href="/companies/circle">View Company</a>`,
			want:    "https://www.realworkfromanywhere.com/companies/circle",
		},
		{
			name:    "built in",
			pageURL: "https://builtin.com/job/staff-software-engineer-local-environments-team/6315940",
			html:    `<a href="/company/gusto">View Gusto Profile</a>`,
			want:    "https://builtin.com/company/gusto",
		},
		{
			name:    "ignore unrelated host",
			pageURL: "https://example.com/jobs/123",
			html:    `<a href="/company/example">View company</a>`,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSourceCompanyProfileURL(tt.html, tt.pageURL)
			if got != tt.want {
				t.Fatalf("extractSourceCompanyProfileURL(%q) = %q; want %q", tt.pageURL, got, tt.want)
			}
		})
	}
}

func TestEnrichJobFromCompanyProfileHTMLRealWorkFromAnywhere(t *testing.T) {
	job := Job{
		Company:         "Circle",
		CompanyWebsite:  "https://circle.so",
		CompanySummary:  "Circle is building the world's leading all-in-one platform for online communities and creator businesses.",
		CompanyIndustry: "Online Communities / SaaS",
	}
	rawHTML := `
<h1>Circle</h1>
<p>Stablecoin issuer and internet financial infrastructure provider.</p>
<p>Industry: Financial Services / Crypto</p>
<a href="https://www.circle.com">Website</a>
<h2>About Circle</h2>
<p>Circle is a global fintech firm focused on creating efficient, open financial infrastructure using stablecoins. They are the issuer of USDC and EURC, facilitating fast cross-border payments, programmable money, and financial products that bridge blockchain with traditional finance.</p>
<h3>Company Culture</h3>`

	enrichJobFromCompanyProfileHTML(&job, rawHTML, "https://www.realworkfromanywhere.com/companies/circle")

	if job.CompanyWebsite != "https://www.circle.com" {
		t.Fatalf("CompanyWebsite = %q; want https://www.circle.com", job.CompanyWebsite)
	}
	if job.CompanyIndustry != "Financial Services / Crypto" {
		t.Fatalf("CompanyIndustry = %q; want Financial Services / Crypto", job.CompanyIndustry)
	}
	if !strings.Contains(job.CompanySummary, "global fintech firm") {
		t.Fatalf("CompanySummary = %q; want company profile summary", job.CompanySummary)
	}
}

func TestEnrichJobFromCompanyProfileHTMLWeWorkRemotely(t *testing.T) {
	job := Job{
		Company:        "EngagedMD",
		CompanySummary: "EngagedMD is an equal opportunity employer.",
	}
	rawHTML := `
<h1>EngagedMD</h1>
<h3>Website</h3><a href="https://engaged-md.com">engaged-md.com</a>
<h3>Industry</h3>Health care
<div>About Culture Benefits Hiring</div>
<p>At EngagedMD, we embrace a mission-driven culture where committed individuals come together to make a real impact in healthcare. Our core values of integrity, collaboration, impact, recognition and growth inform how we work together.</p>
<p>We're sure there's lots more to know about EngagedMD.</p>`

	enrichJobFromCompanyProfileHTML(&job, rawHTML, "https://weworkremotely.com/company/engagedmd")

	if job.CompanyWebsite != "https://engaged-md.com" {
		t.Fatalf("CompanyWebsite = %q; want https://engaged-md.com", job.CompanyWebsite)
	}
	if job.CompanyIndustry != "Health care" {
		t.Fatalf("CompanyIndustry = %q; want Health care", job.CompanyIndustry)
	}
	if !strings.Contains(job.CompanySummary, "mission-driven culture") {
		t.Fatalf("CompanySummary = %q; want company profile summary", job.CompanySummary)
	}
}

func TestEnrichJobFromCompanyProfileHTMLBuiltIn(t *testing.T) {
	job := Job{Company: "Gusto"}
	rawHTML := `
<h1>Gusto</h1>
<a href="https://gusto.com">View Website</a>
<a>Fintech</a> <a>HR Tech</a>
<div>Help us grow the small business economy.</div>
<h2>What We Do</h2>
<p>Gusto is a modern, online small business platform that helps small businesses take care of their teams. On top of full-service payroll, Gusto offers health insurance, 401(k)s, expert HR, and team management tools.</p>
<h2>Why Work With Us</h2>`

	enrichJobFromCompanyProfileHTML(&job, rawHTML, "https://builtin.com/company/gusto")

	if job.CompanyWebsite != "https://gusto.com" {
		t.Fatalf("CompanyWebsite = %q; want https://gusto.com", job.CompanyWebsite)
	}
	if !strings.Contains(job.CompanySummary, "small business platform") {
		t.Fatalf("CompanySummary = %q; want Built In company summary", job.CompanySummary)
	}
}

func TestEnrichJobFromHTMLExtractsStructuredJobPosting(t *testing.T) {
	job := Job{
		Company:      "Unknown",
		Title:        "",
		Compensation: "Not listed",
	}
	rawHTML := `<script type="application/ld+json">{
		"@context":"https://schema.org",
		"@type":"JobPosting",
		"title":"Senior Staff Platform Engineer",
		"description":"<h2>About MongoDB</h2><p>MongoDB empowers innovators to create, transform, and disrupt industries by unleashing the power of software and data.</p>",
		"hiringOrganization":{
			"@type":"Organization",
			"name":"MongoDB",
			"sameAs":"https://www.mongodb.com/company"
		},
		"baseSalary":{
			"@type":"MonetaryAmount",
			"currency":"USD",
			"value":{
				"@type":"QuantitativeValue",
				"minValue":118000,
				"maxValue":231000,
				"unitText":"YEAR"
			}
		},
		"industry":["Database","Cloud"]
	}</script>`

	enrichJobFromHTML(&job, rawHTML, "https://builtin.com/job/senior-staff-platform-engineer/1234567")

	if job.Company != "MongoDB" {
		t.Fatalf("Company = %q; want MongoDB", job.Company)
	}
	if job.Title != "Senior Staff Platform Engineer" {
		t.Fatalf("Title = %q; want Senior Staff Platform Engineer", job.Title)
	}
	if job.CompanyWebsite != "https://www.mongodb.com" {
		t.Fatalf("CompanyWebsite = %q; want https://www.mongodb.com", job.CompanyWebsite)
	}
	if !strings.Contains(job.CompanySummary, "software and data") {
		t.Fatalf("CompanySummary = %q; want structured company summary", job.CompanySummary)
	}
	if job.Compensation != "$118,000 - $231,000 USD/year" {
		t.Fatalf("Compensation = %q; want $118,000 - $231,000 USD/year", job.Compensation)
	}
	if job.CompanyIndustry != "Database" {
		t.Fatalf("CompanyIndustry = %q; want Database", job.CompanyIndustry)
	}
	if job.CompanyIdentity == nil || job.CompanyIdentity.Website == nil || job.CompanyIdentity.Website.Source != "structured_job_posting" {
		t.Fatalf("CompanyIdentity.Website = %#v; want structured_job_posting evidence", job.CompanyIdentity)
	}
}

func TestEnrichJobFromHTMLExtractsBuiltInHowToApplyWebsite(t *testing.T) {
	job := Job{
		Company:      "Unknown",
		Compensation: "Not listed",
	}
	rawHTML := `<script type="application/ld+json">{
		"@context":"https://schema.org",
		"@type":"JobPosting",
		"title":"Staff Site Reliability Engineer, Fabric",
		"description":"<b>About MongoDB</b><p>MongoDB builds a developer data platform for modern applications.</p>",
		"hiringOrganization":{"@type":"Organization","name":"MongoDB","sameAs":"https://builtin.com/company/mongodb"},
		"industry":["Big Data","Cloud","Software","Database"]
	}</script>
	<script>Builtin.jobPostInit({"job":{"id":8994773,"howToApply":"https://www.mongodb.com/careers/job/?gh_jid=7727920\u0026gh_src=abc","companyName":"MongoDB","title":"Staff Site Reliability Engineer, Fabric"}});</script>`

	enrichJobFromHTML(&job, rawHTML, "https://builtin.com/job/staff-site-reliability-engineer-fabric/8994773")

	if job.Company != "MongoDB" {
		t.Fatalf("Company = %q; want MongoDB", job.Company)
	}
	if job.CompanyWebsite != "https://www.mongodb.com" {
		t.Fatalf("CompanyWebsite = %q; want https://www.mongodb.com", job.CompanyWebsite)
	}
	if job.CompanyIndustry != "Big Data" {
		t.Fatalf("CompanyIndustry = %q; want Big Data", job.CompanyIndustry)
	}
	if job.CompanyIdentity == nil || job.CompanyIdentity.Website == nil || job.CompanyIdentity.Website.Source != "builtin_how_to_apply" {
		t.Fatalf("CompanyIdentity.Website = %#v; want builtin_how_to_apply evidence", job.CompanyIdentity)
	}
}

func TestExtractBuiltInJobDetailURLsFromListing(t *testing.T) {
	rawHTML := `<script type="application/ld+json">{
		"@context":"https://schema.org",
		"@type":"ItemList",
		"itemListElement":[
			{"@type":"ListItem","url":"https://builtin.com/job/staff-software-engineer-local-environments-team/6315940"},
			{"@type":"ListItem","item":{"@type":"JobPosting","url":"https://builtin.com/jobs/remote/nyc/staff-devops-engineer/1234567"}}
		]
	}</script>
	<a href="/jobs/remote">Popular Remote Job Searches</a>
	<a href="/job/staff-software-engineer-local-environments-team/6315940?utm_source=feed">Duplicate</a>
	<a href="/jobs/remote/qa-engineer-jobs">Footer search</a>`

	got := extractBuiltInJobDetailURLs(rawHTML, "https://builtin.com/jobs/remote?search=staff")
	want := []string{
		"https://builtin.com/job/staff-software-engineer-local-environments-team/6315940",
		"https://builtin.com/jobs/remote/nyc/staff-devops-engineer/1234567",
	}
	if len(got) != len(want) {
		t.Fatalf("extractBuiltInJobDetailURLs() len = %d; want %d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("extractBuiltInJobDetailURLs()[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

func TestEnrichJobFromHTMLExtractsATSStructuredCompanyWebsite(t *testing.T) {
	tests := []struct {
		name string
		html string
		want string
	}{
		{
			name: "ashby json ld sameAs",
			html: `<script type="application/ld+json">{"@context":"https://schema.org/","@type":"JobPosting","hiringOrganization":{"@type":"Organization","name":"Crusoe","sameAs":"https://crusoe.ai","logo":"https://cdn.ashbyprd.com/logo.png"},"description":"<p>Crusoe is on a mission to accelerate the abundance of energy and intelligence.</p>"}</script>`,
			want: "https://crusoe.ai",
		},
		{
			name: "ashby app data public website",
			html: `<script>window.__appData = {"organization":{"name":"Crusoe","publicWebsite":"https://crusoe.ai"}}</script>`,
			want: "https://crusoe.ai",
		},
		{
			name: "lever home page link",
			html: `<div>At H1, we believe access to the best healthcare information is a basic human right.</div><div class="main-footer-text"><p><a href="https://www.h1.co">H1 Home Page</a></p></div>`,
			want: "https://www.h1.co",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := Job{Company: "Crusoe"}
			if tt.name == "lever home page link" {
				job.Company = "H1"
			}

			enrichJobFromHTML(&job, tt.html, "https://jobs.example.com/posting")

			if job.CompanyWebsite != tt.want {
				t.Fatalf("CompanyWebsite = %q; want %q", job.CompanyWebsite, tt.want)
			}
		})
	}
}

func TestEnrichJobFromHTMLExtractsRemotiveCompanySummary(t *testing.T) {
	job := Job{Company: "easybill GmbH"}
	rawHTML := `<h1>[Hiring] Senior Engineer @easybill GmbH</h1>
<h2>About The Company</h2>
<a> easybill GmbH </a>
<p>Die easybill GmbH ist eine cloudbasierte Rechnungssoftware, die kleinen und mittelständischen Unternehmen hilft, finanzielle Prozesse zu optimieren.</p>
<a>Read more →</a>`

	enrichJobFromHTML(&job, rawHTML, "https://remotive.com/remote-jobs/software-development/example")

	if !strings.Contains(job.CompanySummary, "cloudbasierte Rechnungssoftware") {
		t.Fatalf("CompanySummary = %q; want Remotive company summary", job.CompanySummary)
	}
}

func TestEnrichJobFromDescriptionRejectsJobPitchAsCompanySummary(t *testing.T) {
	job := Job{
		Company:     "EngagedMD",
		Description: `<p>You’re an experienced engineer with exceptional skill and leadership ability, capable of making an impact across our product ecosystem.</p>`,
	}

	enrichJobFromDescription(&job)

	if job.CompanySummary != "" {
		t.Fatalf("CompanySummary = %q; want job pitch rejected", job.CompanySummary)
	}
}

func TestEnrichJobFromHTMLReplacesInvalidCompanySummary(t *testing.T) {
	job := Job{
		Company:        "EngagedMD",
		CompanySummary: "You’re an experienced engineer with exceptional skill and leadership ability, capable of making an impact across our product ecosystem.",
	}
	rawHTML := `<p>EngagedMD is a healthcare technology company that builds software for fertility clinics and patient intake workflows.</p>`

	enrichJobFromHTML(&job, rawHTML, "https://weworkremotely.com/remote-jobs/engagedmd-staff-software-engineer")

	if !strings.Contains(job.CompanySummary, "healthcare technology company") {
		t.Fatalf("CompanySummary = %q; want invalid job pitch replaced", job.CompanySummary)
	}
}

func TestEnrichJobFromApplyPageUsesLLMIdentityWhenAvailable(t *testing.T) {
	applyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
<html>
  <body>
    <h1>Staff Platform Engineer</h1>
    <p>Acme job details for model extraction.</p>
  </body>
</html>`))
	}))
	defer applyServer.Close()

	job := Job{
		Company:  "Acme",
		Title:    "Staff Platform Engineer",
		ApplyURL: applyServer.URL,
	}
	llmEnrich := func(ctx context.Context, job Job, page JobIdentityPage) (*JobIdentityEnrichment, LLMTokenUsage, error) {
		if page.URL != applyServer.URL {
			t.Fatalf("page.URL = %q, want %q", page.URL, applyServer.URL)
		}
		if !strings.Contains(page.Text, "Acme job details") {
			t.Fatalf("page.Text = %q; want apply page text", page.Text)
		}
		return &JobIdentityEnrichment{
			CompanyWebsite:      "https://www.acme.example",
			CompanySummary:      "Acme builds deployment tooling for software engineering teams.",
			CompanyIndustry:     "Developer Tools",
			WebsiteConfidence:   "high",
			SummaryConfidence:   "high",
			IndustryConfidence:  "medium",
			IndustryProvisional: false,
		}, LLMTokenUsage{}, nil
	}

	enrichJobFromApplyPageWithLLM(context.Background(), &job, llmEnrich)

	if job.CompanyWebsite != "https://www.acme.example" {
		t.Fatalf("CompanyWebsite = %q; want https://www.acme.example", job.CompanyWebsite)
	}
	if job.CompanySummary != "Acme builds deployment tooling for software engineering teams." {
		t.Fatalf("CompanySummary = %q; want LLM summary", job.CompanySummary)
	}
	if job.CompanyIndustry != "Developer Tools" {
		t.Fatalf("CompanyIndustry = %q; want Developer Tools", job.CompanyIndustry)
	}
	if job.CompanyIdentity == nil || job.CompanyIdentity.Website == nil || job.CompanyIdentity.Website.Source != "llm_apply_page" {
		t.Fatalf("CompanyIdentity = %#v; want LLM website evidence", job.CompanyIdentity)
	}
}

func TestEnrichJobFromApplyPageSkipsLLMWhenDeterministicIdentityCompletes(t *testing.T) {
	applyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
<h1>Senior Back-End Software Engineer, Infra</h1>
<p>Circle is a global fintech firm focused on creating efficient, open financial infrastructure using stablecoins.</p>
<div>Industry: Financial Services / Crypto</div>
<a href="https://www.circle.com">Website</a>
<section><h2>About the job</h2><div>Salary $130,000 - $140,000 USD</div></section>`))
	}))
	defer applyServer.Close()

	job := Job{
		Company:  "Circle",
		Title:    "Senior Back-End Software Engineer, Infra",
		ApplyURL: applyServer.URL,
	}
	llmCalls := 0
	llmEnrich := func(ctx context.Context, job Job, page JobIdentityPage) (*JobIdentityEnrichment, LLMTokenUsage, error) {
		llmCalls++
		return &JobIdentityEnrichment{
			CompanyWebsite:  "https://llm.example",
			CompanySummary:  "LLM should not be needed for this page.",
			CompanyIndustry: "Artificial Intelligence",
		}, LLMTokenUsage{}, nil
	}

	enrichJobFromApplyPageWithLLM(context.Background(), &job, llmEnrich)

	if llmCalls != 0 {
		t.Fatalf("LLM identity calls = %d; want 0 when deterministic apply-page parsing completes identity", llmCalls)
	}
	if job.CompanyWebsite != "https://www.circle.com" {
		t.Fatalf("CompanyWebsite = %q; want deterministic website", job.CompanyWebsite)
	}
	if job.CompanyIndustry != "Financial Services / Crypto" {
		t.Fatalf("CompanyIndustry = %q; want deterministic industry", job.CompanyIndustry)
	}
}

func TestEnrichJobFromApplyPageSkipsIdentityLLMForCompensationOnlyGap(t *testing.T) {
	applyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><h1>Staff Platform Engineer</h1><p>No salary listed.</p></body></html>`))
	}))
	defer applyServer.Close()

	job := Job{
		Company:         "Acme",
		Title:           "Staff Platform Engineer",
		ApplyURL:        applyServer.URL,
		CompanyWebsite:  "https://www.acme.example",
		CompanySummary:  "Acme builds deployment tooling for software engineering teams.",
		CompanyIndustry: "Developer Tools",
	}
	job.SetCompanyIdentityEvidence("website", JobIdentityEvidence{Value: job.CompanyWebsite, Confidence: "high"})
	job.SetCompanyIdentityEvidence("summary", JobIdentityEvidence{Value: job.CompanySummary, Confidence: "high"})
	job.SetCompanyIdentityEvidence("industry", JobIdentityEvidence{Value: job.CompanyIndustry, Confidence: "high"})
	llmCalls := 0
	llmEnrich := func(ctx context.Context, job Job, page JobIdentityPage) (*JobIdentityEnrichment, LLMTokenUsage, error) {
		llmCalls++
		return nil, LLMTokenUsage{}, nil
	}

	enrichJobFromApplyPageWithLLM(context.Background(), &job, llmEnrich)

	if llmCalls != 0 {
		t.Fatalf("LLM identity calls = %d; want 0 when only compensation is missing", llmCalls)
	}
}

func TestApplyLLMJobIdentityEnrichmentRejectsUnanchoredBadOutput(t *testing.T) {
	job := Job{
		Company:  "Acme",
		Title:    "Staff Platform Engineer",
		ApplyURL: "https://jobs.example/acme/platform",
	}
	llmEnrich := func(ctx context.Context, job Job, page JobIdentityPage) (*JobIdentityEnrichment, LLMTokenUsage, error) {
		return &JobIdentityEnrichment{
			CompanyWebsite:    "https://avatars.githubusercontent.com/u/67208791?s=200&v=4",
			CompanySummary:    "You will build reliable systems and work across product teams.",
			CompanyIndustry:   "Stock Photography",
			WebsiteConfidence: "high",
			SummaryConfidence: "high",
		}, LLMTokenUsage{}, nil
	}

	applyLLMJobIdentityEnrichment(context.Background(), &job, JobIdentityPage{
		URL:  job.ApplyURL,
		Text: "Staff Platform Engineer job description",
	}, llmEnrich, "llm_apply_page")

	if job.CompanyWebsite != "" {
		t.Fatalf("CompanyWebsite = %q; want bad asset URL rejected", job.CompanyWebsite)
	}
	if job.CompanySummary != "" {
		t.Fatalf("CompanySummary = %q; want job pitch rejected", job.CompanySummary)
	}
	if job.CompanyIndustry != "" {
		t.Fatalf("CompanyIndustry = %q; want industry rejected without identity anchor", job.CompanyIndustry)
	}
}

func TestEnrichJobFromCompanyWebsitePagesSkipsLLMWhenHomepageCompletesIdentity(t *testing.T) {
	companyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
<html><body>
  <p>Acme builds deployment tooling for software engineering teams and release automation.</p>
  <div>Industry: Developer Tools</div>
</body></html>`))
	}))
	defer companyServer.Close()

	job := Job{
		Company:        "Acme",
		CompanyWebsite: companyServer.URL,
		Title:          "Staff Platform Engineer",
	}
	llmCalls := 0
	llmEnrich := func(ctx context.Context, job Job, page JobIdentityPage) (*JobIdentityEnrichment, LLMTokenUsage, error) {
		llmCalls++
		return &JobIdentityEnrichment{CompanySummary: "LLM should not be needed."}, LLMTokenUsage{}, nil
	}

	enrichJobFromCompanyWebsitePages(context.Background(), &job, llmEnrich)

	if llmCalls != 0 {
		t.Fatalf("LLM identity calls = %d; want 0 when deterministic company homepage parsing completes identity", llmCalls)
	}
	if job.CompanyIndustry != "Developer Tools" {
		t.Fatalf("CompanyIndustry = %q; want deterministic industry", job.CompanyIndustry)
	}
}

func TestEnrichJobFromCompanyWebsitePagesUsesAboutPage(t *testing.T) {
	companyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><body><a href="/about">About Acme</a></body></html>`))
		case "/about":
			_, _ = w.Write([]byte(`
<html><body>
  <p>Acme builds deployment tooling for software engineering teams and release automation.</p>
  <div>Industry: Developer Tools</div>
</body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer companyServer.Close()

	job := Job{
		Company:        "Acme",
		CompanyWebsite: companyServer.URL,
		Title:          "Staff Platform Engineer",
	}

	enrichJobFromCompanyWebsitePages(context.Background(), &job, nil)

	if !strings.Contains(job.CompanySummary, "deployment tooling") {
		t.Fatalf("CompanySummary = %q; want about-page summary", job.CompanySummary)
	}
	if job.CompanyIndustry != "Developer Tools" {
		t.Fatalf("CompanyIndustry = %q; want Developer Tools", job.CompanyIndustry)
	}
	if job.CompanyIdentity == nil || job.CompanyIdentity.Industry == nil || job.CompanyIdentity.Industry.Source != "company_about" {
		t.Fatalf("CompanyIdentity = %#v; want about-page industry evidence", job.CompanyIdentity)
	}
}

func TestEnrichJobFromCompanyWebsitePagesSuppliesAboutPageToLLM(t *testing.T) {
	companyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(`<html><body><a href="/about">About Acme</a></body></html>`))
		case "/about":
			_, _ = w.Write([]byte(`<html><body><main>Acme company overview for model extraction.</main></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer companyServer.Close()

	job := Job{
		Company:        "Acme",
		CompanyWebsite: companyServer.URL,
		Title:          "Staff Platform Engineer",
	}
	seenAbout := false
	llmEnrich := func(ctx context.Context, job Job, page JobIdentityPage) (*JobIdentityEnrichment, LLMTokenUsage, error) {
		if !strings.Contains(page.URL, "/about") {
			return nil, LLMTokenUsage{}, nil
		}
		seenAbout = true
		return &JobIdentityEnrichment{
			CompanySummary:     "Acme builds deployment tooling for software engineering teams.",
			CompanyIndustry:    "Developer Tools",
			SummaryConfidence:  "high",
			IndustryConfidence: "high",
		}, LLMTokenUsage{}, nil
	}

	enrichJobFromCompanyWebsitePages(context.Background(), &job, llmEnrich)

	if !seenAbout {
		t.Fatalf("LLM enrichment was not called with the about page")
	}
	if job.CompanySummary != "Acme builds deployment tooling for software engineering teams." {
		t.Fatalf("CompanySummary = %q; want LLM about-page summary", job.CompanySummary)
	}
	if job.CompanyIdentity == nil || job.CompanyIdentity.Summary == nil || job.CompanyIdentity.Summary.Source != "llm_company_about" {
		t.Fatalf("CompanyIdentity = %#v; want LLM about-page summary evidence", job.CompanyIdentity)
	}
}

func TestApplyBrowserCompanySiteProfileEnrichesMissingIdentity(t *testing.T) {
	job := Job{Company: "Acme", Title: "Staff Platform Engineer"}
	profile := &domain.CompanySiteProfile{
		SearchURL:   "https://duckduckgo.com/html/?q=Acme+official+site",
		WebsiteURL:  "https://acme.example",
		WebsiteText: "Acme builds deployment tooling for software engineering teams and release automation.",
		AboutURL:    "https://acme.example/about",
		AboutText:   "Industry: Developer Tools",
		SearchQuery: `"Acme" official site`,
	}

	applyBrowserCompanySiteProfile(context.Background(), &job, profile, nil)

	if job.CompanyWebsite != "https://acme.example" {
		t.Fatalf("CompanyWebsite = %q; want https://acme.example", job.CompanyWebsite)
	}
	if !strings.Contains(job.CompanySummary, "deployment tooling") {
		t.Fatalf("CompanySummary = %q; want browser company-site summary", job.CompanySummary)
	}
	if job.CompanyIndustry != "Developer Tools" {
		t.Fatalf("CompanyIndustry = %q; want Developer Tools", job.CompanyIndustry)
	}
	if job.CompanyIdentity == nil || job.CompanyIdentity.Website == nil || job.CompanyIdentity.Website.Source != "browser_company_search" {
		t.Fatalf("CompanyIdentity = %#v; want browser company search website evidence", job.CompanyIdentity)
	}
}

func TestEnrichJobsFromApplyPagesDoesNotUseBrowserCompanySearchFallback(t *testing.T) {
	job := Job{
		Company:  "Acme",
		Title:    "Staff Platform Engineer",
		ApplyURL: "https://www.indeed.com/viewjob?jk=abc123",
		Source:   "Site Search: https://www.indeed.com/jobs",
	}

	enriched := enrichJobsFromApplyPagesWithLLMAndProgress(context.Background(), []Job{job}, nil, nil, nil)

	if len(enriched) != 1 {
		t.Fatalf("enrichJobsFromApplyPagesWithLLMAndProgress(...) len = %d; want 1", len(enriched))
	}
	if enriched[0].CompanyWebsite != "" || enriched[0].CompanySummary != "" || enriched[0].CompanyIndustry != "" {
		t.Fatalf("enriched job identity = website %q summary %q industry %q; want browser fallback not to fill fields", enriched[0].CompanyWebsite, enriched[0].CompanySummary, enriched[0].CompanyIndustry)
	}
}

func TestBrowserCompanySearchTargetsDedupesCompanies(t *testing.T) {
	jobs := []Job{
		{Company: "Acme Inc.", Title: "Platform Engineer"},
		{Company: "Acme", Title: "SRE"},
		{Company: "OtherCo", Title: "SRE", CompanyWebsite: "https://otherco.example"},
		{Company: "Beta Group", Title: "DevOps"},
	}

	targets := browserCompanySearchTargets(jobs)

	if len(targets) != 2 {
		t.Fatalf("browserCompanySearchTargets(%#v) len = %d; want 2", jobs, len(targets))
	}
	if targets[0].Key != "acme" || len(targets[0].Indexes) != 2 {
		t.Fatalf("browserCompanySearchTargets(%#v)[0] = %#v; want Acme target with two indexes", jobs, targets[0])
	}
	if targets[1].Key != "beta" || len(targets[1].Indexes) != 1 {
		t.Fatalf("browserCompanySearchTargets(%#v)[1] = %#v; want Beta target with one index", jobs, targets[1])
	}
}

func TestFetchAllJobsCombinesLLMAndRSSSources(t *testing.T) {
	applyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer applyServer.Close()

	prevInit := fetchAllJobsInitConfiguredLLM
	prevSearch := fetchAllJobsExecuteLLMSearch
	prevIdentity := fetchAllJobsEnrichJobIdentity
	prevReadFile := fetchAllJobsReadFile
	fetchAllJobsInitConfiguredLLM = func(ctx context.Context, appCfg *AppConfig, task string) (llms.Model, func(), error) {
		if task != llmTaskJobSearch && task != llmTaskJobIdentity {
			t.Fatalf("fetchAllJobsInitConfiguredLLM task = %q, want %q or %q", task, llmTaskJobSearch, llmTaskJobIdentity)
		}
		return fakeLLMModel{}, func() {}, nil
	}
	fetchAllJobsExecuteLLMSearch = func(ctx context.Context, llm llms.Model, prompt string) ([]Job, error) {
		return []Job{
			{Company: "LLMCo", Title: "LLM Role", ApplyURL: applyServer.URL},
		}, nil
	}
	fetchAllJobsEnrichJobIdentity = func(ctx context.Context, llm llms.Model, job Job, page JobIdentityPage) (*JobIdentityEnrichment, LLMTokenUsage, error) {
		return nil, LLMTokenUsage{}, nil
	}
	fetchAllJobsReadFile = func(path string) ([]byte, error) {
		return []byte("prompt"), nil
	}
	t.Cleanup(func() {
		fetchAllJobsInitConfiguredLLM = prevInit
		fetchAllJobsExecuteLLMSearch = prevSearch
		fetchAllJobsEnrichJobIdentity = prevIdentity
		fetchAllJobsReadFile = prevReadFile
	})

	rssServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <item>
      <title>RSS Role at RSSCo</title>
      <link>` + applyServer.URL + `</link>
      <description>Remote platform role</description>
    </item>
  </channel>
</rss>`))
	}))
	defer rssServer.Close()

	tmpDir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", tmpDir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
	if err := os.WriteFile(filepath.Join(tmpDir, "SEARCH_PROMPT.md"), []byte("prompt"), 0o644); err != nil {
		t.Fatalf("WriteFile(SEARCH_PROMPT.md) error = %v", err)
	}

	appCfg := defaultAppConfig()
	appCfg.Sources.Enabled = true
	appCfg.Sources.BuiltinsEnabled = false
	appCfg.Sources.RSS.Enabled = true
	appCfg.Sources.RSS.Feeds = []RSSSource{{Name: "RSSCo", URL: rssServer.URL}}
	appCfg.Sources.APIs = nil
	appCfg.Sources.SiteSearch.Enabled = false
	appCfg.Sources.SiteSearch.Sites = nil
	appCfg.Sources.LLMWeb.Enabled = false
	appCfg.LLM.JobSearch = true
	appCfg.LLM.JobFiltering = false

	jobs, summary, err := fetchAllJobs(context.Background(), &appCfg, nil, nil)
	if err != nil {
		t.Fatalf("fetchAllJobs() error = %v", err)
	}
	if len(summary.Notices) != 0 {
		t.Fatalf("fetchAllJobs() notices = %#v; want none", summary.Notices)
	}
	if len(jobs) != 2 {
		t.Fatalf("fetchAllJobs() len = %d; want 2", len(jobs))
	}
	if jobs[0].Company != "LLMCo" {
		t.Fatalf("jobs[0].Company = %q; want LLMCo", jobs[0].Company)
	}
	if jobs[1].Company != "Unknown" {
		t.Fatalf("jobs[1].Company = %q; want Unknown", jobs[1].Company)
	}
}
