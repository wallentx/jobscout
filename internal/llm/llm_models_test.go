package llm

import "testing"

func TestIsOpenAITextModelKeepsChatCompatibleModels(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{id: "gpt-5.5", want: true},
		{id: "gpt-5.5-pro", want: false},
		{id: "gpt-5.4", want: true},
		{id: "gpt-5.4-pro", want: false},
		{id: "gpt-5.4-mini", want: true},
		{id: "gpt-5.4-nano", want: true},
		{id: "gpt-5.2", want: true},
		{id: "gpt-5.2-pro", want: false},
		{id: "gpt-5.1", want: true},
		{id: "gpt-5", want: true},
		{id: "gpt-5-pro", want: false},
		{id: "gpt-5-mini", want: true},
		{id: "gpt-5-nano", want: true},
		{id: "gpt-5.3-chat", want: true},
		{id: "gpt-5.2-chat", want: true},
		{id: "gpt-4", want: false},
		{id: "gpt-4.1", want: true},
		{id: "gpt-4.1-mini", want: true},
		{id: "gpt-4o", want: true},
		{id: "gpt-4o-mini", want: true},
		{id: "o3", want: true},
		{id: "gpt-5.4-2026-03-05", want: false},
		{id: "gpt-4.1-2025-04-14", want: false},
		{id: "gpt-4.1-nano", want: false},
		{id: "gpt-4.1-nano-2025-04-14", want: false},
		{id: "gpt-4o-2024-11-20", want: false},
		{id: "gpt-4-0613", want: false},
		{id: "gpt-4.5-preview", want: false},
		{id: "gpt-4-turbo", want: false},
		{id: "gpt-4-turbo-preview", want: false},
		{id: "gpt-3.5-turbo", want: false},
		{id: "gpt-3.5-turbo-0125", want: false},
		{id: "gpt-5.1-chat", want: false},
		{id: "gpt-5.1-chat-latest", want: false},
		{id: "gpt-5-chat", want: false},
		{id: "gpt-5-chat-latest", want: false},
		{id: "chat-latest", want: false},
		{id: "chatgpt-4o", want: false},
		{id: "o1", want: false},
		{id: "o1-2024-12-17", want: false},
		{id: "o1-mini", want: false},
		{id: "o1-preview", want: false},
		{id: "o1-pro", want: false},
		{id: "o1-pro-2025-03-19", want: false},
		{id: "o3-2025-04-16", want: false},
		{id: "o3-mini", want: false},
		{id: "o3-pro", want: false},
		{id: "o3-pro-2025-06-10", want: false},
		{id: "o4-mini", want: false},
		{id: "o4-mini-deep-research", want: false},
		{id: "gpt-5.3", want: true},
		{id: "gpt-4o-realtime-preview", want: false},
		{id: "gpt-image-1", want: false},
		{id: "gpt-5.2-codex", want: false},
		{id: "gpt-4.1-nano-search-preview", want: false},
		{id: "gpt-4o-search-api", want: false},
		{id: "text-embedding-3-large", want: false},
		{id: "computer-use-preview", want: false},
		{id: "babbage-002", want: false},
		{id: "davinci-002", want: false},
		{id: "codex-mini-latest", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := isOpenAITextModel(tt.id); got != tt.want {
				t.Fatalf("isOpenAITextModel(%q) = %t; want %t", tt.id, got, tt.want)
			}
		})
	}
}

func TestSetupModelOptionsFiltersBlockedOpenAIModels(t *testing.T) {
	cfg := defaultAppConfig()
	cfg.LLM.Model = "chat-latest"
	fetched := map[string][]string{
		"openai": {
			"gpt-4.1",
			"gpt-4",
			"gpt-5.4-pro",
			"gpt-5-pro",
			"gpt-4o",
			"gpt-4o-2024-11-20",
			"gpt-4.1-nano",
			"gpt-4.1-nano-2025-04-14",
			"gpt-4.5-preview",
			"gpt-4-turbo",
			"gpt-3.5-turbo",
			"chat-latest",
			"gpt-5.1-chat",
			"gpt-5-chat-latest",
			"o1",
			"o1-pro",
			"o1-pro-2025-03-19",
			"o1-2024-12-17",
			"o3",
			"o3-2025-04-16",
			"o3-pro",
			"o3-pro-2025-06-10",
			"o3-mini",
			"o4-mini",
		},
	}

	got := setupModelOptions("openai", &cfg, fetched)
	for _, model := range []string{
		"gpt-4o-2024-11-20",
		"gpt-4",
		"gpt-5.4-pro",
		"gpt-5-pro",
		"gpt-4.1-nano",
		"gpt-4.1-nano-2025-04-14",
		"gpt-4.5-preview",
		"gpt-4-turbo",
		"gpt-3.5-turbo",
		"chat-latest",
		"gpt-5.1-chat",
		"gpt-5-chat-latest",
		"o1",
		"o1-pro",
		"o1-pro-2025-03-19",
		"o1-2024-12-17",
		"o3-2025-04-16",
		"o3-pro",
		"o3-pro-2025-06-10",
		"o3-mini",
		"o4-mini",
	} {
		if stringSliceContains(got, model) {
			t.Fatalf("setupModelOptions(openai, ...) included blocked model %q in %#v", model, got)
		}
	}
	for _, model := range []string{"gpt-4.1", "gpt-4o", "o3", manualModelOption} {
		if !stringSliceContains(got, model) {
			t.Fatalf("setupModelOptions(openai, ...) omitted usable model %q from %#v", model, got)
		}
	}
}

