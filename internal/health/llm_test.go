package health

import (
	"testing"

	"github.com/wallentx/jobscout/internal/domain"
)

func TestDebugLLMHealthTokenUsage(t *testing.T) {
	input := 120
	output := 35
	total := 155
	usage := &domain.LLMTokenUsage{
		InputTokens:  &input,
		OutputTokens: &output,
		TotalTokens:  &total,
	}

	if got := debugLLMHealthTokenUsage(usage); got != "input=120 output=35 total=155" {
		t.Fatalf("debugLLMHealthTokenUsage(...) = %q, want input/output/total", got)
	}
	if got := debugLLMHealthTokenUsage(nil); got != "unavailable" {
		t.Fatalf("debugLLMHealthTokenUsage(nil) = %q, want unavailable", got)
	}
}
