package tuiapp

import (
	"os"
	"strings"
	"testing"

	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/domain"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInitialModel(t *testing.T) {
	writeTestJobsFile(t)

	m := initialModel()
	if m.quitting {
		t.Error("Expected quitting to be false")
	}
	if len(m.allJobs) == 0 {
		t.Error("Expected jobs to be loaded")
	}
	if m.cursor != 0 {
		t.Errorf("Expected cursor at 0, got %d", m.cursor)
	}
}

func TestModelUpdate(t *testing.T) {
	writeTestJobsFile(t)

	m := initialModel()

	// Test Quit
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
	updatedModel, cmd := m.Update(msg)

	newModel := updatedModel.(model)
	if !newModel.quitting {
		t.Error("Expected quitting to be true after 'q' key")
	}
	if cmd == nil {
		t.Error("Expected non-nil command")
	}

	// Test Navigation
	writeTestJobsFile(t)
	m = initialModel()

	// Move Down
	msg = tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, _ = m.Update(msg)
	newModel = updatedModel.(model)
	if newModel.cursor != 1 {
		t.Errorf("Expected cursor to be 1 after Down key, got %d", newModel.cursor)
	}

	// Test Status Update (Modal)
	idx := newModel.cursor
	oldStatus := newModel.filteredJobs[idx].Status

	// Open Status Modal
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}
	updatedModel, _ = newModel.Update(msg)
	newModel = updatedModel.(model)
	if newModel.overlay.kind != overlayStatus {
		t.Error("Expected status modal to be shown after 's'")
	}

	// Select next status (Down)
	msg = tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, _ = newModel.Update(msg)
	newModel = updatedModel.(model)

	// Confirm selection (Enter)
	msg = tea.KeyMsg{Type: tea.KeyEnter}
	updatedModel, _ = newModel.Update(msg)
	newModel = updatedModel.(model)

	if newModel.overlay.kind == overlayStatus {
		t.Error("Expected status modal to be hidden after Enter")
	}

	// Verify status changed (assuming we picked a different one)
	if newModel.filteredJobs[idx].Status == oldStatus {
		// Note: This might fail if the statuses list puts the same status next,
		// but with New -> Application Submitted, it should change.
		t.Error("Expected status to change after selection")
	}

	// Test Sorting
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")} // Sort by Company
	updatedModel, _ = newModel.Update(msg)
	newModel = updatedModel.(model)
	if newModel.sortBy != 0 {
		t.Error("Expected sortBy to be 0")
	}

	// Test Filtering
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")}
	updatedModel, _ = newModel.Update(msg)
	newModel = updatedModel.(model)
	if newModel.overlay.kind != overlayFilter {
		t.Error("Expected filter modal to be shown after 'f'")
	}

	msg = tea.KeyMsg{Type: tea.KeySpace}
	updatedModel, _ = newModel.Update(msg)
	newModel = updatedModel.(model)
	if !newModel.overlay.filter.values[statuses[0]] {
		t.Error("Expected first filter option to toggle on")
	}
}

func TestInitialModelDoesNotRequirePromptWhenJobSearchDisabled(t *testing.T) {
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

	appCfg := config.DefaultAppConfig()
	appCfg.Criteria = config.DefaultCriteriaConfig()
	appCfg.Criteria.RoleFamilies = []RoleFamilyID{RoleDevOpsSRESystems}
	appCfg.LLM.JobSearch = false
	if err := config.SaveAppConfig(configFilePath, &appCfg); err != nil {
		t.Fatalf("config.SaveAppConfig(): %v", err)
	}

	m := initialModel()
	if m.setupRequired {
		t.Fatalf("setupRequired = true; want false when search profile is ready and LLM job search is disabled")
	}
}

