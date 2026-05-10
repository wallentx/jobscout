package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	popupHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245"))
	popupSelectActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("60")).
				Bold(true)
	popupSelectInactiveStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("250"))
)

type PopupMenuItem struct {
	Prefix   string
	Label    string
	Detail   string
	Disabled bool
	Selected bool
}

func RenderPopupMenu(items []PopupMenuItem) string {
	var out strings.Builder
	for idx, item := range items {
		if idx > 0 {
			out.WriteString("\n")
		}
		out.WriteString(RenderPopupMenuItem(item))
	}
	return out.String()
}

func RenderPopupMenuItem(item PopupMenuItem) string {
	rowText := item.Label
	if strings.TrimSpace(item.Prefix) != "" {
		rowText = item.Prefix + " " + rowText
	}
	if strings.TrimSpace(item.Detail) != "" {
		rowText += " " + popupHintStyle.Render(item.Detail)
	}

	line := "  " + rowText
	if item.Disabled {
		return popupHintStyle.Render(line)
	}
	if item.Selected {
		return popupSelectActiveStyle.Render("> " + rowText)
	}
	return popupSelectInactiveStyle.Render(line)
}

func RenderPopupMenuViewport(items []PopupMenuItem, width int, maxLines int, selectedIdx int) PopupViewport {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, RenderPopupMenuItem(item))
	}
	return RenderPopupViewport(lines, width, maxLines, PopupMenuScrollOffset(selectedIdx, len(lines), maxLines), func(line string) string {
		return line
	})
}

func PopupMenuScrollOffset(selectedIdx int, itemCount int, maxLines int) int {
	if maxLines < 1 || itemCount <= maxLines {
		return 0
	}
	if selectedIdx < 0 {
		selectedIdx = 0
	}
	if selectedIdx >= itemCount {
		selectedIdx = itemCount - 1
	}
	offset := selectedIdx - maxLines + 1
	if offset < 0 {
		return 0
	}
	maxOffset := itemCount - maxLines
	if offset > maxOffset {
		return maxOffset
	}
	return offset
}
