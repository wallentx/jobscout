package tuiapp

import (
	"context"
	"fmt"
	"maps"
	"os"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/domain"
	healthpkg "github.com/wallentx/jobscout/internal/health"
	"github.com/wallentx/jobscout/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func getStatusStyle(status string, text string) string {
	switch status {
	case "Rejected":
		return renderToken(rejectedStyle, text)
	case "Ignore":
		return renderToken(ignoreStyle, text)
	case "Expired":
		return renderToken(expiredStyle, text)
	case "Unopened":
		return renderToken(unopenedStyle, text)
	case "Viewed":
		return renderToken(viewedStyle, text)
	case "Applied":
		return renderToken(appliedStyle, text)
	case "Interviewing":
		return renderToken(interviewingStyle, text)
	default:
		return text
	}
}

func getDateStyle(dateStr string, text string) string {
	if dateStr == "" {
		return text
	}

	// Parse the date (format is typically YYYY-MM-DD)
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return text
	}

	days := int(time.Since(t).Hours() / 24)

	// Determine color based on age
	var color lipgloss.Color
	if days <= 3 {
		color = lipgloss.Color("46") // Bright Green
	} else if days <= 7 {
		color = lipgloss.Color("120") // Pale Green
	} else if days <= 12 {
		color = lipgloss.Color("220") // Gold
	} else if days <= 15 {
		color = lipgloss.Color("208") // Orange
	} else if days <= 21 {
		color = lipgloss.Color("202") // Light Red
	} else if days <= 30 {
		color = lipgloss.Color("167") // Red
	} else if days <= 60 {
		color = lipgloss.Color("244") // Mid Gray
	} else {
		color = lipgloss.Color("238") // Dark Gray
	}

	style := lipgloss.NewStyle().Foreground(color)
	return renderToken(style, text)
}