func TestSetupModelOptionsPrioritizesRecommendedOpenAIModels(t *testing.T) {
	cfg := defaultAppConfig()
	fetched := map[string][]string{
		"openai": {
			"gpt-5.5",
			"gpt-5-nano",
			"gpt-4o",
			"gpt-5.4-mini",
			"gpt-4o-mini",
			"gpt-5.4-nano",
		},
	}

	got := setupModelOptions("openai", &cfg, fetched)
	assertStringSliceEqual(t, got[:4], []string{
		"gpt-4o-mini",
		"gpt-5.4-mini",
		"gpt-5.4-nano",
		"gpt-4o",
	})
}

func TestModelIDFromOptionReturnsDisplayID(t *testing.T) {
	tests := []struct {
		option string
		want   string
	}{
		{
			option: "gpt-4o-2024-11-20 (aliases: gpt-4o)",
			want:   "gpt-4o-2024-11-20",
		},
		{
			option: "gemini-flash-lite-preview -> gemini-3.1-flash-lite",
			want:   "gemini-flash-lite-preview",
		},
		{
			option: manualModelOption,
			want:   manualModelOption,
		},
	}

	for _, tt := range tests {
		t.Run(tt.option, func(t *testing.T) {
			if got := ModelIDFromOption(tt.option); got != tt.want {
				t.Fatalf("ModelIDFromOption(%q) = %q, want %q", tt.option, got, tt.want)
			}
		})
	}
}

func TestModelRunIDFromOptionReturnsRunnableID(t *testing.T) {
	tests := []struct {
		option string
		want   string
	}{
		{
			option: "gpt-4o-2024-11-20 (aliases: gpt-4o)",
			want:   "gpt-4o-2024-11-20",
		},
		{
			option: "gemini-flash-lite-preview -> gemini-3.1-flash-lite",
			want:   "gemini-3.1-flash-lite",
		},
		{
			option: manualModelOption,
			want:   manualModelOption,
		},
	}

	for _, tt := range tests {
		t.Run(tt.option, func(t *testing.T) {
			if got := ModelRunIDFromOption(tt.option); got != tt.want {
				t.Fatalf("ModelRunIDFromOption(%q) = %q, want %q", tt.option, got, tt.want)
			}
		})
	}
}

func TestModelOptionIDsIncludesAliasesAndAliasTargets(t *testing.T) {
	assertStringSliceEqual(t,
		ModelOptionIDs("gpt-4o-2024-11-20 (aliases: gpt-4o, gpt-4o-latest)"),
		[]string{"gpt-4o-2024-11-20", "gpt-4o", "gpt-4o-latest"},
	)
	assertStringSliceEqual(t,
		ModelOptionIDs("gemini-flash-lite-preview -> gemini-3.1-flash-lite"),
		[]string{"gemini-flash-lite-preview", "gemini-3.1-flash-lite"},
	)
}

func TestAvailableLLMModelsDedupesAliasesAndLabelsThem(t *testing.T) {
	available := newAvailableLLMModels([]llmModelInfo{
		{ID: "gpt-5.5-latest", AliasTarget: "gpt-5.5"},
		{ID: "gpt-5.5"},
		{ID: "gpt-4.1"},
	})

	assertStringSliceEqual(t, available.IDs(), []string{"gpt-5.5", "gpt-4.1"})
	assertStringSliceEqual(t, available.OptionLabels(), []string{
		"gpt-5.5 (aliases: gpt-5.5-latest)",
		"gpt-4.1",
	})
}

