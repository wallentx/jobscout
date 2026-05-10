package tuiapp

import (
	"context"

	appruntime "github.com/wallentx/jobscout/internal/runtime"
	"github.com/wallentx/jobscout/internal/storage"
	"github.com/wallentx/jobscout/internal/updatecheck"
)

const (
	configFilePath       = "config.yaml"
	searchPromptFilePath = "SEARCH_PROMPT.md"
	sqliteFilePath       = "jobscout.db"
)

var runtimeConfigPath = configFilePath
var runtimeSearchPromptPath = searchPromptFilePath
var runtimeSQLitePath = sqliteFilePath
var runtimeDebugEnabled bool
var runtimeSourceSelection []string
var runtimeBuildVersion = "dev"

var defaultRuntimeStores = appruntime.InMemoryStores()
var runtimeJobStore JobStore = defaultRuntimeStores.Jobs
var runtimeHealthStore HealthStore = defaultRuntimeStores.Health
var runtimeCompanyIdentityStore CompanyIdentityStore = storage.NoopCompanyIdentityStore{}
var runtimeUpdateChecker = updatecheck.CheckLatestRelease

func loadRuntimeJobs() ([]Job, error)  { return runtimeJobStore.LoadJobs() }
func saveRuntimeJobs(jobs []Job) error { return runtimeJobStore.SaveJobs(jobs) }

type updateCheckerFunc func(context.Context, string) (updatecheck.Result, error)
