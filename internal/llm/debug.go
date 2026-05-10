package llm

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

var llmDebug = struct {
	sync.Mutex
	enabled bool
	path    string
}{
	path: "debug.log",
}

func ConfigureDebug(enabled bool, path string) func() {
	llmDebug.Lock()
	previousEnabled := llmDebug.enabled
	previousPath := llmDebug.path
	llmDebug.enabled = enabled
	if strings.TrimSpace(path) != "" {
		llmDebug.path = path
	}
	llmDebug.Unlock()

	return func() {
		llmDebug.Lock()
		llmDebug.enabled = previousEnabled
		llmDebug.path = previousPath
		llmDebug.Unlock()
	}
}

func logDebug(format string, args ...interface{}) {
	llmDebug.Lock()
	defer llmDebug.Unlock()

	if !llmDebug.enabled {
		return
	}
	path := strings.TrimSpace(llmDebug.path)
	if path == "" {
		path = "debug.log"
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer func() {
		_ = file.Close()
	}()

	message := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(file, "%s llm: %s\n", time.Now().Format(time.RFC3339), message)
}
