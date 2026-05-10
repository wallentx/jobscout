package llm

import (
	"context"
	"fmt"
	"strings"
)

func benchmarkModelsForRun(ctx context.Context, appCfg *AppConfig, opts benchmarkCLIOptions) ([]string, error) {
	provider, providerCfg, ok := effectiveLLMProvider(appCfg)
	if !ok {
		return nil, fmt.Errorf("no effective llm provider is configured")
	}
	current := strings.TrimSpace(providerCfg.Model)
	if !opts.AllModels {
		if current == "" {
			return nil, fmt.Errorf("no model is configured for %s", provider)
		}
		return []string{current}, nil
	}

	models, err := fetchAvailableLLMModels(ctx, *appCfg)
	if err != nil || len(models) == 0 {
		models = modelOptionsForProvider(provider)
	}
	models = appendUniqueString(models, current)
	models = filterBenchmarkModelList(models)
	if len(models) == 0 {
		return nil, fmt.Errorf("no models available for %s", provider)
	}
	return models, nil
}

func filterBenchmarkModelList(models []string) []string {
	out := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" || model == manualModelOption {
			continue
		}
		out = appendUniqueString(out, model)
	}
	sortModels(out)
	return out
}

func benchmarkConfigForModel(appCfg AppConfig, provider string, modelName string) AppConfig {
	normalizeLLMConfig(&appCfg)
	provider = strings.TrimSpace(provider)
	providerCfg := normalizeLLMProviderConfig(provider, appCfg.LLM.Providers[provider])
	providerCfg.Model = modelName
	appCfg.LLM.Enabled = true
	appCfg.LLM.Provider = provider
	appCfg.LLM.Model = modelName
	appCfg.LLM.Providers[provider] = providerCfg
	return appCfg
}
