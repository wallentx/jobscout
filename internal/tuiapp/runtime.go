package tuiapp

import (
	appruntime "github.com/wallentx/jobscout/internal/runtime"

	tea "github.com/charmbracelet/bubbletea"
)

func ConfigureRuntime(options appruntime.Options, stores appruntime.Stores, buildVersion string) {
	runtimeConfigPath = options.Paths.Config
	runtimeSearchPromptPath = options.Paths.SearchPrompt
	runtimeSQLitePath = options.Paths.SQLite
	setRuntimeDebug(options.Debug, runtimeDebugPath)
	runtimeSourceSelection = append([]string(nil), options.SourceSelection...)
	runtimeCandidateLimitPerSource = cloneRuntimeIntPtr(options.CandidateLimitPerSource)
	runtimeAcceptedLimit = cloneRuntimeIntPtr(options.AcceptedLimit)
	runtimeBuildVersion = buildVersion
	if stores.Jobs != nil {
		runtimeJobStore = stores.Jobs
	}
	if stores.Health != nil {
		runtimeHealthStore = stores.Health
	}
	if stores.CompanyIdentity != nil {
		runtimeCompanyIdentityStore = stores.CompanyIdentity
	}
	if stores.Candidates != nil {
		runtimeCandidateStore = stores.Candidates
	}
}

func NewModel() tea.Model {
	return initialModel()
}

func cloneRuntimeIntPtr(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
