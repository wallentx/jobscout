package tuiapp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/fetcher"

	tea "github.com/charmbracelet/bubbletea"
)

func TestApplyOptionalLLMJobFilteringReturnsNoticeWhenUnavailable(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")

	appCfg := config.DefaultAppConfig()
	appCfg.LLM.JobFiltering = true
	appCfg.LLM.JobSearch = false

	jobs := []Job{{
		Company:     "Acme",
		Title:       "Platform Engineer",
		Source:      "RSS: Example",
		ApplyURL:    "https://jobs.example/acme-platform-engineer",
		Description: "Acme is hiring a platform engineer to build deployment tooling, improve observability, support remote production teams, and own reliability work across distributed systems.",
	}}
	gotJobs, notices := applyOptionalLLMJobFiltering(context.Background(), &appCfg, nil, jobs)

	if len(gotJobs) != 1 || gotJobs[0].Company != "Acme" {
		t.Fatalf("applyOptionalLLMJobFiltering(...) jobs = %#v, want original jobs preserved", gotJobs)
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "LLM job filtering skipped") {
		t.Fatalf("applyOptionalLLMJobFiltering(...) notices = %#v, want skip notice", notices)
	}
}

func TestApplyOptionalLLMJobFilteringWithFreshTimeoutDoesNotReuseExpiredFetchContext(t *testing.T) {
	previous := filterJobsWithLLM
	t.Cleanup(func() {
		filterJobsWithLLM = previous
	})

	called := false
	filterJobsWithLLM = func(ctx context.Context, appCfg *AppConfig, criteriaCfg *CriteriaConfig, jobs []Job) ([]Job, []string) {
		called = true
		if err := ctx.Err(); err != nil {
			t.Fatalf("filter context error = %v; want fresh active context", err)
		}
		if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) <= 0 {
			t.Fatalf("filter context deadline = %v ok=%t; want future deadline", deadline, ok)
		}
		return jobs[:1], nil
	}

	appCfg := config.DefaultAppConfig()
	appCfg.LLM.Enabled = true
	appCfg.LLM.JobFiltering = true
	jobs := []Job{{Company: "Acme", Title: "Platform Engineer"}}

	gotJobs, notices := applyOptionalLLMJobFilteringWithFreshTimeout(&appCfg, nil, jobs)

	if !called {
		t.Fatal("filterJobsWithLLM was not called")
	}
	if len(notices) != 0 {
		t.Fatalf("notices = %#v; want none", notices)
	}
	if len(gotJobs) != 1 || gotJobs[0].Company != "Acme" {
		t.Fatalf("gotJobs = %#v; want filtered jobs from fresh-context call", gotJobs)
	}
}

func TestRecordLLMJobFilteringOutcomeAddsDroppedJobsToSummary(t *testing.T) {
	appCfg := config.DefaultAppConfig()
	appCfg.LLM.Enabled = true
	appCfg.LLM.JobSearch = false
	appCfg.LLM.JobFiltering = true

	before := []Job{
		{Company: "KeepCo", Title: "Platform Engineer", Source: "RSS: Example"},
		{Company: "DropCo", Title: "Product Manager", Source: "RSS: Example"},
	}
	after := []Job{before[0]}
	summary := FetchSummary{}

	recordLLMJobFilteringOutcome(&appCfg, &summary, before, after, nil)

	gotJobs := summary.Filtered[llmJobFilteringReason]
	if len(gotJobs) != 1 {
		t.Fatalf("recordLLMJobFilteringOutcome(...).Filtered[%q] len = %d, want 1", llmJobFilteringReason, len(gotJobs))
	}
	if gotJobs[0].Company != "DropCo" {
		t.Fatalf("recordLLMJobFilteringOutcome(...).Filtered[%q][0].Company = %q, want DropCo", llmJobFilteringReason, gotJobs[0].Company)
	}
}

func TestRecordLLMJobFilteringOutcomeSkipsWhenFilterUnavailable(t *testing.T) {
	appCfg := config.DefaultAppConfig()
	appCfg.LLM.Enabled = true
	appCfg.LLM.JobSearch = false
	appCfg.LLM.JobFiltering = true

	before := []Job{{Company: "Acme", Title: "Platform Engineer", Source: "RSS: Example"}}
	summary := FetchSummary{}

	recordLLMJobFilteringOutcome(&appCfg, &summary, before, nil, []string{"LLM job filtering skipped: unavailable"})

	if got := len(summary.Filtered[llmJobFilteringReason]); got != 0 {
		t.Fatalf("recordLLMJobFilteringOutcome(...).Filtered[%q] len = %d, want 0", llmJobFilteringReason, got)
	}
}

