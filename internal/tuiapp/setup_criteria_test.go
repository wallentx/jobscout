package tuiapp

import (
	"os"
	"strings"
	"testing"

	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/domain"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestSetupCriteriaFromStateParsesRoleFamilies(t *testing.T) {
	m := model{
		setup: setupState{
			fieldValues: map[string]string{
				"candidate.city":                "Example City",
				"candidate.state":               "Texas",
				"candidate.country_code":        "United States",
				"candidate.years_of_experience": "3",
				"role_families":                 "devops_sre_systems, data",
				"filters.title_requires":        "Engineer",
				"filters.title_includes":        "backend, systems",
				"filters.title_excludes":        "manager",
				"filters.work_settings":         "remote",
				"filters.min_base_usd":          "100000",
				"priority_signals":              "reliability, automation",
			},
		},
	}

	criteria, err := m.setupCriteriaFromState()
	if err != nil {
		t.Fatalf("setupCriteriaFromState() error = %v", err)
	}

	if len(criteria.RoleFamilies) != 2 {
		t.Fatalf("criteria.RoleFamilies len = %d; want 2", len(criteria.RoleFamilies))
	}
	if criteria.RoleFamilies[0] != RoleDevOpsSRESystems || criteria.RoleFamilies[1] != RoleData {
		t.Fatalf("criteria.RoleFamilies = %#v; want devops_sre_systems,data", criteria.RoleFamilies)
	}
	if criteria.Candidate.State != "TX" {
		t.Fatalf("criteria.Candidate.State = %q; want TX", criteria.Candidate.State)
	}
	if criteria.Candidate.CountryCode != "US" {
		t.Fatalf("criteria.Candidate.CountryCode = %q; want US", criteria.Candidate.CountryCode)
	}
}

func TestSaveSetupArtifactsCachesResolvedSourceIDs(t *testing.T) {
	tmpDir := t.TempDir()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd(): %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q): %v", tmpDir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	m := model{
		setup: setupState{
			appConfig: config.DefaultAppConfig(),
			useLLM:    true,
			generated: &CriteriaConfig{
				RoleFamilies: []RoleFamilyID{RoleDevOpsSRESystems},
			},
			prompt: "prompt",
		},
	}

	if err := m.saveSetupArtifacts(); err != nil {
		t.Fatalf("saveSetupArtifacts() error = %v", err)
	}

	cfg, err := config.LoadAppConfig(configFilePath)
	if err != nil {
		t.Fatalf("config.LoadAppConfig(): %v", err)
	}
	if len(cfg.Criteria.ResolvedSourceIDs) == 0 {
		t.Fatal("cfg.Criteria.ResolvedSourceIDs empty; want cached resolved source ids")
	}
}

func TestSetupPreviewStaysTemporaryUntilConfirmed(t *testing.T) {
	existing := []Job{{Company: "PersistedCo", Title: "Existing Role", Status: "Viewed"}}
	preview := []Job{{Company: "TempCo", Title: "Preview Role"}}

	m := model{
		allJobs:     append([]Job(nil), existing...),
		setup:       setupState{previewBusy: true, loadingMinimized: true},
		overlay:     overlayState{kind: overlaySetup},
		termWidth:   100,
		termHeight:  30,
		tableHeight: 6,
	}

	updated, _ := m.Update(setupPreviewMsg{jobs: preview})
	got := updated.(model)

	if len(got.allJobs) != 1 || got.allJobs[0].Company != "PersistedCo" {
		t.Fatalf("allJobs = %#v; want existing jobs to remain untouched", got.allJobs)
	}
	if len(got.setup.previewJobs) != 1 || got.setup.previewJobs[0].Company != "TempCo" {
		t.Fatalf("previewJobs = %#v; want temporary preview job", got.setup.previewJobs)
	}
	if got.setup.step != setupStepPreviewConfirm {
		t.Fatalf("setup.step = %v; want setupStepPreviewConfirm", got.setup.step)
	}
	if got.setup.loadingMinimized {
		t.Fatal("setup.loadingMinimized = true after preview completion; want restored popup")
	}
	if !strings.Contains(tempSetupTableMessage(len(got.allJobs)), "Setup jobs view") {
		t.Fatal("tempSetupTableMessage() missing setup preview banner")
	}
}

