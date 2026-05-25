package llm

import (
	"context"

	"github.com/tmc/langchaingo/llms"
	"github.com/wallentx/jobscout/internal/fetcher"
)

type defaultLLMService struct{}

func NewLLMService() fetcher.LLMService {
	return defaultLLMService{}
}

func (defaultLLMService) InitConfiguredLLM(ctx context.Context, appCfg *fetcher.AppConfig, task string) (llms.Model, func(), error) {
	return InitConfiguredLLMForTask(ctx, appCfg, task)
}

func (defaultLLMService) ExecuteSearch(ctx context.Context, llm llms.Model, prompt string) ([]fetcher.Job, error) {
	return ExecuteLLMSearch(ctx, llm, prompt)
}

func (defaultLLMService) ExecuteWebSearch(ctx context.Context, appCfg *fetcher.AppConfig, prompt string) ([]fetcher.Job, error) {
	return ExecuteLLMWebSearch(ctx, appCfg, prompt)
}

func (defaultLLMService) EnrichJobIdentity(ctx context.Context, llm llms.Model, job fetcher.Job, page fetcher.JobIdentityPage) (*fetcher.JobIdentityEnrichment, fetcher.LLMTokenUsage, error) {
	return EnrichJobIdentityWithLLMUsage(ctx, llm, job, page)
}

func (defaultLLMService) IsAvailable(ctx context.Context, task string) bool {
	return true
}
