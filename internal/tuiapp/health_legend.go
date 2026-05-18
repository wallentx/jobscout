package tuiapp

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func healthLegendMessage() string {
	var b strings.Builder
	b.WriteString("Company column marker\n\n")
	b.WriteString("Shape shows confidence. Color shows score.\n\n")

	b.WriteString("Confidence symbols\n")
	symbolStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	fmt.Fprintf(&b, "%s High confidence\n", renderToken(symbolStyle, "●"))
	fmt.Fprintf(&b, "%s Medium confidence\n", renderToken(symbolStyle, "◉"))
	fmt.Fprintf(&b, "%s Low confidence\n", renderToken(symbolStyle, "○"))
	b.WriteString("  No symbol: no health data cached yet\n\n")

	b.WriteString("Score colors\n")
	b.WriteString(healthLegendScoreLine(80, "75-100", "Strong", "good stability signals"))
	b.WriteString(healthLegendScoreLine(65, "60-74", "Stable", "mostly positive signals"))
	b.WriteString(healthLegendScoreLine(50, "45-59", "Watch", "mixed or limited signals"))
	b.WriteString(healthLegendScoreLine(35, "30-44", "Risk", "meaningful caution signals"))
	b.WriteString(healthLegendScoreLine(15, "0-29", "Critical", "serious health concerns"))

	b.WriteString("\nStatus overrides\n")
	fmt.Fprintf(&b, "%s Rejected\n", renderToken(rejectedStyle, "●"))
	fmt.Fprintf(&b, "%s Ignore\n", renderToken(ignoreStyle, "●"))
	fmt.Fprintf(&b, "%s Expired\n", renderToken(expiredStyle, "●"))
	return b.String()
}

func healthLegendScoreLine(score int, band string, label string, description string) string {
	dotStyle := lipgloss.NewStyle().Foreground(getHealthColor(score))
	return fmt.Sprintf("%s %s: %s - %s\n", renderToken(dotStyle, "●"), band, label, description)
}
