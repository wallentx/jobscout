package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDemoAppConfigIsReadyForNonLLMFetch(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	cfg := DemoAppConfig()
	caps := EvaluateCapabilitiesForConfig(&cfg, true)

	if !caps.SearchProfileReady {
		t.Fatal("EvaluateCapabilitiesForConfig(DemoAppConfig(), true).SearchProfileReady = false; want true")
	}
	if !caps.SearchSourcesReady {
		t.Fatal("EvaluateCapabilitiesForConfig(DemoAppConfig(), true).SearchSourcesReady = false; want true")
	}
	if !caps.CanRunNonLLM {
		t.Fatal("EvaluateCapabilitiesForConfig(DemoAppConfig(), true).CanRunNonLLM = false; want true")
	}
	if caps.LLMAuthAvailableNow {
		t.Fatal("EvaluateCapabilitiesForConfig(DemoAppConfig(), true).LLMAuthAvailableNow = true; want false without provider env vars")
	}
	if cfg.Sources.LLMWeb.Enabled {
		t.Fatal("DemoAppConfig().Sources.LLMWeb.Enabled = true; want false")
	}
	for i, source := range cfg.Sources.APIs {
		if source.Enabled {
			t.Fatalf("DemoAppConfig().Sources.APIs[%d].Enabled = true; want false", i)
		}
	}
}

func TestDemoAppConfigUsesAvailableProviderEnv(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "openai-token")
	t.Setenv("ANTHROPIC_API_KEY", "")

	cfg := DemoAppConfig()
	if cfg.LLM.Provider != "openai" {
		t.Fatalf("DemoAppConfig().LLM.Provider = %q; want openai", cfg.LLM.Provider)
	}
	if cfg.LLM.Auth.EnvVar != "OPENAI_API_KEY" {
		t.Fatalf("DemoAppConfig().LLM.Auth.EnvVar = %q; want OPENAI_API_KEY", cfg.LLM.Auth.EnvVar)
	}
	if !LLMAuthAvailableNow(&cfg) {
		t.Fatal("LLMAuthAvailableNow(DemoAppConfig()) = false; want true with OPENAI_API_KEY")
	}
}

func TestInMemoryRuntimeConfigDoesNotWriteFiles(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	promptPath := filepath.Join(tmpDir, "SEARCH_PROMPT.md")
	existingConfig := []byte("criteria:\n  candidate:\n    city: Austin\n")
	existingPrompt := []byte("existing prompt")
	if err := os.WriteFile(configPath, existingConfig, 0600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", configPath, err)
	}
	if err := os.WriteFile(promptPath, existingPrompt, 0600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", promptPath, err)
	}

	cfg := DemoAppConfig()
	restore := ConfigureInMemoryRuntime(cfg, "")
	defer restore()

	loaded, err := LoadAppConfig(configPath)
	if err != nil {
		t.Fatalf("LoadAppConfig(%q) error = %v", configPath, err)
	}
	if loaded.Criteria.Candidate.City != "Seattle" {
		t.Fatalf("LoadAppConfig(%q).Criteria.Candidate.City = %q; want Seattle", configPath, loaded.Criteria.Candidate.City)
	}

	loaded.Criteria.Candidate.City = "Bellevue"
	if err := SaveAppConfig(configPath, loaded); err != nil {
		t.Fatalf("SaveAppConfig(%q) error = %v", configPath, err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", configPath, err)
	}
	if string(data) != string(existingConfig) {
		t.Fatalf("ReadFile(%q) = %q; want existing config unchanged", configPath, data)
	}

	reloaded, err := LoadAppConfig(configPath)
	if err != nil {
		t.Fatalf("LoadAppConfig(%q) after save error = %v", configPath, err)
	}
	if reloaded.Criteria.Candidate.City != "Bellevue" {
		t.Fatalf("LoadAppConfig(%q).Criteria.Candidate.City = %q; want Bellevue", configPath, reloaded.Criteria.Candidate.City)
	}

	if err := SaveSearchPrompt(promptPath, "demo prompt"); err != nil {
		t.Fatalf("SaveSearchPrompt(%q) error = %v", promptPath, err)
	}
	data, err = os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", promptPath, err)
	}
	if string(data) != string(existingPrompt) {
		t.Fatalf("ReadFile(%q) = %q; want existing prompt unchanged", promptPath, data)
	}
	prompt, err := LoadSearchPrompt(promptPath)
	if err != nil {
		t.Fatalf("LoadSearchPrompt(%q) error = %v", promptPath, err)
	}
	if prompt != "demo prompt" {
		t.Fatalf("LoadSearchPrompt(%q) = %q; want demo prompt", promptPath, prompt)
	}
}
