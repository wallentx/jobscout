package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// RenderPopupWrappedContent wraps ANSI-styled popup content while preserving
// indentation and active text styling on continuation lines.
func RenderPopupWrappedContent(content string, width int) string {
	if width < 10 {
		width = 10
	}
	return strings.Join(StructuredPopupLines(content, width), "\n")
}

// StructuredPopupLines splits and wraps popup text into terminal-width lines.
func StructuredPopupLines(text string, width int) []string {
	rawLines := strings.Split(text, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, rawLine := range rawLines {
		lines = append(lines, WrapStructuredPopupLine(rawLine, width)...)
	}
	return lines
}

// WrapStructuredPopupLine wraps one ANSI-styled popup line.
func WrapStructuredPopupLine(line string, width int) []string {
	if line == "" {
		return []string{""}
	}

	if lipgloss.Width(line) <= width {
		return []string{line}
	}

	visibleLine := ansi.Strip(line)
	indentWidth := 0
	for indentWidth < len(visibleLine) && visibleLine[indentWidth] == ' ' {
		indentWidth++
	}
	indent := visibleLine[:indentWidth]
	continuationPrefix := popupContinuationPrefix(visibleLine, indent)
	content := popupTrimVisibleLeadingSpaces(line, indentWidth)
	if strings.TrimSpace(ansi.Strip(content)) == "" {
		return []string{line}
	}

	firstWidth := width - lipgloss.Width(indent)
	if firstWidth < 1 {
		return []string{truncateANSI(line, width)}
	}
	wrappedContent := WrapANSIWords(content, firstWidth)
	wrapped := make([]string, 0, len(wrappedContent))
	activeStylePrefix := popupLineStylePrefix(line)
	if activeStylePrefix == "" {
		activeStylePrefix = popupFirstStyleSequence(line)
	}
	continuationStylePrefix := activeStylePrefix
	for i, part := range wrappedContent {
		prefix := indent
		if i > 0 {
			prefix = continuationPrefix
			part = continuationStylePrefix + part
		}
		lineWidth := width - lipgloss.Width(prefix)
		if lineWidth < 1 {
			wrapped = append(wrapped, truncateANSI(prefix+part, width))
			if nextStyle := popupContinuationStyle(part, continuationStylePrefix); nextStyle != "" {
				continuationStylePrefix = nextStyle
			}
			continue
		}
		if continuationStylePrefix == "" {
			continuationStylePrefix = popupLineStylePrefix(part)
		}
		for _, subPart := range WrapANSIWords(part, lineWidth) {
			if len(wrapped) > 0 && continuationStylePrefix != "" {
				subPart = continuationStylePrefix + subPart
			}
			wrapped = append(wrapped, prefix+subPart)
			if nextStyle := popupContinuationStyle(subPart, continuationStylePrefix); nextStyle != "" {
				continuationStylePrefix = nextStyle
			}
			prefix = continuationPrefix
			lineWidth = width - lipgloss.Width(prefix)
			if lineWidth < 1 {
				break
			}
		}
	}
	if len(wrapped) == 0 {
		return []string{truncateANSI(line, width)}
	}
	return wrapped
}

func popupContinuationStyle(line string, fallback string) string {
	if style := popupActiveStyleAfterLastReset(line); style != "" {
		return style
	}
	if popupHasVisibleTextAfterLastReset(line) {
		return ""
	}
	if style := popupLineStylePrefix(line); style != "" {
		return style
	}
	return fallback
}

func popupActiveStyleAfterLastReset(line string) string {
	start := 0
	for i := 0; i < len(line); i++ {
		if line[i] != '\x1b' {
			continue
		}
		end := strings.IndexByte(line[i:], 'm')
		if end < 0 {
			continue
		}
		seq := line[i : i+end+1]
		if seq == "\x1b[0m" || seq == "\x1b[m" || seq == "\x1b[39m" {
			start = i + end + 1
		}
		i += end
	}

	last := ""
	for i := start; i < len(line); i++ {
		if line[i] != '\x1b' {
			continue
		}
		end := strings.IndexByte(line[i:], 'm')
		if end < 0 {
			continue
		}
		seq := line[i : i+end+1]
		if seq != "\x1b[0m" && seq != "\x1b[m" && seq != "\x1b[39m" {
			last = seq
		}
		i += end
	}
	return last
}

func popupHasVisibleTextAfterLastReset(line string) bool {
	start := 0
	for i := 0; i < len(line); i++ {
		if line[i] != '\x1b' {
			continue
		}
		end := strings.IndexByte(line[i:], 'm')
		if end < 0 {
			continue
		}
		seq := line[i : i+end+1]
		if seq == "\x1b[0m" || seq == "\x1b[m" || seq == "\x1b[39m" {
			start = i + end + 1
		}
		i += end
	}
	return strings.TrimSpace(ansi.Strip(line[start:])) != ""
}

func popupFirstStyleSequence(line string) string {
	for i := 0; i < len(line); i++ {
		if line[i] != '\x1b' {
			continue
		}
		end := strings.IndexByte(line[i:], 'm')
		if end < 0 {
			return ""
		}
		seq := line[i : i+end+1]
		if seq == "\x1b[0m" || seq == "\x1b[m" {
			return ""
		}
		return seq
	}
	return ""
}

// WrapANSIWords wraps ANSI-styled text without splitting words unless needed.
func WrapANSIWords(line string, width int) []string {
	if width < 1 {
		return []string{line}
	}
	wrapped := strings.Split(ansi.Wordwrap(line, width, ""), "\n")
	out := make([]string, 0, len(wrapped))
	for _, part := range wrapped {
		if lipgloss.Width(part) <= width {
			out = append(out, part)
			continue
		}
		out = append(out, strings.Split(ansi.Wrap(part, width, ""), "\n")...)
	}
	return out
}

func popupContinuationPrefix(line string, indent string) string {
	trimmed := strings.TrimLeft(line, " ")
	if len(trimmed) >= 2 {
		switch trimmed[:2] {
		case "+ ", "- ":
			return indent + "  "
		}
	}
	if strings.HasPrefix(trimmed, "• ") {
		return indent + "  "
	}
	return indent
}

func popupLineStylePrefix(line string) string {
	var prefix strings.Builder
	for i := 0; i < len(line); {
		if line[i] == ' ' {
			i++
			continue
		}
		if line[i] != '\x1b' {
			break
		}
		end := strings.IndexByte(line[i:], 'm')
		if end < 0 {
			break
		}
		seq := line[i : i+end+1]
		prefix.WriteString(seq)
		i += end + 1
		if seq == "\x1b[0m" || seq == "\x1b[m" {
			prefix.Reset()
		}
	}
	return prefix.String()
}

func PopupLineStyleSequences(line string) string {
	var prefix strings.Builder
	for i := 0; i < len(line); i++ {
		if line[i] != '\x1b' {
			continue
		}
		end := strings.IndexByte(line[i:], 'm')
		if end < 0 {
			continue
		}
		seq := line[i : i+end+1]
		prefix.WriteString(seq)
		if seq == "\x1b[0m" || seq == "\x1b[m" {
			prefix.Reset()
		}
		i += end
	}
	return prefix.String()
}

func popupTrimVisibleLeadingSpaces(line string, spaces int) string {
	if spaces <= 0 {
		return strings.TrimLeft(line, " ")
	}
	var out strings.Builder
	skipped := 0
	for i := 0; i < len(line); {
		if line[i] == '\x1b' {
			end := strings.IndexByte(line[i:], 'm')
			if end < 0 {
				out.WriteString(line[i:])
				break
			}
			out.WriteString(line[i : i+end+1])
			i += end + 1
			continue
		}
		if skipped < spaces && line[i] == ' ' {
			skipped++
			i++
			continue
		}
		out.WriteString(line[i:])
		break
	}
	return out.String()
}

func truncateANSI(line string, width int) string {
	if width < 1 {
		return ""
	}
	return ansi.Truncate(line, width, "")
}
