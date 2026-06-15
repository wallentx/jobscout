package tuiapp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestParseOperatorCommandLinePreservesQuotedArgs(t *testing.T) {
	got, err := parseOperatorCommandLine(`debug path "./logs/job debug.log"`)
	if err != nil {
		t.Fatalf("parseOperatorCommandLine() error = %v", err)
	}
	want := []string{"debug", "path", "./logs/job debug.log"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("parseOperatorCommandLine() = %#v; want %#v", got, want)
	}
}

func TestExecuteOperatorDebugCommandUpdatesRuntimeDebug(t *testing.T) {
	previousEnabled := runtimeDebugEnabled
	previousPath := runtimeDebugPath
	t.Cleanup(func() {
		setRuntimeDebug(previousEnabled, previousPath)
	})

	result, err := executeOperatorCommand(`debug on`)
	if err != nil {
		t.Fatalf("executeOperatorCommand(debug on) error = %v", err)
	}
	if !runtimeDebugEnabled {
		t.Fatal("runtimeDebugEnabled = false; want true")
	}
	if result.Title != "Debug" || !strings.Contains(result.Message, "on") {
		t.Fatalf("executeOperatorCommand(debug on) = %#v; want Debug notice mentioning on", result)
	}

	result, err = executeOperatorCommand(`debug path "./debug alt.log"`)
	if err != nil {
		t.Fatalf("executeOperatorCommand(debug path) error = %v", err)
	}
	if runtimeDebugPath != "./debug alt.log" {
		t.Fatalf("runtimeDebugPath = %q; want %q", runtimeDebugPath, "./debug alt.log")
	}
	if result.Title != "Debug" || !strings.Contains(result.Message, "./debug alt.log") {
		t.Fatalf("executeOperatorCommand(debug path) = %#v; want Debug notice mentioning path", result)
	}
}

func TestLogDebugUsesRuntimeDebugPath(t *testing.T) {
	previousEnabled := runtimeDebugEnabled
	previousPath := runtimeDebugPath
	t.Cleanup(func() {
		setRuntimeDebug(previousEnabled, previousPath)
	})

	dir := t.TempDir()
	customPath := filepath.Join(dir, "custom-debug.log")
	defaultPath := filepath.Join(dir, "debug.log")
	previousWorkingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd(): %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir(%q): %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previousWorkingDir); err != nil {
			t.Fatalf("os.Chdir(%q): %v", previousWorkingDir, err)
		}
	})

	setRuntimeDebug(true, customPath)
	logDebug("custom path %s", "message")

	got, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", customPath, err)
	}
	if !strings.Contains(string(got), "custom path message") {
		t.Fatalf("custom debug log = %q; want message", string(got))
	}
	if _, err := os.Stat(defaultPath); !os.IsNotExist(err) {
		t.Fatalf("os.Stat(%q) error = %v; want not exist", defaultPath, err)
	}
}

func TestExecuteOperatorHealthCommandRequestsHealthOverlay(t *testing.T) {
	result, err := executeOperatorCommand(`health GitHub`)
	if err != nil {
		t.Fatalf("executeOperatorCommand(health) error = %v", err)
	}
	if result.HealthIdentity.Company != "GitHub" {
		t.Fatalf("HealthIdentity.Company = %q; want %q", result.HealthIdentity.Company, "GitHub")
	}
}

func TestExecuteOperatorHealthCommandParsesOptionalIdentity(t *testing.T) {
	result, err := executeOperatorCommand(`health "Acme Cloud" --aka AcmeCloud --aka "Acme Cloud Inc" --website https://www.acmecloud.example`)
	if err != nil {
		t.Fatalf("executeOperatorCommand(health) error = %v", err)
	}
	identity := result.HealthIdentity
	if identity.Company != "Acme Cloud" {
		t.Fatalf("HealthIdentity.Company = %q; want %q", identity.Company, "Acme Cloud")
	}
	if identity.Website != "https://www.acmecloud.example" {
		t.Fatalf("HealthIdentity.Website = %q; want %q", identity.Website, "https://www.acmecloud.example")
	}
	if strings.Join(identity.Aliases, "\x00") != "AcmeCloud\x00Acme Cloud Inc" {
		t.Fatalf("HealthIdentity.Aliases = %#v; want AcmeCloud and Acme Cloud Inc", identity.Aliases)
	}
}