func renderHealthReport(r *CompanyHealthResult, contentWidth int) string {
	// Define styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("255")).
		Background(lipgloss.Color("63")).
		Padding(0, 1)

	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("87")).
		MarginTop(1)

	// Available width is passed in
	availableWidth := contentWidth
	if availableWidth < 20 {
		availableWidth = 20
	}

	// Score emoji and color (with confidence-based shading)
	healthColor := getHealthColor(r.Score)
	scoreColor := lipgloss.NewStyle().Bold(true).Foreground(healthColor)

	var scoreEmoji string
	if r.Score >= 70 {
		scoreEmoji = "✓"
	} else if r.Score >= 50 {
		scoreEmoji = "⚠"
	} else {
		scoreEmoji = "✗"
	}

	// Build main content
	var main strings.Builder
	main.WriteString(titleStyle.Render(fmt.Sprintf(" %s ", r.Company)))
	main.WriteString("\n\n")

	if r.DiscoveredName != "" && !strings.EqualFold(r.DiscoveredName, r.Company) {
		discoveredStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Italic(true)
		main.WriteString(discoveredStyle.Render(fmt.Sprintf("SEC Entity: %s (%s)", r.DiscoveredName, r.DiscoveredTicker)))
		main.WriteString("\n\n")
	}

	// Health Score with emoji
	main.WriteString(sectionStyle.Render("HEALTH SCORE"))
	main.WriteString("\n")
	fmt.Fprintf(&main, "%s %s ", scoreEmoji,
		scoreColor.Render(fmt.Sprintf("%d/100", r.Score)))
	confidenceStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true)
	main.WriteString(confidenceStyle.Render(fmt.Sprintf("(%s confidence)", r.Confidence)))
	main.WriteString("\n")

	// Employment Risk
	if r.EmploymentRisk != nil {
		risk := r.EmploymentRisk
		main.WriteString("\n")
		main.WriteString(sectionStyle.Render("EMPLOYMENT RISK"))
		main.WriteString("\n")

		var riskEmoji string
		riskColor := lipgloss.NewStyle().Bold(true)
		switch risk.Level {
		case "Critical":
			riskColor = riskColor.Foreground(lipgloss.Color("196"))
			riskEmoji = "⚠⚠⚠"
		case "High":
			riskColor = riskColor.Foreground(lipgloss.Color("196"))
			riskEmoji = "⚠⚠"
		case "Medium":
			riskColor = riskColor.Foreground(lipgloss.Color("226"))
			riskEmoji = "⚠"
		default:
			riskColor = riskColor.Foreground(lipgloss.Color("42"))
			riskEmoji = "✓"
		}

		fmt.Fprintf(&main, "%s %s ", riskEmoji, riskColor.Render(risk.Level))
		main.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(fmt.Sprintf("(Score: %d/100)", risk.Score)))
		main.WriteString("\n")

		if len(risk.Factors) > 0 {
			factorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
			for _, f := range risk.Factors {
				main.WriteString(factorStyle.Render(fmt.Sprintf("  • %s", f)))
				main.WriteString("\n")
			}
		}
	}

	if graphBox := renderHealthStockGraphBox(r, availableWidth); graphBox != "" {
		topContent := strings.TrimRight(main.String(), "\n")
		main.Reset()
		if availableWidth > 80 {
			chartWidth := lipgloss.Width(graphBox)
			textWidth := availableWidth - chartWidth
			if textWidth >= 30 {
				main.WriteString(lipgloss.JoinHorizontal(
					lipgloss.Top,
					lipgloss.NewStyle().Width(textWidth).Render(topContent),
					graphBox,
				))
			} else {
				main.WriteString(topContent)
				main.WriteString("\n\n")
				main.WriteString(graphBox)
			}
		} else {
			main.WriteString(topContent)
			main.WriteString("\n\n")
			main.WriteString(graphBox)
		}
		main.WriteString("\n")
	}

	if r.LLMAssessment != nil {
		assessment := r.LLMAssessment
		main.WriteString("\n")
		main.WriteString(sectionStyle.Render("LLM REVIEW"))
		main.WriteString("\n")

		llmStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("219"))
		if strings.TrimSpace(assessment.Summary) != "" {
			main.WriteString(llmStyle.Render(fmt.Sprintf("  • %s", assessment.Summary)))
			main.WriteString("\n")
		}
		if strings.TrimSpace(assessment.RiskLevel) != "" || strings.TrimSpace(assessment.Recommendation) != "" {
			parts := joinNonEmpty(" - ", assessment.RiskLevel, assessment.Recommendation)
			main.WriteString(llmStyle.Render(fmt.Sprintf("  • %s", parts)))
			main.WriteString("\n")
		}
		for _, signal := range assessment.PositiveSignals {
			main.WriteString(llmStyle.Render(fmt.Sprintf("  + %s", signal)))
			main.WriteString("\n")
		}
		for _, concern := range assessment.Concerns {
			main.WriteString(llmStyle.Render(fmt.Sprintf("  ! %s", concern)))
			main.WriteString("\n")
		}
		for _, question := range assessment.FollowUpQuestions {
			main.WriteString(llmStyle.Render(fmt.Sprintf("  ? %s", question)))
			main.WriteString("\n")
		}
		if runtimeDebugEnabled && assessment.TokenUsage != nil && assessment.TokenUsage.Available() {
			usageStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
			main.WriteString(usageStyle.Render("  tokens: " + formatHealthLLMTokenUsage(*assessment.TokenUsage)))
			main.WriteString("\n")
		}
	}

	// Company Info
	main.WriteString("\n")
	main.WriteString(sectionStyle.Render("COMPANY INFO"))
	main.WriteString("\n")

	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	if r.FoundedYear != nil {
		age := "Unknown age"
		if r.AgeYears != nil {
			age = fmt.Sprintf("%d years old", *r.AgeYears)
		}
		main.WriteString(infoStyle.Render(fmt.Sprintf("Founded: %d (%s)", *r.FoundedYear, age)))
		main.WriteString("\n")
	}

	if r.Public != nil {
		status := "Private / No SEC Match"
		statusColor := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		if *r.Public {
			status = "Public (SEC Listed)"
			statusColor = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
		}
		fmt.Fprintf(&main, "Status: %s", statusColor.Render(status))
		main.WriteString("\n")
	}

	if r.DiscoveredTicker != "" {
		tickerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("51")).Bold(true)
		fmt.Fprintf(&main, "Ticker: %s", tickerStyle.Render(r.DiscoveredTicker))
		main.WriteString("\n")
	}

	// Layoff Signals
	if len(r.LayoffSignals) > 0 {
		main.WriteString("\n")
		warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("202")).Bold(true)
		main.WriteString(warningStyle.Render("⚠ LAYOFF SIGNALS DETECTED"))
		main.WriteString("\n")

		layoffStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
		for i, l := range r.LayoffSignals {
			if i >= 3 {
				main.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(fmt.Sprintf("  ... and %d more", len(r.LayoffSignals)-3)))
				main.WriteString("\n")
				break
			}
			main.WriteString(layoffStyle.Render(fmt.Sprintf("  • %s", l.Title)))
			main.WriteString("\n")
		}
	}

	// HN Signals
	if len(r.HNSignals) > 0 {
		main.WriteString("\n")
		main.WriteString(sectionStyle.Render("HACKER NEWS VIBE"))
		main.WriteString("\n")

		hnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
		for i, s := range r.HNSignals {
			if i >= 3 {
				main.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(fmt.Sprintf("  ... and %d more", len(r.HNSignals)-3)))
				main.WriteString("\n")
				break
			}
			main.WriteString(hnStyle.Render(fmt.Sprintf("  • %s (%d points, %d comments)", s.Title, s.Points, s.NumComments)))
			main.WriteString("\n")
		}
	}

	if len(r.RejectedEvidence) > 0 {
		main.WriteString("\n")
		main.WriteString(sectionStyle.Render("REJECTED EVIDENCE"))
		main.WriteString("\n")

		rejectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
		limit := min(5, len(r.RejectedEvidence))
		for i := 0; i < limit; i++ {
			evidence := r.RejectedEvidence[i]
			reason := evidence.Reason
			if reason == "" {
				reason = "unrelated to resolved company identity"
			}
			main.WriteString(rejectedStyle.Render(fmt.Sprintf("  • %s: %s", reason, evidence.Value)))
			main.WriteString("\n")
		}
		if len(r.RejectedEvidence) > limit {
			main.WriteString(rejectedStyle.Render(fmt.Sprintf("  ... and %d more", len(r.RejectedEvidence)-limit)))
			main.WriteString("\n")
		}
	}

	// Flags
	if len(r.Flags) > 0 {
		main.WriteString("\n")
		main.WriteString(sectionStyle.Render("FLAGS"))
		main.WriteString("\n")

		flagStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
		for _, flag := range r.Flags {
			main.WriteString(flagStyle.Render(fmt.Sprintf("  • %s", flag)))
			main.WriteString("\n")
		}
	}

	// Notes
	if len(r.Notes) > 0 {
		main.WriteString("\n")
		main.WriteString(sectionStyle.Render("ANALYSIS NOTES"))
		main.WriteString("\n")

		noteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
		for _, note := range r.Notes {
			main.WriteString(noteStyle.Render(fmt.Sprintf("  • %s", note)))
			main.WriteString("\n")
		}
	}

	if runtimeDebugEnabled {
		debugAssessments := renderHealthFieldAssessments(r)
		if debugAssessments != "" {
			main.WriteString("\n")
			main.WriteString(sectionStyle.Render("DEBUG EVIDENCE"))
			main.WriteString("\n")
			main.WriteString(debugAssessments)
			main.WriteString("\n")
		}
	}

	return main.String()
}

