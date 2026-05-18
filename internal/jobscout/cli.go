package jobscout

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/domain"
	"github.com/wallentx/jobscout/internal/fetcher"
	llmpkg "github.com/wallentx/jobscout/internal/llm"
	appruntime "github.com/wallentx/jobscout/internal/runtime"
	"github.com/wallentx/jobscout/internal/storage"
)

func runImportCLI(stores appruntime.Stores, args []string) int {
	jobs, err := loadJobsOrEmpty(stores)
	if err != nil {
		jobs = []domain.Job{}
	}

	data, err := readImportFile(args)
	if err != nil {
		fmt.Printf("%v\n", err)
		return 1
	}

	imported := decodeJobsOrExit(data)
	added, merged := storage.MergeJobs(jobs, imported)
	if len(imported) > 0 {
		if err := stores.Jobs.SaveJobs(merged); err != nil {
			fmt.Printf("Error: %v\n", err)
			return 1
		}
	}
	fmt.Printf("Successfully imported %d new jobs.\n", added)
	return 0
}

func readImportFile(args []string) ([]byte, error) {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return nil, fmt.Errorf("--import requires a JSON file path")
	}
	path := args[0]
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading import file %s: %v", path, err)
	}
	return data, nil
}

func decodeJobsOrExit(data []byte) []domain.Job {
	if len(data) == 0 {
		return nil
	}
	var jobs []domain.Job
	if err := json.Unmarshal(data, &jobs); err != nil {
		fmt.Printf("Error: invalid JSON: %v\n", err)
		os.Exit(1)
	}
	return jobs
}

func runExportJSONCLI(stores appruntime.Stores, outputPath string) int {
	jobs, err := stores.Jobs.LoadJobs()
	if err != nil {
		fmt.Printf("Error loading jobs: %v\n", err)
		return 1
	}

	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		fmt.Printf("Error encoding JSON: %v\n", err)
		return 1
	}
	data = append(data, '\n')

	if outputPath == "" || outputPath == "-" {
		if _, err := os.Stdout.Write(data); err != nil {
			fmt.Printf("Error writing JSON to stdout: %v\n", err)
			return 1
		}
		return 0
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		fmt.Printf("Error writing JSON file %s: %v\n", outputPath, err)
		return 1
	}
	fmt.Printf("Exported %d jobs to %s\n", len(jobs), outputPath)
	return 0
}