func TestExecuteOperatorHealthCommandRequiresCompany(t *testing.T) {
	_, err := executeOperatorCommand(`health`)
	if err == nil {
		t.Fatal("executeOperatorCommand(health) error = nil; want usage error")
	}
	if !strings.Contains(err.Error(), "usage: health <company>") {
		t.Fatalf("executeOperatorCommand(health) error = %q; want usage guidance", err)
	}
}

func TestExecuteOperatorFetchCommandTargetsCompany(t *testing.T) {
	result, err := executeOperatorCommand(`fetch "Acme Cloud" --website https://acme.example --aka Acme --aka "Acme Inc" --all`)
	if err != nil {
		t.Fatalf("executeOperatorCommand(fetch) error = %v", err)
	}
	if result.FetchOptions == nil {
		t.Fatal("FetchOptions = nil; want company fetch options")
	}
	options := result.FetchOptions
	if options.Company != "Acme Cloud" {
		t.Fatalf("Company = %q; want Acme Cloud", options.Company)
	}
	if options.Website != "https://acme.example" {
		t.Fatalf("Website = %q; want https://acme.example", options.Website)
	}
	if strings.Join(options.Aliases, "\x00") != "Acme\x00Acme Inc" {
		t.Fatalf("Aliases = %#v; want Acme and Acme Inc", options.Aliases)
	}
	if !options.All {
		t.Fatal("All = false; want true")
	}

	if _, err := executeOperatorCommand(`fetch`); err == nil || !strings.Contains(err.Error(), "usage: fetch <company> [--aka <name>] [--website <url>] [--all]") {
		t.Fatalf("executeOperatorCommand(fetch) error = %v; want usage guidance", err)
	}
	if _, err := executeOperatorCommand(`fetch GitHub --sources site`); err == nil || !strings.Contains(err.Error(), "unknown fetch option") {
		t.Fatalf("executeOperatorCommand(fetch --sources) error = %v; want old option rejected", err)
	}
}

func TestCommandInputTabCompletesAllowlistedSubcommand(t *testing.T) {
	writeTestJobsFile(t)
	m := initialModel()
	m.overlay.kind = overlayNone
	m.isCommanding = true
	m.commandInput.Focus()
	for _, r := range "debug st" {
		updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(model)
	}

	updated, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(model)

	if cmd != nil {
		t.Fatalf("handleKeyMsg(tab) cmd = %v; want nil", cmd)
	}
	if got.commandInput.Value() != "debug status" {
		t.Fatalf("commandInput.Value() = %q; want %q", got.commandInput.Value(), "debug status")
	}
}

func TestCommandInputDisablesInlineSuggestions(t *testing.T) {
	writeTestJobsFile(t)
	m := initialModel()

	if m.commandInput.ShowSuggestions {
		t.Fatal("commandInput.ShowSuggestions = true; want false")
	}
}

func TestCommandInputTabCompletesCommandName(t *testing.T) {
	writeTestJobsFile(t)
	m := initialModel()
	m.overlay.kind = overlayNone
	m.isCommanding = true
	m.commandInput.Focus()
	m.commandInput.SetValue("d")

	updated, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(model)

	if cmd != nil {
		t.Fatalf("handleKeyMsg(tab) cmd = %v; want nil", cmd)
	}
	if got.commandInput.Value() != "debug" {
		t.Fatalf("commandInput.Value() = %q; want %q", got.commandInput.Value(), "debug")
	}
}

func TestCommandInputTabCyclesAmbiguousCompletions(t *testing.T) {
	writeTestJobsFile(t)
	m := initialModel()
	m.overlay.kind = overlayNone
	m.isCommanding = true
	m.commandInput.Focus()
	m.commandInput.SetValue("debug o")

	updated, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(model)
	if cmd != nil {
		t.Fatalf("first handleKeyMsg(tab) cmd = %v; want nil", cmd)
	}
	if got.commandInput.Value() != "debug on" {
		t.Fatalf("first commandInput.Value() = %q; want %q", got.commandInput.Value(), "debug on")
	}

	updated, cmd = got.handleKeyMsg(tea.KeyMsg{Type: tea.KeyTab})
	got = updated.(model)
	if cmd != nil {
		t.Fatalf("second handleKeyMsg(tab) cmd = %v; want nil", cmd)
	}
	if got.commandInput.Value() != "debug off" {
		t.Fatalf("second commandInput.Value() = %q; want %q", got.commandInput.Value(), "debug off")
	}

	updated, cmd = got.handleKeyMsg(tea.KeyMsg{Type: tea.KeyTab})
	got = updated.(model)
	if cmd != nil {
		t.Fatalf("third handleKeyMsg(tab) cmd = %v; want nil", cmd)
	}
	if got.commandInput.Value() != "debug on" {
		t.Fatalf("third commandInput.Value() = %q; want %q", got.commandInput.Value(), "debug on")
	}
}