func formatHealthLLMTokenUsage(usage LLMTokenUsage) string {
	parts := make([]string, 0, 6)
	appendUsage := func(label string, value *int) {
		if value != nil {
			parts = append(parts, fmt.Sprintf("%s %d", label, *value))
		}
	}
	appendUsage("input", usage.InputTokens)
	appendUsage("output", usage.OutputTokens)
	appendUsage("total", usage.TotalTokens)
	appendUsage("cached", usage.CachedTokens)
	appendUsage("reasoning", usage.ReasoningTokens)
	appendUsage("thinking", usage.ThinkingTokens)
	return strings.Join(parts, " / ")
}

func renderHealthStockGraphBox(r *CompanyHealthResult, availableWidth int) string {
	if r == nil || r.Sources == nil {
		return ""
	}

	if stockData, ok := r.Sources["stock_history"]; ok {
		var prices []float64

		// Handle both direct []float64 and []interface{} from JSON unmarshaling
		switch v := stockData.(type) {
		case []float64:
			prices = v
		case []interface{}:
			for _, item := range v {
				if f, ok := item.(float64); ok {
					prices = append(prices, f)
				}
			}
		}

		if len(prices) > 0 {
			// Adjust graph width if terminal is narrow
			currentGraphWidth := tui.DefaultStockGraphWidth
			if availableWidth < 60 {
				currentGraphWidth = availableWidth - 15
				if currentGraphWidth < 10 {
					currentGraphWidth = 10
				}
			}

			graph := tui.RenderStockGraph(prices, currentGraphWidth, tui.DefaultStockGraphHeight)
			if graph != "" {
				graphStyle := lipgloss.NewStyle().
					BorderStyle(lipgloss.NormalBorder()).
					BorderForeground(lipgloss.Color("87")).
					Padding(1, 1).
					MarginLeft(2)

				graphTitle := lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("87")).
					Render("1-Year Stock Chart")

				return graphStyle.Render(graphTitle + "\n\n" + graph)
			}
		}
	}
	return ""
}

