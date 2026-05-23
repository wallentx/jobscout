package fetcher

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

var fetcherDebug = struct {
	sync.Mutex
	enabled bool
	path    string
}{
	path: "debug.log",
}

func ConfigureDebug(enabled bool, path string) func() {
	fetcherDebug.Lock()
	previousEnabled := fetcherDebug.enabled
	previousPath := fetcherDebug.path
	setDebugLocked(enabled, path)
	fetcherDebug.Unlock()

	return func() {
		fetcherDebug.Lock()
		setDebugLocked(previousEnabled, previousPath)
		fetcherDebug.Unlock()
	}
}

func SetDebug(enabled bool, path string) {
	fetcherDebug.Lock()
	defer fetcherDebug.Unlock()
	setDebugLocked(enabled, path)
}

func setDebugLocked(enabled bool, path string) {
	fetcherDebug.enabled = enabled
	if strings.TrimSpace(path) != "" {
		fetcherDebug.path = path
	}
}

func logDebug(format string, args ...interface{}) {
	fetcherDebug.Lock()
	defer fetcherDebug.Unlock()

	if !fetcherDebug.enabled {
		return
	}
	path := strings.TrimSpace(fetcherDebug.path)
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
	_, _ = fmt.Fprintf(file, "%s fetcher: %s\n", time.Now().Format(time.RFC3339), message)
}

func debugSettings() (bool, string) {
	fetcherDebug.Lock()
	defer fetcherDebug.Unlock()

	path := strings.TrimSpace(fetcherDebug.path)
	if path == "" {
		path = "debug.log"
	}
	return fetcherDebug.enabled, path
}
