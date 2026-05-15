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

type llmModelInfo struct {
	ID          string
	AliasTarget string
	Aliases     []string
}

type AvailableLLMModels struct {
	infos []llmModelInfo
}

func newAvailableLLMModels(infos []llmModelInfo) AvailableLLMModels {
	return AvailableLLMModels{infos: normalizeLLMModelInfos(infos)}
}

func (models AvailableLLMModels) IDs() []string {
	return modelInfoIDs(models.infos)
}

func (models AvailableLLMModels) RunIDFor(id string) string {
	id = ModelRunIDFromOption(id)
	if id == "" {
		return ""
	}
	for _, info := range models.infos {
		runID := modelInfoRunID(info)
		if id == strings.TrimSpace(info.ID) || id == strings.TrimSpace(info.AliasTarget) {
			return runID
		}
		for _, alias := range info.Aliases {
			if id == strings.TrimSpace(alias) {
				return runID
			}
		}
	}
	return id
}

func (models AvailableLLMModels) OptionLabels() []string {
	return modelInfoOptionLabels(models.infos)
}

func (models AvailableLLMModels) Len() int {
	return len(models.infos)
}

func setupModelOptions(provider string, cfg *AppConfig, fetched map[string][]string) []string {
	models := modelOptionsForProvider(provider)
	if fetched != nil {
		if providerModels := fetched[strings.ToLower(strings.TrimSpace(provider))]; len(providerModels) > 0 {
			models = providerModels
		}
	}
	models = filterModelOptionsForProvider(provider, models)
	if cfg != nil {
		if current := strings.TrimSpace(cfg.LLM.Model); current != "" {
			if !shouldExcludeModelForProvider(provider, current) {
				models = appendUniqueModelOption(models, current)
			}
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

func appendUniqueModelOption(items []string, item string) []string {
	item = strings.TrimSpace(item)
	ids := ModelOptionIDs(item)
	if len(ids) == 0 {
		return items
	}
	for _, existing := range items {
		if modelOptionIDsOverlap(ModelOptionIDs(existing), ids) {
			return items
		}
	}
	return append(items, item)
}

func ModelIDFromOption(option string) string {
	option = strings.TrimSpace(option)
	if option == "" || option == manualModelOption {
		return option
	}
	if before, _, ok := strings.Cut(option, " ("); ok {
		option = before
	}
	if before, _, ok := strings.Cut(option, " -> "); ok {
		option = before
	}
	return strings.TrimSpace(option)
}

func ModelRunIDFromOption(option string) string {
	option = strings.TrimSpace(option)
	if option == "" || option == manualModelOption {
		return option
	}
	if _, target, ok := strings.Cut(option, " -> "); ok {
		return strings.TrimSpace(target)
	}
	return ModelIDFromOption(option)
}

func ModelOptionIDs(option string) []string {
	option = strings.TrimSpace(option)
	primary := ModelIDFromOption(option)
	if primary == "" {
		return nil
	}
	ids := []string{primary}
	if _, after, ok := strings.Cut(option, "(aliases: "); ok {
		aliases, _, _ := strings.Cut(after, ")")
		for _, alias := range strings.Split(aliases, ",") {
			ids = appendUniqueString(ids, alias)
		}
	}
	if _, target, ok := strings.Cut(option, " -> "); ok {
		ids = appendUniqueString(ids, target)
	}
	return ids
}

func modelOptionIDsOverlap(a []string, b []string) bool {
	for _, left := range a {
		for _, right := range b {
			if strings.TrimSpace(left) != "" && strings.TrimSpace(left) == strings.TrimSpace(right) {
				return true
			}
		}
	}
	return false
}

func fetchAvailableLLMModels(ctx context.Context, cfg AppConfig) (AvailableLLMModels, error) {
	infos, err := fetchAvailableLLMModelInfos(ctx, cfg)
	if err != nil {
		return AvailableLLMModels{}, err
	}
	return newAvailableLLMModels(infos), nil
}

func fetchAvailableLLMModelInfos(ctx context.Context, cfg AppConfig) ([]llmModelInfo, error) {
	normalizeLLMConfig(&cfg)
	provider, providerCfg, ok := effectiveLLMProvider(&cfg)
	if !ok {
		return nil, fmt.Errorf("no effective llm provider is configured")
	}

	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		return fetchOpenAIModelInfos(ctx, &cfg)
	case "anthropic":
		models, err := fetchAnthropicModels(ctx, &cfg)
		return modelInfosFromIDs(models), err
	case "gemini", "googleai":
		return fetchGeminiModelInfos(ctx, &cfg)
	case "openrouter":
		models, err := fetchOpenRouterModels(ctx, &cfg)
		return modelInfosFromIDs(models), err
	case "ollama":
		models, err := fetchOllamaModels(ctx, providerCfg)
		return modelInfosFromIDs(models), err
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", provider)
	}
}

func FetchAvailableLLMModels(ctx context.Context, cfg AppConfig) (AvailableLLMModels, error) {
	return fetchAvailableLLMModels(ctx, cfg)
}

func modelListHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

func fetchOpenAIModelInfos(ctx context.Context, cfg *AppConfig) ([]llmModelInfo, error) {
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
			ID        string `json:"id"`
			Root      string `json:"root"`
			Parent    string `json:"parent"`
			AliasFor  string `json:"alias_for"`
			Canonical string `json:"canonical"`
		} `json:"data"`
	}
	if err := doModelListRequest(req, &resp); err != nil {
		return nil, err
	}

	infos := make([]llmModelInfo, 0, len(resp.Data))
	for _, model := range resp.Data {
		id := strings.TrimSpace(model.ID)
		if isOpenAITextModel(id) {
			infos = append(infos, llmModelInfo{
				ID:          id,
				AliasTarget: openAIModelAliasTarget(id, model.AliasFor, model.Canonical, model.Root, model.Parent),
			})
		}
	}
	return infos, nil
}

