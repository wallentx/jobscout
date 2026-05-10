package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type LineStyler func(string) string

type PopupViewport struct {
	Content   string
	MaxOffset int
}

func RenderScrollablePopupText(text string, width int, maxLines int, offset int, styler LineStyler) PopupViewport {
	if width < 10 {
		width = 10
	}
	if maxLines < 1 {
		maxLines = 1
	}

	lines := StructuredPopupLines(text, width)
	return RenderPopupViewport(lines, width, maxLines, offset, styler)
}

func RenderScrollablePopupTextWithInheritedStyle(text string, width int, maxLines int, offset int, stylerForOrigin func(string) LineStyler) PopupViewport {
	if width < 10 {
		width = 10
	}
	if maxLines < 1 {
		maxLines = 1
	}
	if stylerForOrigin == nil {
		stylerForOrigin = func(string) LineStyler {
			return nil
		}
	}

	rawLines := strings.Split(text, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, rawLine := range rawLines {
		styleLine := stylerForOrigin(rawLine)
		for _, wrappedLine := range WrapStructuredPopupLine(rawLine, width) {
			if styleLine != nil {
				wrappedLine = styleLine(wrappedLine)
			}
			lines = append(lines, wrappedLine)
		}
	}
	return RenderPopupViewport(lines, width, maxLines, offset, func(line string) string {
		return line
	})
}

func RenderPopupViewport(lines []string, width int, maxLines int, offset int, styler LineStyler) PopupViewport {
	if width < 10 {
		width = 10
	}
	if maxLines < 1 {
		maxLines = 1
	}
	if styler == nil {
		styler = func(line string) string {
			return line
		}
	}
	if len(lines) == 0 {
		lines = []string{""}
	}

	maxOffset := len(lines) - maxLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	offset = ClampPopupScroll(offset, maxOffset)

	end := offset + maxLines
	if end > len(lines) {
		end = len(lines)
	}

	visible := lines[offset:end]
	if len(visible) == 0 {
		visible = []string{""}
	}

	if len(lines) <= maxLines {
		styled := make([]string, 0, len(visible))
		for _, line := range visible {
			styled = append(styled, styler(line))
		}
		return PopupViewport{
			Content:   strings.Join(styled, "\n"),
			MaxOffset: 0,
		}
	}

	barHeight := len(visible)
	percentStart := float64(offset) / float64(len(lines))
	percentEnd := float64(end) / float64(len(lines))
	startRow := int(percentStart * float64(barHeight))
	endRow := int(percentEnd * float64(barHeight))
	if endRow <= startRow {
		endRow = startRow + 1
	}
	if endRow > barHeight {
		endRow = barHeight
	}

	sbStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styled := make([]string, 0, len(visible))
	for i, line := range visible {
		scrollChar := "│"
		if i >= startRow && i < endRow {
			scrollChar = "█"
		}
		padded := PadPopupViewportLine(styler(line), width)
		styled = append(styled, padded+" "+sbStyle.Render(scrollChar))
	}

	return PopupViewport{
		Content:   strings.Join(styled, "\n"),
		MaxOffset: maxOffset,
	}
}

func ClampPopupScroll(offset int, maxOffset int) int {
	if offset < 0 {
		return 0
	}
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}

func PadPopupViewportLine(line string, width int) string {
	if width < 1 {
		return ""
	}
	parts := strings.Split(line, "\n")
	if len(parts) > 0 {
		line = parts[0]
	}
	if lipgloss.Width(line) > width {
		line = ansi.Truncate(line, width, "")
	}
	padding := width - lipgloss.Width(line)
	if padding > 0 {
		line += strings.Repeat(" ", padding)
	}
	return line
}
