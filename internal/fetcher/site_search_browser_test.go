package fetcher

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type fakeBrowserCloser struct {
	closed bool
	err    error
}

func (f *fakeBrowserCloser) Close() error {
	f.closed = true
	return f.err
}

type fakeLauncherCleanup struct {
	killed  bool
	cleaned bool
}

func (f *fakeLauncherCleanup) Kill() {
	f.killed = true
}

func (f *fakeLauncherCleanup) Cleanup() {
	f.cleaned = true
}

func TestCleanupLaunchedSiteSearchBrowserClosesAndKillsBrowserProcess(t *testing.T) {
	browser := &fakeBrowserCloser{err: errors.New("close failed")}
	launch := &fakeLauncherCleanup{}

	cleanupLaunchedSiteSearchBrowser(browser, launch)

	if !browser.closed {
		t.Fatal("browser closed = false; want close attempted")
	}
	if !launch.killed {
		t.Fatal("launcher killed = false; want browser process killed")
	}
	if !launch.cleaned {
		t.Fatal("launcher cleaned = false; want browser resources cleaned")
	}
}

func TestFindSiteSearchBrowserBinaryUsesRodLauncherPath(t *testing.T) {
	t.Setenv("ROD_BROWSER_BIN", "")
	previous := siteSearchBrowserLookPath
	siteSearchBrowserLookPath = func() (string, bool) {
		return "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", true
	}
	t.Cleanup(func() {
		siteSearchBrowserLookPath = previous
	})

	got := findSiteSearchBrowserBinary()
	want := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	if got != want {
		t.Fatalf("findSiteSearchBrowserBinary() = %q; want %q", got, want)
	}
}

func TestFindSiteSearchBrowserBinaryPrefersEnvironmentOverride(t *testing.T) {
	t.Setenv("ROD_BROWSER_BIN", "/custom/chrome")
	previous := siteSearchBrowserLookPath
	siteSearchBrowserLookPath = func() (string, bool) {
		return "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", true
	}
	t.Cleanup(func() {
		siteSearchBrowserLookPath = previous
	})

	if got := findSiteSearchBrowserBinary(); got != "/custom/chrome" {
		t.Fatalf("findSiteSearchBrowserBinary() = %q; want environment override", got)
	}
}

func TestSiteSearchURLSkipsBareSharedATSHosts(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   string
	}{
		{name: "workday root", target: "myworkdayjobs.com", want: ""},
		{name: "greenhouse root", target: "boards.greenhouse.io", want: ""},
		{name: "built in bare host", target: "www.builtinaustin.com", want: "https://www.builtinaustin.com/jobs"},
		{name: "built in explicit path", target: "https://builtin.com/jobs/remote", want: "https://builtin.com/jobs/remote"},
		{name: "indeed host", target: "indeed.com", want: "https://www.indeed.com/jobs"},
		{name: "linkedin host", target: "linkedin.com", want: "https://www.linkedin.com/jobs/search"},
		{name: "web search target", target: "site:job-boards.greenhouse.io", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := siteSearchURL(tt.target); got != tt.want {
				t.Fatalf("siteSearchURL(%q) = %q; want %q", tt.target, got, tt.want)
			}
		})
	}
}

func TestSiteSearchURLsForCriteriaSkipsWebSearchTargets(t *testing.T) {
	criteria := &CriteriaConfig{}
	criteria.Filters.TitleRequires = []string{"Staff"}
	criteria.Filters.TitleIncludes = []string{"DevOps Engineer"}

	got := siteSearchURLsForCriteria("site:job-boards.greenhouse.io", criteria)
	if len(got) != 0 {
		t.Fatalf("siteSearchURLsForCriteria() = %#v; want none", got)
	}
}

func TestSiteSearchURLForCriteriaBuildsBuiltInSearchURL(t *testing.T) {
	criteria := &CriteriaConfig{}
	criteria.Candidate.CountryCode = "US"
	criteria.Filters.TitleRequires = []string{"Staff"}
	criteria.Filters.TitleIncludes = []string{"DevOps Engineer"}
	criteria.Filters.WorkSettings.Remote = true

	got := siteSearchURLForCriteria("https://builtin.com/jobs/remote", criteria)
	want := "https://builtin.com/jobs/remote?allLocations=true&country=USA&search=Staff+DevOps+Engineer"
	if got != want {
		t.Fatalf("siteSearchURLForCriteria() = %q; want %q", got, want)
	}
}