func renderHealthFieldAssessments(r *CompanyHealthResult) string {
	if r == nil || len(r.FieldAssessments) == 0 {
		return ""
	}

	fieldLabels := map[string]string{
		"estimated_employees": "Estimated Employees",
		"founded_year":        "Founded Year",
	}
	keys := slices.Sorted(maps.Keys(r.FieldAssessments))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true)
	acceptedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	candidateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208"))

	var out strings.Builder
	for _, key := range keys {
		assessment := r.FieldAssessments[key]
		if assessment == nil {
			continue
		}

		label := key
		if pretty, ok := fieldLabels[key]; ok {
			label = pretty
		}
		fmt.Fprintf(&out, "%s: %s", labelStyle.Render(label), valueStyle.Render(assessment.Status))
		if assessment.Confidence != "" {
			fmt.Fprintf(&out, " %s", valueStyle.Render("("+assessment.Confidence+")"))
		}
		if assessment.Source != "" {
			fmt.Fprintf(&out, " %s", valueStyle.Render("from "+assessment.Source))
		}
		out.WriteString("\n")

		for _, note := range assessment.Notes {
			out.WriteString(valueStyle.Render(fmt.Sprintf("  • %s", note)))
			out.WriteString("\n")
		}

		for _, evidence := range assessment.Evidence {
			statusStyle := candidateStyle
			status := "candidate"
			if evidence.Accepted {
				statusStyle = acceptedStyle
				status = "accepted"
			}
			out.WriteString(statusStyle.Render(fmt.Sprintf("  - %s: %s from %s (%s)", status, evidence.Value, evidence.Source, evidence.Confidence)))
			out.WriteString("\n")
			if evidence.Reason != "" {
				out.WriteString(valueStyle.Render("    " + evidence.Reason))
				out.WriteString("\n")
			}
			if evidence.URL != "" {
				out.WriteString(valueStyle.Render("    " + evidence.URL))
				out.WriteString("\n")
			}
		}
	}

	return strings.TrimRight(out.String(), "\n")
}

type healthLoadedMsg struct {
	company    string
	taskKey    string
	background bool
	result     *CompanyHealthResult
	err        error
	fetchedAt  time.Time
	fromCache  bool
}

var errCompanyHealthIdentityUnresolved = healthpkg.ErrIdentityUnresolved

func newHealthIdentityUnresolvedError(company string) error {
	return healthpkg.NewIdentityUnresolvedError(company)
}

func isHealthIdentityUnresolvedText(errText string) bool {
	return healthpkg.IsIdentityUnresolvedText(errText)
}

