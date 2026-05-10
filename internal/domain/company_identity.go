package domain

import (
	"encoding/json"
	"time"
)

type CompanyIdentity struct {
	Key              string          `json:"key"`
	DisplayName      string          `json:"display_name"`
	Website          string          `json:"website,omitempty"`
	Summary          string          `json:"summary,omitempty"`
	Industry         string          `json:"industry,omitempty"`
	IdentityEvidence json.RawMessage `json:"identity_evidence,omitempty"`
	SourceVersion    string          `json:"source_version,omitempty"`
	CreatedAt        time.Time       `json:"created_at,omitempty"`
	UpdatedAt        time.Time       `json:"updated_at,omitempty"`
	NameAliases      []string        `json:"name_aliases,omitempty"`
	DomainAliases    []string        `json:"domain_aliases,omitempty"`
}
