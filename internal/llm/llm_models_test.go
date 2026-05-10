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
