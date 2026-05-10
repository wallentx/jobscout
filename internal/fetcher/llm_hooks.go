package fetcher

import (
	"context"

	"github.com/tmc/langchaingo/llms"
)

const llmTaskJobSearch = "llm_job_search"
const llmTaskJobIdentity = "job_identity"

type InitLLMFunc func(ctx context.Context, appCfg *AppConfig, task string) (llms.Model, func(), error)
type ExecuteLLMSearchFunc func(ctx context.Context, llm llms.Model, prompt string) ([]Job, error)
type ExecuteLLMWebSearchFunc func(ctx context.Context, appCfg *AppConfig, prompt string) ([]Job, error)
type EnrichJobIdentityFunc func(ctx context.Context, llm llms.Model, job Job, page JobIdentityPage) (*JobIdentityEnrichment, LLMTokenUsage, error)

var (
	fetchAllJobsInitConfiguredLLM   InitLLMFunc
	fetchAllJobsExecuteLLMSearch    ExecuteLLMSearchFunc
	fetchAllJobsExecuteLLMWebSearch ExecuteLLMWebSearchFunc
	fetchAllJobsEnrichJobIdentity   EnrichJobIdentityFunc
)

func ConfigureLLM(init InitLLMFunc, search ExecuteLLMSearchFunc, enrichIdentity EnrichJobIdentityFunc) {
	fetchAllJobsInitConfiguredLLM = init
	fetchAllJobsExecuteLLMSearch = search
	fetchAllJobsEnrichJobIdentity = enrichIdentity
}

func ConfigureLLMWebSearch(search ExecuteLLMWebSearchFunc) {
	fetchAllJobsExecuteLLMWebSearch = search
}
