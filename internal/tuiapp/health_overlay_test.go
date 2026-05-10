package tuiapp

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestLoadCompanyHealthDoesNotMutateCacheInCmd(t *testing.T) {
	prevHealthStore := runtimeHealthStore
	fetchedAt := time.Now().Add(-time.Hour)
	runtimeHealthStore = &fakeHealthStore{
		getResult: &CompanyHealthResult{Company: "Acme", Score: 88},
		getTime:   fetchedAt,
	}
	t.Cleanup(func() {
		runtimeHealthStore = prevHealthStore
	})

	cache := make(HealthCache)
	msg := loadCompanyHealth("Acme", false)().(healthLoadedMsg)

	if len(cache) != 0 {
		t.Fatalf("loadCompanyHealth() mutated cache = %#v; want no mutation in cmd", cache)
	}
	if msg.company != "Acme" {
		t.Fatalf("healthLoadedMsg.company = %q; want Acme", msg.company)
	}
	if !msg.fromCache {
		t.Fatal("healthLoadedMsg.fromCache = false; want true")
	}
	if msg.result == nil || msg.result.Score != 88 {
		t.Fatalf("healthLoadedMsg.result = %#v; want score 88", msg.result)
	}
	if !msg.fetchedAt.Equal(fetchedAt) {
		t.Fatalf("healthLoadedMsg.fetchedAt = %v; want %v", msg.fetchedAt, fetchedAt)
	}
}

func TestLoadCompanyHealthForJobRequiresCompanyWebsite(t *testing.T) {
	msg := loadCompanyHealthForJob(Job{
		Company: "Circle",
		Title:   "Engineer",
	}, false)().(healthLoadedMsg)

	if msg.err == nil {
		t.Fatal("loadCompanyHealthForJob() err = nil, want resolved identity error")
	}
	if !errors.Is(msg.err, errCompanyHealthIdentityUnresolved) {
		t.Fatalf("loadCompanyHealthForJob() err = %v, want errCompanyHealthIdentityUnresolved", msg.err)
	}
	if !strings.Contains(msg.err.Error(), "company website/domain") {
		t.Fatalf("loadCompanyHealthForJob() err = %q, want company website/domain guidance", msg.err)
	}
}

func TestHealthOverlayRendersIdentityUnresolvedState(t *testing.T) {
	m := model{
		termWidth:  100,
		termHeight: 40,
		overlay: overlayState{
			kind: overlayHealth,
			health: healthOverlayState{
				err: newHealthIdentityUnresolvedError("Circle").Error(),
			},
		},
	}

	rendered := strings.Join(strings.Fields(ansi.Strip(m.buildHealthOverlaySpec().body.content)), " ")

	for _, expected := range []string{
		"Company identity unresolved",
		"Company health was skipped",
		"verified company website or domain",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("buildHealthOverlaySpec() body = %q, want %q", rendered, expected)
		}
	}
	if strings.Contains(rendered, "Error loading health data") {
		t.Fatalf("buildHealthOverlaySpec() body = %q, did not want generic error", rendered)
	}
}

func TestRenderHealthReportHidesDebugEvidenceWithoutDebugFlag(t *testing.T) {
	prevDebug := runtimeDebugEnabled
	runtimeDebugEnabled = false
	t.Cleanup(func() {
		runtimeDebugEnabled = prevDebug
	})

	report := &CompanyHealthResult{
		Company: "Acme",
		FieldAssessments: map[string]*CompanyHealthFieldAssessment{
			"founded_year": {
				Status:     fieldStatusConflict,
				Confidence: "medium",
				Source:     "company_site",
				Notes:      []string{"Conflicting founded year candidate."},
				Evidence: []CompanyHealthEvidence{
					{Value: "2018", Source: "wikipedia_summary", Confidence: "medium", Accepted: true},
				},
			},
		},
	}

	rendered := renderHealthReport(report, 100)
	if strings.Contains(rendered, "DEBUG EVIDENCE") {
		t.Fatalf("renderHealthReport() unexpectedly rendered debug evidence: %q", rendered)
	}
}

