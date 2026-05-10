package tuiapp

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestCurrentDetailTextWrapsValuesUnderValueColumn(t *testing.T) {
	m := model{
		filteredJobs: []Job{
			{
				Company:        "MongoDB",
				CompanyWebsite: "https://www.mongodb.com",
				CompanySummary: "MongoDB builds a developer data platform for modern applications.",
				Title:          "Staff Site Reliability Engineer",
				ApplyURL:       "https://www.builtinchicago.org/jobs/remote/dev-engineering/senior/search/site-reliability-engineer",
				WhyMatches: []string{
					"Platform scalability and compliance tooling with multi-cloud governance alignment",
				},
			},
		},
	}

	rendered := ansi.Strip(m.currentDetailText(64))
	lines := strings.Split(rendered, "\n")
	foundApplyContinuation := false
	foundBulletContinuation := false
	for _, line := range lines {
		if strings.Contains(line, "engineering/senior") {
			foundApplyContinuation = true
			if !strings.HasPrefix(line, "       ") {
				t.Fatalf("apply continuation = %q, want value-column indentation", line)
			}
		}
		if strings.Contains(line, "governance alignment") {
			foundBulletContinuation = true
			if !strings.HasPrefix(line, "    ") {
				t.Fatalf("bullet continuation = %q, want bullet continuation indentation", line)
			}
		}
	}
	if !foundApplyContinuation {
		t.Fatalf("detail text missing wrapped apply continuation:\n%s", rendered)
	}
	if !foundBulletContinuation {
		t.Fatalf("detail text missing wrapped bullet continuation:\n%s", rendered)
	}
	for _, want := range []string{
		"Company Website: https://www.mongodb.com",
		"Company Summary: MongoDB builds a developer data platform",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("detail text missing %q:\n%s", want, rendered)
		}
	}
}

func TestLoadingTickAdvancesWhileBusy(t *testing.T) {
	m := model{
		overlay: overlayState{
			kind: overlayNotice,
			notice: noticeState{
				visible: true,
				busy:    true,
				title:   "Fetching Jobs",
			},
		},
	}

	updated, cmd := m.Update(loadingTickMsg{})
	got := updated.(model)

	if got.loading.frame != 1 {
		t.Fatalf("loading.frame = %d; want 1", got.loading.frame)
	}
	if cmd == nil {
		t.Fatal("cmd = nil; want next loading tick")
	}
}

func TestDimPopupBackgroundPreservesVisibleTextAndReappliesAfterReset(t *testing.T) {
	background := "plain \x1b[31mRejected\x1b[0m after"

	got := dimPopupBackground(background)
	stripped := ansi.Strip(got)
	if stripped != "plain Rejected after" {
		t.Fatalf("dimPopupBackground(...) visible text = %q, want original text", stripped)
	}
	if !strings.HasPrefix(got, "\x1b[2m") {
		t.Fatalf("dimPopupBackground(...) = %q; want faint prefix", got)
	}
	if !strings.Contains(got, "\x1b[0m\x1b[2m") {
		t.Fatalf("dimPopupBackground(...) = %q; want faint reapplied after reset", got)
	}
	if !strings.HasSuffix(got, "\x1b[22m") {
		t.Fatalf("dimPopupBackground(...) = %q; want faint reset suffix", got)
	}
}

func TestPopupDialogForDimmedBackgroundClearsAndResumesFaint(t *testing.T) {
	dialog := "╭──╮\n│ Popup │\n╰──╯"

	got := popupDialogForDimmedBackground(dialog)
	lines := strings.Split(got, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "\x1b[22m") {
			t.Fatalf("line %d = %q; want faint reset prefix", i, line)
		}
		if !strings.HasSuffix(line, "\x1b[2m") {
			t.Fatalf("line %d = %q; want faint resume suffix", i, line)
		}
	}
	if ansi.Strip(got) != dialog {
		t.Fatalf("popupDialogForDimmedBackground(...) visible text = %q, want %q", ansi.Strip(got), dialog)
	}
}

func TestLoadingTickAdvancesForBackgroundTask(t *testing.T) {
	m := model{
		backgroundTask: backgroundTaskState{
			active: true,
			title:  "Post-acceptance enrichment",
		},
	}

	updated, cmd := m.Update(loadingTickMsg{})
	got := updated.(model)

	if got.loading.frame != 1 {
		t.Fatalf("loading.frame = %d; want 1", got.loading.frame)
	}
	if cmd == nil {
		t.Fatal("cmd = nil; want next loading tick")
	}
}