func isOpenAITextModel(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return false
	}
	if shouldExcludeOpenAIModelID(id) {
		return false
	}
	excluded := []string{
		"audio",
		"babbage",
		"computer-use",
		"codex",
		"dall-e",
		"davinci",
		"deep-research",
		"embedding",
		"image",
		"instruct",
		"moderation",
		"realtime",
		"search-api",
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
	return strings.HasPrefix(id, "gpt-") ||
		strings.HasPrefix(id, "chatgpt-") ||
		isOpenAIReasoningModelID(id)
}

func isOpenAIReasoningModelID(id string) bool {
	if len(id) < 2 || id[0] != 'o' {
		return false
	}
	return id[1] >= '0' && id[1] <= '9'
}

func filterModelOptionsForProvider(provider string, models []string) []string {
	out := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		id := ModelIDFromOption(model)
		if id == "" || id == manualModelOption || shouldExcludeModelForProvider(provider, id) {
			continue
		}
		out = appendUniqueModelOption(out, model)
	}
	sortModelOptionsForProvider(provider, out)
	return out
}

func sortModelOptionsForProvider(provider string, models []string) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		sort.SliceStable(models, func(i int, j int) bool {
			left := openAIModelRecommendationRank(ModelIDFromOption(models[i]))
			right := openAIModelRecommendationRank(ModelIDFromOption(models[j]))
			if left != right {
				return left < right
			}
			return ModelIDFromOption(models[i]) > ModelIDFromOption(models[j])
		})
	}
}

func openAIModelRecommendationRank(model string) int {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "gpt-4o-mini":
		return 0
	case "gpt-5.4-mini":
		return 1
	case "gpt-5.4-nano":
		return 2
	case "gpt-4.1-mini":
		return 3
	case "gpt-4o":
		return 4
	case "gpt-4.1":
		return 5
	case "gpt-5.3-chat":
		return 6
	case "gpt-5.2-chat":
		return 7
	case "gpt-5.2":
		return 8
	case "gpt-5.1":
		return 9
	case "o3":
		return 10
	case "gpt-5.4":
		return 11
	case "gpt-5.5":
		return 12
	case "gpt-5-mini":
		return 13
	case "gpt-5":
		return 14
	case "gpt-5-nano":
		return 15
	default:
		return 100
	}
}

