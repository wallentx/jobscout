package tuiapp

import (
	"regexp"

	"github.com/wallentx/jobscout/internal/domain"
	healthpkg "github.com/wallentx/jobscout/internal/health"

	"github.com/charmbracelet/lipgloss"
)

func calculateColumnWidths(termWidth int) (int, int, int, int) {
	// Account for borders and padding manually
	// Frame is border(1)+padding(1) ... padding(1)+border(1) = 4 overhead
	// But we use Lipgloss to join, so we just need widths.
	availableWidth := termWidth - 4
	if availableWidth < 20 {
		availableWidth = 20
	}

	statusWidth := 13 // Max status is 12 chars ("Interviewing")
	dateWidth := 12   // 10 chars for date + 2 spaces for padding

	if availableWidth < 60 {
		statusWidth = 10
		dateWidth = 0
	}

	remaining := availableWidth - statusWidth - dateWidth
	// Health dot is part of Company column now, or separate.
	// Let's keep it simple: 2 chars for dot.
	// Company gets 45% of remaining.
	companyWidth := int(float64(remaining) * 0.45)
	titleWidth := remaining - companyWidth

	if companyWidth < 10 {
		companyWidth = 10
	}
	if titleWidth < 10 {
		titleWidth = 10
	}

	return companyWidth, titleWidth, statusWidth, dateWidth
}

func truncate(s string, max int) string {
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

var titleReplacements = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`(?i)\bSite Reliability Engineer\b`), "SRE"},
	{regexp.MustCompile(`(?i)\bSenior\b`), "SR"},
	{regexp.MustCompile(`(?i)\bStaff\b`), "STF"},
	{regexp.MustCompile(`(?i)\bPrincipal\b`), "PRIN"},
	{regexp.MustCompile(`(?i)\bInfrastructure\b`), "Infra."},
	{regexp.MustCompile(`(?i)\bEngineering\b`), "Eng."},
	{regexp.MustCompile(`(?i)\bEngineer\b`), "Eng."},
	{regexp.MustCompile(`(?i)\bMachine Learning\b`), "ML"},
}

func normalizeTitle(t string) string {
	for _, rule := range titleReplacements {
		t = rule.re.ReplaceAllString(t, rule.repl)
	}
	return t
}

func (m model) renderRow(job Job, selected bool, widths []int) string {
	compW, titleW, statW, dateW := widths[0], widths[1], widths[2], widths[3]

	// 1. Health/Company
	healthIndicator := "  "

	if job.Status == "Rejected" {
		healthIndicator = renderToken(rejectedStyle, "● ")
	} else if job.Status == "Ignore" {
		healthIndicator = renderToken(ignoreStyle, "● ")
	} else if job.Status == "Expired" {
		healthIndicator = renderToken(expiredStyle, "● ")
	} else if cached := healthpkg.CachedHealthForJob(m.healthCache, job); cached != nil {
		color := getHealthColor(cached.Score)
		symbol := "○"
		if cached.Confidence == "high" {
			symbol = "●"
		}
		if cached.Confidence == "medium" {
			symbol = "◉"
		}
		healthIndicator = renderToken(lipgloss.NewStyle().Foreground(color), symbol+" ")
	}

	// Truncate Company Name visual width
	// We have compW. Dot uses 2.
	// Relaxed truncation: availComp = compW - 2 (instead of -3)
	availComp := compW - 2
	if availComp < 1 {
		availComp = 1
	}
	truncComp := truncate(job.Company, availComp)

	// Apply style to truncated text to avoid ansi bloat
	switch job.Status {
	case "Rejected":
		truncComp = renderToken(rejectedStyle, truncComp)
	case "Ignore":
		truncComp = renderToken(ignoreStyle, truncComp)
	case "Expired":
		truncComp = renderToken(expiredStyle, truncComp)
	}

	// Pad Company Cell
	compCell := lipgloss.NewStyle().
		Width(compW).
		Render(healthIndicator + truncComp)

	// 2. Title
	// Truncate by 1 extra character to ensure a blank space before the Status column
	availTitle := titleW - 1
	if availTitle < 1 {
		availTitle = 1
	}
	title := normalizeTitle(job.Title)
	truncTitle := truncate(title, availTitle)
	// Shift Title to the right by 1 character
	titleCell := lipgloss.NewStyle().Width(titleW).Render(" " + truncTitle)

	// 3. Status
	// Truncate by 1 character to ensure a blank space before the Date column
	availStat := statW - 1
	if availStat < 1 {
		availStat = 1
	}
	truncStatus := truncate(job.Status, availStat)
	styledStatus := getStatusStyle(job.Status, truncStatus)
	// Shift Status right by 2 characters
	statusCell := lipgloss.NewStyle().Width(statW).Render("  " + styledStatus)

	// 4. Date Discovered
	dateStr := domain.FormatUnixDate(job.DateAdded)
	styledDate := getDateStyle(dateStr, dateStr)
	// Prepend 1 space to shift date column visually (shifted left by 1 from previous 2)
	// We want an empty space at the end, so we pad the cell width to `dateW`
	// which is 12. String is " " (1) + "YYYY-MM-DD" (10) = 11 chars.
	// This naturally leaves 1 space at the end!
	paddedDate := " " + styledDate
	dateCell := lipgloss.NewStyle().Width(dateW).Render(paddedDate)

	// Join
	row := lipgloss.JoinHorizontal(lipgloss.Top, compCell, titleCell, statusCell, dateCell)

	// Selection
	if selected {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57")).
			Bold(false).
			Render(row)
	}
	return row
}
