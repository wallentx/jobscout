package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type PopupDropdownSpec struct {
	Label       string
	Value       string
	Items       []string
	LabelWidth  int
	Width       int
	MaxOpenRows int
	SelectedIdx int
	Open        bool
	Focused     bool
}

type PopupDropdownOverlay struct {
	Content string
	Width   int
	Height  int
}

func RenderPopupDropdown(spec PopupDropdownSpec) string {
	labelWidth := spec.LabelWidth
	if labelWidth < lipgloss.Width(spec.Label) {
		labelWidth = lipgloss.Width(spec.Label)
	}
	fieldWidth := spec.Width
	if fieldWidth < 12 {
		fieldWidth = 12
	}
	selectedIdx := NormalizeDropdownSelectedIdx(spec.SelectedIdx, len(spec.Items))
	value := strings.TrimSpace(spec.Value)
	if value == "" && len(spec.Items) > 0 {
		value = spec.Items[selectedIdx]
	}

	var out strings.Builder
	labelStyle := popupSelectInactiveStyle
	if spec.Focused {
		labelStyle = popupSelectActiveStyle
	}
	out.WriteString(labelStyle.Render(PadVisibleRight(spec.Label, labelWidth)))
	out.WriteString(" ")
	out.WriteString(renderPopupDropdownControl(value, fieldWidth, spec.Focused || spec.Open))
	return out.String()
}

func RenderPopupDropdownOverlay(spec PopupDropdownSpec) PopupDropdownOverlay {
	fieldWidth := spec.Width
	if fieldWidth < 12 {
		fieldWidth = 12
	}
	maxOpenRows := spec.MaxOpenRows
	if maxOpenRows < 1 {
		maxOpenRows = 1
	}
	selectedIdx := NormalizeDropdownSelectedIdx(spec.SelectedIdx, len(spec.Items))
	items := make([]PopupMenuItem, 0, len(spec.Items))
	for idx, item := range spec.Items {
		items = append(items, PopupMenuItem{
			Label:    item,
			Selected: idx == selectedIdx,
		})
	}
	scrollOffset := PopupMenuScrollOffset(selectedIdx, len(items), maxOpenRows)
	viewport := RenderPopupMenuViewport(items, fieldWidth-4, maxOpenRows, selectedIdx)
	panel := renderPopupDropdownPanel(viewport.Content, fieldWidth, dropdownScrollMarker(scrollOffset, viewport.MaxOffset))
	return PopupDropdownOverlay{
		Content: panel,
		Width:   lipgloss.Width(panel),
		Height:  lipgloss.Height(panel),
	}
}

func NormalizeDropdownSelectedIdx(selectedIdx int, itemCount int) int {
	if itemCount <= 0 {
		return 0
	}
	if selectedIdx < 0 {
		return 0
	}
	if selectedIdx >= itemCount {
		return itemCount - 1
	}
	return selectedIdx
}

func renderPopupDropdownControl(value string, width int, focused bool) string {
	innerWidth := width - 4
	if innerWidth < 1 {
		innerWidth = 1
	}
	display := truncateRunes(value, innerWidth-2)
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	if focused {
		borderStyle = borderStyle.Foreground(lipgloss.Color("248"))
	}
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	line := borderStyle.Render("│") +
		valueStyle.Render(" "+PadVisibleRight(display, innerWidth-2)+" ▾ ") +
		borderStyle.Render("│")
	return line
}

func renderPopupDropdownPanel(content string, width int, marker string) string {
	body := content
	if strings.TrimSpace(body) != "" {
		body += "\n"
	}
	body += popupHintStyle.Render(CenterVisible(marker, width-2))
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("245")).
		Render(body)
}

func dropdownScrollMarker(offset int, maxOffset int) string {
	switch {
	case maxOffset <= 0:
		return " "
	case offset <= 0:
		return "↓"
	case offset >= maxOffset:
		return "↑"
	default:
		return "↑ ↓"
	}
}

func PadVisibleRight(text string, width int) string {
	if width < 1 {
		return ""
	}
	text = truncateRunes(text, width)
	padding := width - lipgloss.Width(text)
	if padding < 0 {
		padding = 0
	}
	return text + strings.Repeat(" ", padding)
}

func CenterVisible(text string, width int) string {
	if width < 1 {
		return ""
	}
	text = truncateRunes(text, width)
	left := (width - lipgloss.Width(text)) / 2
	if left < 0 {
		left = 0
	}
	return strings.Repeat(" ", left) + text
}

func truncateRunes(s string, max int) string {
	if max < 1 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}