func TestSetupInputLoadingStateDoesNotMinimize(t *testing.T) {
	m := model{
		setup: setupState{
			step:          setupStepModelChoice,
			modelsLoading: true,
		},
		overlay:    overlayState{kind: overlaySetup},
		termWidth:  100,
		termHeight: 30,
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	got := updated.(*model)

	if got.setup.loadingMinimized {
		t.Fatal("setup.loadingMinimized = true for model choice; user-input loading states must stay focused")
	}
	if _, ok := got.buildMainOverlaySpec(); !ok {
		t.Fatal("buildMainOverlaySpec() did not render user-input setup popup")
	}
}

func TestSetupPreviewConfirmMergesJobsInsteadOfReplacingStore(t *testing.T) {
	prevJobStore := runtimeJobStore
	fakeStore := &fakeJobStore{}
	runtimeJobStore = fakeStore
	t.Cleanup(func() {
		runtimeJobStore = prevJobStore
	})

	existing := []Job{
		{Company: "PersistedCo", Title: "Existing Role", Status: "Viewed"},
	}
	preview := []Job{
		{Company: "PersistedCo", Title: "Existing Role"},
		{Company: "TempCo", Title: "Preview Role"},
	}

	m := model{
		allJobs:       append([]Job(nil), existing...),
		activeFilters: filterValuesFromStatuses(nil),
		setup: setupState{
			step:        setupStepPreviewConfirm,
			previewJobs: append([]Job(nil), preview...),
		},
		overlay: overlayState{kind: overlaySetup},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*model)

	if len(fakeStore.saved) != 2 {
		t.Fatalf("saved jobs len = %d; want 2 merged jobs", len(fakeStore.saved))
	}
	if fakeStore.saved[0].Company != "PersistedCo" || fakeStore.saved[1].Company != "TempCo" {
		t.Fatalf("saved jobs = %#v; want existing job preserved and preview job added", fakeStore.saved)
	}
	if fakeStore.saved[1].Status != "Unopened" {
		t.Fatalf("saved new job status = %q; want Unopened", fakeStore.saved[1].Status)
	}
	if len(got.allJobs) != 2 {
		t.Fatalf("allJobs len = %d; want 2 after merge", len(got.allJobs))
	}
	if got.overlay.kind != overlayNotice {
		t.Fatalf("overlay.kind = %v; want overlayNotice", got.overlay.kind)
	}
	if !strings.Contains(got.overlay.notice.message, "Merged 1 new jobs") {
		t.Fatalf("notice message = %q; want merge summary", got.overlay.notice.message)
	}
}

func TestSetupOverlayMenuQuitKeysWork(t *testing.T) {
	tests := []struct {
		name string
		msg  tea.KeyMsg
	}{
		{
			name: "q",
			msg:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")},
		},
		{
			name: "ctrl+c",
			msg:  tea.KeyMsg{Type: tea.KeyCtrlC},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{
				setup:   setupState{step: setupStepConfigMenu},
				overlay: overlayState{kind: overlaySetup},
			}

			updated, cmd := m.Update(tt.msg)
			got := updated.(model)

			if !got.quitting {
				t.Fatal("quitting = false; want true")
			}
			if cmd == nil {
				t.Fatal("cmd = nil; want tea.Quit")
			}
		})
	}
}