func shouldExcludeModelForProvider(provider string, model string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "gemini", "googleai":
		return shouldExcludeGeminiModelID(model)
	case "openai":
		return shouldExcludeOpenAIModelID(model)
	default:
		return false
	}
}

func shouldExcludeGeminiModelID(id string) bool {
	id = strings.ToLower(trimGeminiModelResourceName(id))
	if id == "" {
		return false
	}
	_, ok := deprecatedGeminiModelReplacements[id]
	return ok
}

// Keep this aligned with https://ai.google.dev/gemini-api/docs/deprecations.
var deprecatedGeminiModelReplacements = map[string]string{
	"gemini-flash-latest":                       "gemini-3-flash-preview",
	"gemini-flash-lite-latest":                  "gemini-3.1-flash-lite",
	"gemini-pro-latest":                         "gemini-3.1-pro-preview",
	"gemini-3.1-flash-lite-preview":             "gemini-3.1-flash-lite",
	"gemini-3-pro-preview":                      "gemini-3.1-pro-preview",
	"gemini-2.5-pro":                            "gemini-3.1-pro-preview",
	"gemini-2.5-pro-preview-03-25":              "gemini-3.1-pro-preview",
	"gemini-2.5-pro-preview-05-06":              "gemini-3.1-pro-preview",
	"gemini-2.5-pro-preview-06-05":              "gemini-3.1-pro-preview",
	"gemini-2.5-flash":                          "gemini-3-flash-preview",
	"gemini-2.5-flash-lite":                     "gemini-3.1-flash-lite",
	"gemini-2.5-flash-lite-preview-09-2025":     "gemini-3.1-flash-lite",
	"gemini-2.5-flash-preview-05-20":            "gemini-3-flash-preview",
	"gemini-2.5-flash-preview-09-25":            "gemini-3-flash-preview",
	"gemini-2.0-flash":                          "gemini-2.5-flash",
	"gemini-2.0-flash-001":                      "gemini-2.5-flash",
	"gemini-2.0-flash-lite":                     "gemini-2.5-flash-lite",
	"gemini-2.0-flash-lite-001":                 "gemini-2.5-flash-lite",
	"gemini-2.0-flash-lite-preview":             "gemini-2.5-flash-lite",
	"gemini-2.0-flash-lite-preview-02-05":       "gemini-2.5-flash-lite",
	"gemini-2.0-flash-preview-image-generation": "gemini-2.5-flash-image",
}

func shouldExcludeOpenAIModelID(id string) bool {
	return isOpenAIModelSnapshotID(id) ||
		isNonProductionOpenAIModel(id) ||
		isKnownDeprecatedOpenAIModel(id) ||
		isKnownUnsupportedOpenAIModel(id)
}

func isOpenAIModelSnapshotID(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return false
	}
	for i := 0; i+9 < len(id); i++ {
		if isDigitByte(id[i]) &&
			isDigitByte(id[i+1]) &&
			isDigitByte(id[i+2]) &&
			isDigitByte(id[i+3]) &&
			id[i+4] == '-' &&
			isDigitByte(id[i+5]) &&
			isDigitByte(id[i+6]) &&
			id[i+7] == '-' &&
			isDigitByte(id[i+8]) &&
			isDigitByte(id[i+9]) {
			return true
		}
	}
	_, suffix, ok := strings.Cut(strings.TrimPrefix(id, "ft:"), "-")
	if !ok {
		return false
	}
	parts := strings.Split(id, "-")
	last := parts[len(parts)-1]
	return isAllDigits(last) && (len(last) == 4 || len(last) == 6 || len(last) == 8) && suffix != ""
}