func TestRenderHealthReportShowsLLMAssessment(t *testing.T) {
	report := &CompanyHealthResult{
		Company:    "Acme",
		Score:      62,
		Confidence: "medium",
		LLMAssessment: &LLMCompanyHealthAssessment{
			Summary:           "Mixed signals, but not an automatic reject.",
			Recommendation:    "Investigate before applying.",
			RiskLevel:         "medium",
			PositiveSignals:   []string{"Some stabilizing signals exist."},
			Concerns:          []string{"Recent negative news needs review."},
			FollowUpQuestions: []string{"Ask about team stability."},
		},
	}

	rendered := ansi.Strip(renderHealthReport(report, 100))
	for _, expected := range []string{
		"LLM REVIEW",
		"Mixed signals, but not an automatic reject.",
		"Some stabilizing signals exist.",
		"Recent negative news needs review.",
		"Ask about team stability.",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("renderHealthReport() missing %q in:\n%s", expected, rendered)
		}
	}
}

func TestRenderHealthReportShowsLLMTokenUsageOnlyInDebug(t *testing.T) {
	prevDebug := runtimeDebugEnabled
	t.Cleanup(func() {
		runtimeDebugEnabled = prevDebug
	})

	report := &CompanyHealthResult{
		Company:    "Acme",
		Score:      62,
		Confidence: "medium",
		LLMAssessment: &LLMCompanyHealthAssessment{
			Summary: "Mixed signals.",
			TokenUsage: &LLMTokenUsage{
				InputTokens:     healthTestIntPtr(120),
				OutputTokens:    healthTestIntPtr(34),
				TotalTokens:     healthTestIntPtr(154),
				CachedTokens:    healthTestIntPtr(12),
				ReasoningTokens: healthTestIntPtr(7),
			},
		},
	}

	runtimeDebugEnabled = false
	rendered := ansi.Strip(renderHealthReport(report, 100))
	if strings.Contains(rendered, "tokens:") {
		t.Fatalf("renderHealthReport() rendered token usage without debug flag:\n%s", rendered)
	}

	runtimeDebugEnabled = true
	rendered = ansi.Strip(renderHealthReport(report, 100))
	want := "tokens: input 120 / output 34 / total 154 / cached 12 / reasoning 7"
	if !strings.Contains(rendered, want) {
		t.Fatalf("renderHealthReport() =\n%s\nwant token usage line %q", rendered, want)
	}
}

func TestRenderHealthReportStockGraphDoesNotConstrainLLMReview(t *testing.T) {
	report := &CompanyHealthResult{
		Company:    "Acme",
		Score:      82,
		Confidence: "high",
		Sources: map[string]interface{}{
			"stock_history": []float64{10, 12, 11, 14, 16, 15, 18, 20, 19, 21, 24, 23},
		},
		LLMAssessment: &LLMCompanyHealthAssessment{
			Summary: "This sentence appears below the stock graph and should use the full report width.",
		},
	}

	rendered := ansi.Strip(renderHealthReport(report, 120))
	chartIdx := strings.Index(rendered, "1-Year Stock Chart")
	reviewIdx := strings.Index(rendered, "LLM REVIEW")
	if chartIdx < 0 {
		t.Fatalf("renderHealthReport() missing stock chart in:\n%s", rendered)
	}
	if reviewIdx < 0 {
		t.Fatalf("renderHealthReport() missing LLM review in:\n%s", rendered)
	}
	if reviewIdx < chartIdx {
		t.Fatalf("LLM review rendered before chart; want text below chart to return to full width")
	}
	lines := strings.Split(rendered, "\n")
	for _, line := range lines {
		if strings.Contains(line, "This sentence appears below") && len(line) < 80 {
			t.Fatalf("LLM review line width = %d, appears constrained by chart column: %q", len(line), line)
		}
	}
}

func TestRenderHealthReportHNLineWrapsThroughPopupWrapper(t *testing.T) {
	report := &CompanyHealthResult{
		Company: "Hugging Face",
		HNSignals: []HNSignal{
			{
				Title:       "Ggml.ai joins Hugging Face to ensure the long-term progress of Local AI",
				Points:      839,
				NumComments: 223,
			},
		},
		Sources: map[string]interface{}{},
	}

	rendered := renderHealthReport(report, 72)
	lines := structuredPopupLines(rendered, 72)
	foundHN := false
	for i, line := range lines {
		stripped := ansi.Strip(line)
		if strings.Contains(stripped, "Ggml.ai joins") {
			foundHN = true
		}
		if strings.HasSuffix(stripped, "Local A") {
			t.Fatalf("line %d split Local AI after A: %#v", i, stripped)
		}
		if strings.TrimSpace(stripped) == "I" {
			t.Fatalf("line %d split Local AI before I: %#v", i, stripped)
		}
		if i > 0 && strings.Contains(stripped, "points") && !strings.HasPrefix(stripped, "    ") {
			t.Fatalf("HN continuation line %d = %#v, want bullet continuation indent", i, stripped)
		}
	}
	if !foundHN {
		t.Fatalf("rendered health report missing HN signal: %q", rendered)
	}
}

