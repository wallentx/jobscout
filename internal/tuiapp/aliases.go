package tuiapp

import (
	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/domain"
	"github.com/wallentx/jobscout/internal/fetcher"
	"github.com/wallentx/jobscout/internal/storage"
)

type AppConfig = config.AppConfig
type CriteriaConfig = domain.CriteriaConfig
type WorkSettingsConfig = domain.WorkSettingsConfig
type Job = domain.Job
type FetchSummary = fetcher.FetchSummary
type CompanyHealthResult = domain.CompanyHealthResult
type CompanyHealthContext = domain.CompanyHealthContext
type LayoffSignal = domain.LayoffSignal
type HNSignal = domain.HNSignal
type EmploymentRisk = domain.EmploymentRisk
type LLMCompanyHealthAssessment = domain.LLMCompanyHealthAssessment
type LLMTokenUsage = domain.LLMTokenUsage
type CompanyHealthEvidence = domain.CompanyHealthEvidence
type CompanyHealthFieldAssessment = domain.CompanyHealthFieldAssessment
type HealthCache = storage.HealthCache
type HealthCacheEntry = storage.HealthCacheEntry
type JobStore = storage.JobStore
type HealthStore = storage.HealthStore
type CompanyIdentityStore = storage.CompanyIdentityStore
type LLMAuthConfig = config.LLMAuthConfig
type LLMConfig = config.LLMConfig
type LLMProviderConfig = config.LLMProviderConfig
type RoleFamilyID = domain.RoleFamilyID
type ReviewSession = fetcher.ReviewSession

const (
	RoleFrontendEngineering  = domain.RoleFrontendEngineering
	RoleBackendEngineering   = domain.RoleBackendEngineering
	RoleFullStackEngineering = domain.RoleFullStackEngineering
	RoleDevOpsSRESystems     = domain.RoleDevOpsSRESystems
	RoleAIMLEngineering      = domain.RoleAIMLEngineering
	RoleData                 = domain.RoleData
	RoleDesign               = domain.RoleDesign
	RoleProductManagement    = domain.RoleProductManagement
	RoleOtherSpecialized     = domain.RoleOtherSpecialized
)
