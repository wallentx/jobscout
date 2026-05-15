package llm

import "strings"

func normalizeBenchmarkTaskName(task string) string {
	return normalizeLLMTaskKey(strings.TrimSpace(task))
}
