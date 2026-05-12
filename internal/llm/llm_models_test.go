package llm

import "testing"

func TestIsOpenAITextModelKeepsChatCompatibleModels(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{id: "gpt-5.3-chat-latest", want: true},
		{id: "gpt-4.1-mini", want: true},
		{id: "gpt-4o", want: true},
		{id: "o3-mini", want: true},
		{id: "gpt-5.3", want: false},
		{id: "gpt-4o-realtime-preview", want: false},
		{id: "gpt-image-1", want: false},
		{id: "gpt-4.1-nano-search-preview", want: false},
		{id: "text-embedding-3-large", want: false},
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
			id:          "gemini-3-pro-preview",
			displayName: "Gemini 3 Pro Preview",
			description: "General purpose reasoning model.",
			want:        true,
		},
		{
			name:        "nano banana image model",
			id:          "gemini-2.5-flash-image",
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
		{name: "anthropic chat model", id: "anthropic/claude-sonnet-4", want: true},
		{name: "deepseek chat model", id: "deepseek/deepseek-chat", want: true},
		{name: "meta chat model", id: "meta-llama/llama-3.3-70b-instruct", want: true},
		{name: "google chat model", id: "google/gemini-2.5-flash", want: true},
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
