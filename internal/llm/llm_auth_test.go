package llm

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/tmc/langchaingo/llms"
)

func TestResolveLLMAuthEnv(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-token")

	appCfg := defaultAppConfig()
	appCfg.LLM.Provider = "openai"
	appCfg.LLM.Model = "gpt-4.1-mini"
	appCfg.LLM.Auth.Mode = llmAuthModeEnv
	appCfg.LLM.Auth.EnvVar = "OPENAI_API_KEY"

	value, targetEnv, err := resolveLLMAuth(&appCfg)
	if err != nil {
		t.Fatalf("resolveLLMAuth() error = %v", err)
	}
	if value != "env-token" {
		t.Fatalf("resolveLLMAuth() value = %q; want env-token", value)
	}
	if targetEnv != "OPENAI_API_KEY" {
		t.Fatalf("resolveLLMAuth() targetEnv = %q; want OPENAI_API_KEY", targetEnv)
	}
}

func TestResolveLLMAuthLiteral(t *testing.T) {
	appCfg := defaultAppConfig()
	appCfg.LLM.Provider = "openai"
	appCfg.LLM.Model = "gpt-4.1-mini"
	appCfg.LLM.Auth.Mode = llmAuthModeLiteral
	appCfg.LLM.Auth.Value = "literal-token"

	value, targetEnv, err := resolveLLMAuth(&appCfg)
	if err != nil {
		t.Fatalf("resolveLLMAuth() error = %v", err)
	}
	if value != "literal-token" {
		t.Fatalf("resolveLLMAuth() value = %q; want literal-token", value)
	}
	if targetEnv != "OPENAI_API_KEY" {
		t.Fatalf("resolveLLMAuth() targetEnv = %q; want OPENAI_API_KEY", targetEnv)
	}
}

func TestResolveLLMAuthCommand(t *testing.T) {
	appCfg := defaultAppConfig()
	appCfg.LLM.Provider = "anthropic"
	appCfg.LLM.Model = "claude-3-5-haiku-latest"
	appCfg.LLM.Auth.Mode = llmAuthModeCommand
	appCfg.LLM.Auth.Command = "printf 'command-token'"

	value, targetEnv, err := resolveLLMAuth(&appCfg)
	if err != nil {
		t.Fatalf("resolveLLMAuth() error = %v", err)
	}
	if value != "command-token" {
		t.Fatalf("resolveLLMAuth() value = %q; want command-token", value)
	}
	if targetEnv != "ANTHROPIC_API_KEY" {
		t.Fatalf("resolveLLMAuth() targetEnv = %q; want ANTHROPIC_API_KEY", targetEnv)
	}
}

func TestApplyResolvedLLMAuthRestoresPreviousValue(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "previous-token")

	appCfg := defaultAppConfig()
	appCfg.LLM.Provider = "openai"
	appCfg.LLM.Model = "gpt-4.1-mini"
	appCfg.LLM.Auth.Mode = llmAuthModeLiteral
	appCfg.LLM.Auth.Value = "temporary-token"

	restore, err := applyResolvedLLMAuth(&appCfg)
	if err != nil {
		t.Fatalf("applyResolvedLLMAuth() error = %v", err)
	}
	if got := getenvOrEmpty("OPENAI_API_KEY"); got != "temporary-token" {
		t.Fatalf("OPENAI_API_KEY during apply = %q; want temporary-token", got)
	}

	restore()

	if got := getenvOrEmpty("OPENAI_API_KEY"); got != "previous-token" {
		t.Fatalf("OPENAI_API_KEY after restore = %q; want previous-token", got)
	}
}

func TestEffectiveLLMProviderForTaskUsesTaskModel(t *testing.T) {
	appCfg := defaultAppConfig()
	appCfg.LLM.Provider = "gemini"
	appCfg.LLM.Providers["gemini"] = LLMProviderConfig{
		Model: "gemini-default",
		Auth: LLMAuthConfig{
			Mode:   llmAuthModeEnv,
			EnvVar: "GEMINI_API_KEY",
		},
		Models: map[string]string{
			"filtering":          "gemini-filter",
			"resume-to-criteria": "gemini-resume",
			"health":             "gemini-health",
		},
	}

	_, providerCfg, ok := effectiveLLMProviderForTask(&appCfg, llmTaskFiltering)
	if !ok {
		t.Fatal("effectiveLLMProviderForTask(filtering) ok = false, want true")
	}
	if providerCfg.Model != "gemini-filter" {
		t.Fatalf("effectiveLLMProviderForTask(filtering).Model = %q, want gemini-filter", providerCfg.Model)
	}

	_, providerCfg, ok = effectiveLLMProviderForTask(&appCfg, llmTaskResumeCriteria)
	if !ok {
		t.Fatal("effectiveLLMProviderForTask(resume_to_criteria) ok = false, want true")
	}
	if providerCfg.Model != "gemini-resume" {
		t.Fatalf("effectiveLLMProviderForTask(resume_to_criteria).Model = %q, want gemini-resume", providerCfg.Model)
	}

	_, providerCfg, ok = effectiveLLMProviderForTask(&appCfg, llmTaskCompanyHealth)
	if !ok {
		t.Fatal("effectiveLLMProviderForTask(company_health) ok = false, want true")
	}
	if providerCfg.Model != "gemini-health" {
		t.Fatalf("effectiveLLMProviderForTask(company_health).Model = %q, want gemini-health", providerCfg.Model)
	}

	_, providerCfg, ok = effectiveLLMProviderForTask(&appCfg, "unconfigured_task")
	if !ok {
		t.Fatal("effectiveLLMProviderForTask(unconfigured_task) ok = false, want true")
	}
	if providerCfg.Model != "gemini-default" {
		t.Fatalf("effectiveLLMProviderForTask(unconfigured_task).Model = %q, want gemini-default", providerCfg.Model)
	}
}