func TestInitialModelAllowsDegradedLLMWhenAuthMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "")

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

	appCfg := config.DefaultAppConfig()
	appCfg.Criteria = config.DefaultCriteriaConfig()
	appCfg.Criteria.RoleFamilies = []RoleFamilyID{RoleDevOpsSRESystems}
	appCfg.LLM.Provider = "openai"
	appCfg.LLM.Model = "gpt-4.1-mini"
	appCfg.LLM.JobSearch = false
	if err := config.SaveAppConfig(configFilePath, &appCfg); err != nil {
		t.Fatalf("config.SaveAppConfig(): %v", err)
	}

	m := initialModel()
	if m.setupRequired {
		t.Fatalf("setupRequired = true; want false when non-LLM path is runnable and only auth is missing")
	}
}

func TestInitialModelOpensLLMRecoveryOverlayWhenAuthMissing(t *testing.T) {
	tmpDir := t.TempDir()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", tmpDir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	t.Setenv("GEMINI_API_KEY", "")

	appCfg := config.DefaultAppConfig()
	appCfg.Criteria.RoleFamilies = []RoleFamilyID{RoleDevOpsSRESystems}
	appCfg.LLM.JobFiltering = true
	appCfg.LLM.JobSearch = false
	if err := config.SaveAppConfig(configFilePath, &appCfg); err != nil {
		t.Fatalf("config.SaveAppConfig(%q) error = %v", configFilePath, err)
	}

	m := initialModel()
	if m.overlay.kind != overlaySetup {
		t.Fatalf("overlay.kind = %v, want overlaySetup", m.overlay.kind)
	}
	if m.setup.mode != setupModeRecovery {
		t.Fatalf("setup.mode = %v, want setupModeRecovery", m.setup.mode)
	}
	if m.setup.step != setupStepRecoveryChoice {
		t.Fatalf("setup.step = %v, want setupStepRecoveryChoice", m.setup.step)
	}
}

func TestRecoveryChoiceContinueDisablesLLMForSession(t *testing.T) {
	m := model{
		setup: setupState{
			mode:    setupModeRecovery,
			step:    setupStepRecoveryChoice,
			section: setupSectionLLM,
		},
		overlay: overlayState{kind: overlaySetup},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*model)

	if !got.sessionLLMDisabled {
		t.Fatal("sessionLLMDisabled = false, want true")
	}
	if got.overlay.kind != overlayNotice {
		t.Fatalf("overlay.kind = %v, want overlayNotice", got.overlay.kind)
	}
	if !strings.Contains(got.overlay.notice.message, "Continuing with non-LLM sources") {
		t.Fatalf("overlay.notice.message = %q, want non-LLM recovery message", got.overlay.notice.message)
	}
}

func TestRepairJobIdentityTargetsOnlyIncompleteActiveJobs(t *testing.T) {
	jobs := []Job{
		{
			Company:         "Complete",
			CompanyWebsite:  "https://complete.example",
			CompanySummary:  "Complete builds software for infrastructure teams.",
			CompanyIndustry: "Developer Tools",
			Compensation:    "$120,000 - $150,000",
			Status:          "Unopened",
		},
		{
			Company:        "MissingIndustry",
			CompanyWebsite: "https://missing.example",
			CompanySummary: "MissingIndustry builds software for infrastructure teams.",
			Compensation:   "$120,000 - $150,000",
			Status:         "Unopened",
		},
		{
			Company:        "Expired",
			CompanySummary: "Expired builds software for infrastructure teams.",
			Status:         "Expired",
		},
		{
			Company:         "Provisional",
			CompanyWebsite:  "https://provisional.example",
			CompanySummary:  "Provisional builds software for infrastructure teams.",
			CompanyIndustry: "Developer Tools",
			Compensation:    "$120,000 - $150,000",
			Status:          "Unopened",
			CompanyIdentity: &domain.JobIdentityMetadata{Industry: &domain.JobIdentityEvidence{Provisional: true}},
		},
	}

	targets, indexes := domain.IdentityRepairTargets(jobs)

	if len(targets) != 2 {
		t.Fatalf("domain.IdentityRepairTargets(...) returned %d targets; want 2", len(targets))
	}
	if indexes[0] != 1 || indexes[1] != 3 {
		t.Fatalf("domain.IdentityRepairTargets(...) indexes = %#v; want [1 3]", indexes)
	}
}
