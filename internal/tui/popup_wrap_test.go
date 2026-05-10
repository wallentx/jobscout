package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestWrapStructuredPopupLinePreservesANSIStyleAndBulletIndent(t *testing.T) {
	lines := WrapStructuredPopupLine("\x1b[38;5;203m  • this is a longer company health factor that wraps\x1b[0m", 24)

	if len(lines) < 2 {
		t.Fatalf("len(lines) = %d, want at least 2", len(lines))
	}
	if got := ansi.Strip(lines[0]); !strings.HasPrefix(got, "  • ") {
		t.Fatalf("first wrapped line = %q, want bullet prefix", got)
	}
	if got := ansi.Strip(lines[1]); !strings.HasPrefix(got, "    ") {
		t.Fatalf("continuation line = %q, want bullet continuation indent", got)
	}
	for i, line := range lines {
		if !strings.Contains(line, "\x1b[") {
			t.Fatalf("line %d lost ANSI styling: %q", i, line)
		}
	}
}

func TestWrapStructuredPopupLineDoesNotSplitShortWords(t *testing.T) {
	lines := WrapStructuredPopupLine("\x1b[38;5;208m  • Ggml.ai joins Hugging Face to ensure the long-term progress of Local AI (839 points, 223 comments)\x1b[0m", 72)

	for i, line := range lines {
		stripped := ansi.Strip(line)
		if strings.HasSuffix(stripped, " A") {
			t.Fatalf("line %d split AI after A: %#v", i, stripped)
		}
		if strings.HasPrefix(stripped, "    I ") || stripped == "    I" {
			t.Fatalf("line %d split AI before I: %#v", i, stripped)
		}
		if i > 0 && !strings.HasPrefix(stripped, "    ") {
			t.Fatalf("continuation line %d = %q, want bullet continuation indent", i, stripped)
		}
		if !strings.Contains(line, "\x1b[") {
			t.Fatalf("line %d lost ANSI styling: %q", i, line)
		}
	}
}

func TestRenderPopupWrappedContentPreservesIndent(t *testing.T) {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Margin(0, 0, 0, 1)
	wrapped := RenderPopupWrappedContent(style.Render("Your resume will be parsed by your configured LLM provider to prefill criteria. jobscout does not store the resume file."), 48)
	lines := strings.Split(wrapped, "\n")
	if len(lines) < 2 {
		t.Fatalf("wrapped line count = %d, want at least 2 in %q", len(lines), wrapped)
	}
	firstVisible := ansi.Strip(lines[0])
	secondVisible := ansi.Strip(lines[1])
	if !strings.HasPrefix(firstVisible, " ") || !strings.HasPrefix(secondVisible, " ") {
		t.Fatalf("wrapped visible lines = %#v, want inherited leading indent", []string{firstVisible, secondVisible})
	}
}

func TestRenderScrollablePopupTextWithInheritedStylePreservesWrappedOriginStyle(t *testing.T) {
	text := "Filtered\n  duplicate (1)\n    Site\n      https://builtin.com/jobs/remote?allLocations=true&country=USA&search=Staff+DevOps+Engineer (1)"
	viewport := RenderScrollablePopupTextWithInheritedStyle(text, 52, 20, 0, func(origin string) LineStyler {
		if strings.Contains(origin, "builtin.com") {
			return func(line string) string {
				return "\x1b[38;5;210m" + line + "\x1b[39m"
			}
		}
		return func(line string) string {
			return line
		}
	})
	lines := strings.Split(viewport.Content, "\n")

	var wrappedURLLines []string
	for _, line := range lines {
		if strings.Contains(ansi.Strip(line), "builtin.com") || strings.Contains(ansi.Strip(line), "Staff+DevOps") {
			wrappedURLLines = append(wrappedURLLines, line)
		}
	}
	if len(wrappedURLLines) < 2 {
		t.Fatalf("wrapped URL lines len = %d, want at least 2 in %q", len(wrappedURLLines), viewport.Content)
	}
	firstStyle := PopupLineStyleSequences(wrappedURLLines[0])
	if firstStyle == "" {
		t.Fatalf("first wrapped URL line has no style: %q", wrappedURLLines[0])
	}
	for i, line := range wrappedURLLines[1:] {
		if got := PopupLineStyleSequences(line); got != firstStyle {
			t.Fatalf("wrapped URL continuation %d style = %q, want %q; line=%q", i, got, firstStyle, line)
		}
	}
}

func TestRenderPopupViewportDoesNotWrapStyledRows(t *testing.T) {
	lines := []string{
		"short",
		"this line is intentionally too long for the viewport width",
		"tail",
	}

	viewport := RenderPopupViewport(lines, 16, 2, 1, func(line string) string {
		return "\x1b[38;5;210m" + line + "\x1b[39m"
	})
	renderedLines := strings.Split(viewport.Content, "\n")

	if len(renderedLines) != 2 {
		t.Fatalf("rendered line count = %d; want 2; content=%q", len(renderedLines), viewport.Content)
	}
	for i, line := range renderedLines {
		if got := lipgloss.Width(ansi.Strip(line)); got != 18 {
			t.Fatalf("line %d width = %d; want 18 including scrollbar gutter; line=%q", i, got, line)
		}
	}
}