func isNonProductionOpenAIModel(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	return id == "chat-latest" || strings.HasSuffix(id, "-chat-latest")
}

func isKnownDeprecatedOpenAIModel(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		return false
	}
	deprecatedExact := map[string]struct{}{
		"babbage-002":                {},
		"chatgpt-4o":                 {},
		"codex-mini-latest":          {},
		"computer-use-preview":       {},
		"davinci-002":                {},
		"gpt-3.5-turbo":              {},
		"gpt-4.1-nano":               {},
		"gpt-4.5-preview":            {},
		"gpt-4-turbo":                {},
		"gpt-4-turbo-preview":        {},
		"gpt-4o-mini-search-preview": {},
		"gpt-4o-search-preview":      {},
		"gpt-5-chat":                 {},
		"gpt-5.1-chat":               {},
		"gpt-5-codex":                {},
		"gpt-5.1-codex":              {},
		"gpt-5.1-codex-max":          {},
		"gpt-5.1-codex-mini":         {},
		"gpt-5.2-codex":              {},
		"o1":                         {},
		"o1-mini":                    {},
		"o1-preview":                 {},
		"o1-pro":                     {},
		"o3-deep-research":           {},
		"o3-mini":                    {},
		"o4-mini":                    {},
		"o4-mini-deep-research":      {},
		"text-moderation":            {},
		"text-moderation-stable":     {},
	}
	if _, ok := deprecatedExact[id]; ok {
		return true
	}
	deprecatedPrefixes := []string{
		"chatgpt-4o-",
		"gpt-3.5-turbo-",
		"gpt-4.1-nano-",
		"gpt-4.5-preview-",
		"gpt-4-turbo-",
		"gpt-5-chat-",
		"gpt-5.1-chat-",
		"o1-",
		"o3-mini-",
		"o4-mini-",
	}
	for _, prefix := range deprecatedPrefixes {
		if strings.HasPrefix(id, prefix) {
			return true
		}
	}
	return false
}

func isKnownUnsupportedOpenAIModel(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	return id == "gpt-4" ||
		id == "o1" ||
		id == "o1-pro" ||
		strings.HasPrefix(id, "o1-pro-") ||
		id == "o3-pro" ||
		strings.HasPrefix(id, "o3-pro-") ||
		isOpenAIProModel(id)
}

func isOpenAIProModel(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	return strings.HasPrefix(id, "gpt-") && (strings.HasSuffix(id, "-pro") || strings.Contains(id, "-pro-"))
}

func isDigitByte(b byte) bool {
	return b >= '0' && b <= '9'
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if !isDigitByte(value[i]) {
			return false
		}
	}
	return true
}

func openAIModelAliasTarget(id string, candidates ...string) string {
	id = strings.TrimSpace(id)
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || candidate == id || !isOpenAITextModel(candidate) {
			continue
		}
		return candidate
	}
	return ""
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

