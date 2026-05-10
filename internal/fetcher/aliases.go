package fetcher

import (
	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/domain"
)

type Job = domain.Job
type JobIdentityEnrichment = domain.JobIdentityEnrichment
type JobIdentityEvidence = domain.JobIdentityEvidence
type JobIdentityPage = domain.JobIdentityPage
type LLMTokenUsage = domain.LLMTokenUsage
type CriteriaConfig = domain.CriteriaConfig
type WorkSettingsConfig = domain.WorkSettingsConfig
type LayoffSignal = domain.LayoffSignal

type AppConfig = config.AppConfig
type RSSSource = config.RSSSource
type APISource = config.APISource

func defaultAppConfig() AppConfig {
	return config.DefaultAppConfig()
}

func defaultCriteriaConfig() CriteriaConfig {
	return config.DefaultCriteriaConfig()
}