func TestEffectiveLLMProviderForTaskFallsBackToProviderModel(t *testing.T) {
	appCfg := defaultAppConfig()
	appCfg.LLM.Provider = "openai"
	appCfg.LLM.Providers["openai"] = LLMProviderConfig{
		Model: "gpt-provider-default",
		Auth: LLMAuthConfig{
			Mode:   llmAuthModeEnv,
			EnvVar: "OPENAI_API_KEY",
		},
	}

	_, providerCfg, ok := effectiveLLMProviderForTask(&appCfg, llmTaskFiltering)
	if !ok {
		t.Fatal("effectiveLLMProviderForTask(filtering) ok = false, want true")
	}
	if providerCfg.Model != "gpt-provider-default" {
		t.Fatalf("effectiveLLMProviderForTask(filtering).Model = %q, want gpt-provider-default", providerCfg.Model)
	}
}

func TestEffectiveLLMProviderRequiresEnabled(t *testing.T) {
	appCfg := defaultAppConfig()
	appCfg.LLM.Enabled = false
	appCfg.LLM.JobSearch = true
	appCfg.LLM.JobFiltering = true

	_, _, ok := effectiveLLMProvider(&appCfg)
	if ok {
		t.Fatal("effectiveLLMProvider(...) ok = true; want false when LLM is disabled")
	}
}

func TestNormalizeLLMConfigMigratesTopLevelTaskModelsToProvider(t *testing.T) {
	appCfg := defaultAppConfig()
	appCfg.LLM.Provider = "gemini"
	appCfg.LLM.Models = map[string]string{
		"LLM job filtering": "gemini-filter",
	}

	normalizeLLMConfig(&appCfg)

	if len(appCfg.LLM.Models) != 0 {
		t.Fatalf("LLM.Models = %#v, want migrated away from top level", appCfg.LLM.Models)
	}
	if got := appCfg.LLM.Providers["gemini"].Models[llmTaskFiltering]; got != "gemini-filter" {
		t.Fatalf("provider Models[%q] = %q, want gemini-filter", llmTaskFiltering, got)
	}
}

func TestNormalizeLLMConfigPreservesExplicitDisabled(t *testing.T) {
	appCfg := defaultAppConfig()
	appCfg.LLM.Enabled = false
	appCfg.LLM.JobSearch = true
	appCfg.LLM.JobFiltering = true
	appCfg.LLM.CompanyHealth = true

	normalizeLLMConfig(&appCfg)

	if appCfg.LLM.Enabled {
		t.Fatal("normalizeLLMConfig(...).LLM.Enabled = true; want false")
	}
}

type closeableTestLLM struct {
	closed bool
	err    error
}

func (m *closeableTestLLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	return &llms.ContentResponse{}, nil
}

func (m *closeableTestLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return "", nil
}

func (m *closeableTestLLM) Close() error {
	m.closed = true
	return m.err
}

func TestCleanupConfiguredLLMClosesModelAndRestoresAuth(t *testing.T) {
	model := &closeableTestLLM{}
	restored := false

	cleanupConfiguredLLM(model, func() {
		restored = true
	})()

	if !model.closed {
		t.Fatal("model.closed = false; want close called")
	}
	if !restored {
		t.Fatal("restored = false; want auth restore called")
	}
}

func TestCleanupConfiguredLLMRestoresAuthAfterCloseError(t *testing.T) {
	model := &closeableTestLLM{err: errors.New("close failed")}
	restored := false

	cleanupConfiguredLLM(model, func() {
		restored = true
	})()

	if !model.closed {
		t.Fatal("model.closed = false; want close called")
	}
	if !restored {
		t.Fatal("restored = false; want auth restore called even after close error")
	}
}

func getenvOrEmpty(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}
