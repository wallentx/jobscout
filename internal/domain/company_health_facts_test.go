package domain

import "testing"

func TestObserveFoundedYearMarksConflict(t *testing.T) {
	result := &CompanyHealthResult{}
	initCompanyHealthAssessments(result)

	if !observeFoundedYear(result, 2018, "wikipedia_summary", "", "medium", "first source") {
		t.Fatal("observeFoundedYear() first source accepted = false; want true")
	}
	if observeFoundedYear(result, 2001, "company_site", "", "low", "conflicting source") {
		t.Fatal("observeFoundedYear() conflicting low-confidence source accepted = true; want false")
	}

	assessment := result.FieldAssessments["founded_year"]
	if assessment == nil || assessment.Status != fieldStatusConflict {
		t.Fatalf("founded_year status = %#v; want conflict", assessment)
	}
}

func TestObserveEmployeeCountPromotesHigherConfidenceSource(t *testing.T) {
	result := &CompanyHealthResult{}
	initCompanyHealthAssessments(result)

	if !observeEmployeeCount(result, 500, "company_site", "", "medium", "site text") {
		t.Fatal("observeEmployeeCount() medium source accepted = false; want true")
	}
	if !observeEmployeeCount(result, 550, "sec_10k", "", "high", "sec filing") {
		t.Fatal("observeEmployeeCount() higher-confidence source accepted = false; want true")
	}

	if result.EstimatedEmployees == nil || *result.EstimatedEmployees != 550 {
		t.Fatalf("EstimatedEmployees = %#v; want 550", result.EstimatedEmployees)
	}

	assessment := result.FieldAssessments["estimated_employees"]
	if assessment == nil || assessment.Source != "sec_10k" || assessment.Confidence != "high" {
		t.Fatalf("estimated_employees assessment = %#v; want high-confidence SEC source", assessment)
	}
}

func TestFinalizeCompanyHealthAssessmentsMarksGap(t *testing.T) {
	result := &CompanyHealthResult{}
	initCompanyHealthAssessments(result)

	finalizeCompanyHealthAssessments(result)

	for _, field := range []string{"founded_year", "estimated_employees"} {
		assessment := result.FieldAssessments[field]
		if assessment == nil || assessment.Status != fieldStatusGap {
			t.Fatalf("%s assessment = %#v; want gap", field, assessment)
		}
	}
}

func TestHealthEvidenceRejectsCircleGamingFalsePositive(t *testing.T) {
	identity := CompanyHealthContext{
		Company:  "Circle",
		Website:  "https://www.circle.com",
		Summary:  "Circle provides financial technology for stablecoin payments.",
		Industry: "Financial Technology",
	}

	ok, reason := healthEvidenceMatchesCompanyContext(
		"Full Circle developer hit by layoffs at game publisher",
		"https://www.gamesindustry.biz/full-circle-layoffs",
		identity,
	)

	if ok {
		t.Fatal("healthEvidenceMatchesCompanyContext() accepted gaming evidence for Circle fintech")
	}
	if reason == "" {
		t.Fatal("healthEvidenceMatchesCompanyContext() reason is empty")
	}
}

func TestHealthEvidenceAcceptsResolvedDomain(t *testing.T) {
	identity := CompanyHealthContext{
		Company:  "Circle",
		Website:  "https://www.circle.com",
		Summary:  "Circle provides financial technology for stablecoin payments.",
		Industry: "Financial Technology",
	}

	ok, reason := healthEvidenceMatchesCompanyContext(
		"Circle announces new payment infrastructure partnership",
		"https://www.circle.com/news/payment-infrastructure-partnership",
		identity,
	)

	if !ok {
		t.Fatalf("healthEvidenceMatchesCompanyContext() rejected domain evidence: %s", reason)
	}
}

func TestHealthEvidenceAcceptsCompanyMentionWithAdjacentWords(t *testing.T) {
	identity := CompanyHealthContext{
		Company:  "OpenAI",
		Website:  "https://openai.com",
		Summary:  "OpenAI develops artificial intelligence products and research.",
		Industry: "Artificial Intelligence",
	}

	cases := []struct {
		name  string
		title string
		url   string
	}{
		{
			name:  "word before company",
			title: "Leaked OpenAI documents reveal aggressive tactics toward former employees",
			url:   "https://www.vox.com/future-perfect/351132/openai-vested-equity-nda-sam-altman-documents-employees",
		},
		{
			name:  "word after company",
			title: "OpenAI adds AI pets to its Codex coding tool - Mashable",
			url:   "https://news.google.com/rss/articles/example",
		},
		{
			name:  "headline with partner names",
			title: "AWS and OpenAI announce expanded partnership",
			url:   "https://www.aboutamazon.com/news/aws/openai-partnership",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, reason := healthEvidenceMatchesCompanyContext(tc.title, tc.url, identity)
			if !ok {
				t.Fatalf("healthEvidenceMatchesCompanyContext() rejected valid OpenAI evidence: %s", reason)
			}
		})
	}
}

func TestFilterLayoffSignalsForContextReturnsRejectedEvidence(t *testing.T) {
	identity := CompanyHealthContext{
		Company:  "Circle",
		Website:  "https://www.circle.com",
		Summary:  "Circle provides financial technology for stablecoin payments.",
		Industry: "Financial Technology",
	}
	signals := []LayoffSignal{
		{Title: "Circle cuts 100 jobs", URL: "https://www.circle.com/news/jobs"},
		{Title: "Full Circle studio hit by layoffs", URL: "https://www.gamesindustry.biz/full-circle-layoffs"},
	}

	filtered, rejected := filterLayoffSignalsForContext(signals, identity)

	if len(filtered) != 1 || filtered[0].Title != "Circle cuts 100 jobs" {
		t.Fatalf("filtered signals = %#v, want only Circle domain signal", filtered)
	}
	if len(rejected) != 1 {
		t.Fatalf("rejected evidence = %#v, want one rejected signal", rejected)
	}
	if rejected[0].Accepted {
		t.Fatalf("rejected evidence Accepted = true: %#v", rejected[0])
	}
}
