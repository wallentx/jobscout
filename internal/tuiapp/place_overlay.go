package tuiapp

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func placeOverlay(x, y int, fg string, bg string) string {
	fgLines := strings.Split(fg, "\n")
	bgLines := strings.Split(bg, "\n")
	fgWidth := lipgloss.Width(fg)
	fgHeight := len(fgLines)
	bgWidth := lipgloss.Width(bg)
	bgHeight := len(bgLines)

	if fgWidth >= bgWidth && fgHeight >= bgHeight {
		return fg
	}

	x = clampInt(x, 0, bgWidth-fgWidth)
	y = clampInt(y, 0, bgHeight-fgHeight)

	var b strings.Builder
	for i, bgLine := range bgLines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if i < y || i >= y+fgHeight {
			b.WriteString(bgLine)
			continue
		}

		fgLine := fgLines[i-y]
		bgLineWidth := ansi.StringWidth(bgLine)
		fgLineWidth := ansi.StringWidth(fgLine)
		left := ansi.Cut(bgLine, 0, x)
		b.WriteString(left)
		if leftWidth := ansi.StringWidth(left); leftWidth < x {
			b.WriteString(strings.Repeat(" ", x-leftWidth))
		}

		b.WriteString(fgLine)

		rightStart := x + fgLineWidth
		if rightStart < bgLineWidth {
			right := ansi.Cut(bgLine, rightStart, bgLineWidth)
			if gap := bgLineWidth - rightStart - ansi.StringWidth(right); gap > 0 {
				b.WriteString(strings.Repeat(" ", gap))
			}
			b.WriteString(right)
		}
	}

	return b.String()
}

func clampInt(v int, lower int, upper int) int {
	if upper < lower {
		return lower
	}
	if v < lower {
		return lower
	}
	if v > upper {
		return upper
	}
	return v
}