func renderHealthIdentityUnresolvedText(errText string) string {
	var content strings.Builder
	content.WriteString("Company identity unresolved\n\n")
	content.WriteString("Company health was skipped because this job does not have a verified company website or domain.\n\n")
	content.WriteString("Why this matters:\n")
	content.WriteString("Health checks use the company website to avoid mixing same-name companies, unrelated layoff records, and unrelated news.\n\n")
	if detail := strings.TrimSpace(errText); detail != "" {
		content.WriteString("Details: ")
		content.WriteString(detail)
		content.WriteString("\n\n")
	}
	content.WriteString("Next steps:\n")
	content.WriteString("Edit the job to add the company website, or run identity repair after fetching richer job data.\n\n")
	content.WriteString("Press Enter or Esc to return.")
	return content.String()
}

type bulkHealthStepMsg struct {
	company string
	result  *CompanyHealthResult
	err     error
	elapsed time.Duration
	mem     bulkHealthMemStats
}

const defaultBulkHealthConcurrency = 3
const termuxBulkHealthConcurrency = 1

type bulkHealthMemStats struct {
	allocMB      uint64
	totalAllocMB uint64
	sysMB        uint64
	numGC        uint32
	goroutines   int
}

func loadCompanyHealth(company string, forceRefresh bool) tea.Cmd {
	return loadCompanyHealthWithIdentity(CompanyHealthContext{Company: company}, forceRefresh)
}

func loadCompanyHealthForJob(job Job, forceRefresh bool) tea.Cmd {
	return loadCompanyHealthForJobWithContext(context.Background(), job, forceRefresh, "", false)
}

func loadCompanyHealthForJobWithContext(ctx context.Context, job Job, forceRefresh bool, taskKey string, background bool) tea.Cmd {
	return loadCompanyHealthWithIdentityAndContext(ctx, CompanyHealthContext{
		Company:                 job.Company,
		Website:                 job.CompanyWebsite,
		Summary:                 job.CompanySummary,
		Industry:                job.CompanyIndustry,
		RequireResolvedIdentity: true,
	}, forceRefresh, taskKey, background)
}

func loadCompanyHealthWithIdentity(identity CompanyHealthContext, forceRefresh bool) tea.Cmd {
	return loadCompanyHealthWithIdentityAndContext(context.Background(), identity, forceRefresh, "", false)
}

func loadCompanyHealthWithIdentityAndContext(ctx context.Context, identity CompanyHealthContext, forceRefresh bool, taskKey string, background bool) tea.Cmd {
	return func() tea.Msg {
		appCfg, cfgErr := config.LoadAppConfig(runtimeConfigPath)
		if cfgErr != nil {
			appCfg = nil
		}
		if ctx == nil {
			ctx = context.Background()
		}
		ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
		loaded := healthpkg.LoadCompanyHealth(ctx, identity, forceRefresh, runtimeHealthStore, appCfg)
		return healthLoadedMsg{
			company:    loaded.Company,
			taskKey:    taskKey,
			background: background,
			result:     loaded.Result,
			err:        loaded.Err,
			fetchedAt:  loaded.FetchedAt,
			fromCache:  loaded.FromCache,
		}
	}
}

