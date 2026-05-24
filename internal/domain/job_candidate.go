package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

const (
	CandidateDecisionAccepted = "accepted"
	CandidateDecisionRejected = "rejected"

	CandidateDecisionStageDeterministic = "deterministic"
	CandidateDecisionStageLLMFiltering  = "llm_job_filtering"

	CandidateDecisionVersionDeterministic = "deterministic:v1"
	CandidateDecisionVersionLLMFiltering  = "llm_job_filtering:v1"
)

type JobCandidate struct {
	Key               string    `json:"key"`
	Source            string    `json:"source,omitempty"`
	SourceKey         string    `json:"source_key,omitempty"`
	ApplyURL          string    `json:"apply_url,omitempty"`
	CanonicalApplyURL string    `json:"canonical_apply_url,omitempty"`
	Company           string    `json:"company,omitempty"`
	Title             string    `json:"title,omitempty"`
	Job               Job       `json:"job"`
	Active            bool      `json:"active"`
	FirstSeen         time.Time `json:"first_seen,omitempty"`
	LastSeen          time.Time `json:"last_seen,omitempty"`
}

type JobCandidateDecisionKey struct {
	CandidateKey    string
	CriteriaHash    string
	Stage           string
	DecisionVersion string
	LLMProvider     string
	LLMModel        string
}

type JobCandidateDecision struct {
	JobCandidateDecisionKey
	Decision  string
	Reason    string
	Job       Job
	DecidedAt time.Time
	ExpiresAt time.Time
}

func CriteriaHash(criteria *CriteriaConfig) string {
	if criteria == nil {
		return "none"
	}
	normalized := *criteria
	normalizeStringSlice(&normalized.Filters.TitleRequires)
	normalizeStringSlice(&normalized.Filters.TitleIncludes)
	normalizeStringSlice(&normalized.Filters.TitleExcludes)
	normalizeStringSlice(&normalized.Filters.IndustryIncludes)
	normalizeStringSlice(&normalized.Filters.IndustryExcludes)
	normalizeRoleFamilySlice(&normalized.RoleFamilies)
	normalizeStringSlice(&normalized.ResolvedSourceIDs)
	normalizeStringSlice(&normalized.PrioritySignals)

	data, err := json.Marshal(normalized)
	if err != nil {
		return "invalid"
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:16])
}

func normalizeStringSlice(values *[]string) {
	if values == nil || len(*values) == 0 {
		return
	}
	normalized := make([]string, 0, len(*values))
	seen := make(map[string]struct{}, len(*values))
	for _, value := range *values {
		value = strings.ToLower(strings.Join(strings.Fields(value), " "))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	sort.Strings(normalized)
	*values = normalized
}

func normalizeRoleFamilySlice(values *[]RoleFamilyID) {
	if values == nil || len(*values) == 0 {
		return
	}
	normalized := make([]RoleFamilyID, 0, len(*values))
	seen := make(map[RoleFamilyID]struct{}, len(*values))
	for _, value := range *values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i] < normalized[j]
	})
	*values = normalized
}
