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
	for _, band := range healthScoreBands {
		b.WriteString(healthLegendScoreLine(band))
	}

	b.WriteString("\nStatus overrides\n")
	fmt.Fprintf(&b, "%s Rejected\n", renderToken(rejectedStyle, "●"))
	fmt.Fprintf(&b, "%s Ignore\n", renderToken(ignoreStyle, "●"))
	fmt.Fprintf(&b, "%s Expired\n", renderToken(expiredStyle, "●"))
	return b.String()
}

func healthLegendScoreLine(band healthScoreBand) string {
	dotStyle := lipgloss.NewStyle().Foreground(band.Color)
	return fmt.Sprintf("%s %s: %s - %s\n", renderToken(dotStyle, "●"), band.Range, band.Label, band.Description)
}