func TestCommandModeShowsCommandPoolByDefault(t *testing.T) {
	m := model{
		termWidth:     100,
		termHeight:    30,
		tableHeight:   calculateTableHeight(30),
		activeFilters: filterValuesFromStatuses(nil),
		commandInput:  newOperatorCommandInput(),
		isCommanding:  true,
	}
	m.commandInput.Focus()

	rendered := ansi.Strip(m.View())
	if !strings.Contains(rendered, "debug") || !strings.Contains(rendered, "health") || !strings.Contains(rendered, "fetch") {
		t.Fatalf("command mode view missing default command pool:\n%s", rendered)
	}
	if strings.Contains(rendered, "help") || strings.Contains(rendered, "?") {
		t.Fatalf("command mode view includes removed help command:\n%s", rendered)
	}
}

func TestCommandModeHelpCommandIsNotCallable(t *testing.T) {
	for _, input := range []string{"help", "?"} {
		if _, err := executeOperatorCommand(input); err == nil {
			t.Fatalf("executeOperatorCommand(%q) error = nil; want unknown command", input)
		}
	}
}

func TestCommandModeTypingFiltersPoolWithoutHighlight(t *testing.T) {
	m := model{
		termWidth:     100,
		termHeight:    30,
		tableHeight:   calculateTableHeight(30),
		activeFilters: filterValuesFromStatuses(nil),
		commandInput:  newOperatorCommandInput(),
		isCommanding:  true,
	}
	m.commandInput.Focus()
	m.commandInput.SetValue("debug o")
	m.commandTypedInput = "debug o"

	if m.commandCompletionActive {
		t.Fatal("commandCompletionActive = true; want false before tab")
	}
	if m.commandInput.Value() != "debug o" {
		t.Fatalf("commandInput.Value() = %q; want %q", m.commandInput.Value(), "debug o")
	}
	gotPool := m.operatorCommandPool()
	if labels := commandCompletionLabels(gotPool); strings.Join(labels, "\x00") != "on\x00off" {
		t.Fatalf("operatorCommandPool labels = %#v; want %#v", labels, []string{"on", "off"})
	}
}

func TestCommandModeSpaceConfirmsHighlightedCompletion(t *testing.T) {
	writeTestJobsFile(t)
	m := initialModel()
	m.overlay.kind = overlayNone
	m.isCommanding = true
	m.commandInput.Focus()
	m.commandInput.SetValue("d")
	m.commandTypedInput = "d"

	updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(model)
	if got.commandInput.Value() != "debug" {
		t.Fatalf("after tab commandInput.Value() = %q; want %q", got.commandInput.Value(), "debug")
	}
	if !got.commandCompletionActive {
		t.Fatal("commandCompletionActive = false; want true after tab")
	}

	updated, cmd := got.handleKeyMsg(tea.KeyMsg{Type: tea.KeySpace})
	got = updated.(model)
	if cmd != nil {
		t.Fatalf("handleKeyMsg(space) cmd = %v; want nil", cmd)
	}
	if got.commandCompletionActive {
		t.Fatal("commandCompletionActive = true; want false after confirming selection")
	}
	if got.commandTypedInput != "debug " {
		t.Fatalf("commandTypedInput = %q; want %q", got.commandTypedInput, "debug ")
	}
	if got.commandInput.Value() != "debug " {
		t.Fatalf("commandInput.Value() = %q; want %q", got.commandInput.Value(), "debug ")
	}
}

