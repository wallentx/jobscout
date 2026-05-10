package setup

import (
	"strings"
	"testing"

	"github.com/wallentx/jobscout/internal/domain"
)

func TestSearchProfileFieldsIncludeExamples(t *testing.T) {
	for _, field := range SearchProfileGroupSpec().Fields {
		if !strings.Contains(strings.ToLower(field.Help), "e.g.") && !strings.Contains(strings.ToLower(field.Help), "example") {
			t.Fatalf("field %q help = %q, want example text", field.Key, field.Help)
		}
	}
}

func TestSearchProfileFieldSavesParsedValues(t *testing.T) {
	var cfg domain.CriteriaConfig
	fields := SearchProfileGroupSpec().Fields

	for _, field := range fields {
		if field.Key == "filters.title_includes" {
			if err := field.Save(&cfg, "Frontend Engineer\nPlatform Eng, Software Dev"); err != nil {
				t.Fatalf("Save(%q) error = %v", field.Key, err)
			}
			break
		}
	}

	want := []string{"Frontend Engineer", "Platform Engineer", "Software Developer"}
	if len(cfg.Filters.TitleIncludes) != len(want) {
		t.Fatalf("TitleIncludes len = %d, want %d (%#v)", len(cfg.Filters.TitleIncludes), len(want), cfg.Filters.TitleIncludes)
	}
	for i := range want {
		if cfg.Filters.TitleIncludes[i] != want[i] {
			t.Fatalf("TitleIncludes[%d] = %q, want %q", i, cfg.Filters.TitleIncludes[i], want[i])
		}
	}
}

func TestCriteriaFromSearchProfileValuesParsesRoleFamiliesAndLocation(t *testing.T) {
	criteria, err := CriteriaFromSearchProfileValues(map[string]string{
		"candidate.city":                "Example City",
		"candidate.state":               "Texas",
		"candidate.country_code":        "United States",
		"candidate.years_of_experience": "3",
		"role_families":                 "devops_sre_systems, data",
		"filters.title_requires":        "Engineer",
		"filters.title_includes":        "backend, systems",
		"filters.title_excludes":        "manager",
		"filters.work_settings":         "remote",
		"filters.min_base_usd":          "100000",
		"priority_signals":              "reliability, automation",
	})
	if err != nil {
		t.Fatalf("CriteriaFromSearchProfileValues() error = %v", err)
	}

	if len(criteria.RoleFamilies) != 2 {
		t.Fatalf("criteria.RoleFamilies len = %d; want 2", len(criteria.RoleFamilies))
	}
	if criteria.RoleFamilies[0] != domain.RoleDevOpsSRESystems || criteria.RoleFamilies[1] != domain.RoleData {
		t.Fatalf("criteria.RoleFamilies = %#v; want devops_sre_systems,data", criteria.RoleFamilies)
	}
	if criteria.Candidate.State != "TX" {
		t.Fatalf("criteria.Candidate.State = %q; want TX", criteria.Candidate.State)
	}
	if criteria.Candidate.CountryCode != "US" {
		t.Fatalf("criteria.Candidate.CountryCode = %q; want US", criteria.Candidate.CountryCode)
	}
}