func TestSiteSearchURLForCriteriaKeepsBuiltInRegionalSearchRegional(t *testing.T) {
	criteria := &CriteriaConfig{}
	criteria.Candidate.CountryCode = "US"
	criteria.Filters.TitleRequires = []string{"Staff"}
	criteria.Filters.TitleIncludes = []string{"DevOps Engineer"}
	criteria.Filters.WorkSettings.Remote = true

	got := siteSearchURLForCriteria("https://www.builtinseattle.com", criteria)
	want := "https://www.builtinseattle.com/jobs?country=USA&search=Staff+DevOps+Engineer"
	if got != want {
		t.Fatalf("siteSearchURLForCriteria() = %q; want %q", got, want)
	}
}

func TestSiteSearchURLForCriteriaBuildsAggregatorSearchURLs(t *testing.T) {
	restoreCache := replaceLinkedInTypeaheadCacheForTest(t)
	defer restoreCache()
	if err := saveLinkedInTypeaheadCacheEntry("geo", "Seattle, WA", linkedInTypeaheadHit{
		Type:        "GEO",
		ID:          "104116203",
		DisplayName: "Seattle, Washington, United States",
	}); err != nil {
		t.Fatalf("saveLinkedInTypeaheadCacheEntry() error = %v", err)
	}

	criteria := &CriteriaConfig{}
	criteria.Candidate.City = "Seattle"
	criteria.Candidate.State = "WA"
	criteria.Candidate.CountryCode = "US"
	criteria.Filters.TitleRequires = []string{"Senior"}
	criteria.Filters.TitleIncludes = []string{"Software Engineer"}
	criteria.Filters.WorkSettings.Remote = true
	criteria.Filters.WorkSettings.Hybrid = true
	criteria.Candidate.YearsOfExperience = 5
	criteria.Filters.MinBaseUSD = 120000

	tests := []struct {
		name   string
		target string
		want   string
	}{
		{
			name:   "indeed",
			target: "https://www.indeed.com/jobs",
			want:   "https://www.indeed.com/jobs?l=Seattle%2C+WA&q=Senior+Software+Engineer",
		},
		{
			name:   "linkedin",
			target: "https://www.linkedin.com/jobs/search",
			want:   "https://www.linkedin.com/jobs/search?f_E=3%2C4&f_PP=104116203&f_SB2=5&f_WT=2%2C3&keywords=Senior+Software+Engineer&location=Seattle%2C+WA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := siteSearchURLForCriteria(tt.target, criteria); got != tt.want {
				t.Fatalf("siteSearchURLForCriteria(%q) = %q; want %q", tt.target, got, tt.want)
			}
		})
	}
}

