package domain

import "testing"

func TestShouldUseBrowserCompanyProfile(t *testing.T) {
	foundedYear := 2018
	employees := 120

	tests := []struct {
		name   string
		result *CompanyHealthResult
		want   bool
	}{
		{
			name:   "nil result",
			result: nil,
			want:   false,
		},
		{
			name:   "missing founded year",
			result: &CompanyHealthResult{EstimatedEmployees: &employees, Confidence: "medium"},
			want:   true,
		},
		{
			name:   "missing employees",
			result: &CompanyHealthResult{FoundedYear: &foundedYear, Confidence: "medium"},
			want:   true,
		},
		{
			name:   "low confidence",
			result: &CompanyHealthResult{FoundedYear: &foundedYear, EstimatedEmployees: &employees, Confidence: "low"},
			want:   true,
		},
		{
			name:   "complete medium confidence",
			result: &CompanyHealthResult{FoundedYear: &foundedYear, EstimatedEmployees: &employees, Confidence: "medium"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldUseBrowserCompanyProfile(tt.result); got != tt.want {
				t.Errorf("ShouldUseBrowserCompanyProfile(%#v) = %v, want %v", tt.result, got, tt.want)
			}
		})
	}
}

func TestApplyCompanySiteProfileAddsCompanySiteEvidence(t *testing.T) {
	result := &CompanyHealthResult{
		Confidence:  "low",
		SignalsUsed: []string{},
		Notes:       []string{},
		Sources:     make(map[string]any),
	}
	initCompanyHealthAssessments(result)

	profile := &CompanySiteProfile{
		SearchQuery: "Acme official site",
		SearchURL:   "https://search.example/?q=Acme",
		WebsiteURL:  "https://www.acme.example",
		AboutURL:    "https://www.acme.example/about",
		WebsiteText: "Acme makes anvils.",
		AboutText:   "Founded in 2012, Acme has 1,200 employees worldwide.",
	}

	ApplyCompanySiteProfile(result, profile)

	if len(result.SignalsUsed) != 1 || result.SignalsUsed[0] != "browser_company_site" {
		t.Fatalf("ApplyCompanySiteProfile() SignalsUsed = %#v, want browser_company_site", result.SignalsUsed)
	}
	source, ok := result.Sources["company_site"].(map[string]any)
	if !ok {
		t.Fatalf("ApplyCompanySiteProfile() company_site source = %#v, want map[string]any", result.Sources["company_site"])
	}
	wantSource := map[string]string{
		"search_query": profile.SearchQuery,
		"search_url":   profile.SearchURL,
		"website_url":  profile.WebsiteURL,
		"about_url":    profile.AboutURL,
	}
	for key, want := range wantSource {
		if got := source[key]; got != want {
			t.Errorf("ApplyCompanySiteProfile() source[%q] = %v, want %q", key, got, want)
		}
	}

	if result.FoundedYear == nil || *result.FoundedYear != 2012 {
		t.Fatalf("ApplyCompanySiteProfile() FoundedYear = %#v, want 2012", result.FoundedYear)
	}
	if result.EstimatedEmployees == nil || *result.EstimatedEmployees != 1200 {
		t.Fatalf("ApplyCompanySiteProfile() EstimatedEmployees = %#v, want 1200", result.EstimatedEmployees)
	}
	if result.Confidence != "medium" {
		t.Errorf("ApplyCompanySiteProfile() Confidence = %q, want medium", result.Confidence)
	}

	foundedAssessment := result.FieldAssessments["founded_year"]
	if foundedAssessment == nil || foundedAssessment.Source != "company_site" || foundedAssessment.URL != profile.AboutURL {
		t.Fatalf("ApplyCompanySiteProfile() founded_year assessment = %#v, want company_site with about URL", foundedAssessment)
	}
	employeesAssessment := result.FieldAssessments["estimated_employees"]
	if employeesAssessment == nil || employeesAssessment.Source != "company_site" || employeesAssessment.URL != profile.AboutURL {
		t.Fatalf("ApplyCompanySiteProfile() estimated_employees assessment = %#v, want company_site with about URL", employeesAssessment)
	}
}

func TestApplyCompanySiteProfileAddsFieldGapsWithoutSiteEvidence(t *testing.T) {
	result := &CompanyHealthResult{
		Confidence:  "medium",
		SignalsUsed: []string{},
		Notes:       []string{},
		Sources:     make(map[string]any),
	}
	initCompanyHealthAssessments(result)

	ApplyCompanySiteProfile(result, &CompanySiteProfile{
		WebsiteURL: "://invalid",
	})

	foundedAssessment := result.FieldAssessments["founded_year"]
	if foundedAssessment == nil || len(foundedAssessment.Notes) != 1 {
		t.Fatalf("ApplyCompanySiteProfile() founded_year assessment = %#v, want one gap note", foundedAssessment)
	}
	if want := "Browser company-site lookup found no trustworthy founded-year evidence."; foundedAssessment.Notes[0] != want {
		t.Errorf("ApplyCompanySiteProfile() founded_year gap note = %q, want %q", foundedAssessment.Notes[0], want)
	}

	employeesAssessment := result.FieldAssessments["estimated_employees"]
	if employeesAssessment == nil || len(employeesAssessment.Notes) != 1 {
		t.Fatalf("ApplyCompanySiteProfile() estimated_employees assessment = %#v, want one gap note", employeesAssessment)
	}
	if want := "Browser company-site lookup found no trustworthy employee-count evidence."; employeesAssessment.Notes[0] != want {
		t.Errorf("ApplyCompanySiteProfile() estimated_employees gap note = %q, want %q", employeesAssessment.Notes[0], want)
	}
}