func TestBackgroundTaskRendersActivityAndLegendHotkey(t *testing.T) {
	m := model{
		termWidth:     100,
		termHeight:    30,
		tableHeight:   calculateTableHeight(30),
		activeFilters: filterValuesFromStatuses(nil),
		backgroundTask: backgroundTaskState{
			active:   true,
			title:    "Post-acceptance enrichment",
			progress: "Enriching accepted jobs in the background...",
		},
	}

	rendered := ansi.Strip(m.View())

	if !strings.Contains(rendered, "✨ POST-ACCEPTANCE ENRICHMENT") {
		t.Fatalf("View() missing background task activity title:\n%s", rendered)
	}
	if !strings.Contains(rendered, "t: Task") {
		t.Fatalf("View() missing task legend hotkey:\n%s", rendered)
	}
}

func TestViewRendersLogoWhenJobTableIsEmpty(t *testing.T) {
	restoreRuntimePathsAfterTest(t)
	runtimeBuildVersion = "v1.2.3-abcdef0"

	m := model{
		termWidth:     110,
		termHeight:    28,
		tableHeight:   calculateTableHeight(28),
		activeFilters: filterValuesFromStatuses(nil),
	}

	rendered := m.View()
	stripped := ansi.Strip(rendered)

	if !strings.Contains(stripped, "██████╗") {
		t.Fatalf("View() missing empty-state logo:\n%s", stripped)
	}
	if !strings.Contains(stripped, "v1.2.3-abcdef0") {
		t.Fatalf("View() missing version beside empty-state logo:\n%s", stripped)
	}
	if !strings.Contains(rendered, "\x1b[38;2;198;216;255m") {
		t.Fatalf("View() missing top logo accent color")
	}
	if !strings.Contains(rendered, "\x1b[38;2;104;124;197m") {
		t.Fatalf("View() missing lower logo depth color")
	}
	if !strings.Contains(rendered, "\x1b[38;2;154;154;154m╗") {
		t.Fatalf("View() missing gray shadow/outline glyph")
	}
	if !strings.Contains(rendered, "\x1b[38;2;30;30;30m╚") {
		t.Fatalf("View() missing darkest gray bottom shadow")
	}
}

func TestBackgroundTaskActivityBlinksSparkleAndAnimatesTitle(t *testing.T) {
	m := model{
		backgroundTask: backgroundTaskState{
			active: true,
			title:  "Post-acceptance enrichment",
		},
	}

	visible := ansi.Strip(m.backgroundTaskActivityView())
	if !strings.Contains(visible, "✨ POST-ACCEPTANCE ENRICHMENT") {
		t.Fatalf("visible activity = %q; want sparkle and animated title", visible)
	}

	offFrame := -1
	for frame := 1; frame < 200; frame++ {
		if taskActivityGlyphStateForFrame(frame) == taskActivityGlyphOff {
			offFrame = frame
			break
		}
	}
	if offFrame == -1 {
		t.Fatal("task activity pulse never hides the sparkle")
	}
	m.loading.frame = offFrame
	hidden := ansi.Strip(m.backgroundTaskActivityView())
	if strings.Contains(hidden, "✨") {
		t.Fatalf("hidden activity = %q; want sparkle blinked off", hidden)
	}
	if !strings.Contains(hidden, "POST-ACCEPTANCE ENRICHMENT") {
		t.Fatalf("hidden activity = %q; want title to stay visible", hidden)
	}
}

