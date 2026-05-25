package tuiapp

import (
	"context"

	"github.com/wallentx/jobscout/internal/fetcher"
	healthpkg "github.com/wallentx/jobscout/internal/health"
	llmpkg "github.com/wallentx/jobscout/internal/llm"
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
var runtimeDebugPath = "debug.log"
var runtimeSourceSelection []string
var runtimeCandidateLimitPerSource *int
var runtimeAcceptedLimit *int
var runtimeBuildVersion = "dev"

var defaultRuntimeStores = appruntime.InMemoryStores()
var runtimeJobStore JobStore = defaultRuntimeStores.Jobs
var runtimeHealthStore HealthStore = defaultRuntimeStores.Health
var runtimeCompanyIdentityStore CompanyIdentityStore = storage.NoopCompanyIdentityStore{}
var runtimeCandidateStore CandidateStore = defaultRuntimeStores.Candidates
var runtimeUpdateChecker = updatecheck.CheckLatestRelease

func loadRuntimeJobs() ([]Job, error)  { return runtimeJobStore.LoadJobs() }
func saveRuntimeJobs(jobs []Job) error { return runtimeJobStore.SaveJobs(jobs) }

type updateCheckerFunc func(context.Context, string) (updatecheck.Result, error)

func setRuntimeDebug(enabled bool, path string) {
	runtimeDebugEnabled = enabled
	if path != "" {
		runtimeDebugPath = path
	}
	fetcher.SetDebug(runtimeDebugEnabled, runtimeDebugPath)
	healthpkg.SetDebug(runtimeDebugEnabled, runtimeDebugPath)
	llmpkg.SetDebug(runtimeDebugEnabled, runtimeDebugPath)
}