func TestSetupModelOptionsDedupesCurrentAliasFromFetchedLabels(t *testing.T) {
	cfg := defaultAppConfig()
	cfg.LLM.Model = "gpt-5.5-latest"
	fetched := map[string][]string{
		"openai": {"gpt-5.5 (aliases: gpt-5.5-latest)"},
	}

	got := setupModelOptions("openai", &cfg, fetched)
	assertStringSliceEqual(t, got, []string{
		"gpt-5.5 (aliases: gpt-5.5-latest)",
		manualModelOption,
	})
}

func TestAvailableLLMModelsUsesAliasTargetWhenCanonicalIsMissing(t *testing.T) {
	available := newAvailableLLMModels([]llmModelInfo{
		{ID: "gemini-flash-lite-preview", AliasTarget: "gemini-3.1-flash-lite"},
	})

	assertStringSliceEqual(t, available.IDs(), []string{"gemini-3.1-flash-lite"})
	assertStringSliceEqual(t, available.OptionLabels(), []string{"gemini-flash-lite-preview -> gemini-3.1-flash-lite"})
}

func TestAvailableLLMModelsRunIDForResolvesAliases(t *testing.T) {
	available := newAvailableLLMModels([]llmModelInfo{
		{ID: "gemini-flash-lite-preview", AliasTarget: "gemini-3.1-flash-lite"},
		{ID: "gemini-3.1-flash-lite"},
		{ID: "gemini-3-pro-preview"},
	})

	tests := map[string]string{
		"gemini-flash-lite-preview":                          "gemini-3.1-flash-lite",
		"gemini-flash-lite-preview -> gemini-3.1-flash-lite": "gemini-3.1-flash-lite",
		"gemini-3.1-flash-lite":                              "gemini-3.1-flash-lite",
		"gemini-3-pro-preview":                               "gemini-3-pro-preview",
		"custom-model":                                       "custom-model",
	}
	for input, want := range tests {
		if got := available.RunIDFor(input); got != want {
			t.Fatalf("RunIDFor(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestAvailableLLMModelsDedupesGeminiResourceAliasTargets(t *testing.T) {
	available := newAvailableLLMModels([]llmModelInfo{
		{ID: "gemini-flash-lite-preview", AliasTarget: geminiModelAliasTarget("models/gemini-flash-lite-preview", "models/gemini-3.1-flash-lite")},
		{ID: "gemini-3.1-flash-lite"},
	})

	assertStringSliceEqual(t, available.IDs(), []string{"gemini-3.1-flash-lite"})
	assertStringSliceEqual(t, available.OptionLabels(), []string{"gemini-3.1-flash-lite (aliases: gemini-flash-lite-preview)"})
}

func TestGeminiModelAliasTargetNormalizesResourceNames(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		baseModelID string
		want        string
	}{
		{
			name:        "plain base model",
			id:          "gemini-flash-lite-preview",
			baseModelID: "gemini-3.1-flash-lite",
			want:        "gemini-3.1-flash-lite",
		},
		{
			name:        "models resource base model",
			id:          "gemini-flash-lite-preview",
			baseModelID: "models/gemini-3.1-flash-lite",
			want:        "gemini-3.1-flash-lite",
		},
		{
			name:        "full publisher resource base model",
			id:          "models/gemini-flash-lite-preview",
			baseModelID: "publishers/google/models/gemini-3.1-flash-lite",
			want:        "gemini-3.1-flash-lite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := geminiModelAliasTarget(tt.id, tt.baseModelID); got != tt.want {
				t.Fatalf("geminiModelAliasTarget(%q, %q) = %q, want %q", tt.id, tt.baseModelID, got, tt.want)
			}
		})
	}
}

func assertStringSliceEqual(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("slice len = %d (%#v), want %d (%#v)", len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("slice[%d] = %q in %#v, want %q in %#v", i, got[i], got, want[i], want)
		}
	}
}

