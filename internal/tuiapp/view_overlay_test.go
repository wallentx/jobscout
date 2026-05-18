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

func TestStaleLoadingTickDoesNotAdvanceAnimation(t *testing.T) {
	m := model{
		backgroundHealth: backgroundHealthState{
			tasks: map[string]singleHealthTaskState{
				"acme":  {company: "Acme"},
				"bravo": {company: "Bravo"},
			},
		},
	}

	_ = m.restartLoadingIndicator()
	staleTick := loadingTickMsg{generation: m.loading.generation}
	_ = m.restartLoadingIndicator()
	currentTick := loadingTickMsg{generation: m.loading.generation}

	updated, cmd := m.Update(staleTick)
	got := updated.(model)
	if got.loading.frame != 0 {
		t.Fatalf("loading.frame = %d after stale tick; want 0", got.loading.frame)
	}
	if cmd != nil {
		t.Fatal("cmd != nil after stale tick; want no replacement tick")
	}

	updated, cmd = got.Update(currentTick)
	got = updated.(model)
	if got.loading.frame != 1 {
		t.Fatalf("loading.frame = %d after current tick; want 1", got.loading.frame)
	}
	if cmd == nil {
		t.Fatal("cmd = nil after current tick; want next loading tick")
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

func TestMainListLegendKeyShowsHealthLegend(t *testing.T) {
	m := model{
		termWidth:     100,
		termHeight:    30,
		tableHeight:   calculateTableHeight(30),
		activeFilters: filterValuesFromStatuses(nil),
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	got := updated.(model)

	if cmd != nil {
		t.Fatalf("Update(l) cmd = %v; want nil", cmd)
	}
	if got.overlay.kind != overlayNotice {
		t.Fatalf("overlay.kind = %v; want health legend notice", got.overlay.kind)
	}
	if got.overlay.notice.title != "Health Legend" {
		t.Fatalf("notice title = %q; want Health Legend", got.overlay.notice.title)
	}
	message := ansi.Strip(got.overlay.notice.message)
	for _, want := range []string{
		"● High confidence",
		"◉ Medium confidence",
		"○ Low confidence",
		"75-100",
		"60-74",
		"45-59",
		"30-44",
		"0-29",
		"Rejected",
		"Ignore",
		"Expired",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("health legend missing %q:\n%s", want, message)
		}
	}
}

func TestViewHelpIncludesHealthLegendHotkey(t *testing.T) {
	m := model{
		termWidth:     100,
		termHeight:    30,
		tableHeight:   calculateTableHeight(30),
		activeFilters: filterValuesFromStatuses(nil),
	}

	rendered := ansi.Strip(m.View())
	if !strings.Contains(rendered, "l: Legend") {
		t.Fatalf("View() missing health legend hotkey:\n%s", rendered)
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
	logoLine := -1
	for i, line := range strings.Split(stripped, "\n") {
		if strings.Contains(line, "██████╗") {
			logoLine = i
			break
		}
	}
	if logoLine == -1 || logoLine > 4 {
		t.Fatalf("logoLine = %d; want normal title logo near top:\n%s", logoLine, stripped)
	}
	if !strings.Contains(stripped, "v1.2.3-abcdef0") {
		t.Fatalf("View() missing version below empty-state logo:\n%s", stripped)
	}
	for _, line := range strings.Split(stripped, "\n") {
		if strings.Contains(line, "v1.2.3-abcdef0") && strings.Contains(line, "█") {
			t.Fatalf("version rendered inside logo line %q; want version below logo", line)
		}
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

func TestSetupJobsTableRendersLogoAboveBottomMessage(t *testing.T) {
	restoreRuntimePathsAfterTest(t)
	runtimeBuildVersion = "v1.2.3-abcdef0"

	height := 18
	rendered := ansi.Strip(renderSetupEmptyTable(96, height, 2, runtimeBuildVersion))
	lines := strings.Split(rendered, "\n")
	logoLine := -1
	messageLine := -1
	versionLine := -1
	for i, line := range lines {
		switch {
		case logoLine == -1 && strings.Contains(line, "██████╗"):
			logoLine = i
		case versionLine == -1 && strings.Contains(line, "v1.2.3-abcdef0"):
			versionLine = i
		case messageLine == -1 && strings.Contains(line, "Setup jobs view"):
			messageLine = i
		}
	}
	if logoLine == -1 {
		t.Fatalf("View() missing setup jobs logo:\n%s", rendered)
	}
	if versionLine == -1 || versionLine <= logoLine {
		t.Fatalf("versionLine = %d, logoLine = %d; want version below logo:\n%s", versionLine, logoLine, rendered)
	}
	if messageLine == -1 {
		t.Fatalf("View() missing setup jobs message:\n%s", rendered)
	}
	if messageLine <= versionLine {
		t.Fatalf("messageLine = %d, versionLine = %d; want setup message below logo/version:\n%s", messageLine, versionLine, rendered)
	}
	if messageLine < height-3 {
		t.Fatalf("messageLine = %d; want setup message near bottom of table:\n%s", messageLine, rendered)
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

func TestDismissedHealthLoadingCompletionStaysDismissed(t *testing.T) {
	keys := []tea.KeyType{tea.KeyEnter, tea.KeyEsc}
	for _, key := range keys {
		t.Run(key.String(), func(t *testing.T) {
			m := model{
				healthCache: make(HealthCache),
				overlay: overlayState{
					kind: overlayHealth,
					health: healthOverlayState{
						loading:     true,
						loadingText: "Refreshing Acme",
					},
				},
				backgroundHealth: backgroundHealthState{
					expanded: true,
					progress: 1,
					tasks: map[string]singleHealthTaskState{
						"acme.example": {company: "Acme"},
					},
				},
				termWidth:  90,
				termHeight: 30,
			}

			dismissed, _ := m.Update(tea.KeyMsg{Type: key})
			got := dismissed.(model)
			if got.overlay.kind != overlayNone {
				t.Fatalf("overlay.kind after %s = %v; want overlayNone", key, got.overlay.kind)
			}

			completed, _ := got.Update(healthLoadedMsg{
				company:   "Acme",
				taskKey:   "acme.example",
				result:    &CompanyHealthResult{Company: "Acme", Sources: map[string]any{}},
				fetchedAt: time.Now(),
			})
			got = completed.(model)

			if got.overlay.kind != overlayNone {
				t.Fatalf("overlay.kind after dismissed health load completes = %v; want overlayNone", got.overlay.kind)
			}
			if cached := got.healthCache["Acme"].Result; cached == nil || cached.Company != "Acme" {
				t.Fatalf("healthCache[Acme] = %#v; want completed result cached", got.healthCache["Acme"])
			}
		})
	}
}

func TestBackgroundHealthActivityUsesOneGenericLoadingLabel(t *testing.T) {
	m := model{
		termWidth:  100,
		termHeight: 30,
		overlay: overlayState{
			kind: overlayHealth,
			health: healthOverlayState{
				loading:     true,
				minimized:   true,
				loadingText: "Checking Acme",
			},
		},
		backgroundHealth: backgroundHealthState{
			tasks: map[string]singleHealthTaskState{
				"acme":  {company: "Acme"},
				"bravo": {company: "Bravo"},
			},
		},
	}

	rendered := ansi.Strip(m.backgroundTaskActivityView())
	if !strings.Contains(rendered, "REFRESHING HEALTH DATA") {
		t.Fatalf("activity = %q; want generic health refresh label", rendered)
	}
	if strings.Contains(rendered, "CHECKING ACME") {
		t.Fatalf("activity = %q; want no stale per-company minimized label", rendered)
	}

	view := ansi.Strip(m.View())
	if count := strings.Count(view, "REFRESHING HEALTH DATA"); count != 1 {
		t.Fatalf("View() rendered REFRESHING HEALTH DATA %d times; want one compact activity", count)
	}
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

func TestEnterAndEscapeCloseDetailAndHealthOverlays(t *testing.T) {
	tests := []struct {
		name    string
		overlay overlayState
	}{
		{
			name:    "detail",
			overlay: overlayState{kind: overlayDetail},
		},
		{
			name: "health",
			overlay: overlayState{
				kind: overlayHealth,
				health: healthOverlayState{
					report: &CompanyHealthResult{Company: "Acme", Score: 82},
				},
			},
		},
	}

	keys := []tea.KeyType{tea.KeyEnter, tea.KeyEsc}
	for _, tt := range tests {
		for _, key := range keys {
			t.Run(tt.name+"/"+key.String(), func(t *testing.T) {
				m := model{
					allJobs:      []Job{{Company: "Acme", Title: "Backend Engineer"}},
					filteredJobs: []Job{{Company: "Acme", Title: "Backend Engineer"}},
					overlay:      tt.overlay,
				}

				updated, _ := m.Update(tea.KeyMsg{Type: key})
				got := updated.(model)
				if got.overlay.kind != overlayNone {
					t.Fatalf("overlay.kind = %v; want overlayNone", got.overlay.kind)
				}
			})
		}
	}
}

func TestBackgroundHealthOverlayCyclesTasksWithLeftRight(t *testing.T) {
	m := model{
		termWidth:  100,
		termHeight: 30,
		backgroundHealth: backgroundHealthState{
			tasks: map[string]singleHealthTaskState{
				"bravo": {company: "Bravo"},
				"acme":  {company: "Acme"},
			},
			expanded: true,
			progress: 1,
		},
	}

	spec, ok := m.buildSingleHealthTaskOverlaySpecIfActive()
	if !ok {
		t.Fatal("buildSingleHealthTaskOverlaySpecIfActive() ok = false; want expanded overlay")
	}
	rendered := ansi.Strip(spec.body.content + "\n" + spec.footer)
	if !strings.Contains(rendered, "Acme") || !strings.Contains(rendered, "[1/2]") {
		t.Fatalf("initial background health overlay = %q; want first task and [1/2]", rendered)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	got := updated.(model)
	spec, ok = got.buildSingleHealthTaskOverlaySpecIfActive()
	if !ok {
		t.Fatal("buildSingleHealthTaskOverlaySpecIfActive() ok = false after right")
	}
	rendered = ansi.Strip(spec.body.content + "\n" + spec.footer)
	if !strings.Contains(rendered, "Bravo") || !strings.Contains(rendered, "[2/2]") {
		t.Fatalf("background health overlay after right = %q; want second task and [2/2]", rendered)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyLeft})
	got = updated.(model)
	spec, ok = got.buildSingleHealthTaskOverlaySpecIfActive()
	if !ok {
		t.Fatal("buildSingleHealthTaskOverlaySpecIfActive() ok = false after left")
	}
	rendered = ansi.Strip(spec.body.content + "\n" + spec.footer)
	if !strings.Contains(rendered, "Acme") || !strings.Contains(rendered, "[1/2]") {
		t.Fatalf("background health overlay after left = %q; want first task and [1/2]", rendered)
	}
}

func TestMinimizedHealthPopupTaskHotkeyExpandsBackgroundHealthTasks(t *testing.T) {
	m := model{
		termWidth:  100,
		termHeight: 30,
		overlay: overlayState{
			kind: overlayHealth,
			health: healthOverlayState{
				loading:     true,
				minimized:   true,
				loadingText: "Checking Acme",
			},
		},
		backgroundHealth: backgroundHealthState{
			tasks: map[string]singleHealthTaskState{
				"acme":  {company: "Acme"},
				"bravo": {company: "Bravo"},
			},
		},
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	got := updated.(model)
	if !got.overlay.health.minimized {
		t.Fatal("overlay.health.minimized = false; want stale health popup to stay minimized")
	}
	if !got.backgroundHealth.animating {
		t.Fatal("backgroundHealth.animating = false; want task overlay expansion")
	}
	if cmd == nil {
		t.Fatal("cmd = nil; want background task animation tick")
	}
}

func TestDetailOverlayNavigatesBetweenJobsWithUpDown(t *testing.T) {
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

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := updated.(model)
	if got.cursor != 1 {
		t.Fatalf("cursor = %d; want 1 after down in detail overlay", got.cursor)
	}
	rendered := ansi.Strip(got.currentDetailText(80))
	if !strings.Contains(rendered, "Company: Bravo") {
		t.Fatalf("detail text = %q; want Bravo after navigation", rendered)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyUp})
	got = updated.(model)
	if got.cursor != 0 {
		t.Fatalf("cursor = %d; want 0 after up in detail overlay", got.cursor)
	}

	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyRight})
	got = updated.(model)
	if got.cursor != 0 {
		t.Fatalf("cursor = %d after right; want detail job navigation to stay on up/down", got.cursor)
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

func TestMainListBulkUpdateTargetsOnlyJobsWithMissingEnrichment(t *testing.T) {
	complete := Job{
		Company:         "Complete",
		Title:           "Platform Engineer",
		ApplyURL:        "https://complete.example/jobs/platform-engineer",
		CompanyWebsite:  "https://complete.example",
		CompanySummary:  "Complete builds deployment software for infrastructure teams and platform operators.",
		CompanyIndustry: "Developer Tools",
		Compensation:    "$120,000 - $150,000",
	}
	missingWebsite := Job{
		Company:         "NeedsWebsite",
		Title:           "Site Reliability Engineer",
		ApplyURL:        "https://jobs.example/needswebsite-sre",
		CompanySummary:  "NeedsWebsite builds incident response tools for production engineering teams.",
		CompanyIndustry: "Developer Tools",
		Compensation:    "$130,000 - $160,000",
	}
	missingCompensation := Job{
		Company:         "NeedsPay",
		Title:           "Backend Engineer",
		ApplyURL:        "https://needspay.example/jobs/backend-engineer",
		CompanyWebsite:  "https://needspay.example",
		CompanySummary:  "NeedsPay provides payroll automation software for distributed operations teams.",
		CompanyIndustry: "HR Tech",
		Compensation:    "Not listed",
	}
	m := model{
		allJobs:      []Job{complete, missingWebsite, missingCompensation},
		filteredJobs: []Job{complete, missingWebsite, missingCompensation},
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("U")})
	got := updated.(model)

	if cmd == nil {
		t.Fatal("Update(U) cmd = nil; want bulk enrichment command")
	}
	if !got.backgroundTask.active || !got.backgroundTask.expanded {
		t.Fatalf("backgroundTask = %#v; want active expanded update task", got.backgroundTask)
	}
	if got.backgroundTask.title != "Updating Job Details" {
		t.Fatalf("backgroundTask.title = %q; want Updating Job Details", got.backgroundTask.title)
	}
	if fields := got.backgroundTask.pendingFields[backgroundJobKey(complete)]; len(fields) != 0 {
		t.Fatalf("pendingFields[%q] = %#v; want complete job skipped", backgroundJobKey(complete), fields)
	}
	websiteFields := got.backgroundTask.pendingFields[backgroundJobKey(missingWebsite)]
	if !websiteFields["company_website"] {
		t.Fatalf("pendingFields[%q] = %#v; want company_website pending", backgroundJobKey(missingWebsite), websiteFields)
	}
	compensationFields := got.backgroundTask.pendingFields[backgroundJobKey(missingCompensation)]
	if !compensationFields["compensation"] {
		t.Fatalf("pendingFields[%q] = %#v; want compensation pending", backgroundJobKey(missingCompensation), compensationFields)
	}
	if len(got.backgroundTask.pendingFields) != 2 {
		t.Fatalf("pendingFields len = %d; want 2 incomplete jobs: %#v", len(got.backgroundTask.pendingFields), got.backgroundTask.pendingFields)
	}
}

func TestMainListBulkUpdateNoopsWhenNoJobsNeedEnrichment(t *testing.T) {
	jobs := []Job{
		{
			Company:         "Complete",
			Title:           "Platform Engineer",
			ApplyURL:        "https://complete.example/jobs/platform-engineer",
			CompanyWebsite:  "https://complete.example",
			CompanySummary:  "Complete builds deployment software for infrastructure teams and platform operators.",
			CompanyIndustry: "Developer Tools",
			Compensation:    "$120,000 - $150,000",
		},
	}
	m := model{allJobs: jobs, filteredJobs: jobs}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("U")})
	got := updated.(model)

	if cmd != nil {
		t.Fatalf("Update(U) cmd = %v; want nil when no jobs need enrichment", cmd)
	}
	if got.backgroundTask.active {
		t.Fatalf("backgroundTask.active = true with no incomplete jobs; task = %#v", got.backgroundTask)
	}
	if got.overlay.kind != overlayNotice {
		t.Fatalf("overlay.kind = %v; want notice when no jobs need enrichment", got.overlay.kind)
	}
	if !strings.Contains(got.overlay.notice.message, "No jobs need missing-field updates") {
		t.Fatalf("overlay notice = %q; want no-op update message", got.overlay.notice.message)
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