func TestRecordLLMJobFilteringBypassReasonsAddsKeptBypassGroups(t *testing.T) {
	appCfg := config.DefaultAppConfig()
	appCfg.LLM.Enabled = true
	appCfg.LLM.JobSearch = false
	appCfg.LLM.JobFiltering = true

	jobs := []Job{
		{
			Company:     "UNKNOWN",
			Title:       "Backend Engineer",
			Source:      "Site Search: LinkedIn",
			ApplyURL:    "https://www.linkedin.com/jobs/view/123",
			Description: "A backend engineering role.",
		},
	}
	summary := FetchSummary{}

	recordLLMJobFilteringBypassReasons(&appCfg, nil, &summary, jobs)

	gotJobs := summary.LLMFilterBypass["weak_identity"]
	if len(gotJobs) != 1 {
		t.Fatalf("summary.LLMFilterBypass[weak_identity] len = %d, want 1; all = %#v", len(gotJobs), summary.LLMFilterBypass)
	}
	if gotJobs[0].Title != "Backend Engineer" {
		t.Fatalf("summary.LLMFilterBypass[weak_identity][0].Title = %q, want Backend Engineer", gotJobs[0].Title)
	}
}

func TestJobsDroppedByLLMFilterHandlesDuplicateKeys(t *testing.T) {
	before := []Job{
		{Company: "Acme", Title: "Platform Engineer", Source: "RSS: A"},
		{Company: "Acme", Title: "Platform Engineer", Source: "RSS: B"},
		{Company: "Beta", Title: "Backend Engineer", Source: "RSS: C"},
	}
	after := []Job{before[1]}

	got := jobsDroppedByLLMFilter(before, after)

	if len(got) != 2 {
		t.Fatalf("jobsDroppedByLLMFilter(%#v, %#v) len = %d, want 2", before, after, len(got))
	}
	acmeDropped := 0
	betaDropped := 0
	for _, job := range got {
		switch job.Company {
		case "Acme":
			acmeDropped++
		case "Beta":
			betaDropped++
		}
	}
	if acmeDropped != 1 || betaDropped != 1 {
		t.Fatalf("jobsDroppedByLLMFilter(%#v, %#v) dropped counts = Acme:%d Beta:%d, want Acme:1 Beta:1", before, after, acmeDropped, betaDropped)
	}
}

func TestFetchReviewRequiresExplicitConfirmation(t *testing.T) {
	prevJobStore := runtimeJobStore
	fakeStore := &fakeJobStore{}
	runtimeJobStore = fakeStore
	t.Cleanup(func() {
		runtimeJobStore = prevJobStore
	})

	existing := []Job{{Company: "PersistedCo", Title: "Existing Role", Status: "Viewed"}}
	fetched := []Job{{
		Company:         "TempCo",
		Title:           "Preview Role",
		Source:          "RSS: https://example.com/feed",
		CompanyWebsite:  "https://tempco.example",
		CompanySummary:  "TempCo builds internal developer tools.",
		CompanyIndustry: "Developer Tools",
		Compensation:    "$100,000",
	}}
	summary := FetchSummary{Searches: map[string]string{fetcher.FetchSearchRSS: "executed; found 1 results"}}

	m := model{
		allJobs:            append([]Job(nil), existing...),
		activeFilters:      filterValuesFromStatuses(nil),
		sessionLLMDisabled: true,
		termWidth:          100,
		termHeight:         30,
	}

	updated, _ := m.Update(fetchJobsMsg{jobs: fetched, summary: summary})
	got := updated.(model)

	if got.pendingFetch == nil {
		t.Fatal("pendingFetch = nil; want review state")
	}
	if len(fakeStore.saved) != 0 {
		t.Fatalf("saved jobs = %#v; want no save before confirmation", fakeStore.saved)
	}

	discarded, _ := got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	discardedModel := discarded.(model)
	if discardedModel.pendingFetch != nil {
		t.Fatal("pendingFetch != nil after Esc; want discarded review")
	}
	if len(fakeStore.saved) != 0 {
		t.Fatalf("saved jobs after Esc = %#v; want no save", fakeStore.saved)
	}

	confirmed, cmd := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	confirmedModel := confirmed.(model)
	if len(fakeStore.saved) != 2 {
		t.Fatalf("saved jobs len = %d immediately after Enter; want 2 raw accepted jobs saved before background enrichment", len(fakeStore.saved))
	}
	if confirmedModel.pendingFetch != nil {
		t.Fatal("pendingFetch != nil after Enter; want cleared after acceptance")
	}
	if confirmedModel.overlay.kind != overlayNone {
		t.Fatalf("overlay.kind = %v; want overlayNone after accepting fetch review", confirmedModel.overlay.kind)
	}
	if !confirmedModel.backgroundTask.active {
		t.Fatal("backgroundTask.active = false; want enrichment task after acceptance")
	}
	if !confirmedModel.backgroundTask.expanded {
		t.Fatal("backgroundTask.expanded = false; want task expanded by default")
	}
	if confirmedModel.backgroundTask.animProgress != 1 {
		t.Fatalf("backgroundTask.animProgress = %v; want 1 for expanded task", confirmedModel.backgroundTask.animProgress)
	}
	if cmd == nil {
		t.Fatal("accept command = nil; want background enrichment command")
	}

	enriched := fetched[0]
	enriched.CompanyWebsite = "https://tempco.example"
	enriched.CompanySummary = "TempCo builds internal developer tools."
	enriched.CompanyIndustry = "Developer Tools"
	enriched.Compensation = "$100,000"
	savedModelRaw, _ := confirmedModel.Update(acceptedFetchEnrichedMsg{taskID: confirmedModel.backgroundTask.id, jobs: []Job{enriched}})
	savedModel := savedModelRaw.(model)
	if savedModel.backgroundTask.active {
		t.Fatal("backgroundTask.active = true after acceptedFetchEnrichedMsg; want task cleared")
	}
	if len(savedModel.allJobs) != 2 {
		t.Fatalf("allJobs len = %d after acceptedFetchEnrichedMsg; want 2", len(savedModel.allJobs))
	}
	if savedModel.allJobs[1].CompanyWebsite != "https://tempco.example" {
		t.Fatalf("enriched CompanyWebsite = %q, want https://tempco.example", savedModel.allJobs[1].CompanyWebsite)
	}
}