func TestRenderHealthReportShowsDebugEvidenceWithDebugFlag(t *testing.T) {
	prevDebug := runtimeDebugEnabled
	runtimeDebugEnabled = true
	t.Cleanup(func() {
		runtimeDebugEnabled = prevDebug
	})

	report := &CompanyHealthResult{
		Company: "Acme",
		FieldAssessments: map[string]*CompanyHealthFieldAssessment{
			"estimated_employees": {
				Status:     fieldStatusGap,
				Confidence: "low",
				Source:     "company_site",
				Notes:      []string{"Browser company-site lookup found no trustworthy employee-count evidence."},
				Evidence: []CompanyHealthEvidence{
					{Value: "500", Source: "company_site", Confidence: "medium", Accepted: false},
				},
			},
		},
	}

	rendered := renderHealthReport(report, 100)
	if !strings.Contains(rendered, "DEBUG EVIDENCE") {
		t.Fatalf("renderHealthReport() did not render debug evidence: %q", rendered)
	}
	if !strings.Contains(rendered, "estimated") && !strings.Contains(rendered, "gap") {
		t.Fatalf("renderHealthReport() missing field assessment details: %q", rendered)
	}
}

func TestScheduleBulkHealthCommandsUsesConcurrencyLimit(t *testing.T) {
	m := model{
		bulkHealthFetching: true,
		bulkHealthCompanies: []string{
			"Acme",
			"Globex",
			"Initech",
			"Umbrella",
		},
	}

	cmd := m.scheduleBulkHealthCommands()

	if cmd == nil {
		t.Fatal("scheduleBulkHealthCommands() = nil, want batch command")
	}
	want := bulkHealthConcurrency(len(m.bulkHealthCompanies))
	if m.bulkHealthInFlight != want {
		t.Fatalf("bulkHealthInFlight = %d, want %d", m.bulkHealthInFlight, want)
	}
	if m.bulkHealthIdx != want {
		t.Fatalf("bulkHealthIdx = %d, want %d", m.bulkHealthIdx, want)
	}
}

func healthTestIntPtr(value int) *int {
	return &value
}

func TestBulkHealthProgressMessageUsesCompactBar(t *testing.T) {
	rendered := bulkHealthProgressMessage(2, 5, 1, 0, 1, 3, "Acme")

	for _, expected := range []string{
		"Refreshing company health",
		"2 of 5 companies complete (40%)",
		"████████████",
		"░░░░░░░░░░░░",
		"Updated  1",
		"Failed   1",
		"Last finished: Acme",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("bulkHealthProgressMessage() = %q, want %q", rendered, expected)
		}
	}
	if strings.Contains(rendered, "[") || strings.Contains(rendered, "=") {
		t.Fatalf("bulkHealthProgressMessage() = %q, want compact block bar without bracket/equals style", rendered)
	}
}

func TestBulkHealthRefreshStartsMinimizableTaskPopup(t *testing.T) {
	prevStore := runtimeHealthStore
	runtimeHealthStore = &fakeHealthStore{}
	t.Cleanup(func() {
		runtimeHealthStore = prevStore
	})

	job := Job{
		Company:        "Acme",
		CompanyWebsite: "https://acme.example",
		Title:          "Backend Engineer",
		Status:         "Unopened",
	}
	m := model{
		termWidth:     100,
		termHeight:    40,
		allJobs:       []Job{job},
		filteredJobs:  []Job{job},
		activeFilters: filterValuesFromStatuses(nil),
		overlay:       overlayState{kind: overlayNone},
		healthCache:   make(HealthCache),
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("H")})
	if cmd == nil {
		t.Fatal("Update(H) cmd = nil, want bulk health command")
	}
	got := updated.(model)
	if !got.bulkHealthFetching {
		t.Fatal("bulkHealthFetching = false, want true")
	}
	if got.overlay.kind != overlayNone {
		t.Fatalf("overlay.kind = %v, want overlayNone while task popup owns progress", got.overlay.kind)
	}
	if !got.backgroundTask.active || !got.backgroundTask.expanded {
		t.Fatalf("backgroundTask = %#v, want active expanded task popup", got.backgroundTask)
	}
	if got.backgroundTask.title != "Refreshing Health Data" {
		t.Fatalf("backgroundTask.title = %q, want Refreshing Health Data", got.backgroundTask.title)
	}
	if !strings.Contains(got.backgroundTask.progress, "Refreshing company health") {
		t.Fatalf("backgroundTask.progress = %q, want health progress", got.backgroundTask.progress)
	}
}

