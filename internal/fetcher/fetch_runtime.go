package fetcher

import (
	"path/filepath"

	"github.com/wallentx/jobscout/internal/config"
)

var (
	fetchAllJobsReadFile              = readSearchPromptFile
	runtimeSearchPromptPath           = "SEARCH_PROMPT.md"
	runtimeLinkedInTypeaheadCachePath = "linkedin_cache.json"
)

func readSearchPromptFile(path string) ([]byte, error) {
	prompt, err := config.LoadSearchPrompt(path)
	if err != nil {
		return nil, err
	}
	return []byte(prompt), nil
}

func ConfigureRuntime(searchPromptPath string, readFile func(string) ([]byte, error)) func() {
	previousPath := runtimeSearchPromptPath
	previousLinkedInCachePath := runtimeLinkedInTypeaheadCachePath
	previousReadFile := fetchAllJobsReadFile
	if searchPromptPath != "" {
		runtimeSearchPromptPath = searchPromptPath
		runtimeLinkedInTypeaheadCachePath = filepath.Join(filepath.Dir(searchPromptPath), "linkedin_cache.json")
	}
	if readFile != nil {
		fetchAllJobsReadFile = readFile
	}
	return func() {
		runtimeSearchPromptPath = previousPath
		runtimeLinkedInTypeaheadCachePath = previousLinkedInCachePath
		fetchAllJobsReadFile = previousReadFile
	}
}

func ConfigureLinkedInTypeaheadCache(path string) func() {
	previous := runtimeLinkedInTypeaheadCachePath
	runtimeLinkedInTypeaheadCachePath = path
	return func() {
		runtimeLinkedInTypeaheadCachePath = previous
	}
}
