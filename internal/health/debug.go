package health

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

var healthDebug = struct {
	sync.Mutex
	enabled bool
	path    string
}{
	path: "debug.log",
}

func ConfigureDebug(enabled bool, path string) func() {
	healthDebug.Lock()
	previousEnabled := healthDebug.enabled
	previousPath := healthDebug.path
	setDebugLocked(enabled, path)
	healthDebug.Unlock()

	return func() {
		healthDebug.Lock()
		setDebugLocked(previousEnabled, previousPath)
		healthDebug.Unlock()
	}
}

func SetDebug(enabled bool, path string) {
	healthDebug.Lock()
	defer healthDebug.Unlock()
	setDebugLocked(enabled, path)
}

func setDebugLocked(enabled bool, path string) {
	healthDebug.enabled = enabled
	if strings.TrimSpace(path) != "" {
		healthDebug.path = path
	}
}

func logDebug(format string, args ...interface{}) {
	healthDebug.Lock()
	defer healthDebug.Unlock()

	if !healthDebug.enabled {
		return
	}
	path := strings.TrimSpace(healthDebug.path)
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
	_, _ = fmt.Fprintf(file, "%s health: %s\n", time.Now().Format(time.RFC3339), message)
}
