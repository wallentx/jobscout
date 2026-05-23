package domain

import (
	"encoding/json"
	"strings"
)

type JobMetadata struct {
	Source  *JobSourceMetadata `json:"source,omitempty"`
	Company *CompanyMetadata   `json:"company,omitempty"`
}

type JobSourceMetadata struct {
	PostingURL        string   `json:"posting_url,omitempty"`
	ExternalApplyURL  string   `json:"external_apply_url,omitempty"`
	CompanyProfileURL string   `json:"company_profile_url,omitempty"`
	PostedLabel       string   `json:"posted_label,omitempty"`
	DatePosted        string   `json:"date_posted,omitempty"`
	ValidThrough      string   `json:"valid_through,omitempty"`
	EmploymentTypes   []string `json:"employment_types,omitempty"`
	Locations         []string `json:"locations,omitempty"`
	Industries        []string `json:"industries,omitempty"`
	Skills            []string `json:"skills,omitempty"`
}

type CompanyMetadata struct {
	Industries         []string `json:"industries,omitempty"`
	EmployeeRange      string   `json:"employee_range,omitempty"`
	EstimatedEmployees *int     `json:"estimated_employees,omitempty"`
	FoundedYear        *int     `json:"founded_year,omitempty"`
	Headquarters       string   `json:"headquarters,omitempty"`
	Revenue            string   `json:"revenue,omitempty"`
	Ticker             string   `json:"ticker,omitempty"`
	Exchange           string   `json:"exchange,omitempty"`
	Public             *bool    `json:"public,omitempty"`
}