func TestCommandInputTabCompletesHealthCommandName(t *testing.T) {
	writeTestJobsFile(t)
	m := initialModel()
	m.overlay.kind = overlayNone
	m.isCommanding = true
	m.commandInput.Focus()
	m.commandInput.SetValue("hea")
	m.commandTypedInput = "hea"

	updated, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(model)
	if cmd != nil {
		t.Fatalf("handleKeyMsg(tab) cmd = %v; want nil", cmd)
	}
	if got.commandInput.Value() != "health" {
		t.Fatalf("commandInput.Value() = %q; want %q", got.commandInput.Value(), "health")
	}
	if got.commandInput.Position() != len([]rune(got.commandInput.Value())) {
		t.Fatalf("commandInput.Position() = %d; want cursor at end of %q", got.commandInput.Position(), got.commandInput.Value())
	}
	if got.operatorCommandGhostHint() != "" {
		t.Fatalf("operatorCommandGhostHint() = %q; want no argument hint until command has a space", got.operatorCommandGhostHint())
	}
}

func TestCommandInputTypingAfterTabSelectionExtendsSelection(t *testing.T) {
	writeTestJobsFile(t)
	m := initialModel()
	m.overlay.kind = overlayNone
	m.isCommanding = true
	m.commandInput.Focus()
	m.commandInput.SetValue("hea")
	m.commandTypedInput = "hea"

	updated, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(model)
	if cmd != nil {
		t.Fatalf("handleKeyMsg(tab) cmd = %v; want nil", cmd)
	}
	if got.commandInput.Value() != "health" {
		t.Fatalf("after tab commandInput.Value() = %q; want %q", got.commandInput.Value(), "health")
	}

	updated, _ = got.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	got = updated.(model)
	if got.commandInput.Value() != "healthG" {
		t.Fatalf("after typing commandInput.Value() = %q; want %q", got.commandInput.Value(), "healthG")
	}
}

func TestCommandModeConfirmedCommandNameAdvancesSpace(t *testing.T) {
	writeTestJobsFile(t)
	m := initialModel()
	m.overlay.kind = overlayNone
	m.isCommanding = true
	m.commandInput.Focus()
	m.commandInput.SetValue("hea")
	m.commandTypedInput = "hea"

	updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(model)
	if !got.commandCompletionActive {
		t.Fatal("commandCompletionActive = false; want active selection")
	}

	updated, cmd := got.handleKeyMsg(tea.KeyMsg{Type: tea.KeySpace})
	got = updated.(model)
	if cmd != nil {
		t.Fatalf("handleKeyMsg(space) cmd = %v; want nil", cmd)
	}
	if got.commandInput.Value() != "health " {
		t.Fatalf("commandInput.Value() = %q; want confirmed command with trailing space", got.commandInput.Value())
	}
	if got.operatorCommandGhostHint() != "<company>" {
		t.Fatalf("operatorCommandGhostHint() = %q; want company hint after confirmed command space", got.operatorCommandGhostHint())
	}
}

func TestCommandModeHealthHintIsNotSelectablePool(t *testing.T) {
	m := model{
		termWidth:     100,
		termHeight:    30,
		tableHeight:   calculateTableHeight(30),
		activeFilters: filterValuesFromStatuses(nil),
		commandInput:  newOperatorCommandInput(),
		isCommanding:  true,
	}
	m.commandInput.Focus()
	m.commandInput.SetValue("health ")
	m.commandTypedInput = "health "

	if labels := commandCompletionLabels(m.operatorCommandPool()); len(labels) != 0 {
		t.Fatalf("operatorCommandPool labels = %#v; want no selectable company placeholder", labels)
	}
	if got := operatorCommandGhostHint("health "); got != "<company>" {
		t.Fatalf("operatorCommandGhostHint(health) = %q; want company-only hint", got)
	}
	rendered := ansi.Strip(m.View())
	if !strings.Contains(rendered, "<company>") {
		t.Fatalf("command mode view missing company ghost hint:\n%s", rendered)
	}
	if strings.Contains(rendered, "--aka") || strings.Contains(rendered, "--website") {
		t.Fatalf("command mode view includes option syntax in company ghost hint:\n%s", rendered)
	}
	if !strings.Contains(rendered, ": health  <company>") {
		t.Fatalf("command mode view renders company ghost away from input position:\n%s", rendered)
	}
}

