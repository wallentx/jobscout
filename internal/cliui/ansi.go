package cliui

import (
	"os"
	"strings"
	"sync/atomic"

	"golang.org/x/term"
)

const (
	Reset  = "\x1b[0m"
	Bold   = "\x1b[1m"
	Dim    = "\x1b[2m"
	Red    = "\x1b[31m"
	Green  = "\x1b[32m"
	Yellow = "\x1b[33m"
	Blue   = "\x1b[34m"
	Cyan   = "\x1b[36m"
)

var colorEnabled atomic.Bool

func init() {
	colorEnabled.Store(true)
}

func ConfigureColor(enabled bool) func() {
	previous := colorEnabled.Swap(enabled)
	return func() {
		colorEnabled.Store(previous)
	}
}

func ColorEnabledForFile(file *os.File) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if file == nil {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

func Style(text string, codes ...string) string {
	if text == "" || len(codes) == 0 {
		return text
	}
	if !colorEnabled.Load() {
		return text
	}
	var out strings.Builder
	for _, code := range codes {
		if code != "" {
			out.WriteString(code)
		}
	}
	out.WriteString(text)
	out.WriteString(Reset)
	return out.String()
}
