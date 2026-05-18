package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/wallentx/jobscout/internal/cliui"
)

func runLLMBenchmarkCLI(args []string) {
	opts, err := parseBenchmarkCLIOptions(args)
	if err != nil {
		fmt.Printf("Benchmark argument error: %v\n", err)
		os.Exit(1)
	}

	cases, err := loadLLMBenchmarkCases()
	if err != nil {
		fmt.Printf("Benchmark case error: %v\n", err)
		os.Exit(1)
	}

	if opts.List {
		for _, benchCase := range cases {
			fmt.Printf("%s\t%s\n", benchCase.Task, benchCase.ID)
		}
		return
	}

	appCfg, err := loadAppConfig(runtimeConfigPath)
	if err != nil {
		fmt.Printf("Config error: %v\n", err)
		os.Exit(1)
	}
	normalizeLLMConfig(appCfg)
	applyBenchmarkModelOverrides(appCfg, opts)

	provider, _, ok := effectiveLLMProvider(appCfg)
	if !ok {
		fmt.Println("Benchmark error: no effective LLM provider is configured")
		os.Exit(1)
	}

	selected := filterBenchmarkCases(cases, opts.Task)
	if len(selected) == 0 {
		fmt.Printf("Benchmark error: no cases matched task %q\n", opts.Task)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	models, err := benchmarkModelsForRun(ctx, appCfg, opts)
	if err != nil {
		fmt.Printf("Benchmark model selection error: %v\n", err)
		os.Exit(1)
	}

	selectedCount := len(selected)
	modelCount := len(models)
	if modelCount > 0 && selectedCount > math.MaxInt/modelCount {
		fmt.Println("Benchmark error: too many benchmark records to allocate safely")
		os.Exit(1)
	}
	recordCapacity := selectedCount * modelCount
	records := make([]llmBenchmarkRunRecord, 0, recordCapacity)
	for _, modelName := range models {
		if !opts.JSON {
			fmt.Printf("%s %s\n", cliui.Style("==>", cliui.Cyan, cliui.Bold), cliui.Style(provider+"/"+modelName, cliui.Bold))
		}
		runCfg := benchmarkConfigForModel(*appCfg, provider, modelName)
		llm, restoreAuth, err := initConfiguredLLM(ctx, &runCfg)
		if err != nil {
			for _, benchCase := range selected {
				record := llmBenchmarkRunRecord{
					Timestamp:             time.Now().Format(time.RFC3339),
					BenchmarkVersion:      benchmarkVersion,
					Provider:              provider,
					Model:                 modelName,
					Task:                  normalizeBenchmarkTaskName(benchCase.Task),
					CaseID:                benchCase.ID,
					RequiredFieldsPresent: false,
					ScoreCap:              40,
					Error:                 fmt.Sprintf("LLM initialization failed: %v", err),
				}
				records = append(records, record)
				if !opts.JSON {
					printBenchmarkRecordSummary(record)
				}
			}
			continue
		}
		for _, benchCase := range selected {
			if !opts.JSON {
				fmt.Printf("  %s %s\n", cliui.Style("running", cliui.Blue), benchCase.ID)
			}
			record := runLLMBenchmarkCase(ctx, llm, provider, modelName, benchCase)
			records = append(records, record)
			if !opts.JSON {
				printBenchmarkRecordSummary(record)
			}
		}
		restoreAuth()
	}

	normalizeBenchmarkRunScores(records)
	if !opts.JSON {
		printBenchmarkRunSummary(records)
	}

	outputPath, err := writeLLMBenchmarkRecords(records)
	if err != nil {
		fmt.Printf("Benchmark output error: %v\n", err)
		os.Exit(1)
	}

	if opts.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(records); err != nil {
			fmt.Printf("Benchmark JSON output error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Printf("\n%s %s\n", cliui.Style("Wrote benchmark records to", cliui.Green, cliui.Bold), outputPath)
}

func RunLLMBenchmarkCLI(args []string) {
	runLLMBenchmarkCLI(args)
}