func TestCommandModeHealthGhostClearsAfterCompany(t *testing.T) {
	if got := operatorCommandGhostHint("health"); got != "" {
		t.Fatalf("operatorCommandGhostHint(health) = %q; want no hint before argument space", got)
	}
	if got := operatorCommandGhostHint("health Acme "); got != "" {
		t.Fatalf("operatorCommandGhostHint(health Acme) = %q; want no hint after company", got)
	}
}

func TestCommandModeHealthPoolShowsSelectableFlagsAfterCompany(t *testing.T) {
	m := model{
		termWidth:     100,
		termHeight:    30,
		tableHeight:   calculateTableHeight(30),
		activeFilters: filterValuesFromStatuses(nil),
		commandInput:  newOperatorCommandInput(),
		isCommanding:  true,
	}
	m.commandInput.Focus()
	m.commandInput.SetValue("health GitHub ")
	m.commandTypedInput = "health GitHub "

	labels := commandCompletionLabels(m.operatorCommandPool())
	if strings.Join(labels, "\x00") != "--aka\x00--website" {
		t.Fatalf("operatorCommandPool labels = %#v; want selectable health flags", labels)
	}
}

func TestCommandModeFetchCompanyHintAndOptionCompletion(t *testing.T) {
	m := model{
		termWidth:     100,
		termHeight:    30,
		tableHeight:   calculateTableHeight(30),
		activeFilters: filterValuesFromStatuses(nil),
		commandInput:  newOperatorCommandInput(),
		isCommanding:  true,
	}
	m.commandInput.Focus()

	m.commandInput.SetValue("fetch ")
	m.commandTypedInput = "fetch "
	if labels := commandCompletionLabels(m.operatorCommandPool()); len(labels) != 0 {
		t.Fatalf("operatorCommandPool labels = %#v; want no company placeholder completion", labels)
	}
	if hint := m.operatorCommandGhostHint(); hint != "<company>" {
		t.Fatalf("operatorCommandGhostHint() = %q; want company hint", hint)
	}

	m.commandInput.SetValue("fetch GitHub ")
	m.commandTypedInput = "fetch GitHub "
	if labels := commandCompletionLabels(m.operatorCommandPool()); strings.Join(labels, "\x00") != "--aka\x00--website\x00--all" {
		t.Fatalf("operatorCommandPool labels = %#v; want fetch identity flags", labels)
	}

	m.commandInput.SetValue("fetch GitHub --website ")
	m.commandTypedInput = "fetch GitHub --website "
	if hint := m.operatorCommandGhostHint(); hint != "<url>" {
		t.Fatalf("operatorCommandGhostHint() = %q; want website URL hint", hint)
	}

	m.commandInput.SetValue("fetch GitHub --website https://github.com ")
	m.commandTypedInput = "fetch GitHub --website https://github.com "
	if labels := commandCompletionLabels(m.operatorCommandPool()); strings.Join(labels, "\x00") != "--aka\x00--all" {
		t.Fatalf("operatorCommandPool labels = %#v; want --aka and --all after website", labels)
	}
}

func TestCommandInputInlineViewSuppressesGhostWhenCursorInsideValue(t *testing.T) {
	m := model{commandInput: newOperatorCommandInput()}
	m.commandInput.Focus()
	m.commandInput.SetValue("health GitHub --website https://github.com")
	m.commandInput.SetCursor(len([]rune("health GitHub --website ")))

	rendered := ansi.Strip(m.commandInputInlineView("https://github.com", lipgloss.NewStyle()))
	if strings.Contains(rendered, "health GitHub --website https://github.comhttps://github.com") {
		t.Fatalf("commandInputInlineView duplicated ghost inside value: %q", rendered)
	}
	if !strings.Contains(rendered, "health GitHub --website https://github.com") {
		t.Fatalf("commandInputInlineView() = %q; want original input text", rendered)
	}
}

