package fetcher

import (
	"context"
	"fmt"
	"sync"

	"github.com/tmc/langchaingo/llms"
)

const llmTaskJobSearch = "llm_job_search"
const llmTaskJobIdentity = "job_identity"

type InitLLMFunc func(ctx context.Context, appCfg *AppConfig, task string) (llms.Model, func(), error)
type ExecuteLLMSearchFunc func(ctx context.Context, llm llms.Model, prompt string) ([]Job, error)
type ExecuteLLMWebSearchFunc func(ctx context.Context, appCfg *AppConfig, prompt string) ([]Job, error)
type EnrichJobIdentityFunc func(ctx context.Context, llm llms.Model, job Job, page JobIdentityPage) (*JobIdentityEnrichment, LLMTokenUsage, error)

type LLMService interface {
	InitConfiguredLLM(ctx context.Context, appCfg *AppConfig, task string) (llms.Model, func(), error)
	ExecuteSearch(ctx context.Context, llm llms.Model, prompt string) ([]Job, error)
	ExecuteWebSearch(ctx context.Context, appCfg *AppConfig, prompt string) ([]Job, error)
	EnrichJobIdentity(ctx context.Context, llm llms.Model, job Job, page JobIdentityPage) (*JobIdentityEnrichment, LLMTokenUsage, error)
	IsAvailable(ctx context.Context, task string) bool
}

type llmServiceContextKey struct{}

var contextKey = llmServiceContextKey{}

func WithLLMService(ctx context.Context, service LLMService) context.Context {
	return context.WithValue(ctx, contextKey, service)
}

var (
	globalServiceMutex sync.RWMutex
	globalLLMService   LLMService
)

func RegisterLLMService(service LLMService) {
	globalServiceMutex.Lock()
	defer globalServiceMutex.Unlock()
	globalLLMService = service
}

func getLLMService(ctx context.Context) LLMService {
	if svc, ok := ctx.Value(contextKey).(LLMService); ok {
		return svc
	}
	globalServiceMutex.RLock()
	defer globalServiceMutex.RUnlock()
	if globalLLMService != nil {
		return globalLLMService
	}
	return fallbackLLMService{}
}

type fallbackLLMService struct{}

func (fallbackLLMService) InitConfiguredLLM(ctx context.Context, appCfg *AppConfig, task string) (llms.Model, func(), error) {
	if fetchAllJobsInitConfiguredLLM != nil {
		return fetchAllJobsInitConfiguredLLM(ctx, appCfg, task)
	}
	return nil, nil, fmt.Errorf("LLM init hook not configured")
}

func (fallbackLLMService) ExecuteSearch(ctx context.Context, llm llms.Model, prompt string) ([]Job, error) {
	if fetchAllJobsExecuteLLMSearch != nil {
		return fetchAllJobsExecuteLLMSearch(ctx, llm, prompt)
	}
	return nil, fmt.Errorf("LLM search hook not configured")
}

func (fallbackLLMService) ExecuteWebSearch(ctx context.Context, appCfg *AppConfig, prompt string) ([]Job, error) {
	if fetchAllJobsExecuteLLMWebSearch != nil {
		return fetchAllJobsExecuteLLMWebSearch(ctx, appCfg, prompt)
	}
	if fetchAllJobsInitConfiguredLLM == nil || fetchAllJobsExecuteLLMSearch == nil {
		return nil, fmt.Errorf("LLM web search hook not configured")
	}
	llm, restoreAuth, err := fetchAllJobsInitConfiguredLLM(ctx, appCfg, llmTaskJobSearch)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}
	defer restoreAuth()
	return fetchAllJobsExecuteLLMSearch(ctx, llm, prompt)
}

func (fallbackLLMService) EnrichJobIdentity(ctx context.Context, llm llms.Model, job Job, page JobIdentityPage) (*JobIdentityEnrichment, LLMTokenUsage, error) {
	if fetchAllJobsEnrichJobIdentity != nil {
		return fetchAllJobsEnrichJobIdentity(ctx, llm, job, page)
	}
	return nil, LLMTokenUsage{}, fmt.Errorf("LLM identity enrichment hook not configured")
}

func (fallbackLLMService) IsAvailable(ctx context.Context, task string) bool {
	if task == "llm_web_search" {
		return fetchAllJobsExecuteLLMWebSearch != nil || (fetchAllJobsInitConfiguredLLM != nil && fetchAllJobsExecuteLLMSearch != nil)
	}
	if task == "llm_job_search" {
		return fetchAllJobsInitConfiguredLLM != nil && fetchAllJobsExecuteLLMSearch != nil
	}
	if task == "llm_job_enrichment" {
		return fetchAllJobsInitConfiguredLLM != nil && fetchAllJobsEnrichJobIdentity != nil
	}
	return false
}

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