func TestSetupTextEntryAcceptsPlainQ(t *testing.T) {
	tests := []struct {
		name  string
		model func() model
		value func(*model) string
	}{
		{
			name: "resume path",
			model: func() model {
				m := model{
					setup: setupState{
						step:  setupStepResumePathField,
						input: textinput.New(),
					},
					overlay: overlayState{kind: overlaySetup},
				}
				m.setup.syncInput()
				return m
			},
			value: func(m *model) string {
				return m.setup.input.Value()
			},
		},
		{
			name: "auth command",
			model: func() model {
				m := model{
					setup: setupState{
						step:  setupStepAuthValueField,
						input: textinput.New(),
						appConfig: AppConfig{
							LLM: LLMConfig{
								Provider: "openai",
								Providers: map[string]LLMProviderConfig{
									"openai": {
										Auth: LLMAuthConfig{Mode: llmAuthModeCommand},
									},
								},
							},
						},
					},
					overlay: overlayState{kind: overlaySetup},
				}
				m.setup.syncInput()
				return m
			},
			value: func(m *model) string {
				return m.setup.input.Value()
			},
		},
		{
			name: "default model",
			model: func() model {
				m := model{
					setup: setupState{
						step:  setupStepModelValueField,
						input: textinput.New(),
					},
					overlay: overlayState{kind: overlaySetup},
				}
				m.setup.syncInput()
				return m
			},
			value: func(m *model) string {
				return m.setup.input.Value()
			},
		},
		{
			name: "task model",
			model: func() model {
				m := model{
					setup: setupState{
						step:         setupStepTaskModelValueField,
						taskModelKey: llmTaskFiltering,
						input:        textinput.New(),
					},
					overlay: overlayState{kind: overlaySetup},
				}
				m.setup.syncInput()
				return m
			},
			value: func(m *model) string {
				return m.setup.input.Value()
			},
		},
		{
			name: "criteria text input",
			model: func() model {
				fieldValues := setupFieldValues(config.DefaultCriteriaConfig())
				fieldValues["candidate.city"] = ""
				m := model{
					setup: setupState{
						step:        setupStepCriteriaField,
						fieldIdx:    setupFieldIndexForTest(t, "candidate.city"),
						fieldValues: fieldValues,
						input:       textinput.New(),
						textarea:    textarea.New(),
					},
					overlay: overlayState{kind: overlaySetup},
				}
				m.setup.syncInput()
				return m
			},
			value: func(m *model) string {
				return m.setup.input.Value()
			},
		},
		{
			name: "criteria textarea",
			model: func() model {
				fieldValues := setupFieldValues(config.DefaultCriteriaConfig())
				fieldValues["priority_signals"] = ""
				m := model{
					setup: setupState{
						step:        setupStepCriteriaField,
						fieldIdx:    setupFieldIndexForTest(t, "priority_signals"),
						fieldValues: fieldValues,
						input:       textinput.New(),
						textarea:    textarea.New(),
					},
					overlay: overlayState{kind: overlaySetup},
				}
				m.setup.textarea.ShowLineNumbers = false
				m.setup.syncInput()
				return m
			},
			value: func(m *model) string {
				return m.setup.textarea.Value()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.model()

			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
			got, ok := updated.(*model)
			if !ok {
				t.Fatalf("Update(q) returned %T; want *model", updated)
			}

			if got.quitting {
				t.Fatal("quitting = true after typing q in setup text entry; want false")
			}
			if value := tt.value(got); !strings.HasSuffix(value, "q") {
				t.Fatalf("setup text entry after q = %q; want value ending with typed q", value)
			}
		})
	}
}

func TestSetupTextEntryCtrlCQuits(t *testing.T) {
	m := model{
		setup: setupState{
			step:  setupStepResumePathField,
			input: textinput.New(),
		},
		overlay: overlayState{kind: overlaySetup},
	}
	m.setup.syncInput()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := updated.(model)

	if !got.quitting {
		t.Fatal("quitting = false after Ctrl+C in setup text entry; want true")
	}
	if cmd == nil {
		t.Fatal("cmd = nil after Ctrl+C in setup text entry; want tea.Quit")
	}
}

func TestSetupCriteriaFieldAcceptsTypedInput(t *testing.T) {
	m := model{
		setup: setupState{
			step:        setupStepCriteriaField,
			fieldIdx:    0,
			fieldValues: map[string]string{"candidate.city": ""},
			input:       textinput.New(),
		},
		overlay: overlayState{kind: overlaySetup},
	}
	m.setup.syncInput()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("A")})
	got := updated.(*model)

	if got.setup.input.Value() != "A" {
		t.Fatalf("setup.input.Value() = %q; want %q", got.setup.input.Value(), "A")
	}
}

func TestSetupCriteriaTextareaEnterSavesAndAdvances(t *testing.T) {
	fieldValues := setupFieldValues(config.DefaultCriteriaConfig())
	fieldValues["priority_signals"] = ""
	fieldIdx := setupFieldIndexForTest(t, "priority_signals")

	m := model{
		setup: setupState{
			step:        setupStepCriteriaField,
			fieldIdx:    fieldIdx,
			fieldValues: fieldValues,
			input:       textinput.New(),
			textarea:    textarea.New(),
		},
		overlay: overlayState{kind: overlaySetup},
	}
	m.setup.syncInput()
	m.setup.textarea.SetValue("Kubernetes, Go, reliability")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*model)

	if got.setup.fieldValues["priority_signals"] != "Kubernetes, Go, reliability" {
		t.Fatalf("setup.fieldValues[priority_signals] = %q; want comma-separated textarea value", got.setup.fieldValues["priority_signals"])
	}
	if got.setup.step != setupStepSummary {
		t.Fatalf("setup.step = %v; want setupStepSummary after Enter", got.setup.step)
	}
}

func TestSetupCriteriaTextareaLongPrefillStaysVisibleAfterNavigation(t *testing.T) {
	fieldValues := setupFieldValues(config.DefaultCriteriaConfig())
	fieldValues["priority_signals"] = strings.Repeat("AWS, Kubernetes, Terraform, ", 12)
	fieldIdx := setupFieldIndexForTest(t, "priority_signals")

	m := model{
		termWidth:  100,
		termHeight: 30,
		setup: setupState{
			step:        setupStepCriteriaField,
			fieldIdx:    fieldIdx,
			fieldValues: fieldValues,
			input:       textinput.New(),
			textarea:    textarea.New(),
		},
		overlay: overlayState{kind: overlaySetup},
	}
	m.setup.textarea.ShowLineNumbers = false
	m.setup.syncInput()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	got := updated.(*model)
	spec := got.buildSetupOverlaySpec()
	body := ansi.Strip(spec.body.content)

	if !strings.Contains(body, "AWS") {
		t.Fatalf("setup textarea body after right key = %q; want prefilled text still visible", body)
	}
}