func TestCommandModeHealthPoolHidesDuplicateWebsiteFlag(t *testing.T) {
	m := model{
		termWidth:     100,
		termHeight:    30,
		tableHeight:   calculateTableHeight(30),
		activeFilters: filterValuesFromStatuses(nil),
		commandInput:  newOperatorCommandInput(),
		isCommanding:  true,
	}
	m.commandInput.Focus()
	m.commandInput.SetValue("health GitHub --website https://github.com ")
	m.commandTypedInput = "health GitHub --website https://github.com "

	labels := commandCompletionLabels(m.operatorCommandPool())
	if strings.Join(labels, "\x00") != "--aka" {
		t.Fatalf("operatorCommandPool labels = %#v; want only repeatable health flags", labels)
	}
}

func TestCommandInputTabCompletesHealthFlag(t *testing.T) {
	writeTestJobsFile(t)
	m := initialModel()
	m.overlay.kind = overlayNone
	m.isCommanding = true
	m.commandInput.Focus()
	m.commandInput.SetValue("health GitHub --w")
	m.commandTypedInput = "health GitHub --w"

	updated, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(model)
	if cmd != nil {
		t.Fatalf("handleKeyMsg(tab) cmd = %v; want nil", cmd)
	}
	if got.commandInput.Value() != "health GitHub --website" {
		t.Fatalf("commandInput.Value() = %q; want %q", got.commandInput.Value(), "health GitHub --website")
	}
}

func TestCommandInputTabSelectedHealthFlagShowsValueGhost(t *testing.T) {
	writeTestJobsFile(t)
	m := initialModel()
	m.overlay.kind = overlayNone
	m.isCommanding = true
	m.commandInput.Focus()
	m.commandInput.SetValue("health GitHub ")
	m.commandTypedInput = "health GitHub "

	updated, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(model)
	if cmd != nil {
		t.Fatalf("handleKeyMsg(tab) cmd = %v; want nil", cmd)
	}
	if got.commandInput.Value() != "health GitHub --aka" {
		t.Fatalf("commandInput.Value() = %q; want %q", got.commandInput.Value(), "health GitHub --aka")
	}
	if hint := got.operatorCommandGhostHint(); hint != " <name>" {
		t.Fatalf("operatorCommandGhostHint() = %q; want selected flag value hint", hint)
	}
	rendered := ansi.Strip(got.View())
	if !strings.Contains(rendered, ": health GitHub --aka  <name>") {
		t.Fatalf("command mode view missing selected flag ghost at input position:\n%s", rendered)
	}

	updated, cmd = got.handleKeyMsg(tea.KeyMsg{Type: tea.KeySpace})
	got = updated.(model)
	if cmd != nil {
		t.Fatalf("handleKeyMsg(space) cmd = %v; want nil", cmd)
	}
	if got.commandInput.Value() != "health GitHub --aka " {
		t.Fatalf("confirmed commandInput.Value() = %q; want trailing space after confirmed flag", got.commandInput.Value())
	}
	if got.commandCompletionActive {
		t.Fatal("commandCompletionActive = true; want false after confirm")
	}
	if hint := got.operatorCommandGhostHint(); hint != "<name>" {
		t.Fatalf("operatorCommandGhostHint() = %q; want selected flag value hint after confirm", hint)
	}
}

func TestCommandModeHealthCommandOpensHealthOverlay(t *testing.T) {
	writeTestJobsFile(t)
	m := initialModel()
	m.overlay.kind = overlayNone
	m.isCommanding = true
	m.commandInput.Focus()
	m.commandInput.SetValue("health GitHub")
	m.commandTypedInput = "health GitHub"

	updated, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(model)
	if cmd == nil {
		t.Fatal("handleKeyMsg(enter) cmd = nil; want health load command")
	}
	if got.isCommanding {
		t.Fatal("isCommanding = true; want false while health overlay is loading")
	}
	if got.overlay.kind != overlayHealth || !got.overlay.health.loading {
		t.Fatalf("overlay = %#v; want loading health overlay", got.overlay)
	}
	if !strings.Contains(got.overlay.health.loadingText, "GitHub") {
		t.Fatalf("loadingText = %q; want company name", got.overlay.health.loadingText)
	}
}