func runFetchDryRun(options appruntime.Options, stores appruntime.Stores, jsonOutput bool) int {
	if !jsonOutput {
		fmt.Println("Starting dry-run fetch...")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	appCfg, err := config.LoadAppConfig(options.Paths.Config)
	if err != nil {
		if !jsonOutput {
			fmt.Fprintf(os.Stderr, "Failed to load config.yaml: %v\n", err)
		}
		return 1
	}
	config.ApplyFetchSourceSelection(appCfg, options.SourceSelection)

	criteriaCfg, err := config.LoadCriteriaConfig(options.Paths.Config)
	if err != nil {
		if !jsonOutput {
			fmt.Fprintf(os.Stderr, "Warning: Could not load criteria from config.yaml, filtering disabled (%v)\n", err)
		}
		criteriaCfg = nil
	}

	existing, err := loadJobsOrEmpty(stores)
	if err != nil {
		existing = []domain.Job{}
	}

	fetcher.ConfigureLLM(llmpkg.InitConfiguredLLMForTask, llmpkg.ExecuteLLMSearch, llmpkg.EnrichJobIdentityWithLLMUsage)
	fetcher.ConfigureLLMWebSearch(llmpkg.ExecuteLLMWebSearch)
	newJobs, summary, err := fetcher.FetchAllJobsSkippingExisting(ctx, appCfg, criteriaCfg, existing, nil)
	if err != nil {
		if !jsonOutput {
			fmt.Fprintf(os.Stderr, "Fetch failed: %v\n", err)
		}
		return 1
	}
	printFetchSummary(summary, options.Debug, jsonOutput)

	if appCfg.LLM.JobFiltering && len(newJobs) > 0 {
		if !jsonOutput {
			fmt.Println("Running LLM job filtering on fetched jobs...")
		}
		filterCtx, filterCancel := context.WithTimeout(context.Background(), 180*time.Second)
		var notices []string
		newJobs, notices = llmpkg.FilterJobsWithLLM(filterCtx, appCfg, criteriaCfg, newJobs)
		filterCancel()
		for _, notice := range notices {
			if !jsonOutput {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", notice)
			}
		}
		if !jsonOutput && len(notices) == 0 {
			fmt.Printf("LLM job filtering complete: kept %d jobs\n", len(newJobs))
		}
	}

	added, merged := storage.MergeJobs(existing, newJobs)
	newlyAdded := merged[len(merged)-added:]

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(newlyAdded); err != nil {
			fmt.Fprintf(os.Stderr, "JSON encoding error: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Printf("\nFound %d total jobs from sources.\n", len(newJobs))
	fmt.Printf("After deduplication, %d NEW jobs would be added.\n\n", added)

	if added > 0 {
		fmt.Println("--- NEW JOBS ---")
		for i, job := range newlyAdded {
			fmt.Printf("%d. %s - %s\n", i+1, job.Company, job.Title)
			fmt.Printf("   Pay: %s\n\n", job.Compensation)
		}
	}
	return 0
}

func printFetchSummary(summary fetcher.FetchSummary, debug bool, jsonOutput bool) {
	if jsonOutput {
		return
	}
	for _, notice := range summary.Notices {
		fmt.Fprintf(os.Stderr, "%s\n", notice)
	}
	rejected := flattenRejectedSummary(summary.Rejected)
	for _, reason := range sortedStringKeys(rejected) {
		fmt.Fprintf(os.Stderr, "Rejected %d entries (%s)\n", len(rejected[reason]), reason)
	}
	if !debug {
		return
	}
	for _, searchType := range orderedSearchTypes(mapKeys(summary.Searches)) {
		status := summary.Searches[searchType]
		if strings.TrimSpace(status) == "" {
			continue
		}
		fmt.Fprintf(os.Stderr, "%s: %s\n", searchType, status)
	}
	for _, reason := range sortedStringKeys(summary.Filtered) {
		fmt.Fprintf(os.Stderr, "Filtered %d entries (%s)\n", len(summary.Filtered[reason]), reason)
	}
}

func runRepairJobIdentityCLI(options appruntime.Options, stores appruntime.Stores) int {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	jobs, err := stores.Jobs.LoadJobs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load jobs: %v\n", err)
		return 1
	}

	beforeJobs := append([]domain.Job(nil), jobs...)
	appCfg, cfgErr := config.LoadAppConfig(options.Paths.Config)
	if cfgErr != nil {
		fmt.Fprintf(os.Stderr, "LLM identity repair unavailable: %v\n", cfgErr)
	}
	repairJobs, repairIndexes := domain.IdentityRepairTargets(jobs)
	fmt.Printf("Repairing identity data for %d of %d jobs.\n", len(repairJobs), len(jobs))
	if len(repairJobs) > 0 {
		fetcher.ConfigureLLM(llmpkg.InitConfiguredLLMForTask, llmpkg.ExecuteLLMSearch, llmpkg.EnrichJobIdentityWithLLMUsage)
		fetcher.ConfigureLLMWebSearch(llmpkg.ExecuteLLMWebSearch)
		repairJobs = fetcher.EnrichJobsFromApplyPagesWithConfigStoreAndProgress(ctx, repairJobs, appCfg, stores.CompanyIdentity, func(message string) {
			fmt.Println(message)
		}, nil)
		repairJobs = fetcher.EnrichJobsFromPublicProfileIndustryWithProgress(ctx, repairJobs, func(message string) {
			fmt.Println(message)
		})
		fetcher.PersistTrustedCompanyIdentities(ctx, repairJobs, stores.CompanyIdentity)
		for i, jobIdx := range repairIndexes {
			jobs[jobIdx] = repairJobs[i]
		}
	}
	if err := ctx.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Repair stopped early: %v. Saving completed updates.\n", err)
	}

	updated := 0
	expired := 0
	for i := range jobs {
		if reason := fetcher.UnusableJobReason(jobs[i]); reason != "" && jobs[i].Status != "Expired" {
			jobs[i].Status = "Expired"
			expired++
		}
		if beforeJobs[i].CompanyWebsite != jobs[i].CompanyWebsite ||
			beforeJobs[i].CompanySummary != jobs[i].CompanySummary ||
			beforeJobs[i].CompanyIndustry != jobs[i].CompanyIndustry ||
			beforeJobs[i].Compensation != jobs[i].Compensation ||
			beforeJobs[i].Status != jobs[i].Status {
			updated++
		}
	}

	if err := stores.Jobs.SaveJobs(jobs); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save repaired jobs: %v\n", err)
		return 1
	}
	fmt.Printf("Repaired identity data for %d of %d jobs.\n", updated, len(jobs))
	if expired > 0 {
		fmt.Printf("Marked %d unusable jobs as Expired.\n", expired)
	}
	return 0
}

func loadJobsOrEmpty(stores appruntime.Stores) ([]domain.Job, error) {
	jobs, err := stores.Jobs.LoadJobs()
	if err != nil {
		return nil, err
	}
	return jobs, nil
}

func flattenRejectedSummary(rejected map[string]map[string][]string) map[string][]string {
	if len(rejected) == 0 {
		return nil
	}
	flat := make(map[string][]string)
	for _, grouped := range rejected {
		for reason, entries := range grouped {
			flat[reason] = append(flat[reason], entries...)
		}
	}
	return flat
}

func sortedStringKeys[V any](items map[string]V) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func mapKeys[V any](items map[string]V) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	return keys
}

func orderedSearchTypes(values []string) []string {
	priority := map[string]int{
		fetcher.FetchSearchLLM:    0,
		fetcher.FetchSearchLLMWeb: 1,
		fetcher.FetchSearchRSS:    2,
		fetcher.FetchSearchAPI:    3,
		fetcher.FetchSearchSite:   4,
	}
	sort.Slice(values, func(i, j int) bool {
		pi, iok := priority[values[i]]
		pj, jok := priority[values[j]]
		switch {
		case iok && jok:
			return pi < pj
		case iok:
			return true
		case jok:
			return false
		default:
			return values[i] < values[j]
		}
	})
	return values
}
