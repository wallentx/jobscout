package setup

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wallentx/jobscout/internal/domain"
)

// FieldSpec describes one criteria input field and how it maps to criteria config.
type FieldSpec struct {
	Key   string
	Label string
	Help  string
	Load  func(domain.CriteriaConfig) string
	Save  func(*domain.CriteriaConfig, string) error
}

// GroupSpec groups related setup fields for rendering.
type GroupSpec struct {
	ID     string
	Label  string
	Fields []FieldSpec
}

// SearchProfileGroupSpec returns the ordered criteria fields used by setup.
func SearchProfileGroupSpec() GroupSpec {
	return GroupSpec{
		ID:    "search_profile",
		Label: "Search Profile",
		Fields: []FieldSpec{
			{
				Key:   "candidate.city",
				Label: "City",
				Help:  "Candidate city, e.g. Chicago",
				Load: func(cfg domain.CriteriaConfig) string {
					return cfg.Candidate.City
				},
				Save: func(cfg *domain.CriteriaConfig, value string) error {
					cfg.Candidate.City = strings.TrimSpace(value)
					return nil
				},
			},
			{
				Key:   "candidate.state",
				Label: "State",
				Help:  "Candidate state or region, e.g. IL",
				Load: func(cfg domain.CriteriaConfig) string {
					return cfg.Candidate.State
				},
				Save: func(cfg *domain.CriteriaConfig, value string) error {
					cfg.Candidate.State = strings.TrimSpace(value)
					return nil
				},
			},
			{
				Key:   "candidate.country_code",
				Label: "Country Code",
				Help:  "Two-letter country code, e.g. US",
				Load: func(cfg domain.CriteriaConfig) string {
					return cfg.Candidate.CountryCode
				},
				Save: func(cfg *domain.CriteriaConfig, value string) error {
					cfg.Candidate.CountryCode = domain.NormalizeCountryCode(value)
					return nil
				},
			},
			{
				Key:   "candidate.years_of_experience",
				Label: "Years of Experience",
				Help:  "Whole number, e.g. 5",
				Load: func(cfg domain.CriteriaConfig) string {
					if cfg.Candidate.YearsOfExperience <= 0 {
						return ""
					}
					return strconv.Itoa(cfg.Candidate.YearsOfExperience)
				},
				Save: func(cfg *domain.CriteriaConfig, value string) error {
					value = strings.TrimSpace(value)
					if value == "" {
						cfg.Candidate.YearsOfExperience = 0
						return nil
					}
					parsed, err := strconv.Atoi(value)
					if err != nil {
						return fmt.Errorf("years of experience must be a whole number")
					}
					cfg.Candidate.YearsOfExperience = parsed
					return nil
				},
			},
			{
				Key:   "role_families",
				Label: "Role Families",
				Help:  "Comma-separated IDs, e.g. backend, data",
				Load: func(cfg domain.CriteriaConfig) string {
					return domain.FormatRoleFamilyIDs(cfg.RoleFamilies)
				},
				Save: func(cfg *domain.CriteriaConfig, value string) error {
					roleFamilies, err := domain.ParseRoleFamilyCSV(value)
					if err != nil {
						return err
					}
					cfg.RoleFamilies = roleFamilies
					return nil
				},
			},
			{
				Key:   "filters.title_requires",
				Label: "Required Title Prefixes",
				Help:  "Comma-separated title prefixes or levels, e.g. senior, staff",
				Load: func(cfg domain.CriteriaConfig) string {
					return strings.Join(cfg.Filters.TitleRequires, ", ")
				},
				Save: func(cfg *domain.CriteriaConfig, value string) error {
					cfg.Filters.TitleRequires = domain.NormalizeTitlePrefixes(domain.ParseCSVList(value))
					return nil
				},
			},
			{
				Key:   "filters.title_includes",
				Label: "Target Title Names",
				Help:  "Comma-separated role titles, e.g. software engineer, platform engineer",
				Load: func(cfg domain.CriteriaConfig) string {
					return strings.Join(cfg.Filters.TitleIncludes, ", ")
				},
				Save: func(cfg *domain.CriteriaConfig, value string) error {
					cfg.Filters.TitleIncludes = domain.NormalizeTargetTitleNames(domain.ParseCSVList(value), cfg.RoleFamilies)
					return nil
				},
			},
			{
				Key:   "filters.title_excludes",
				Label: "Excluded Title Terms",
				Help:  "Comma-separated blocked keywords, e.g. manager, contract",
				Load: func(cfg domain.CriteriaConfig) string {
					return strings.Join(cfg.Filters.TitleExcludes, ", ")
				},
				Save: func(cfg *domain.CriteriaConfig, value string) error {
					cfg.Filters.TitleExcludes = domain.ParseCSVList(value)
					return nil
				},
			},
			{
				Key:   "filters.work_settings",
				Label: "Work Settings",
				Help:  "Comma-separated: remote, hybrid, onsite. Example: remote, hybrid",
				Load: func(cfg domain.CriteriaConfig) string {
					return strings.Join(domain.SelectedWorkSettings(cfg.Filters.WorkSettings), ", ")
				},
				Save: func(cfg *domain.CriteriaConfig, value string) error {
					cfg.Filters.WorkSettings = domain.ParseWorkSettings(value)
					return nil
				},
			},
			{
				Key:   "filters.min_base_usd",
				Label: "Minimum Base USD",
				Help:  "Whole number, e.g. 100000",
				Load: func(cfg domain.CriteriaConfig) string {
					if cfg.Filters.MinBaseUSD <= 0 {
						return ""
					}
					return strconv.Itoa(cfg.Filters.MinBaseUSD)
				},
				Save: func(cfg *domain.CriteriaConfig, value string) error {
					value = strings.TrimSpace(value)
					if value == "" {
						cfg.Filters.MinBaseUSD = 0
						return nil
					}
					parsed, err := strconv.Atoi(value)
					if err != nil {
						return fmt.Errorf("minimum base USD must be a whole number")
					}
					cfg.Filters.MinBaseUSD = parsed
					return nil
				},
			},
			{
				Key:   "priority_signals",
				Label: "Priority Signals",
				Help:  "Comma-separated skills/keywords, e.g. reliability, automation",
				Load: func(cfg domain.CriteriaConfig) string {
					return strings.Join(cfg.PrioritySignals, ", ")
				},
				Save: func(cfg *domain.CriteriaConfig, value string) error {
					cfg.PrioritySignals = domain.ParseCSVList(value)
					return nil
				},
			},
		},
	}
}

// SearchProfileValues converts criteria config into setup form values.
func SearchProfileValues(cfg domain.CriteriaConfig) map[string]string {
	values := make(map[string]string)
	for _, field := range SearchProfileGroupSpec().Fields {
		values[field.Key] = field.Load(cfg)
	}
	return values
}

// CriteriaFromSearchProfileValues converts setup form values into criteria config.
func CriteriaFromSearchProfileValues(values map[string]string) (*domain.CriteriaConfig, error) {
	var cfg domain.CriteriaConfig
	cfg.Filters.IndustryIncludes = []string{}
	cfg.Filters.IndustryExcludes = []string{}

	for _, field := range SearchProfileGroupSpec().Fields {
		if err := field.Save(&cfg, values[field.Key]); err != nil {
			return nil, err
		}
	}
	domain.NormalizeCriteriaLocation(&cfg)

	return &cfg, nil
}

// FieldSpecAt returns the setup field at idx.
func FieldSpecAt(idx int) (FieldSpec, bool) {
	fields := SearchProfileGroupSpec().Fields
	if idx < 0 || idx >= len(fields) {
		return FieldSpec{}, false
	}
	return fields[idx], true
}
