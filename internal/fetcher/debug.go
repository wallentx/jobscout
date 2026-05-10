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
	fetcherDebug.enabled = enabled
	if strings.TrimSpace(path) != "" {
		fetcherDebug.path = path
	}
	fetcherDebug.Unlock()

	return func() {
		fetcherDebug.Lock()
		fetcherDebug.enabled = previousEnabled
		fetcherDebug.path = previousPath
		fetcherDebug.Unlock()
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
