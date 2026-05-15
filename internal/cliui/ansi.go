package cliui

import "strings"

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

func Style(text string, codes ...string) string {
	if text == "" || len(codes) == 0 {
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