func fetchBulkHealthStepWithContext(ctx context.Context, job Job) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		startMem := currentBulkHealthMemStats()
		appCfg, cfgErr := config.LoadAppConfig(runtimeConfigPath)
		if cfgErr != nil {
			appCfg = nil
		}
		identity := CompanyHealthContext{
			Company:                 job.Company,
			Website:                 job.CompanyWebsite,
			Summary:                 job.CompanySummary,
			Industry:                job.CompanyIndustry,
			RequireResolvedIdentity: true,
		}
		company := strings.TrimSpace(job.Company)
		logBulkHealthDebug("step start company=%q website=%q cache_key=%q mem=%s", company, job.CompanyWebsite, healthpkg.CacheKeyForJob(job), startMem)
		if domain.CompanyHealthContextDomain(identity) == "" {
			elapsed := time.Since(start)
			endMem := currentBulkHealthMemStats()
			logBulkHealthDebug("step skipped company=%q reason=identity_unresolved elapsed=%s mem=%s", company, elapsed.Round(time.Millisecond), endMem)
			return bulkHealthStepMsg{
				company: company,
				err:     newHealthIdentityUnresolvedError(company),
				elapsed: elapsed,
				mem:     endMem,
			}
		}
		if ctx == nil {
			ctx = context.Background()
		}
		ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
		loaded := healthpkg.RefreshCompanyHealth(ctx, identity, appCfg)
		elapsed := time.Since(start)
		endMem := currentBulkHealthMemStats()
		if loaded.Err != nil {
			logBulkHealthDebug("step done company=%q status=error elapsed=%s error=%v mem=%s", loaded.Company, elapsed.Round(time.Millisecond), loaded.Err, endMem)
		} else {
			logBulkHealthDebug("step done company=%q status=ok elapsed=%s result=%s mem=%s", loaded.Company, elapsed.Round(time.Millisecond), describeBulkHealthResult(loaded.Result), endMem)
		}
		return bulkHealthStepMsg{
			company: loaded.Company,
			result:  loaded.Result,
			err:     loaded.Err,
			elapsed: elapsed,
			mem:     endMem,
		}
	}
}

func bulkHealthConcurrency(total int) int {
	if total < 1 {
		return 0
	}
	limit := defaultBulkHealthConcurrency
	if isTermuxRuntime() {
		limit = termuxBulkHealthConcurrency
	}
	if total < limit {
		return total
	}
	return limit
}

func isTermuxRuntime() bool {
	if runtime.GOOS == "android" {
		return true
	}
	prefix := strings.ToLower(os.Getenv("PREFIX"))
	home := strings.ToLower(os.Getenv("HOME"))
	return strings.Contains(prefix, "com.termux") || strings.Contains(home, "com.termux")
}

func (m *model) scheduleBulkHealthCommands() tea.Cmd {
	total := len(m.bulkHealthJobs)
	if total == 0 {
		total = len(m.bulkHealthCompanies)
	}
	if !m.bulkHealthFetching || total == 0 {
		return nil
	}
	limit := bulkHealthConcurrency(total)
	if limit <= 0 {
		return nil
	}

	logBulkHealthDebug(
		"schedule total=%d idx=%d in_flight=%d limit=%d termux=%t mem=%s",
		total,
		m.bulkHealthIdx,
		m.bulkHealthInFlight,
		limit,
		isTermuxRuntime(),
		currentBulkHealthMemStats(),
	)

	cmds := make([]tea.Cmd, 0, limit)
	for m.bulkHealthInFlight < limit && m.bulkHealthIdx < total {
		var job Job
		if len(m.bulkHealthJobs) > 0 {
			job = m.bulkHealthJobs[m.bulkHealthIdx]
		} else {
			job = Job{Company: m.bulkHealthCompanies[m.bulkHealthIdx]}
		}
		m.bulkHealthIdx++
		m.bulkHealthInFlight++
		logBulkHealthDebug("schedule job index=%d company=%q website=%q cache_key=%q", m.bulkHealthIdx, job.Company, job.CompanyWebsite, healthpkg.CacheKeyForJob(job))
		cmds = append(cmds, fetchBulkHealthStepWithContext(modelTaskContext(*m), job))
	}
	return tea.Batch(cmds...)
}

func currentBulkHealthMemStats() bulkHealthMemStats {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return bulkHealthMemStats{
		allocMB:      stats.Alloc / 1024 / 1024,
		totalAllocMB: stats.TotalAlloc / 1024 / 1024,
		sysMB:        stats.Sys / 1024 / 1024,
		numGC:        stats.NumGC,
		goroutines:   runtime.NumGoroutine(),
	}
}

func (s bulkHealthMemStats) String() string {
	return fmt.Sprintf("alloc=%dMB total_alloc=%dMB sys=%dMB gc=%d goroutines=%d", s.allocMB, s.totalAllocMB, s.sysMB, s.numGC, s.goroutines)
}