func TestSetupCriteriaTextareaLongPrefillShowsScrollbar(t *testing.T) {
	fieldValues := setupFieldValues(config.DefaultCriteriaConfig())
	fieldValues["priority_signals"] = strings.Repeat("AWS, Kubernetes, Terraform, ", 20)
	fieldIdx := setupFieldIndexForTest(t, "priority_signals")

	m := model{
		termWidth:  100,
		termHeight: 30,
		setup: setupState{
			step:        setupStepCriteriaField,
			fieldIdx:    fieldIdx,
			fieldValues: fieldValues,
			input:       textinput.New(),
			textarea:    textarea.New(),
		},
		overlay: overlayState{kind: overlaySetup},
	}
	m.setup.textarea.ShowLineNumbers = false
	m.setup.syncInput()

	spec := m.buildSetupOverlaySpec()

	if !strings.Contains(spec.body.content, "█") {
		t.Fatalf("setup textarea body = %q; want scrollbar thumb for long value", spec.body.content)
	}
}

func TestSetupCriteriaTextareaClearShortcut(t *testing.T) {
	fieldValues := setupFieldValues(config.DefaultCriteriaConfig())
	fieldValues["priority_signals"] = "AWS, Kubernetes"
	fieldIdx := setupFieldIndexForTest(t, "priority_signals")

	m := model{
		setup: setupState{
			step:        setupStepCriteriaField,
			fieldIdx:    fieldIdx,
			fieldValues: fieldValues,
			input:       textinput.New(),
			textarea:    textarea.New(),
		},
		overlay: overlayState{kind: overlaySetup},
	}
	m.setup.textarea.ShowLineNumbers = false
	m.setup.syncInput()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	got := updated.(*model)

	if got.setup.textarea.Value() != "" {
		t.Fatalf("setup.textarea.Value() = %q; want empty after Ctrl+U", got.setup.textarea.Value())
	}
}

func TestParseCSVListAcceptsNewlineSeparatedValues(t *testing.T) {
	got := domain.ParseCSVList("Kubernetes\nGo, reliability")
	want := []string{"Kubernetes", "Go", "reliability"}
	if len(got) != len(want) {
		t.Fatalf("domain.ParseCSVList() len = %d; want %d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("domain.ParseCSVList()[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

func TestSetupRoleFamiliesFieldTogglesDisplayedSelection(t *testing.T) {
	fieldValues := setupFieldValues(config.DefaultCriteriaConfig())
	fieldValues["role_families"] = ""
	fieldIdx := setupFieldIndexForTest(t, "role_families")

	m := model{
		setup: setupState{
			step:        setupStepCriteriaField,
			fieldIdx:    fieldIdx,
			fieldValues: fieldValues,
			input:       textinput.New(),
		},
		overlay: overlayState{kind: overlaySetup},
	}
	m.setup.syncInput()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	got := updated.(*model)

	if got.setup.fieldValues["role_families"] != "frontend" {
		t.Fatalf("setup.fieldValues[role_families] = %q, want frontend", got.setup.fieldValues["role_families"])
	}
}

func TestSetupWorkSettingsFieldTogglesDisplayedSelection(t *testing.T) {
	fieldValues := setupFieldValues(config.DefaultCriteriaConfig())
	fieldValues["filters.work_settings"] = ""
	fieldIdx := setupFieldIndexForTest(t, "filters.work_settings")

	m := model{
		setup: setupState{
			step:        setupStepCriteriaField,
			fieldIdx:    fieldIdx,
			fieldValues: fieldValues,
			input:       textinput.New(),
		},
		overlay: overlayState{kind: overlaySetup},
	}
	m.setup.syncInput()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	got := updated.(*model)

	if got.setup.fieldValues["filters.work_settings"] != "remote" {
		t.Fatalf("setup.fieldValues[filters.work_settings] = %q, want remote", got.setup.fieldValues["filters.work_settings"])
	}
}

func setupFieldIndexForTest(t *testing.T, key string) int {
	t.Helper()

	for i, field := range searchProfileGroupSpec().Fields {
		if field.Key == key {
			return i
		}
	}
	t.Fatalf("searchProfileGroupSpec() missing field %q", key)
	return -1
}
