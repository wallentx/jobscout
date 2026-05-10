package tuiapp

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240")).
	Margin(1, 3, 1, 1) // Top, Right, Bottom, Left (Shifted left by 1 from 1,2,1,2)

var helpStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("241")).
	Margin(0, 0, 0, 1)

var helpKeyStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("110")).
	Bold(true)

var helpValueStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("248"))

var popupTitleStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("117")).
	Bold(true)

var popupBodyStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("252"))

var popupHintStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("245"))

var popupSelectActiveStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("230")).
	Background(lipgloss.Color("60")).
	Bold(true)

var popupSelectInactiveStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("250"))

var detailLabelStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("117")).
	Bold(true)

var detailValueStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("252"))

var detailSectionStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("110")).
	Bold(true)

var detailBulletStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("150"))

var rejectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
var ignoreStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
var expiredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
var unopenedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("87"))     // Cyan
var viewedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("147"))      // Light Purple/Blue
var appliedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))     // Orange
var interviewingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("46")) // Bright Green

func logDebug(format string, args ...interface{}) {
	if !runtimeDebugEnabled {
		return
	}
	f, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer func() {
		_ = f.Close()
	}()
	_, _ = fmt.Fprintf(f, format+"\n", args...)
}

// renderToken renders text with a style but replaces the "Reset All" ANSI code (\x1b[0m)
// with "Default Foreground" (\x1b[39m). This prevents the inner style from resetting
// the background color applied by the parent (e.g., the table selection highlight).

func renderToken(style lipgloss.Style, text string) string {
	out := style.Render(text)
	return strings.ReplaceAll(out, "\x1b[0m", "\x1b[39m")
}

var statuses = []string{
	"Unopened",
	"Viewed",
	"Applied",
	"Interviewing",
	"Rejected",
	"Ignore",
	"Expired",
}

var statusEmojis = map[string]string{
	"Unopened":     "📪",
	"Viewed":       "📄",
	"Applied":      "📨",
	"Interviewing": "💬",
	"Rejected":     "❌",
	"Ignore":       "🙈",
	"Expired":      "⏰",
}