func describeBulkHealthResult(result *CompanyHealthResult) string {
	if result == nil {
		return "nil"
	}
	return fmt.Sprintf(
		"score=%d confidence=%q signals=%d notes=%d sources=%d rejected=%d llm=%t",
		result.Score,
		result.Confidence,
		len(result.SignalsUsed),
		len(result.Notes),
		len(result.Sources),
		len(result.RejectedEvidence),
		result.LLMAssessment != nil,
	)
}

func logBulkHealthDebug(format string, args ...interface{}) {
	logDebug("bulk health: "+format, args...)
}

func renderProgressBar(done int, total int, width int) string {
	if total <= 0 {
		total = 1
	}
	if width < 8 {
		width = 8
	}
	if done < 0 {
		done = 0
	}
	if done > total {
		done = total
	}

	filled := int(float64(done) / float64(total) * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}

	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func bulkHealthProgressMessage(done int, total int, updated int, skipped int, failed int, running int, company string) string {
	percent := 0
	if total > 0 {
		percent = int(float64(done) / float64(total) * 100)
	}
	lines := []string{
		"Refreshing company health",
		"",
		fmt.Sprintf("%d of %d companies complete (%d%%)", done, total, percent),
		renderProgressBar(done, total, 30),
		"",
		fmt.Sprintf("Updated  %d", updated),
		fmt.Sprintf("Skipped  %d", skipped),
		fmt.Sprintf("Failed   %d", failed),
		fmt.Sprintf("Running  %d", running),
	}
	if strings.TrimSpace(company) != "" {
		lines = append(lines, "", fmt.Sprintf("Last finished: %s", company))
	}
	return strings.Join(lines, "\n")
}

func (m model) buildHealthOverlaySpec() popupSpec {
	width := clampPopupWidth(m.termWidth, 40, 0)
	maxHealthLines := popupMaxViewportLinesWithChrome(m.termHeight, 1, 8)

	var (
		body   string
		footer string
	)
	if m.overlay.health.loading {
		viewport := renderScrollablePopupText(
			renderLoadingBody(
				m.overlay.health.loadingText,
				"Loading company health data...\n\nPress Enter or Esc to return.",
				m.loading.frame,
				width-6,
			),
			width-6,
			maxHealthLines,
			m.overlay.health.scrollOffset,
			nil,
		)
		body = viewport.content
		footer = popupHintStyle.Render("Enter/Esc: Return")
		if viewport.maxOffset > 0 {
			footer = popupHintStyle.Render("↑/↓/PgUp/PgDn: Scroll • Enter/Esc: Return")
		}
	} else if m.overlay.health.err != "" {
		errBody := fmt.Sprintf("Error loading health data:\n%s\n\nPress Enter or Esc to return.", m.overlay.health.err)
		if isHealthIdentityUnresolvedText(m.overlay.health.err) {
			errBody = renderHealthIdentityUnresolvedText(m.overlay.health.err)
		}
		viewport := renderScrollablePopupText(errBody, width-6, maxHealthLines, m.overlay.health.scrollOffset, nil)
		body = viewport.content
		footer = popupHintStyle.Render("Enter/Esc: Return")
		if viewport.maxOffset > 0 {
			footer = popupHintStyle.Render("↑/↓/PgUp/PgDn: Scroll • Enter/Esc: Return")
		}
	} else if m.overlay.health.report != nil {
		targetLineWidth := width - 6
		fullReport := renderHealthReport(m.overlay.health.report, targetLineWidth)
		lines := structuredPopupLines(fullReport, targetLineWidth)
		viewport := renderPopupViewport(lines, targetLineWidth, maxHealthLines, m.overlay.health.scrollOffset, nil)
		body = viewport.content
		footerText := "h: Refresh • Enter/Esc: Return"
		if viewport.maxOffset > 0 {
			footerText = "↑/↓/PgUp/PgDn: Scroll • " + footerText
		}
		footer = popupHintStyle.Render(footerText)
	}

	return popupSpec{width: width, body: popupScrollableTextBody(body), footer: footer}
}