func TestSiteSearchURLsForCriteriaBuildsTargetedTitleMatrix(t *testing.T) {
	criteria := &CriteriaConfig{}
	criteria.Candidate.City = "Seattle"
	criteria.Candidate.State = "WA"
	criteria.Filters.TitleRequires = []string{"Senior", "Staff"}
	criteria.Filters.TitleIncludes = []string{"Software Engineer", "Platform Engineer"}
	criteria.Filters.WorkSettings.Remote = true

	got := siteSearchURLsForCriteria("https://www.linkedin.com/jobs/search", criteria)
	want := []string{
		"https://www.linkedin.com/jobs/search?f_WT=2&keywords=Senior+Software+Engineer&location=Seattle%2C+WA",
		"https://www.linkedin.com/jobs/search?f_WT=2&keywords=Senior+Platform+Engineer&location=Seattle%2C+WA",
		"https://www.linkedin.com/jobs/search?f_WT=2&keywords=Staff+Software+Engineer&location=Seattle%2C+WA",
		"https://www.linkedin.com/jobs/search?f_WT=2&keywords=Staff+Platform+Engineer&location=Seattle%2C+WA",
	}
	if len(got) != len(want) {
		t.Fatalf("siteSearchURLsForCriteria() len = %d; want %d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("siteSearchURLsForCriteria()[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

func TestSiteSearchURLsForCriteriaAvoidsDuplicatingTitlePrefix(t *testing.T) {
	criteria := &CriteriaConfig{}
	criteria.Filters.TitleRequires = []string{"Senior"}
	criteria.Filters.TitleIncludes = []string{"Sr. Software Eng"}

	got := siteSearchURLsForCriteria("https://www.indeed.com/jobs", criteria)
	want := []string{
		"https://www.indeed.com/jobs?q=Senior+Software+Engineer",
	}
	if len(got) != len(want) {
		t.Fatalf("siteSearchURLsForCriteria() len = %d; want %d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("siteSearchURLsForCriteria()[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

func TestLinkedInGeoIDRequiresMatchingLocationQuery(t *testing.T) {
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
	criteria.Candidate.City = "Seattle"
	criteria.Candidate.State = "WA"

	if got := linkedInGeoID(criteria); got != "" {
		t.Fatalf("linkedInGeoID() = %q; want empty for stale location hint", got)
	}
}

func TestSiteSearchURLsForCriteriaUsesCachedLinkedInTitleSuggestion(t *testing.T) {
	restoreCache := replaceLinkedInTypeaheadCacheForTest(t)
	defer restoreCache()
	if err := saveLinkedInTypeaheadCacheEntry("title", "Senior Frontend Engineer", linkedInTypeaheadHit{
		Type:        "TITLE",
		ID:          "17265",
		DisplayName: "Senior Frontend Developer",
	}); err != nil {
		t.Fatalf("saveLinkedInTypeaheadCacheEntry() error = %v", err)
	}

	criteria := &CriteriaConfig{}
	criteria.Filters.TitleRequires = []string{"Sr."}
	criteria.Filters.TitleIncludes = []string{"Frontend"}
	criteria.RoleFamilies = []RoleFamilyID{RoleFrontendEngineering}

	got := siteSearchURLsForCriteria("https://www.linkedin.com/jobs/search", criteria)
	want := []string{
		"https://www.linkedin.com/jobs/search?keywords=Senior+Frontend+Developer",
	}
	if len(got) != len(want) {
		t.Fatalf("siteSearchURLsForCriteria() len = %d; want %d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("siteSearchURLsForCriteria()[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

func TestLinkedInLocationQueryUsesCandidateAreaForRemoteOnlyCriteria(t *testing.T) {
	criteria := &CriteriaConfig{}
	criteria.Candidate.City = "Austin"
	criteria.Candidate.State = "TX"
	criteria.Filters.WorkSettings.Remote = true

	if got := linkedInLocationQuery(criteria); got != "Austin, TX" {
		t.Fatalf("linkedInLocationQuery() = %q; want Austin, TX", got)
	}
	if got := siteSearchLocation(criteria); got != "Remote" {
		t.Fatalf("siteSearchLocation() = %q; want generic remote location unchanged", got)
	}
}

func TestLinkedInCriteriaMappings(t *testing.T) {
	remoteEntry := CriteriaConfig{}
	remoteEntry.Candidate.YearsOfExperience = 2
	remoteEntry.Filters.WorkSettings.Remote = true
	remoteEntry.Filters.MinBaseUSD = 60000

	allWorkplacesSenior := CriteriaConfig{}
	allWorkplacesSenior.Candidate.YearsOfExperience = 9
	allWorkplacesSenior.Filters.WorkSettings.Remote = true
	allWorkplacesSenior.Filters.WorkSettings.Hybrid = true
	allWorkplacesSenior.Filters.WorkSettings.Onsite = true
	allWorkplacesSenior.Filters.MinBaseUSD = 120000

	tests := []struct {
		name           string
		criteria       CriteriaConfig
		wantWorkplace  string
		wantExperience string
		wantSalary     string
	}{
		{
			name:           "remote only entry associate lower salary",
			criteria:       remoteEntry,
			wantWorkplace:  "2",
			wantExperience: "2,3",
			wantSalary:     "2",
		},
		{
			name:           "all workplace senior high salary",
			criteria:       allWorkplacesSenior,
			wantWorkplace:  "1,2,3",
			wantExperience: "4",
			wantSalary:     "5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := linkedInWorkplaceTypes(&tt.criteria); got != tt.wantWorkplace {
				t.Fatalf("linkedInWorkplaceTypes() = %q; want %q", got, tt.wantWorkplace)
			}
			if got := linkedInExperienceLevels(&tt.criteria); got != tt.wantExperience {
				t.Fatalf("linkedInExperienceLevels() = %q; want %q", got, tt.wantExperience)
			}
			if got := linkedInSalaryBucket(&tt.criteria); got != tt.wantSalary {
				t.Fatalf("linkedInSalaryBucket() = %q; want %q", got, tt.wantSalary)
			}
		})
	}
}

func TestScoreSiteSearchCandidateRecognizesAggregatorJobURLs(t *testing.T) {
	criteria := &CriteriaConfig{}
	criteria.Filters.TitleIncludes = []string{"engineer"}

	tests := []struct {
		name  string
		title string
		raw   string
	}{
		{name: "indeed", title: "Software Engineer", raw: "https://www.indeed.com/viewjob?jk=abc123"},
		{name: "linkedin", title: "Software Engineer", raw: "https://www.linkedin.com/jobs/view/software-engineer-123"},
		{name: "yc", title: "Backend Engineer", raw: "https://www.ycombinator.com/companies/acme/jobs/backend-engineer"},
		{name: "remoteok", title: "Backend Engineer", raw: "https://remoteok.com/remote-jobs/123-backend-engineer-acme"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scoreSiteSearchCandidate(tt.title, tt.raw, criteria); got <= 0 {
				t.Fatalf("scoreSiteSearchCandidate(%q, %q) = %d; want positive score", tt.title, tt.raw, got)
			}
		})
	}
}

func TestIsSiteSearchDirectJobCandidateRejectsPageChromeAndCategoryLinks(t *testing.T) {
	tests := []struct {
		name     string
		baseHost string
		title    string
		raw      string
		want     bool
	}{
		{
			name:     "linkedin job detail",
			baseHost: "www.linkedin.com",
			title:    "Software Developer",
			raw:      "https://www.linkedin.com/jobs/view/software-developer-at-omni-cleanair-4373195639",
			want:     true,
		},
		{
			name:     "linkedin login link",
			baseHost: "www.linkedin.com",
			title:    "Sign in",
			raw:      "https://www.linkedin.com/login?fromSignIn=true",
			want:     false,
		},
		{
			name:     "google chrome link",
			baseHost: "www.google.com",
			title:    "Images",
			raw:      "https://www.google.com/search?tbm=isch&q=software+engineer",
			want:     false,
		},
		{
			name:     "indeed job detail",
			baseHost: "www.indeed.com",
			title:    "Software Engineer - Frontend",
			raw:      "https://www.indeed.com/viewjob?jk=41d4506fd9d52e5b",
			want:     true,
		},
		{
			name:     "indeed salary page",
			baseHost: "www.indeed.com",
			title:    "Software Engineer salaries in Seattle, WA",
			raw:      "https://www.indeed.com/career/software-engineer/salaries/Seattle--WA?fromjk=41d4506fd9d52e5b",
			want:     false,
		},
		{
			name:     "indeed query page",
			baseHost: "www.indeed.com",
			title:    "software engineer",
			raw:      "https://www.indeed.com/q-software-engineer-l-seattle,-wa-jobs.html",
			want:     false,
		},
		{
			name:     "yc job detail",
			baseHost: "www.ycombinator.com",
			title:    "Full Stack Software Engineer",
			raw:      "https://www.ycombinator.com/companies/truss/jobs/ghFZ9yT-full-stack-software-engineer",
			want:     true,
		},
		{
			name:     "yc role directory",
			baseHost: "www.ycombinator.com",
			title:    "Software Engineer Jobs",
			raw:      "https://www.ycombinator.com/jobs/role/software-engineer",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.raw)
			if err != nil {
				t.Fatal(err)
			}
			if got := isSiteSearchDirectJobCandidate(tt.baseHost, tt.title, u); got != tt.want {
				t.Fatalf("isSiteSearchDirectJobCandidate(%q, %q) = %t; want %t", tt.title, tt.raw, got, tt.want)
			}
		})
	}
}

func TestUnwrapGoogleSearchResultURL(t *testing.T) {
	raw := "https://www.google.com/url?q=https%3A%2F%2Fjob-boards.greenhouse.io%2Fmuckrack%2Fjobs%2F8523017002&sa=U"
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	got := unwrapGoogleSearchResultURL(u)
	if got.String() != "https://job-boards.greenhouse.io/muckrack/jobs/8523017002" {
		t.Fatalf("unwrapGoogleSearchResultURL() = %q", got.String())
	}
}

func TestInferCompanyFromSiteSearchURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "linkedin slug",
			raw:  "https://www.linkedin.com/jobs/view/software-developer-at-omni-cleanair-4373195639",
			want: "Omni Cleanair",
		},
		{
			name: "yc company slug",
			raw:  "https://www.ycombinator.com/companies/just-appraised/jobs/KmLmKMP-senior-software-engineer",
			want: "Just Appraised",
		},
		{
			name: "himalayas company slug",
			raw:  "https://himalayas.app/companies/acme-inc/jobs/software-engineer",
			want: "Acme Inc",
		},
		{
			name: "greenhouse board slug",
			raw:  "https://job-boards.greenhouse.io/muckrack/jobs/8523017002",
			want: "Muckrack",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inferCompanyFromSiteSearchURL(tt.raw); got != tt.want {
				t.Fatalf("inferCompanyFromSiteSearchURL(%q) = %q; want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestInferCompanyFromCandidateContext(t *testing.T) {
	tests := []struct {
		name string
		host string
		text string
		want string
	}{
		{
			name: "indeed job card",
			host: "www.indeed.com",
			text: "Software Engineer - Frontend\nSalesforce\nSeattle, WA\nFull-time",
			want: "Salesforce",
		},
		{
			name: "indeed skips easily apply badge",
			host: "www.indeed.com",
			text: "Software Engineer\nEasily apply\nAllied Telesis\nEverett, WA\n$100,000 - $140,000 a year",
			want: "Allied Telesis",
		},
		{
			name: "indeed skips new badge",
			host: "www.indeed.com",
			text: "Principal Software Engineer\nNew\nWhitepages, Inc.\nHybrid work in Seattle, WA 98121",
			want: "Whitepages, Inc.",
		},
		{
			name: "indeed skips response badge",
			host: "www.indeed.com",
			text: "Software Engineer - Hybrid/Seattle Metro, WA\nOften responds within 1 day\nBrook Inc\nHybrid work in Seattle, WA 98104",
			want: "Brook Inc",
		},
		{
			name: "indeed title variant still scans card lines",
			host: "www.indeed.com",
			text: "Senior Cloud Engineer\nPremier Inc.\n3.7\nRemote\n$90,000 - $150,000 a year",
			want: "Premier Inc.",
		},
		{
			name: "indeed skips generic company logo alt before company line",
			host: "www.indeed.com",
			text: "Senior Cloud Engineer\ncompany logo\nPremier Inc.\nRemote\n$90,000 - $150,000 a year",
			want: "Premier Inc.",
		},
		{
			name: "indeed can use specific logo alt",
			host: "www.indeed.com",
			text: "Senior Cloud Engineer\nAcme Cloud logo\nRemote\n$90,000 - $150,000 a year",
			want: "Acme Cloud",
		},
		{
			name: "indeed skips remote location before logo alt",
			host: "www.indeed.com",
			text: "Senior Cloud Engineer\nRemote in Fort Belvoir, VA 22060\nAcme Cloud logo",
			want: "Acme Cloud",
		},
		{
			name: "linkedin job card",
			host: "www.linkedin.com",
			text: "Software Developer\nOmni CleanAir\nMonroe, WA\n2 days ago",
			want: "Omni CleanAir",
		},
		{
			name: "reject location after title",
			host: "www.indeed.com",
			text: "Software Engineer\nSeattle, WA\nPosted today",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title := strings.Split(tt.text, "\n")[0]
			if tt.name == "indeed title variant still scans card lines" {
				title = "Senior Cloud Engineer - Remote"
			}
			if got := inferCompanyFromCandidateContext(tt.host, title, tt.text); got != tt.want {
				t.Fatalf("inferCompanyFromCandidateContext() = %q; want %q", got, tt.want)
			}
		})
	}
}

func TestSiteSearchCompanyMissingOrInvalid(t *testing.T) {
	tests := []struct {
		company string
		want    bool
	}{
		{company: "", want: true},
		{company: "Unknown", want: true},
		{company: "Easily apply", want: true},
		{company: "New", want: true},
		{company: "Often responds within 1 day", want: true},
		{company: "Multiple openings", want: true},
		{company: "Allied Telesis", want: false},
		{company: "New Relic", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.company, func(t *testing.T) {
			if got := siteSearchCompanyMissingOrInvalid(tt.company); got != tt.want {
				t.Fatalf("siteSearchCompanyMissingOrInvalid(%q) = %t; want %t", tt.company, got, tt.want)
			}
		})
	}
}

func TestSiteSearchCandidateMissingRequiredIdentity(t *testing.T) {
	tests := []struct {
		name     string
		baseHost string
		company  string
		rawURL   string
		want     bool
	}{
		{
			name:     "indeed rejects missing company at probe",
			baseHost: "www.indeed.com",
			company:  "",
			rawURL:   "https://www.indeed.com/viewjob?jk=41d4506fd9d52e5b",
			want:     true,
		},
		{
			name:     "indeed rejects listing badge as company at probe",
			baseHost: "www.indeed.com",
			company:  "Easily apply",
			rawURL:   "https://www.indeed.com/pagead/clk?jk=41d4506fd9d52e5b",
			want:     true,
		},
		{
			name:     "indeed accepts real company from card context",
			baseHost: "www.indeed.com",
			company:  "Allied Telesis",
			rawURL:   "https://www.indeed.com/pagead/clk?jk=41d4506fd9d52e5b",
			want:     false,
		},
		{
			name:     "linkedin accepts company inferred from URL",
			baseHost: "www.linkedin.com",
			company:  "",
			rawURL:   "https://www.linkedin.com/jobs/view/software-developer-at-omni-cleanair-4373195639",
			want:     false,
		},
		{
			name:     "generic sites may be repaired after probe",
			baseHost: "example.com",
			company:  "",
			rawURL:   "https://example.com/jobs/software-engineer",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := siteSearchCandidateMissingRequiredIdentity(tt.baseHost, tt.company, tt.rawURL)
			if got != tt.want {
				t.Fatalf("siteSearchCandidateMissingRequiredIdentity(%q, %q, %q) = %t; want %t", tt.baseHost, tt.company, tt.rawURL, got, tt.want)
			}
		})
	}
}

func TestEnrichSiteSearchJobIdentityBeforeFilterUsesDeterministicHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><head><script type="application/ld+json">{
			"@context":"https://schema.org",
			"@type":"JobPosting",
			"title":"Software Engineer",
			"hiringOrganization":{
				"@type":"Organization",
				"name":"Acme Tools",
				"sameAs":"https://www.acmetools.example"
			},
			"description":"<p>About Acme Tools</p><p>Acme Tools builds developer tools for software teams.</p>"
		}</script></head><body></body></html>`))
	}))
	defer server.Close()

	job := Job{
		Company:      "Unknown",
		Title:        "Software Engineer",
		ApplyURL:     server.URL + "/jobs/software-engineer",
		Source:       "Site Search",
		Compensation: "Not listed",
	}

	enrichSiteSearchJobIdentityBeforeFilter(t.Context(), &job)

	if job.Company != "Acme Tools" {
		t.Fatalf("Company = %q; want Acme Tools", job.Company)
	}
	if job.CompanyWebsite != "https://www.acmetools.example" {
		t.Fatalf("CompanyWebsite = %q; want https://www.acmetools.example", job.CompanyWebsite)
	}
}

func TestEnrichSiteSearchJobIdentityBeforeFilterFetchesWeakDetailRows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.UserAgent(), "Mozilla/5.0") {
			http.Error(w, "blocked", http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`<html><head><script type="application/ld+json">{
			"@context":"https://schema.org",
			"@type":"JobPosting",
			"title":"Staff Platform Engineer",
			"hiringOrganization":{"@type":"Organization","name":"Acme Tools","sameAs":"https://www.acmetools.example"},
			"description":"<p>About Acme Tools</p><p>Acme Tools builds developer tools for software teams.</p>",
			"baseSalary":{"@type":"MonetaryAmount","currency":"USD","value":{"@type":"QuantitativeValue","minValue":180000,"maxValue":220000,"unitText":"YEAR"}},
			"industry":["Developer Tools"]
		}</script></head><body></body></html>`))
	}))
	defer server.Close()

	job := Job{
		Company:      "Acme Tools",
		Title:        "Staff Platform Engineer",
		ApplyURL:     server.URL + "/jobs/staff-platform-engineer",
		Source:       "Site Search",
		Compensation: "Not listed",
		Description:  server.URL + "/jobs/staff-platform-engineer",
	}

	enrichSiteSearchJobIdentityBeforeFilter(t.Context(), &job)

	if job.CompanyWebsite != "https://www.acmetools.example" {
		t.Fatalf("CompanyWebsite = %q; want https://www.acmetools.example", job.CompanyWebsite)
	}
	if !strings.Contains(job.CompanySummary, "developer tools") {
		t.Fatalf("CompanySummary = %q; want developer tools summary", job.CompanySummary)
	}
	if job.CompanyIndustry != "Developer Tools" {
		t.Fatalf("CompanyIndustry = %q; want Developer Tools", job.CompanyIndustry)
	}
	if job.Compensation != "$180,000 - $220,000 USD/year" {
		t.Fatalf("Compensation = %q; want $180,000 - $220,000 USD/year", job.Compensation)
	}
}

func TestIsSiteSearchVerificationPageDetectsCloudflare(t *testing.T) {
	body := strings.Join([]string{
		"Find jobs",
		"Additional Verification Required",
		"Your Ray ID for this request is 9f6eee6a785a68ff",
		"Troubleshooting Cloudflare Errors",
	}, "\n")

	if !isSiteSearchVerificationPage("Just a moment...", body) {
		t.Fatal("isSiteSearchVerificationPage() = false; want true for Indeed Cloudflare verification page")
	}
}

func TestSiteSearchBlocklistNormalizesWWWHosts(t *testing.T) {
	blocked := newSiteSearchBlocklist()

	if !blocked.block("https://www.indeed.com/jobs?q=Software+Developer", "verification required") {
		t.Fatal("block() = false; want first block to be recorded")
	}
	if blocked.block("https://indeed.com/jobs?q=Software+Engineer", "verification required") {
		t.Fatal("block() = true; want equivalent host to already be blocked")
	}
	if got := blocked.reasonFor("https://indeed.com/jobs?q=Frontend+Developer"); got != "verification required" {
		t.Fatalf("reasonFor() = %q; want verification required", got)
	}
}

func TestIsBuiltInJobCandidateRejectsNonJobLinks(t *testing.T) {
	tests := []struct {
		name  string
		title string
		raw   string
		want  bool
	}{
		{
			name:  "job detail",
			title: "Staff DevOps Engineer",
			raw:   "https://builtin.com/jobs/remote/nyc/staff-devops-engineer/1234567",
			want:  true,
		},
		{
			name:  "singular job detail",
			title: "Staff Software Engineer",
			raw:   "https://builtin.com/job/staff-software-engineer-local-environments-team/6315940",
			want:  true,
		},
		{
			name:  "footer search link",
			title: "Remote QA Engineer Jobs",
			raw:   "https://builtin.com/jobs/remote/qa-engineer-jobs",
			want:  false,
		},
		{
			name:  "page selector",
			title: "2",
			raw:   "https://builtin.com/jobs/remote?page=2",
			want:  false,
		},
		{
			name:  "popular remote searches",
			title: "Popular Remote Job Searches",
			raw:   "https://builtin.com/jobs/remote",
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.raw)
			if err != nil {
				t.Fatal(err)
			}
			if got := isBuiltInJobCandidate(tt.title, u); got != tt.want {
				t.Fatalf("isBuiltInJobCandidate(%q, %q) = %t; want %t", tt.title, tt.raw, got, tt.want)
			}
		})
	}
}

func TestSimplifySiteSearchErrorRemovesRodStackTrace(t *testing.T) {
	err := simplifySiteSearchError(assertError("error value: &rod.NavigationError{Reason:\"net::ERR_CONNECTION_REFUSED\"}\ngoroutine 74 [running]:\nruntime/debug.Stack()"))
	if err == nil {
		t.Fatal("simplifySiteSearchError() = nil; want simplified error")
	}
	if got, want := err.Error(), "navigation failed: net::ERR_CONNECTION_REFUSED"; got != want {
		t.Fatalf("simplifySiteSearchError() = %q; want %q", got, want)
	}
}

func TestSimplifySiteSearchErrorKeepsCDPMessageOnly(t *testing.T) {
	err := simplifySiteSearchError(assertError("error value: &cdp.Error{Code:-32000, Message:\"Object reference chain is too long\", Data:\"\"}\ngoroutine 74 [running]:\nruntime/debug.Stack()"))
	if err == nil {
		t.Fatal("simplifySiteSearchError() = nil; want simplified error")
	}
	want := `error value: &cdp.Error{Code:-32000, Message:"Object reference chain is too long", Data:""}`
	if got := err.Error(); got != want {
		t.Fatalf("simplifySiteSearchError() = %q; want %q", got, want)
	}
}

type assertError string

func (e assertError) Error() string {
	return string(e)
}