func (m *JobSourceMetadata) UnmarshalJSON(data []byte) error {
	type sourceAlias JobSourceMetadata
	var raw struct {
		sourceAlias
		EmploymentType  json.RawMessage `json:"employment_type"`
		EmploymentTypes json.RawMessage `json:"employment_types"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*m = JobSourceMetadata(raw.sourceAlias)
	if values := metadataStringList(raw.EmploymentTypes); len(values) > 0 {
		m.EmploymentTypes = values
	} else if values := metadataStringList(raw.EmploymentType); len(values) > 0 {
		m.EmploymentTypes = values
	}
	NormalizeJobSourceMetadata(m)
	return nil
}

func (j *Job) EnsureSourceMetadata() *JobSourceMetadata {
	if j == nil {
		return nil
	}
	if j.Metadata == nil {
		j.Metadata = &JobMetadata{}
	}
	if j.Metadata.Source == nil {
		j.Metadata.Source = &JobSourceMetadata{}
	}
	return j.Metadata.Source
}

func (j *Job) EnsureCompanyMetadata() *CompanyMetadata {
	if j == nil {
		return nil
	}
	if j.Metadata == nil {
		j.Metadata = &JobMetadata{}
	}
	if j.Metadata.Company == nil {
		j.Metadata.Company = &CompanyMetadata{}
	}
	return j.Metadata.Company
}

func NormalizeJobMetadata(metadata *JobMetadata) *JobMetadata {
	if metadata == nil {
		return nil
	}
	metadata.Source = NormalizeJobSourceMetadata(metadata.Source)
	metadata.Company = NormalizeCompanyMetadata(metadata.Company)
	if metadata.Source == nil && metadata.Company == nil {
		return nil
	}
	return metadata
}

func NormalizeJobSourceMetadata(metadata *JobSourceMetadata) *JobSourceMetadata {
	if metadata == nil {
		return nil
	}
	metadata.PostingURL = strings.TrimSpace(metadata.PostingURL)
	metadata.ExternalApplyURL = strings.TrimSpace(metadata.ExternalApplyURL)
	metadata.CompanyProfileURL = strings.TrimSpace(metadata.CompanyProfileURL)
	metadata.PostedLabel = strings.TrimSpace(metadata.PostedLabel)
	metadata.DatePosted = strings.TrimSpace(metadata.DatePosted)
	metadata.ValidThrough = strings.TrimSpace(metadata.ValidThrough)
	metadata.EmploymentTypes = compactUniqueStrings(metadata.EmploymentTypes)
	metadata.Locations = compactUniqueStrings(metadata.Locations)
	metadata.Industries = compactUniqueStrings(metadata.Industries)
	metadata.Skills = compactUniqueStrings(metadata.Skills)
	if metadata.PostingURL == "" &&
		metadata.ExternalApplyURL == "" &&
		metadata.CompanyProfileURL == "" &&
		metadata.PostedLabel == "" &&
		metadata.DatePosted == "" &&
		metadata.ValidThrough == "" &&
		len(metadata.EmploymentTypes) == 0 &&
		len(metadata.Locations) == 0 &&
		len(metadata.Industries) == 0 &&
		len(metadata.Skills) == 0 {
		return nil
	}
	return metadata
}

func NormalizeCompanyMetadata(metadata *CompanyMetadata) *CompanyMetadata {
	if metadata == nil {
		return nil
	}
	metadata.Industries = compactUniqueStrings(metadata.Industries)
	metadata.EmployeeRange = strings.TrimSpace(metadata.EmployeeRange)
	metadata.Headquarters = strings.TrimSpace(metadata.Headquarters)
	metadata.Revenue = strings.TrimSpace(metadata.Revenue)
	metadata.Ticker = strings.ToUpper(strings.TrimSpace(metadata.Ticker))
	metadata.Exchange = strings.ToUpper(strings.TrimSpace(metadata.Exchange))
	if metadata.EstimatedEmployees != nil && !plausibleEmployeeCount(*metadata.EstimatedEmployees) {
		metadata.EstimatedEmployees = nil
	}
	if len(metadata.Industries) == 0 &&
		metadata.EmployeeRange == "" &&
		metadata.EstimatedEmployees == nil &&
		metadata.FoundedYear == nil &&
		metadata.Headquarters == "" &&
		metadata.Revenue == "" &&
		metadata.Ticker == "" &&
		metadata.Exchange == "" &&
		metadata.Public == nil {
		return nil
	}
	return metadata
}

func CloneJobMetadata(metadata *JobMetadata) *JobMetadata {
	if metadata == nil {
		return nil
	}
	clone := &JobMetadata{}
	if metadata.Source != nil {
		source := *metadata.Source
		source.EmploymentTypes = append([]string(nil), metadata.Source.EmploymentTypes...)
		source.Locations = append([]string(nil), metadata.Source.Locations...)
		source.Industries = append([]string(nil), metadata.Source.Industries...)
		source.Skills = append([]string(nil), metadata.Source.Skills...)
		clone.Source = &source
	}
	if metadata.Company != nil {
		company := *metadata.Company
		company.Industries = append([]string(nil), metadata.Company.Industries...)
		if metadata.Company.EstimatedEmployees != nil {
			company.EstimatedEmployees = new(int)
			*company.EstimatedEmployees = *metadata.Company.EstimatedEmployees
		}
		if metadata.Company.FoundedYear != nil {
			company.FoundedYear = new(int)
			*company.FoundedYear = *metadata.Company.FoundedYear
		}
		if metadata.Company.Public != nil {
			company.Public = new(bool)
			*company.Public = *metadata.Company.Public
		}
		clone.Company = &company
	}
	return NormalizeJobMetadata(clone)
}

func MergeJobMetadataFields(existing *Job, incoming Job) {
	if existing == nil || incoming.Metadata == nil {
		return
	}
	source := incoming.Metadata.Source
	if source != nil {
		target := existing.EnsureSourceMetadata()
		mergeStringField(&target.PostingURL, source.PostingURL)
		mergeStringField(&target.ExternalApplyURL, source.ExternalApplyURL)
		mergeStringField(&target.CompanyProfileURL, source.CompanyProfileURL)
		mergeStringField(&target.PostedLabel, source.PostedLabel)
		mergeStringField(&target.DatePosted, source.DatePosted)
		mergeStringField(&target.ValidThrough, source.ValidThrough)
		target.EmploymentTypes = appendUniqueNonEmptyStrings(target.EmploymentTypes, source.EmploymentTypes...)
		target.Locations = appendUniqueNonEmptyStrings(target.Locations, source.Locations...)
		target.Industries = appendUniqueNonEmptyStrings(target.Industries, source.Industries...)
		target.Skills = appendUniqueNonEmptyStrings(target.Skills, source.Skills...)
	}
	company := incoming.Metadata.Company
	if company != nil {
		target := existing.EnsureCompanyMetadata()
		target.Industries = appendUniqueNonEmptyStrings(target.Industries, company.Industries...)
		mergeStringField(&target.EmployeeRange, company.EmployeeRange)
		mergeStringField(&target.Headquarters, company.Headquarters)
		mergeStringField(&target.Revenue, company.Revenue)
		mergeStringField(&target.Ticker, company.Ticker)
		mergeStringField(&target.Exchange, company.Exchange)
		if target.EstimatedEmployees == nil && company.EstimatedEmployees != nil {
			target.EstimatedEmployees = new(int)
			*target.EstimatedEmployees = *company.EstimatedEmployees
		}
		if target.FoundedYear == nil && company.FoundedYear != nil {
			target.FoundedYear = new(int)
			*target.FoundedYear = *company.FoundedYear
		}
		if target.Public == nil && company.Public != nil {
			target.Public = new(bool)
			*target.Public = *company.Public
		}
	}
	existing.Metadata = NormalizeJobMetadata(existing.Metadata)
}

func CompanyHealthContextForJob(job Job) CompanyHealthContext {
	website := strings.TrimSpace(job.CompanyWebsite)
	if JobCompanyWebsiteMissingOrInvalid(website) {
		website = ""
	}
	identity := CompanyHealthContext{
		Company:  strings.TrimSpace(job.Company),
		Website:  website,
		Summary:  strings.TrimSpace(job.CompanySummary),
		Industry: strings.TrimSpace(job.CompanyIndustry),
	}
	if job.Metadata != nil {
		if job.Metadata.Source != nil {
			identity.Industries = appendUniqueNonEmptyStrings(identity.Industries, job.Metadata.Source.Industries...)
		}
		if job.Metadata.Company != nil {
			identity.Industries = appendUniqueNonEmptyStrings(identity.Industries, job.Metadata.Company.Industries...)
			identity.Ticker = strings.ToUpper(strings.TrimSpace(job.Metadata.Company.Ticker))
			if job.Metadata.Company.EstimatedEmployees != nil {
				identity.EstimatedEmployees = new(int)
				*identity.EstimatedEmployees = *job.Metadata.Company.EstimatedEmployees
			}
			if job.Metadata.Company.FoundedYear != nil {
				identity.FoundedYear = new(int)
				*identity.FoundedYear = *job.Metadata.Company.FoundedYear
			}
		}
	}
	if identity.Industry != "" {
		identity.Industries = appendUniqueNonEmptyStrings([]string{identity.Industry}, identity.Industries...)
	}
	return identity
}

func appendUniqueNonEmptyStrings(values []string, more ...string) []string {
	seen := make(map[string]bool, len(values)+len(more))
	out := make([]string, 0, len(values)+len(more))
	for _, value := range append(values, more...) {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func compactUniqueStrings(values []string) []string {
	return appendUniqueNonEmptyStrings(nil, values...)
}

func mergeStringField(target *string, value string) {
	if target == nil || strings.TrimSpace(*target) != "" {
		return
	}
	*target = strings.TrimSpace(value)
}

func metadataStringList(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		if strings.TrimSpace(single) == "" {
			return nil
		}
		return []string{single}
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err == nil {
		return compactUniqueStrings(values)
	}
	return nil
}