func fetchGeminiModelInfos(ctx context.Context, cfg *AppConfig) ([]llmModelInfo, error) {
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
			BaseModelID                string   `json:"baseModelId"`
			Version                    string   `json:"version"`
			DisplayName                string   `json:"displayName"`
			Description                string   `json:"description"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := doModelListRequest(req, &resp); err != nil {
		return nil, err
	}

	infos := make([]llmModelInfo, 0, len(resp.Models))
	for _, model := range resp.Models {
		if !stringSliceContains(model.SupportedGenerationMethods, "generateContent") {
			continue
		}
		id := trimGeminiModelResourceName(model.Name)
		if !isGeminiTextModel(id, model.DisplayName, model.Description) {
			continue
		}
		aliasTarget := geminiModelAliasTarget(id, model.BaseModelID)
		if shouldExcludeGeminiModelID(aliasTarget) {
			continue
		}
		infos = append(infos, llmModelInfo{
			ID:          id,
			AliasTarget: aliasTarget,
		})
	}
	return infos, nil
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
	if shouldExcludeGeminiModelID(id) {
		return false
	}
	return strings.HasPrefix(id, "gemini-")
}

func geminiModelAliasTarget(id string, baseModelID string) string {
	id = trimGeminiModelResourceName(id)
	baseModelID = trimGeminiModelResourceName(baseModelID)
	if baseModelID == "" || baseModelID == id || !isGeminiTextModel(baseModelID, "", "") {
		return ""
	}
	return baseModelID
}

func trimGeminiModelResourceName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "models/") {
		return strings.TrimSpace(strings.TrimPrefix(value, "models/"))
	}
	if idx := strings.LastIndex(value, "/models/"); idx >= 0 {
		return strings.TrimSpace(value[idx+len("/models/"):])
	}
	return value
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://openrouter.ai/api/v1/models/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	var resp struct {
		Data []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
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
	sort.Strings(models)
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

func modelInfosFromIDs(ids []string) []llmModelInfo {
	infos := make([]llmModelInfo, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			infos = append(infos, llmModelInfo{ID: id})
		}
	}
	return infos
}

func normalizeLLMModelInfos(infos []llmModelInfo) []llmModelInfo {
	type groupedInfo struct {
		items []llmModelInfo
	}
	byKey := make(map[string]*groupedInfo)
	order := make([]string, 0, len(infos))
	for _, info := range infos {
		info.ID = strings.TrimSpace(info.ID)
		info.AliasTarget = strings.TrimSpace(info.AliasTarget)
		if info.ID == "" {
			continue
		}
		key := info.ID
		if info.AliasTarget != "" {
			key = info.AliasTarget
		}
		group := byKey[key]
		if group == nil {
			group = &groupedInfo{}
			byKey[key] = group
			order = append(order, key)
		}
		group.items = append(group.items, info)
	}

	out := make([]llmModelInfo, 0, len(byKey))
	for _, key := range order {
		group := byKey[key]
		selected := group.items[0]
		for _, item := range group.items {
			if item.ID == key {
				selected = item
				break
			}
		}
		seenAliases := make(map[string]bool)
		for _, item := range group.items {
			if item.ID == selected.ID || seenAliases[item.ID] {
				continue
			}
			selected.Aliases = append(selected.Aliases, item.ID)
			seenAliases[item.ID] = true
		}
		sortModels(selected.Aliases)
		out = append(out, selected)
	}
	sort.SliceStable(out, func(i int, j int) bool {
		return out[i].ID > out[j].ID
	})
	return out
}

func modelInfoIDs(infos []llmModelInfo) []string {
	ids := make([]string, 0, len(infos))
	for _, info := range infos {
		ids = appendUniqueString(ids, modelInfoRunID(info))
	}
	sortModels(ids)
	return ids
}

func modelInfoRunID(info llmModelInfo) string {
	if target := strings.TrimSpace(info.AliasTarget); target != "" {
		return target
	}
	return strings.TrimSpace(info.ID)
}

func modelInfoOptionLabels(infos []llmModelInfo) []string {
	labels := make([]string, 0, len(infos))
	for _, info := range infos {
		labels = appendUniqueModelOption(labels, modelInfoOptionLabel(info))
	}
	sort.SliceStable(labels, func(i int, j int) bool {
		return ModelIDFromOption(labels[i]) > ModelIDFromOption(labels[j])
	})
	return labels
}

func modelInfoOptionLabel(info llmModelInfo) string {
	label := strings.TrimSpace(info.ID)
	if label == "" {
		return ""
	}
	if len(info.Aliases) > 0 {
		return fmt.Sprintf("%s (aliases: %s)", label, strings.Join(info.Aliases, ", "))
	}
	if target := strings.TrimSpace(info.AliasTarget); target != "" && target != label {
		return fmt.Sprintf("%s -> %s", label, target)
	}
	return label
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
