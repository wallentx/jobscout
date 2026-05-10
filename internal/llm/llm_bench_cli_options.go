package llm

import (
	"fmt"
	"strings"
)

type benchmarkCLIOptions struct {
	Task      string
	Provider  string
	Model     string
	JSON      bool
	List      bool
	AllModels bool
}

func parseBenchmarkCLIOptions(args []string) (benchmarkCLIOptions, error) {
	var opts benchmarkCLIOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			opts.JSON = true
		case arg == "--list":
			opts.List = true
		case arg == "--all-models":
			opts.AllModels = true
		case strings.HasPrefix(arg, "--task="):
			opts.Task = strings.TrimSpace(strings.TrimPrefix(arg, "--task="))
		case arg == "--task":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--task requires a value")
			}
			opts.Task = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--provider="):
			opts.Provider = strings.TrimSpace(strings.TrimPrefix(arg, "--provider="))
		case arg == "--provider":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--provider requires a value")
			}
			opts.Provider = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--model="):
			opts.Model = strings.TrimSpace(strings.TrimPrefix(arg, "--model="))
		case arg == "--model":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--model requires a value")
			}
			opts.Model = strings.TrimSpace(args[i+1])
			i++
		default:
			return opts, fmt.Errorf("unknown benchmark option %q", arg)
		}
	}
	if opts.AllModels && strings.TrimSpace(opts.Model) != "" {
		return opts, fmt.Errorf("--all-models cannot be combined with --model")
	}
	return opts, nil
}

func applyBenchmarkModelOverrides(appCfg *AppConfig, opts benchmarkCLIOptions) {
	if strings.TrimSpace(opts.Provider) != "" {
		appCfg.LLM.Enabled = true
		appCfg.LLM.Provider = strings.ToLower(strings.TrimSpace(opts.Provider))
		appCfg.LLM.Auth = LLMAuthConfig{}
		if appCfg.LLM.Providers == nil {
			appCfg.LLM.Providers = defaultLLMProviders()
		}
		if _, ok := appCfg.LLM.Providers[appCfg.LLM.Provider]; !ok {
			appCfg.LLM.Providers[appCfg.LLM.Provider] = LLMProviderConfig{
				Auth: LLMAuthConfig{
					Mode:   llmAuthModeEnv,
					EnvVar: envVarForProvider(appCfg.LLM.Provider),
				},
			}
		}
	}
	if strings.TrimSpace(opts.Model) != "" {
		provider := strings.TrimSpace(appCfg.LLM.Provider)
		providerCfg := normalizeLLMProviderConfig(provider, appCfg.LLM.Providers[provider])
		providerCfg.Model = strings.TrimSpace(opts.Model)
		appCfg.LLM.Providers[provider] = providerCfg
		appCfg.LLM.Model = providerCfg.Model
	}
	normalizeLLMConfig(appCfg)
}