func TestFetchReviewWithoutNewJobsShowsInformationalNotice(t *testing.T) {
	prevJobStore := runtimeJobStore
	fakeStore := &fakeJobStore{}
	runtimeJobStore = fakeStore
	t.Cleanup(func() {
		runtimeJobStore = prevJobStore
	})

	existing := []Job{{Company: "Acme", Title: "Existing Role", Status: "Viewed"}}
	fetched := []Job{{Company: "Acme", Title: "Existing Role", Source: "RSS: https://example.com/feed"}}
	summary := FetchSummary{Searches: map[string]string{fetcher.FetchSearchRSS: "executed; found 1 results"}}

	m := model{
		allJobs:       append([]Job(nil), existing...),
		activeFilters: filterValuesFromStatuses(nil),
		termWidth:     100,
		termHeight:    30,
	}

	updated, _ := m.Update(fetchJobsMsg{jobs: fetched, summary: summary})
	got := updated.(model)

	if got.pendingFetch != nil {
		t.Fatal("pendingFetch != nil; want informational notice when no new jobs were added")
	}
	if got.overlay.notice.title != "Fetch Complete" {
		t.Fatalf("notice title = %q; want Fetch Complete", got.overlay.notice.title)
	}
	if !strings.Contains(got.overlay.notice.message, "Fetched 1 results. Added 0 new jobs.") {
		t.Fatalf("notice message = %q; want zero-add summary", got.overlay.notice.message)
	}
	if strings.Contains(got.overlay.notice.message, "Press Enter to keep") {
		t.Fatalf("notice message = %q; did not want save confirmation prompt", got.overlay.notice.message)
	}
	closed, _ := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	closedModel := closed.(model)
	if len(fakeStore.saved) != 0 {
		t.Fatalf("saved jobs = %#v; want no save from informational notice", fakeStore.saved)
	}
	if closedModel.overlay.kind != overlayNone {
		t.Fatalf("overlay.kind = %v; want overlayNone after closing informational notice", closedModel.overlay.kind)
	}
}

func TestRemoveExistingJobsBeforeReviewUsesCanonicalApplyURL(t *testing.T) {
	existing := []Job{{
		Company:  "Acme Inc.",
		Title:    "Software Engineer",
		ApplyURL: "https://jobs.example/acme/software-engineer",
	}}
	fetched := []Job{
		{
			Company:  "Acme",
			Title:    "Software Developer",
			ApplyURL: "https://jobs.example/acme/software-engineer/",
			Source:   "Site Search: example",
		},
		{
			Company:  "Beta",
			Title:    "Platform Engineer",
			ApplyURL: "https://jobs.example/beta/platform-engineer",
			Source:   "Site Search: example",
		},
	}
	summary := FetchSummary{}

	kept := removeExistingJobsBeforeReview(fetched, existing, &summary)

	if len(kept) != 1 || kept[0].Company != "Beta" {
		t.Fatalf("kept = %#v; want only new Beta job", kept)
	}
	if got := len(summary.Filtered["already saved"]); got != 1 {
		t.Fatalf("summary.Filtered[already saved] len = %d; want 1", got)
	}
}

func TestRemoveExistingJobsBeforeReviewUsesMergeKey(t *testing.T) {
	existing := []Job{{
		Company:  "Acme",
		Title:    "Software Engineer",
		ApplyURL: "https://jobs.example/acme/software-engineer-1",
	}}
	fetched := []Job{
		{
			Company:  "Acme",
			Title:    "Software Engineer",
			ApplyURL: "https://jobs.example/acme/software-engineer-2",
			Source:   "Site Search: example",
		},
		{
			Company:  "Beta",
			Title:    "Platform Engineer",
			ApplyURL: "https://jobs.example/beta/platform-engineer",
			Source:   "Site Search: example",
		},
	}
	summary := FetchSummary{}

	kept := removeExistingJobsBeforeReview(fetched, existing, &summary)

	if len(kept) != 1 || kept[0].Company != "Beta" {
		t.Fatalf("kept = %#v; want only new Beta job", kept)
	}
	if got := len(summary.Filtered["already saved"]); got != 1 {
		t.Fatalf("summary.Filtered[already saved] len = %d; want 1", got)
	}
}
