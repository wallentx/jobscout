package llm

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func writeLLMBenchmarkRecords(records []llmBenchmarkRunRecord) (string, error) {
	outputPath := filepath.Join(
		defaultRuntimeDir(),
		"benchmarks",
		"llm-bench-"+time.Now().Format("20060102-150405")+".jsonl",
	)
	if err := ensureParentDir(outputPath); err != nil {
		return "", err
	}

	file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close()
	}()

	writer := bufio.NewWriter(file)
	enc := json.NewEncoder(writer)
	for _, record := range records {
		if err := enc.Encode(record); err != nil {
			return "", err
		}
	}
	if err := writer.Flush(); err != nil {
		return "", err
	}
	return outputPath, nil
}
