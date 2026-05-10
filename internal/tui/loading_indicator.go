package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	loadingBeamRadius      = 3
	loadingBeamPauseFrames = 6
)

var (
	loadingIdleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "255", Dark: "0"})
	loadingBeamStyles = []lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("#C6D8FF")).Bold(true),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#A8BEF8")).Bold(true),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#849DE4")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#657CC4")),
	}
)

func normalizeLoadingLabel(label string) string {
	normalized := strings.Join(strings.Fields(strings.ToUpper(label)), " ")
	if normalized == "" {
		return "LOADING"
	}
	return normalized
}

func loadingBeamTravelFrames(runeCount int) int {
	if runeCount < 1 {
		return 0
	}
	return runeCount + loadingBeamRadius*2
}

func loadingBeamFrame(frame int, runeCount int) (int, bool) {
	travelFrames := loadingBeamTravelFrames(runeCount)
	if travelFrames == 0 {
		return 0, true
	}

	step := frame % (travelFrames + loadingBeamPauseFrames)
	if step >= travelFrames {
		return 0, true
	}

	return step - loadingBeamRadius, false
}

func renderLoadingText(label string, frame int) string {
	text := normalizeLoadingLabel(label)
	runes := []rune(text)
	if len(runes) == 0 {
		return ""
	}
	center, paused := loadingBeamFrame(frame, len(runes))

	var out strings.Builder
	for idx, char := range runes {
		if char == ' ' {
			out.WriteRune(char)
			continue
		}

		style := loadingIdleStyle
		if !paused {
			distance := idx - center
			if distance < 0 {
				distance = -distance
			}
			if distance <= loadingBeamRadius {
				style = loadingBeamStyles[distance]
			}
		}
		out.WriteString(style.Render(string(char)))
	}

	return out.String()
}

func RenderLoadingTitle(label string, frame int) string {
	return renderLoadingText(label, frame)
}

func renderLoadingBanner(label string, frame int, width int) string {
	return lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Render(renderLoadingText(label, frame))
}

func RenderLoadingBody(label string, message string, frame int, width int) string {
	parts := []string{renderLoadingBanner(label, frame, width)}
	if trimmed := strings.TrimSpace(message); trimmed != "" {
		parts = append(parts, trimmed)
	}
	return strings.Join(parts, "\n\n")
}