func TestIsGeminiTextModelFiltersNonTextFamilies(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		displayName string
		description string
		want        bool
	}{
		{
			name:        "text model",
			id:          "gemini-3.1-pro-preview",
			displayName: "Gemini 3.1 Pro Preview",
			description: "General purpose reasoning model.",
			want:        true,
		},
		{
			name:        "preview model without replacement",
			id:          "gemini-3-flash-preview",
			displayName: "Gemini 3 Flash Preview",
			description: "General purpose reasoning model.",
			want:        true,
		},
		{
			name:        "stable replacement model",
			id:          "gemini-3.1-flash-lite",
			displayName: "Gemini 3.1 Flash-Lite",
			description: "General purpose model.",
			want:        true,
		},
		{
			name:        "preview model with replacement",
			id:          "gemini-3.1-flash-lite-preview",
			displayName: "Gemini 3.1 Flash-Lite Preview",
			description: "General purpose model.",
			want:        false,
		},
		{
			name:        "deprecated preview model",
			id:          "gemini-3-pro-preview",
			displayName: "Gemini 3 Pro Preview",
			description: "General purpose reasoning model.",
			want:        false,
		},
		{
			name:        "deprecated stable flash lite model",
			id:          "gemini-2.5-flash-lite",
			displayName: "Gemini 2.5 Flash-Lite",
			description: "General purpose model.",
			want:        false,
		},
		{
			name:        "deprecated stable flash model",
			id:          "gemini-2.5-flash",
			displayName: "Gemini 2.5 Flash",
			description: "General purpose model.",
			want:        false,
		},
		{
			name:        "deprecated stable pro model",
			id:          "gemini-2.5-pro",
			displayName: "Gemini 2.5 Pro",
			description: "General purpose model.",
			want:        false,
		},
		{
			name:        "legacy latest alias",
			id:          "gemini-flash-latest",
			displayName: "Gemini Flash Latest",
			description: "General purpose model.",
			want:        false,
		},
		{
			name:        "nano banana image model",
			id:          "gemini-3.1-flash-lite-image",
			displayName: "Nano Banana",
			description: "Native image generation and editing model.",
			want:        false,
		},
		{
			name:        "embedding model",
			id:          "gemini-embedding-001",
			displayName: "Gemini Embedding",
			description: "Embeddings for semantic retrieval.",
			want:        false,
		},
		{
			name:        "video model",
			id:          "veo-3.1-generate-preview",
			displayName: "Veo 3.1",
			description: "Video generation model.",
			want:        false,
		},
		{
			name:        "text to speech model",
			id:          "gemini-2.5-pro-tts",
			displayName: "Gemini 2.5 Pro TTS",
			description: "Text-to-speech generation model.",
			want:        false,
		},
		{
			name:        "robotics model",
			id:          "gemini-robotics-er-1.5-preview",
			displayName: "Gemini Robotics-ER 1.5",
			description: "Vision-language model for robotics.",
			want:        false,
		},
		{
			name:        "computer use model",
			id:          "gemini-2.5-computer-use-preview-10-2025",
			displayName: "Gemini 2.5 Computer Use",
			description: "Requires the Computer Use tool.",
			want:        false,
		},
		{
			name:        "unavailable legacy flash model",
			id:          "gemini-2.0-flash-001",
			displayName: "Gemini 2.0 Flash",
			description: "Legacy model.",
			want:        false,
		},
		{
			name:        "unavailable legacy flash lite model",
			id:          "gemini-2.0-flash-lite",
			displayName: "Gemini 2.0 Flash Lite",
			description: "Legacy model.",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGeminiTextModel(tt.id, tt.displayName, tt.description)
			if got != tt.want {
				t.Fatalf("isGeminiTextModel(%q, %q, %q) = %t, want %t", tt.id, tt.displayName, tt.description, got, tt.want)
			}
		})
	}
}

func TestIsOpenRouterChatModelFiltersNonChatFamilies(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{name: "openai chat model", id: "openai/gpt-4o", want: true},
		{name: "anthropic chat model", id: "deepseek/deepseek-v4-flash", want: true},
		{name: "deepseek chat model", id: "deepseek/deepseek-chat", want: true},
		{name: "meta chat model", id: "meta-llama/llama-3.3-70b-instruct", want: true},
		{name: "google chat model", id: "google/gemini-3.1-flash-lite", want: true},
		{name: "embedding model", id: "openai/text-embedding-3-large", want: false},
		{name: "moderation model", id: "openai/omni-moderation-latest", want: false},
		{name: "image model", id: "openai/dall-e-3", want: false},
		{name: "whisper model", id: "openai/whisper-1", want: false},
		{name: "tts model", id: "openai/tts-1", want: false},
		{name: "video model", id: "google/veo-3.1-generate-preview", want: false},
		{name: "janus embed model", id: "deepseek/deepseek-ai/deepseek-janus-pro", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOpenRouterChatModel(tt.id)
			if got != tt.want {
				t.Fatalf("isOpenRouterChatModel(%q) = %t, want %t", tt.id, got, tt.want)
			}
		})
	}
}
