package fetcher

import (
	"context"
	"fmt"
	"sync"
)

func newJobIdentityLLMEnricher(ctx context.Context, appCfg *AppConfig) (jobIdentityPageEnrichFunc, func(), string) {
	llmSvc := getLLMService(ctx)
	if appCfg == nil || !appCfg.LLM.Enabled || !llmSvc.IsAvailable(ctx, LLMTaskJobEnrichment) {
		return nil, nil, ""
	}
	llm, restoreAuth, err := llmSvc.InitConfiguredLLM(ctx, appCfg, llmTaskJobIdentity)
	if err != nil {
		return nil, nil, fmt.Sprintf("LLM job identity enrichment skipped: %v", err)
	}
	var mu sync.Mutex
	enrich := func(ctx context.Context, job Job, page JobIdentityPage) (*JobIdentityEnrichment, LLMTokenUsage, error) {
		mu.Lock()
		defer mu.Unlock()
		return llmSvc.EnrichJobIdentity(ctx, llm, job, page)
	}
	return enrich, restoreAuth, ""
}
