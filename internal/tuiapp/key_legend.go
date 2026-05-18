package tuiapp

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func keyLegendMessage() string {
	return strings.Join([]string{
		keyLegendSection("Main view"),
		"",
		keyLegendItem("↑/↓ or j/k", "Move selection"),
		keyLegendItem("Enter", "Show job details"),
		keyLegendItem("h", "Show selected company health"),
		keyLegendItem("H", "Refresh health for all companies"),
		keyLegendItem("l", "Explain health symbols and colors"),
		keyLegendItem("?", "Show this key legend"),
		keyLegendItem("s", "Change status"),
		keyLegendItem("m", "Mark selected job viewed"),
		keyLegendItem("r", "Fetch jobs"),
		keyLegendItem("U", "Update missing job and company details"),
		keyLegendItem("V", "Check active postings"),
		keyLegendItem("c", "Configure Jobscout"),
		keyLegendItem("D", "Delete selected job"),
		keyLegendItem("E", "Edit selected job"),
		keyLegendItem("/", "Search company/title"),
		keyLegendItem("1-5", "Sort"),
		keyLegendItem("f", "Filter"),
		keyLegendItem("t", "Show or hide background task"),
		keyLegendItem("q", "Quit"),
		"",
		keyLegendSection("Detail window"),
		"",
		keyLegendItem("u", "Update selected job and company details"),
		keyLegendItem("o", "Open posting URL"),
		keyLegendItem("Enter/Esc", "Return"),
		"",
		keyLegendSection("Health window"),
		"",
		keyLegendItem("h", "Refresh current company health"),
		keyLegendItem("Enter/Esc", "Return"),
	}, "\n")
}

func keyLegendSection(text string) string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true)
	return renderToken(style, text)
}

func keyLegendItem(key string, description string) string {
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("110")).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	return fmt.Sprintf("%s: %s", renderToken(keyStyle, key), renderToken(descStyle, description))
}
