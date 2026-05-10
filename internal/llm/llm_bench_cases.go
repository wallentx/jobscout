package llm

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed testdata/benchcases/*.json
var benchCaseFS embed.FS

func loadLLMBenchmarkCases() ([]llmBenchmarkCase, error) {
	entries, err := fs.ReadDir(benchCaseFS, "testdata/benchcases")
	if err != nil {
		return nil, err
	}

	cases := make([]llmBenchmarkCase, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := benchCaseFS.ReadFile(filepath.Join("testdata/benchcases", entry.Name()))
		if err != nil {
			return nil, err
		}
		var benchCase llmBenchmarkCase
		if err := json.Unmarshal(data, &benchCase); err != nil {
			return nil, fmt.Errorf("parse %s: %w", entry.Name(), err)
		}
		if err := validateLLMBenchmarkCase(benchCase); err != nil {
			return nil, fmt.Errorf("validate %s: %w", entry.Name(), err)
		}
		cases = append(cases, benchCase)
	}

	sort.Slice(cases, func(i, j int) bool {
		if cases[i].Task == cases[j].Task {
			return cases[i].ID < cases[j].ID
		}
		return cases[i].Task < cases[j].Task
	})
	return cases, nil
}

func validateLLMBenchmarkCase(benchCase llmBenchmarkCase) error {
	if strings.TrimSpace(benchCase.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(benchCase.Task) == "" {
		return fmt.Errorf("task is required")
	}
	if benchCase.Version <= 0 {
		return fmt.Errorf("version must be greater than zero")
	}
	if len(benchCase.Input) == 0 {
		return fmt.Errorf("input is required")
	}
	return nil
}

func filterBenchmarkCases(cases []llmBenchmarkCase, task string) []llmBenchmarkCase {
	task = strings.ToLower(strings.TrimSpace(task))
	if task == "" || task == "all" {
		return cases
	}

	var filtered []llmBenchmarkCase
	for _, benchCase := range cases {
		if strings.EqualFold(benchCase.Task, task) || strings.EqualFold(benchCase.ID, task) {
			filtered = append(filtered, benchCase)
		}
	}
	return filtered
}
