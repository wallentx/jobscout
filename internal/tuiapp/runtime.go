package tuiapp

import (
	appruntime "github.com/wallentx/jobscout/internal/runtime"

	tea "github.com/charmbracelet/bubbletea"
)

func ConfigureRuntime(options appruntime.Options, stores appruntime.Stores, buildVersion string) {
	runtimeConfigPath = options.Paths.Config
	runtimeSearchPromptPath = options.Paths.SearchPrompt
	runtimeSQLitePath = options.Paths.SQLite
	runtimeDebugEnabled = options.Debug
	runtimeSourceSelection = append([]string(nil), options.SourceSelection...)
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
}

func NewModel() tea.Model {
	return initialModel()
}
