package tuiapp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wallentx/jobscout/internal/config"
)

type fakeHealthStore struct {
	cache     HealthCache
	getResult *CompanyHealthResult
	getTime   time.Time
	err       error
	setCalls  int
}

func (s *fakeHealthStore) LoadHealthCache() (HealthCache, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.cache == nil {
		return make(HealthCache), nil
	}
	return s.cache, nil
}

func (s *fakeHealthStore) SaveHealthCache(cache HealthCache) error {
	s.cache = cache
	return s.err
}

func (s *fakeHealthStore) GetHealth(company string) (*CompanyHealthResult, time.Time, error) {
	if s.err != nil {
		return nil, time.Time{}, s.err
	}
	return s.getResult, s.getTime, nil
}

func (s *fakeHealthStore) SetHealth(company string, result *CompanyHealthResult, fetchedAt time.Time) error {
	s.setCalls++
	if s.cache == nil {
		s.cache = make(HealthCache)
	}
	s.cache[company] = HealthCacheEntry{Result: result, Timestamp: fetchedAt}
	return s.err
}

func (s *fakeHealthStore) ClearHealthCache() error {
	s.cache = make(HealthCache)
	return s.err
}

func writeTestJobsFile(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	jobs := []Job{
		{
			Company: "Acme",
			Title:   "Backend Engineer",
			Status:  "Unopened",
		},
		{
			Company: "Bravo",
			Title:   "Go Developer",
			Status:  "Viewed",
		},
	}
	jobs[0].SetDateFromString("2026-02-28")
	jobs[1].SetDateFromString("2026-02-27")

	prevJobStore := runtimeJobStore
	prevHealthStore := runtimeHealthStore
	runtimeJobStore = &fakeJobStore{loaded: jobs}
	runtimeHealthStore = &fakeHealthStore{}
	t.Cleanup(func() {
		runtimeJobStore = prevJobStore
		runtimeHealthStore = prevHealthStore
	})

	appCfg := config.DefaultAppConfig()
	appCfg.Criteria = config.DefaultCriteriaConfig()
	appCfg.Criteria.RoleFamilies = []RoleFamilyID{RoleDevOpsSRESystems}
	appCfg.LLM.JobFiltering = false
	appCfg.LLM.JobSearch = false
	appCfg.Sources.LLMWeb.Enabled = false
	if err := config.SaveAppConfig(filepath.Join(tmpDir, configFilePath), &appCfg); err != nil {
		t.Fatalf("config.SaveAppConfig(): %v", err)
	}
	if err := config.SaveSearchPrompt(filepath.Join(tmpDir, searchPromptFilePath), config.DefaultSearchPrompt(&appCfg.Criteria)); err != nil {
		t.Fatalf("config.SaveSearchPrompt(): %v", err)
	}

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
}

func restoreRuntimePathsAfterTest(t *testing.T) {
	t.Helper()

	prevConfig := runtimeConfigPath
	prevPrompt := runtimeSearchPromptPath
	prevSQLite := runtimeSQLitePath
	prevDebug := runtimeDebugEnabled
	prevBuildVersion := runtimeBuildVersion
	prevUpdateChecker := runtimeUpdateChecker
	t.Cleanup(func() {
		runtimeConfigPath = prevConfig
		runtimeSearchPromptPath = prevPrompt
		runtimeSQLitePath = prevSQLite
		runtimeDebugEnabled = prevDebug
		runtimeBuildVersion = prevBuildVersion
		runtimeUpdateChecker = prevUpdateChecker
	})
}
