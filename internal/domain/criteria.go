package domain

type WorkSettingsConfig struct {
	Remote bool `yaml:"remote,omitempty"`
	Hybrid bool `yaml:"hybrid,omitempty"`
	Onsite bool `yaml:"onsite,omitempty"`
}

type CriteriaConfig struct {
	Candidate struct {
		City              string `yaml:"city"`
		State             string `yaml:"state"`
		CountryCode       string `yaml:"country_code"`
		YearsOfExperience int    `yaml:"years_of_experience"`
	} `yaml:"candidate"`
	Filters struct {
		TitleRequires    []string           `yaml:"title_requires"`
		TitleIncludes    []string           `yaml:"title_includes"`
		TitleExcludes    []string           `yaml:"title_excludes"`
		WorkSettings     WorkSettingsConfig `yaml:"work_settings"`
		MaxDistanceMiles int                `yaml:"max_distance_miles"`
		MinBaseUSD       int                `yaml:"min_base_usd"`
		IndustryIncludes []string           `yaml:"industry_includes"`
		IndustryExcludes []string           `yaml:"industry_excludes"`
	} `yaml:"filters"`
	RoleFamilies      []RoleFamilyID `yaml:"role_families,omitempty"`
	ResolvedSourceIDs []string       `yaml:"resolved_source_ids,omitempty"`
	PrioritySignals   []string       `yaml:"priority_signals"`
}
