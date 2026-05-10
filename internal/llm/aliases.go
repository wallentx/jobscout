package llm

import (
	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/domain"
)

type AppConfig = config.AppConfig
type LLMAuthConfig = config.LLMAuthConfig
type LLMProviderConfig = config.LLMProviderConfig
type CriteriaConfig = domain.CriteriaConfig
type CompanyHealthEvidence = domain.CompanyHealthEvidence
type CompanyHealthResult = domain.CompanyHealthResult
type EmployerReviewSignal = domain.EmployerReviewSignal
type EmploymentRisk = domain.EmploymentRisk
type HNSignal = domain.HNSignal
type Job = domain.Job
type JobIdentityEnrichment = domain.JobIdentityEnrichment
type JobIdentityPage = domain.JobIdentityPage
type LayoffSignal = domain.LayoffSignal
type LLMCompanyHealthAssessment = domain.LLMCompanyHealthAssessment
type LLMTokenUsage = domain.LLMTokenUsage
type RoleFamilyID = domain.RoleFamilyID

const (
	RoleBackendEngineering = domain.RoleBackendEngineering
	RoleDevOpsSRESystems   = domain.RoleDevOpsSRESystems
)