func TestCommandModeTypingAfterHighlightExtendsOriginalQuery(t *testing.T) {
	writeTestJobsFile(t)
	m := initialModel()
	m.overlay.kind = overlayNone
	m.isCommanding = true
	m.commandInput.Focus()
	m.commandInput.SetValue("debug o")
	m.commandTypedInput = "debug o"

	updated, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(model)
	if got.commandInput.Value() != "debug on" {
		t.Fatalf("after tab commandInput.Value() = %q; want %q", got.commandInput.Value(), "debug on")
	}

	updated, _ = got.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	got = updated.(model)
	if got.commandCompletionActive {
		t.Fatal("commandCompletionActive = true; want false after typing")
	}
	if got.commandInput.Value() != "debug onf" {
		t.Fatalf("commandInput.Value() = %q; want %q", got.commandInput.Value(), "debug onf")
	}
	gotPool := got.operatorCommandPool()
	if labels := commandCompletionLabels(gotPool); len(labels) != 0 {
		t.Fatalf("operatorCommandPool labels = %#v; want no completion after extending selected text", labels)
	}
}

func commandCompletionLabels(completions []operatorCommandCompletion) []string {
	labels := make([]string, 0, len(completions))
	for _, completion := range completions {
		labels = append(labels, completion.Label)
	}
	return labels
}

func TestCommandModeHelpMentionsTabSelection(t *testing.T) {
	m := model{
		termWidth:     100,
		termHeight:    30,
		tableHeight:   calculateTableHeight(30),
		activeFilters: filterValuesFromStatuses(nil),
		commandInput:  newOperatorCommandInput(),
		isCommanding:  true,
	}
	m.commandInput.Focus()

	rendered := ansi.Strip(m.View())
	if !strings.Contains(rendered, "Tab Select/Cycle") {
		t.Fatalf("command mode help missing tab selection hint:\n%s", rendered)
	}
}

func TestCommandModeResultLineDoesNotChangeViewHeight(t *testing.T) {
	base := model{
		termWidth:     100,
		termHeight:    30,
		tableHeight:   calculateTableHeight(30),
		activeFilters: filterValuesFromStatuses(nil),
		commandInput:  newOperatorCommandInput(),
		isCommanding:  true,
	}
	base.commandInput.Focus()
	base.commandInput.SetValue("debug status")

	withResult := base
	withResult.commandResultTitle = "Debug"
	withResult.commandResultMessage = "Debug is off."

	baseLines := strings.Count(ansi.Strip(base.View()), "\n")
	resultLines := strings.Count(ansi.Strip(withResult.View()), "\n")
	if resultLines != baseLines {
		t.Fatalf("command view lines with result = %d; want %d", resultLines, baseLines)
	}
}

func TestCommandModeFooterPlacesControlsOnBottomLine(t *testing.T) {
	m := model{
		termWidth:     100,
		termHeight:    30,
		tableHeight:   calculateTableHeight(30),
		activeFilters: filterValuesFromStatuses(nil),
		commandInput:  newOperatorCommandInput(),
		isCommanding:  true,
	}
	m.commandInput.Focus()
	m.commandInput.SetValue("debug ")
	m.commandTypedInput = "debug "

	lines := strings.Split(strings.TrimRight(ansi.Strip(m.View()), "\n"), "\n")
	if got := strings.TrimSpace(lines[len(lines)-1]); !strings.Contains(got, "Tab Select/Cycle") {
		t.Fatalf("bottom command line = %q; want controls", got)
	}
	if len(lines) < 2 || !strings.Contains(lines[len(lines)-2], "status") {
		t.Fatalf("line above controls = %q; want command completion pool", lines[len(lines)-2])
	}
}

func TestCommandModeFooterReservesCompletionLine(t *testing.T) {
	m := model{
		termWidth:     100,
		termHeight:    30,
		tableHeight:   calculateTableHeight(30),
		activeFilters: filterValuesFromStatuses(nil),
		commandInput:  newOperatorCommandInput(),
		isCommanding:  true,
	}
	m.commandInput.Focus()
	m.commandInput.SetValue("debug statusx")
	m.commandTypedInput = "debug statusx"

	lines := strings.Split(strings.TrimRight(ansi.Strip(m.View()), "\n"), "\n")
	if got := strings.TrimSpace(lines[len(lines)-1]); !strings.Contains(got, "Tab Select/Cycle") {
		t.Fatalf("bottom command line = %q; want controls", got)
	}
	if got := strings.TrimSpace(lines[len(lines)-2]); got != "" {
		t.Fatalf("reserved completion line = %q; want blank", got)
	}
	if got := strings.TrimSpace(lines[len(lines)-3]); !strings.Contains(got, ": debug statusx") {
		t.Fatalf("line above reserved completion line = %q; want command input", got)
	}
}

