package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const manualModelOption = "Enter model manually..."

const ManualModelOption = manualModelOption

func setupModelOptions(provider string, cfg *AppConfig, fetched map[string][]string) []string {
	models := modelOptionsForProvider(provider)
	if fetched != nil {
		if providerModels := fetched[strings.ToLower(strings.TrimSpace(provider))]; len(providerModels) > 0 {
			models = providerModels
		}
	}
	if cfg != nil {
		if current := strings.TrimSpace(cfg.LLM.Model); current != "" {
			models = appendUniqueString(models, current)
		}
	}
	models = appendUniqueString(models, manualModelOption)
	return models
}

func SetupModelOptions(provider string, cfg *AppConfig, fetched map[string][]string) []string {
	return setupModelOptions(provider, cfg, fetched)
}

func appendUniqueString(items []string, item string) []string {
	item = strings.TrimSpace(item)
	if item == "" {
		return items
	}
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}

func fetchAvailableLLMModels(ctx context.Context, cfg AppConfig) ([]string, error) {
	normalizeLLMConfig(&cfg)
	provider, providerCfg, ok := effectiveLLMProvider(&cfg)
	if !ok {
		return nil, fmt.Errorf("no effective llm provider is configured")
	}

	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		return fetchOpenAIModels(ctx, &cfg)
	case "anthropic":
		return fetchAnthropicModels(ctx, &cfg)
	case "gemini", "googleai":
		return fetchGeminiModels(ctx, &cfg)
	case "openrouter":
		return fetchOpenRouterModels(ctx, &cfg)
	case "ollama":
		return fetchOllamaModels(ctx, providerCfg)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", provider)
	}
}

func FetchAvailableLLMModels(ctx context.Context, cfg AppConfig) ([]string, error) {
	return fetchAvailableLLMModels(ctx, cfg)
}

func modelListHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

func fetchOpenAIModels(ctx context.Context, cfg *AppConfig) ([]string, error) {
	apiKey, _, err := resolveLLMAuth(cfg)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.openai.com/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := doModelListRequest(req, &resp); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(resp.Data))
	for _, model := range resp.Data {
		id := strings.TrimSpace(model.ID)
		if isOpenAITextModel(id) {
			models = append(models, id)
		}
	}
	sortModels(models)
	return models, nil
}

func isOpenAITextModel(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return false
	}
	excluded := []string{
		"audio",
		"codex",
		"deep-research",
		"embedding",
		"image",
		"instruct",
		"moderation",
		"realtime",
		"search-preview",
		"transcribe",
		"tts",
		"whisper",
	}
	for _, token := range excluded {
		if strings.Contains(id, token) {
			return false
		}
	}
	return strings.Contains(id, "chat") ||
		strings.HasPrefix(id, "gpt-4.1") ||
		strings.HasPrefix(id, "gpt-4o") ||
		strings.HasPrefix(id, "o1") ||
		strings.HasPrefix(id, "o3")
}

func fetchAnthropicModels(ctx context.Context, cfg *AppConfig) ([]string, error) {
	apiKey, _, err := resolveLLMAuth(cfg)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.anthropic.com/v1/models?limit=1000", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := doModelListRequest(req, &resp); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(resp.Data))
	for _, model := range resp.Data {
		models = appendUniqueString(models, model.ID)
	}
	return models, nil
}

func fetchGeminiModels(ctx context.Context, cfg *AppConfig) ([]string, error) {
	apiKey, _, err := resolveLLMAuth(cfg)
	if err != nil {
		return nil, err
	}
	endpoint := "https://generativelanguage.googleapis.com/v1beta/models?pageSize=1000&key=" + url.QueryEscape(apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Models []struct {
			Name                       string   `json:"name"`
			DisplayName                string   `json:"displayName"`
			Description                string   `json:"description"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := doModelListRequest(req, &resp); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(resp.Models))
	for _, model := range resp.Models {
		if !stringSliceContains(model.SupportedGenerationMethods, "generateContent") {
			continue
		}
		id := strings.TrimPrefix(strings.TrimSpace(model.Name), "models/")
		if !isGeminiTextModel(id, model.DisplayName, model.Description) {
			continue
		}
		models = appendUniqueString(models, id)
	}
	sortModels(models)
	return models, nil
}

func isGeminiTextModel(id string, displayName string, description string) bool {
	text := strings.ToLower(strings.Join([]string{id, displayName, description}, " "))
	excluded := []string{
		"banana",
		"computer use",
		"computer-use",
		"embedding",
		"imagen",
		"image generation",
		"image-generation",
		"image editing",
		"image-editing",
		"native image",
		"robotics",
		"speech",
		"text-to-speech",
		"tts",
		"veo",
		"video",
	}
	for _, token := range excluded {
		if strings.Contains(text, token) {
			return false
		}
	}
	deprecatedIDs := []string{
		"gemini-2.0-flash-001",
		"gemini-2.0-flash-lite",
		"gemini-2.0-flash-lite-001",
	}
	for _, deprecatedID := range deprecatedIDs {
		if id == deprecatedID {
			return false
		}
	}
	return strings.HasPrefix(id, "gemini-")
}

func fetchOllamaModels(ctx context.Context, providerCfg LLMProviderConfig) ([]string, error) {
	endpoint := strings.TrimSpace(providerCfg.Endpoint)
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}
	endpoint = strings.TrimRight(endpoint, "/") + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := doModelListRequest(req, &resp); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(resp.Models))
	for _, model := range resp.Models {
		models = appendUniqueString(models, model.Name)
	}
	sortModels(models)
	return models, nil
}

func fetchOpenRouterModels(ctx context.Context, cfg *AppConfig) ([]string, error) {
	apiKey, _, err := resolveLLMAuth(cfg)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	var resp struct {
		Data []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Pricing struct {
				Prompt string `json:"prompt"`
			} `json:"pricing"`
		} `json:"data"`
	}
	if err := doModelListRequest(req, &resp); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(resp.Data))
	for _, model := range resp.Data {
		id := strings.TrimSpace(model.ID)
		if id == "" || !isOpenRouterChatModel(id) {
			continue
		}
		models = appendUniqueString(models, id)
	}
	sortModels(models)
	return models, nil
}

func isOpenRouterChatModel(id string) bool {
	lower := strings.ToLower(id)
	excluded := []string{
		"embedding",
		"moderation",
		"whisper",
		"tts",
		"dall-e",
		"veo",
		"janus",
		"/embed",
	}
	for _, token := range excluded {
		if strings.Contains(lower, token) {
			return false
		}
	}
	return true
}

func doModelListRequest(req *http.Request, target any) error {
	resp, err := modelListHTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("model list request failed: %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return err
	}
	return nil
}

func stringSliceContains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func sortModels(models []string) {
	sort.SliceStable(models, func(i int, j int) bool {
		return models[i] > models[j]
	})
}