func TestBlockingBusyNoticeCanMinimizeToTaskActivity(t *testing.T) {
	m := model{
		overlay: overlayState{
			kind: overlayNotice,
			notice: noticeState{
				visible: true,
				busy:    true,
				title:   "Fetching Jobs",
				message: "Starting configured source search...",
			},
		},
		termWidth:  90,
		termHeight: 30,
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	got := updated.(model)
	if !got.overlay.notice.minimized {
		t.Fatal("notice.minimized = false; want busy notice minimized")
	}
	if _, ok := got.buildMainOverlaySpec(); ok {
		t.Fatal("buildMainOverlaySpec() rendered minimized busy notice")
	}
	if activity := ansi.Strip(got.backgroundTaskActivityView()); !strings.Contains(activity, "FETCHING JOBS") {
		t.Fatalf("activity = %q; want minimized loading title", activity)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	got = updated.(model)
	if got.overlay.notice.minimized {
		t.Fatal("notice.minimized = true after second t; want restored")
	}
	if _, ok := got.buildMainOverlaySpec(); !ok {
		t.Fatal("buildMainOverlaySpec() did not render restored busy notice")
	}
}

func TestHealthLoadingCompletionStaysBackgroundedWhenMinimized(t *testing.T) {
	m := model{
		healthCache: make(HealthCache),
		overlay: overlayState{
			kind: overlayHealth,
			health: healthOverlayState{
				loading:     true,
				minimized:   true,
				loadingText: "Refreshing Acme",
			},
		},
		backgroundHealth: backgroundHealthState{
			tasks: map[string]singleHealthTaskState{
				"acme.example": {company: "Acme"},
			},
		},
		termWidth:  90,
		termHeight: 30,
	}

	updated, _ := m.Update(healthLoadedMsg{
		company:   "Acme",
		taskKey:   "acme.example",
		result:    &CompanyHealthResult{Company: "Acme", Sources: map[string]any{}},
		fetchedAt: time.Now(),
	})
	got := updated.(model)

	if got.overlay.kind != overlayNone {
		t.Fatalf("overlay.kind = %v after minimized load completion; want overlayNone", got.overlay.kind)
	}
	if _, ok := got.buildMainOverlaySpec(); !ok {
		return
	}
	t.Fatal("buildMainOverlaySpec() rendered completed health popup; want background completion")
}

func TestBackgroundTaskHotkeyTogglesExpandedPopup(t *testing.T) {
	m := model{
		termWidth:     100,
		termHeight:    30,
		activeFilters: filterValuesFromStatuses(nil),
		backgroundTask: backgroundTaskState{
			active:   true,
			title:    "Post-acceptance enrichment",
			progress: "Enriching accepted jobs in the background...",
		},
	}

	// First 't' starts expansion animation
	animating, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	animatingModel := animating.(model)
	if !animatingModel.backgroundTask.animating {
		t.Fatal("backgroundTask.animating = false after first t; want true")
	}
	if animatingModel.backgroundTask.expanded {
		t.Fatal("backgroundTask.expanded = true immediately after first t; want false until animation finishes")
	}

	// Drive animation to completion
	curr := animating
	for i := 0; i < 10; i++ {
		if cmd == nil {
			break
		}
		msg := cmd()
		curr, cmd = curr.Update(msg)
	}

	expandedModel := curr.(model)
	if expandedModel.backgroundTask.animating {
		t.Fatal("backgroundTask.animating = true after animation ticks; want false")
	}
	if !expandedModel.backgroundTask.expanded {
		t.Fatal("backgroundTask.expanded = false after animation ticks; want true")
	}
	if spec, ok := expandedModel.buildBackgroundTaskOverlaySpecIfActive(); !ok || !strings.Contains(ansi.Strip(spec.body.content), "Enriching accepted jobs") {
		t.Fatalf("expanded task overlay = %#v, %t; want progress popup", spec, ok)
	}

	// Second 't' starts minimization animation
	animating, cmd = expandedModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	animatingModel = animating.(model)
	if !animatingModel.backgroundTask.animating {
		t.Fatal("backgroundTask.animating = false after second t; want true")
	}

	// Drive animation to completion
	curr = animating
	for i := 0; i < 10; i++ {
		if cmd == nil {
			break
		}
		msg := cmd()
		curr, cmd = curr.Update(msg)
	}

	minimizedModel := curr.(model)
	if minimizedModel.backgroundTask.expanded {
		t.Fatal("backgroundTask.expanded = true after animation finished; want false")
	}
}

func TestDetailShowsPendingEnrichmentAndRefreshesAfterCompletion(t *testing.T) {
	prevJobStore := runtimeJobStore
	fakeStore := &fakeJobStore{}
	runtimeJobStore = fakeStore
	t.Cleanup(func() {
		runtimeJobStore = prevJobStore
	})

	job := Job{
		Company:  "TempCo",
		Title:    "Backend Engineer",
		ApplyURL: "https://jobs.example/tempco/backend-engineer",
		Status:   "Unopened",
	}
	m := model{
		allJobs:       []Job{job},
		filteredJobs:  []Job{job},
		activeFilters: filterValuesFromStatuses(nil),
		overlay: overlayState{
			kind: overlayDetail,
		},
		backgroundTask: backgroundTaskState{
			active: true,
			id:     7,
			pendingFields: map[string]map[string]bool{
				backgroundJobKey(job): pendingEnrichmentFields(job),
			},
		},
	}

	pending := ansi.Strip(m.currentDetailText(80))
	if !strings.Contains(pending, "Company Website: Enrichment loading") {
		t.Fatalf("currentDetailText() missing pending website marker:\n%s", pending)
	}
	if !strings.Contains(pending, "Company Summary: Enrichment loading") {
		t.Fatalf("currentDetailText() missing pending summary marker:\n%s", pending)
	}

	enriched := job
	enriched.CompanyWebsite = "https://tempco.example"
	enriched.CompanySummary = "TempCo builds developer tools for software teams that need reliable internal platforms and deployment workflows."
	enriched.CompanyIndustry = "Developer Tools"
	enriched.Compensation = "$100,000"
	updated, _ := m.Update(acceptedFetchEnrichedMsg{taskID: 7, jobs: []Job{enriched}})
	got := updated.(model)

	if got.overlay.kind != overlayDetail {
		t.Fatalf("overlay.kind = %v; want detail overlay preserved", got.overlay.kind)
	}
	rendered := ansi.Strip(got.currentDetailText(80))
	for _, want := range []string{
		"Company Website: https://tempco.example",
		"Company Summary: TempCo builds developer tools for software teams",
		"Company Industry: Developer Tools",
		"Compensation: $100,000",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("currentDetailText() missing %q after enrichment:\n%s", want, rendered)
		}
	}
}

func TestDetailOverlayNavigatesBetweenJobs(t *testing.T) {
	jobs := []Job{
		{Company: "Acme", Title: "Backend Engineer"},
		{Company: "Bravo", Title: "Frontend Engineer"},
	}
	m := model{
		allJobs:      jobs,
		filteredJobs: jobs,
		cursor:       0,
		overlay:      overlayState{kind: overlayDetail},
		tableHeight:  10,
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	got := updated.(model)
	if got.cursor != 1 {
		t.Fatalf("cursor = %d; want 1 after right in detail overlay", got.cursor)
	}
	rendered := ansi.Strip(got.currentDetailText(80))
	if !strings.Contains(rendered, "Company: Bravo") {
		t.Fatalf("detail text = %q; want Bravo after navigation", rendered)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyLeft})
	got = updated.(model)
	if got.cursor != 0 {
		t.Fatalf("cursor = %d; want 0 after left in detail overlay", got.cursor)
	}
}

func TestBusyNoticeAnimatesTitleWithoutDuplicatingBodyLabel(t *testing.T) {
	m := model{
		overlay: overlayState{
			kind: overlayNotice,
			notice: noticeState{
				visible: true,
				busy:    true,
				title:   "Fetching Jobs",
				message: "Starting configured source search...",
			},
		},
		termWidth:  90,
		termHeight: 30,
	}

	spec := m.buildNoticeOverlaySpec()
	renderedTitle := ansi.Strip(spec.titleView)
	renderedBody := ansi.Strip(spec.body.content)

	if !strings.Contains(renderedTitle, "FETCHING JOBS") {
		t.Fatalf("busy notice title = %q; want animated title text", renderedTitle)
	}
	if !strings.Contains(renderedBody, "Starting configured source search...") {
		t.Fatalf("busy notice body = %q; want original message", renderedBody)
	}
	if strings.Contains(renderedBody, "FETCHING JOBS") {
		t.Fatalf("busy notice body = %q; want no duplicated loading label", renderedBody)
	}
}

func TestRealTimeJobEnrichmentUpdatesDetailView(t *testing.T) {
	job := Job{Company: "TestCorp", Title: "Dev"}
	m := model{
		allJobs:      []Job{job},
		filteredJobs: []Job{job},
		cursor:       0,
		backgroundTask: backgroundTaskState{
			active: true,
			id:     123,
			pendingFields: map[string]map[string]bool{
				backgroundJobKey(job): {"company_website": true, "company_summary": true},
			},
		},
		overlay: overlayState{kind: overlayDetail},
	}

	// 1. Initial view shows loading state
	rendered := m.currentDetailText(80)
	if !strings.Contains(rendered, enrichmentLoadingText) {
		t.Fatalf("Initial detail view missing loading text:\n%s", rendered)
	}

	// 2. Job is enriched in background
	enrichedJob := job
	enrichedJob.CompanyWebsite = "https://testcorp.com"
	updated, _ := m.Update(backgroundJobEnrichedMsg{taskID: 123, job: enrichedJob})
	updatedModel := updated.(model)

	// 3. Updated view shows enriched data while still tracking pending fields.
	renderedUpdated := ansi.Strip(updatedModel.currentDetailText(80))
	if !strings.Contains(renderedUpdated, "https://testcorp.com") {
		t.Fatalf("Updated detail view missing enriched website:\n%s", renderedUpdated)
	}
	if !strings.Contains(renderedUpdated, "Company Summary: "+enrichmentLoadingText) {
		t.Fatalf("Updated detail view missing pending summary loading text:\n%s", renderedUpdated)
	}

	enrichedJob.CompanySummary = "TestCorp builds reliable developer tools for software delivery teams."
	updated, _ = updatedModel.Update(backgroundJobEnrichedMsg{taskID: 123, job: enrichedJob})
	updatedModel = updated.(model)
	renderedUpdated = updatedModel.currentDetailText(80)
	if strings.Contains(renderedUpdated, enrichmentLoadingText) {
		t.Fatalf("Fully updated detail view still shows loading text:\n%s", renderedUpdated)
	}
}

func TestBackgroundJobEnrichedMsgKeepsListeningForUpdates(t *testing.T) {
	job := Job{Company: "TestCorp", Title: "Dev"}
	progressCh := make(chan string)
	jobCh := make(chan Job)
	m := model{
		allJobs:      []Job{job},
		filteredJobs: []Job{job},
		backgroundTask: backgroundTaskState{
			active: true,
			id:     123,
			pendingFields: map[string]map[string]bool{
				backgroundJobKey(job): {"company_website": true},
			},
		},
	}

	enrichedJob := job
	enrichedJob.CompanyWebsite = "https://testcorp.com"
	_, cmd := m.Update(backgroundJobEnrichedMsg{
		taskID: 123,
		job:    enrichedJob,
		ch:     progressCh,
		jobCh:  jobCh,
	})

	if cmd == nil {
		t.Fatal("backgroundJobEnrichedMsg command = nil; want continued channel listener")
	}
}

func TestWaitForBackgroundTaskProgressHandlesClosedJobChannel(t *testing.T) {
	jobCh := make(chan Job)
	close(jobCh)

	cmd := waitForBackgroundTaskProgress(123, nil, jobCh)
	if cmd == nil {
		t.Fatal("waitForBackgroundTaskProgress() = nil; want completion command")
	}

	msgCh := make(chan tea.Msg, 1)
	go func() {
		msgCh <- cmd()
	}()

	select {
	case msg := <-msgCh:
		progressMsg, ok := msg.(backgroundTaskProgressMsg)
		if !ok {
			t.Fatalf("message type = %T; want backgroundTaskProgressMsg", msg)
		}
		if !progressMsg.done {
			t.Fatal("backgroundTaskProgressMsg.done = false; want true")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("waitForBackgroundTaskProgress blocked after job channel closed")
	}
}

func TestNoticePopupUsesHealthPopupWidth(t *testing.T) {
	m := model{
		termWidth:  100,
		termHeight: 40,
	}
	m.showNotice("Fetch Review", "Fetched 3 results. Added 2 new jobs.", false)

	noticeSpec := m.buildNoticeOverlaySpec()
	healthSpec := model{
		termWidth:  100,
		termHeight: 40,
		overlay: overlayState{
			kind: overlayHealth,
			health: healthOverlayState{
				report: &CompanyHealthResult{
					Company: "Acme",
					Sources: map[string]interface{}{},
				},
			},
		},
	}.buildHealthOverlaySpec()

	if noticeSpec.width != healthSpec.width {
		t.Fatalf("notice width = %d; want health width %d", noticeSpec.width, healthSpec.width)
	}
}