func TestCommandModeViewStaysWithinTerminalHeight(t *testing.T) {
	m := model{
		termWidth:     100,
		termHeight:    30,
		tableHeight:   calculateTableHeight(30),
		activeFilters: filterValuesFromStatuses(nil),
		commandInput:  newOperatorCommandInput(),
		isCommanding:  true,
	}
	m.commandInput.Focus()

	lines := strings.Split(strings.TrimRight(ansi.Strip(m.View()), "\n"), "\n")
	if len(lines) > m.termHeight {
		t.Fatalf("command mode rendered %d lines; want at most terminal height %d", len(lines), m.termHeight)
	}
}

func TestCommandModeResultClearsOnNextInput(t *testing.T) {
	m := model{
		termWidth:               100,
		termHeight:              30,
		tableHeight:             calculateTableHeight(30),
		activeFilters:           filterValuesFromStatuses(nil),
		commandInput:            newOperatorCommandInput(),
		isCommanding:            true,
		commandResultTitle:      "Debug",
		commandResultMessage:    "Debug is off.",
		commandResultError:      false,
		commandTypedInput:       "d",
		commandCompletionActive: false,
	}
	m.commandInput.Focus()
	m.commandInput.SetValue("d")

	updated, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyTab})
	got := updated.(model)

	if cmd != nil {
		t.Fatalf("handleKeyMsg(tab) cmd = %v; want nil", cmd)
	}
	if got.commandResultMessage != "" || got.commandResultTitle != "" || got.commandResultError {
		t.Fatalf("command result = %q/%q/%t; want cleared", got.commandResultTitle, got.commandResultMessage, got.commandResultError)
	}
	if got.commandInput.Value() != "debug" {
		t.Fatalf("commandInput.Value() = %q; want tab completion to still run", got.commandInput.Value())
	}
}

func TestCommandModeControlsDoNotShareInputLine(t *testing.T) {
	m := model{
		termWidth:     100,
		termHeight:    30,
		tableHeight:   calculateTableHeight(30),
		activeFilters: filterValuesFromStatuses(nil),
		commandInput:  newOperatorCommandInput(),
		isCommanding:  true,
	}
	m.commandInput.Focus()
	m.commandInput.SetValue("debug status")

	rendered := ansi.Strip(m.View())
	for _, line := range strings.Split(rendered, "\n") {
		if strings.Contains(line, "Tab Select/Cycle") && strings.Contains(line, "debug status") {
			t.Fatalf("command controls share input line:\n%s", line)
		}
	}
}

func TestCommandModeEnterKeepsPromptOpen(t *testing.T) {
	writeTestJobsFile(t)
	m := initialModel()
	m.overlay.kind = overlayNone
	m.isCommanding = true
	m.commandInput.Focus()
	m.commandInput.SetValue("debug status")

	updated, cmd := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(model)

	if cmd != nil {
		t.Fatalf("handleKeyMsg(enter) cmd = %v; want nil", cmd)
	}
	if !got.isCommanding {
		t.Fatal("isCommanding = false; want true after command runs")
	}
	if got.overlay.kind == overlayNotice {
		t.Fatal("overlay.kind = overlayNotice; want inline command result")
	}
	if got.commandInput.Value() != "" {
		t.Fatalf("commandInput.Value() = %q; want cleared input", got.commandInput.Value())
	}
	rendered := ansi.Strip(got.View())
	if !strings.Contains(rendered, "Debug is") {
		t.Fatalf("command mode view missing inline command result:\n%s", rendered)
	}
}

func TestCommandModeShowsContextualDebugPool(t *testing.T) {
	m := model{
		termWidth:     100,
		termHeight:    30,
		tableHeight:   calculateTableHeight(30),
		activeFilters: filterValuesFromStatuses(nil),
		commandInput:  newOperatorCommandInput(),
		isCommanding:  true,
	}
	m.commandInput.Focus()
	m.commandInput.SetValue("debug ")

	rendered := ansi.Strip(m.View())
	for _, want := range []string{"status", "on", "off", "path <file>"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("command mode view missing debug option %q:\n%s", want, rendered)
		}
	}
}
