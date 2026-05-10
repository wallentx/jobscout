package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Job struct {
	Company         string               `json:"company"`
	CompanyWebsite  string               `json:"company_website,omitempty"`
	CompanySummary  string               `json:"company_summary,omitempty"`
	CompanyIndustry string               `json:"company_industry,omitempty"`
	CompanyIdentity *JobIdentityMetadata `json:"company_identity,omitempty"`
	Title           string               `json:"title"`
	Remote          string               `json:"remote"`
	Compensation    string               `json:"compensation"`
	Source          string               `json:"source"`
	ApplyURL        string               `json:"apply_url"`
	WhyMatches      []string             `json:"why_matches"`
	Status          string               `json:"status"`
	DateAdded       int64                `json:"date_added,omitempty"`
	DateDiscovered  string               `json:"-"`
	Description     string               `json:"description,omitempty"`
}

type JobIdentityEvidence struct {
	Value       string `json:"value,omitempty"`
	Source      string `json:"source,omitempty"`
	URL         string `json:"url,omitempty"`
	Confidence  string `json:"confidence,omitempty"`
	Provisional bool   `json:"provisional,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

type JobIdentityMetadata struct {
	Website  *JobIdentityEvidence `json:"website,omitempty"`
	Summary  *JobIdentityEvidence `json:"summary,omitempty"`
	Industry *JobIdentityEvidence `json:"industry,omitempty"`
}

type JobIdentityPage struct {
	URL  string `json:"url,omitempty"`
	Text string `json:"text,omitempty"`
}

type JobIdentityEnrichment struct {
	CompanyWebsite        string `json:"company_website,omitempty"`
	CompanySummary        string `json:"company_summary,omitempty"`
	CompanyIndustry       string `json:"company_industry,omitempty"`
	WebsiteConfidence     string `json:"website_confidence,omitempty"`
	SummaryConfidence     string `json:"summary_confidence,omitempty"`
	IndustryConfidence    string `json:"industry_confidence,omitempty"`
	IndustryProvisional   bool   `json:"industry_provisional,omitempty"`
	CompanyWebsiteReason  string `json:"company_website_reason,omitempty"`
	CompanySummaryReason  string `json:"company_summary_reason,omitempty"`
	CompanyIndustryReason string `json:"company_industry_reason,omitempty"`
}

func (j *Job) SetCompanyIdentityEvidence(field string, evidence JobIdentityEvidence) {
	if j == nil || strings.TrimSpace(evidence.Value) == "" {
		return
	}
	if j.CompanyIdentity == nil {
		j.CompanyIdentity = &JobIdentityMetadata{}
	}
	evidence.Value = strings.TrimSpace(evidence.Value)
	evidence.Source = strings.TrimSpace(evidence.Source)
	evidence.URL = strings.TrimSpace(evidence.URL)
	evidence.Confidence = strings.TrimSpace(evidence.Confidence)
	evidence.Reason = strings.TrimSpace(evidence.Reason)
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "website":
		j.CompanyIdentity.Website = &evidence
	case "summary":
		j.CompanyIdentity.Summary = &evidence
	case "industry":
		j.CompanyIdentity.Industry = &evidence
	}
}

func (j *Job) UnmarshalJSON(data []byte) error {
	type rawJob struct {
		Company         string          `json:"company"`
		CompanyName     string          `json:"company_name"`
		CompanyWebsite  string          `json:"company_website"`
		CompanyURL      string          `json:"company_url"`
		Website         string          `json:"website"`
		CompanySummary  string          `json:"company_summary"`
		AboutCompany    string          `json:"about_company"`
		CompanyIndustry string          `json:"company_industry"`
		Industry        string          `json:"industry"`
		CompanyIdentity json.RawMessage `json:"company_identity"`
		Title           string          `json:"title"`
		JobTitle        string          `json:"job_title"`
		Remote          json.RawMessage `json:"remote"`
		Compensation    string          `json:"compensation"`
		Salary          string          `json:"salary"`
		Pay             string          `json:"pay"`
		Source          string          `json:"source"`
		ApplyURL        string          `json:"apply_url"`
		URL             string          `json:"url"`
		Link            string          `json:"link"`
		ApplyLink       string          `json:"applyLink"`
		WhyMatches      []string        `json:"why_matches"`
		WhyMatch        []string        `json:"why_match"`
		Status          string          `json:"status"`
		DateAdded       int64           `json:"date_added"`
		DateDiscovered  string          `json:"date_discovered"`
		Description     string          `json:"description"`
	}

	var raw rawJob
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	j.Company = firstNonEmpty(raw.Company, raw.CompanyName)
	j.CompanyWebsite = firstNonEmpty(raw.CompanyWebsite, raw.CompanyURL, raw.Website)
	j.CompanySummary = firstNonEmpty(raw.CompanySummary, raw.AboutCompany)
	j.CompanyIndustry = firstNonEmpty(raw.CompanyIndustry, raw.Industry)
	j.CompanyIdentity = parseJobIdentityMetadata(raw.CompanyIdentity)
	if j.CompanyIdentity != nil {
		if j.CompanyWebsite == "" && j.CompanyIdentity.Website != nil {
			j.CompanyWebsite = strings.TrimSpace(j.CompanyIdentity.Website.Value)
		}
		if j.CompanySummary == "" && j.CompanyIdentity.Summary != nil {
			j.CompanySummary = strings.TrimSpace(j.CompanyIdentity.Summary.Value)
		}
		if j.CompanyIndustry == "" && j.CompanyIdentity.Industry != nil {
			j.CompanyIndustry = strings.TrimSpace(j.CompanyIdentity.Industry.Value)
		}
	}
	j.Title = firstNonEmpty(raw.Title, raw.JobTitle)
	j.Compensation = firstNonEmpty(raw.Compensation, raw.Salary, raw.Pay)
	j.Source = raw.Source
	j.ApplyURL = firstNonEmpty(raw.ApplyURL, raw.URL, raw.Link, raw.ApplyLink)
	j.Status = raw.Status
	j.Description = raw.Description
	if raw.DateAdded > 0 {
		j.DateAdded = raw.DateAdded
		j.DateDiscovered = formatUnixDate(raw.DateAdded)
	} else {
		j.setDateFromString(raw.DateDiscovered)
	}

	if len(raw.WhyMatches) > 0 {
		j.WhyMatches = raw.WhyMatches
	} else {
		j.WhyMatches = raw.WhyMatch
	}

	if len(raw.Remote) == 0 || string(raw.Remote) == "null" {
		j.Remote = ""
		return nil
	}

	var remoteString string
	if err := json.Unmarshal(raw.Remote, &remoteString); err == nil {
		j.Remote = remoteString
		return nil
	}

	var remoteBool bool
	if err := json.Unmarshal(raw.Remote, &remoteBool); err == nil {
		if remoteBool {
			j.Remote = "Remote"
		} else {
			j.Remote = "Not remote"
		}
		return nil
	}

	var remoteList []string
	if err := json.Unmarshal(raw.Remote, &remoteList); err == nil {
		j.Remote = strings.Join(remoteList, ", ")
		return nil
	}

	return fmt.Errorf("unsupported remote field format")
}

func parseJobIdentityMetadata(raw json.RawMessage) *JobIdentityMetadata {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil
	}
	metadata := &JobIdentityMetadata{
		Website:  parseJobIdentityEvidence(fields["website"]),
		Summary:  parseJobIdentityEvidence(firstRawMessage(fields, "summary", "company_summary", "about_company")),
		Industry: parseJobIdentityEvidence(firstRawMessage(fields, "industry", "company_industry")),
	}
	if metadata.Website == nil && metadata.Summary == nil && metadata.Industry == nil {
		return nil
	}
	return metadata
}

func parseJobIdentityEvidence(raw json.RawMessage) *JobIdentityEvidence {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil
		}
		return &JobIdentityEvidence{Value: value}
	}
	var evidence JobIdentityEvidence
	if err := json.Unmarshal(raw, &evidence); err != nil {
		return nil
	}
	evidence.Value = strings.TrimSpace(evidence.Value)
	if evidence.Value == "" {
		return nil
	}
	evidence.Source = strings.TrimSpace(evidence.Source)
	evidence.URL = strings.TrimSpace(evidence.URL)
	evidence.Confidence = strings.TrimSpace(evidence.Confidence)
	evidence.Reason = strings.TrimSpace(evidence.Reason)
	return &evidence
}

func firstRawMessage(fields map[string]json.RawMessage, keys ...string) json.RawMessage {
	for _, key := range keys {
		if raw, ok := fields[key]; ok {
			return raw
		}
	}
	return nil
}

func (j Job) MarshalJSON() ([]byte, error) {
	type wireJob struct {
		Company         string               `json:"company"`
		CompanyWebsite  string               `json:"company_website,omitempty"`
		CompanySummary  string               `json:"company_summary,omitempty"`
		CompanyIndustry string               `json:"company_industry,omitempty"`
		CompanyIdentity *JobIdentityMetadata `json:"company_identity,omitempty"`
		Title           string               `json:"title"`
		Remote          string               `json:"remote"`
		Compensation    string               `json:"compensation"`
		Source          string               `json:"source"`
		ApplyURL        string               `json:"apply_url"`
		WhyMatches      []string             `json:"why_matches,omitempty"`
		Status          string               `json:"status"`
		DateAdded       int64                `json:"date_added,omitempty"`
		Description     string               `json:"description,omitempty"`
	}

	return json.Marshal(wireJob{
		Company:         j.Company,
		CompanyWebsite:  j.CompanyWebsite,
		CompanySummary:  j.CompanySummary,
		CompanyIndustry: j.CompanyIndustry,
		CompanyIdentity: j.CompanyIdentity,
		Title:           j.Title,
		Remote:          j.Remote,
		Compensation:    j.Compensation,
		Source:          j.Source,
		ApplyURL:        j.ApplyURL,
		WhyMatches:      j.WhyMatches,
		Status:          j.Status,
		DateAdded:       j.DateAdded,
		Description:     j.Description,
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func formatUnixDate(ts int64) string {
	if ts <= 0 {
		return ""
	}
	return time.Unix(ts, 0).Format("2006-01-02")
}

func unixFromDateString(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if len(value) > 10 {
		value = value[:10]
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return 0
	}
	return t.Unix()
}

func (j *Job) setDateFromString(value string) {
	j.DateDiscovered = ""
	j.DateAdded = 0
	if ts := unixFromDateString(value); ts > 0 {
		j.DateAdded = ts
		j.DateDiscovered = formatUnixDate(ts)
	}
}

func (j *Job) setDateAdded(ts int64) {
	j.DateAdded = ts
	j.DateDiscovered = formatUnixDate(ts)
}

func (j *Job) SetDateFromString(value string) {
	j.setDateFromString(value)
}

func (j *Job) SetDateAdded(ts int64) {
	j.setDateAdded(ts)
}

func FormatUnixDate(ts int64) string {
	return formatUnixDate(ts)
}

func UnixFromDateString(value string) int64 {
	return unixFromDateString(value)
}