func TestSingleHealthRefreshSuppressesDuplicateJob(t *testing.T) {
	job := Job{
		Company:        "Acme",
		CompanyWebsite: "https://acme.example",
		Title:          "Backend Engineer",
		Status:         "Unopened",
	}
	key := healthTaskKeyForJob(job)
	m := model{
		allJobs:       []Job{job},
		filteredJobs:  []Job{job},
		activeFilters: filterValuesFromStatuses(nil),
		overlay:       overlayState{kind: overlayNone},
		healthCache:   make(HealthCache),
		backgroundHealth: backgroundHealthState{
			tasks: map[string]singleHealthTaskState{
				key: {job: job, company: "Acme"},
			},
		},
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	got := updated.(model)
	if cmd != nil {
		t.Fatal("Update(h) cmd != nil for duplicate single health refresh")
	}
	if len(got.backgroundHealth.tasks) != 1 {
		t.Fatalf("single health tasks = %d; want existing task only", len(got.backgroundHealth.tasks))
	}
}

func TestBulkHealthRefreshExcludesRunningSingleHealthJobs(t *testing.T) {
	prevStore := runtimeHealthStore
	runtimeHealthStore = &fakeHealthStore{}
	t.Cleanup(func() {
		runtimeHealthStore = prevStore
	})

	running := Job{
		Company:        "Acme",
		CompanyWebsite: "https://acme.example",
		Title:          "Backend Engineer",
		Status:         "Unopened",
	}
	queued := Job{
		Company:        "Bravo",
		CompanyWebsite: "https://bravo.example",
		Title:          "Frontend Engineer",
		Status:         "Unopened",
	}
	m := model{
		termWidth:     100,
		termHeight:    40,
		allJobs:       []Job{running, queued},
		filteredJobs:  []Job{running, queued},
		activeFilters: filterValuesFromStatuses(nil),
		overlay:       overlayState{kind: overlayNone},
		healthCache:   make(HealthCache),
		backgroundHealth: backgroundHealthState{
			tasks: map[string]singleHealthTaskState{
				healthTaskKeyForJob(running): {job: running, company: "Acme"},
			},
		},
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("H")})
	if cmd == nil {
		t.Fatal("Update(H) cmd = nil; want bulk health command for non-running job")
	}
	got := updated.(model)
	if len(got.bulkHealthJobs) != 1 || got.bulkHealthJobs[0].Company != "Bravo" {
		t.Fatalf("bulkHealthJobs = %#v; want only Bravo", got.bulkHealthJobs)
	}
}

func TestBulkHealthUnresolvedIdentityCountsAsSkipped(t *testing.T) {
	m := model{
		termWidth:           100,
		termHeight:          40,
		bulkHealthFetching:  true,
		bulkHealthCompanies: []string{"Circle"},
		bulkHealthJobs:      []Job{{Company: "Circle"}},
		bulkHealthInFlight:  1,
	}

	updated, cmd := m.Update(bulkHealthStepMsg{
		company: "Circle",
		err:     newHealthIdentityUnresolvedError("Circle"),
	})
	if cmd != nil {
		t.Fatalf("Update(bulkHealthStepMsg unresolved) cmd = %v, want nil", cmd)
	}
	got := updated.(model)

	if got.bulkHealthFetching {
		t.Fatal("bulkHealthFetching = true, want false after final skipped result")
	}
	for _, expected := range []string{
		"Updated health data for 0 companies",
		"Skipped: 1",
		"Failed: 0",
	} {
		if !strings.Contains(got.overlay.notice.message, expected) {
			t.Fatalf("notice message = %q, want %q", got.overlay.notice.message, expected)
		}
	}
}
