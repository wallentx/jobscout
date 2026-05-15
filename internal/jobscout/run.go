package jobscout

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/wallentx/jobscout/internal/cliui"
	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/fetcher"
	healthpkg "github.com/wallentx/jobscout/internal/health"
	llmpkg "github.com/wallentx/jobscout/internal/llm"
	appruntime "github.com/wallentx/jobscout/internal/runtime"
	"github.com/wallentx/jobscout/internal/tuiapp"

	tea "github.com/charmbracelet/bubbletea"
)

var version = "dev"

func Run(args []string) int {
	options, err := appruntime.ParseArgs(args)
	if err != nil {
		fmt.Printf("Argument error: %v\n", err)
		return 1
	}

	switch options.Command {
	case appruntime.CommandHelp:
		printHelp()
		return 0
	case appruntime.CommandVersion:
		fmt.Printf("jobscout %s\n", currentVersion())
		return 0
	case appruntime.CommandDeleteDB:
		return runDeleteDBCLI(options)
	}

	var stores appruntime.Stores
	cleanupStores := func() {}
	restoreConfigRuntime := config.ConfigureRuntime(options.Paths.Config, options.Paths.SearchPrompt)
	defer restoreConfigRuntime()
	restoreFetcherRuntime := fetcher.ConfigureRuntime(options.Paths.SearchPrompt, nil)
	defer restoreFetcherRuntime()
	restoreFetcherDebug := fetcher.ConfigureDebug(options.Debug, "debug.log")
	defer restoreFetcherDebug()
	restoreHealthDebug := healthpkg.ConfigureDebug(options.Debug, "debug.log")
	defer restoreHealthDebug()
	restoreLLMDebug := llmpkg.ConfigureDebug(options.Debug, "debug.log")
	defer restoreLLMDebug()
	restoreLLMRuntime := llmpkg.ConfigureRuntime(options.Paths.Config)
	defer restoreLLMRuntime()
	if options.Demo {
		demoCfg := config.DemoAppConfig()
		restoreConfig := config.ConfigureInMemoryRuntime(demoCfg, config.DefaultSearchPrompt(&demoCfg.Criteria))
		defer restoreConfig()
		restoreLinkedInCache := fetcher.ConfigureLinkedInTypeaheadCache("")
		defer restoreLinkedInCache()
		stores = appruntime.InMemoryStores()
	} else {
		var err error
		stores, cleanupStores, err = appruntime.OpenStores(options.Paths)
		if err != nil {
			fmt.Printf("Storage initialization error: %v\n", err)
			return 1
		}
	}
	defer cleanupStores()
	tuiapp.ConfigureRuntime(options, stores, currentVersion())

	switch options.Command {
	case appruntime.CommandImportShort, appruntime.CommandImport:
		return runImportCLI(stores)
	case appruntime.CommandExportJSON:
		outputPath := ""
		if len(options.CommandArgs) > 0 {
			outputPath = options.CommandArgs[0]
		}
		return runExportJSONCLI(stores, outputPath)
	case appruntime.CommandFetchDryRun:
		return runFetchDryRun(options, stores, hasArg(options.CommandArgs, "--json"))
	case appruntime.CommandRepairJobIdentity:
		return runRepairJobIdentityCLI(options, stores)
	case appruntime.CommandBenchLLM:
		llmpkg.RunLLMBenchmarkCLI(options.CommandArgs)
		return 0
	case appruntime.CommandBenchReport:
		llmpkg.RunLLMBenchmarkReportCLI(options.CommandArgs)
		return 0
	}

	if err := runTUI(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		return 1
	}
	return 0
}

func currentVersion() string {
	if value := strings.TrimSpace(version); value != "" && value != "dev" {
		return value
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if value := strings.TrimSpace(info.Main.Version); value != "" && value != "(devel)" {
			return value
		}
	}
	return "dev"
}

func runTUI() error {
	p := tea.NewProgram(
		tuiapp.NewModel(),
		tea.WithAltScreen(),
		tea.WithInput(os.Stdin),
		tea.WithMouseCellMotion(),
		tea.WithOutput(os.Stdout),
	)
	_, err := p.Run()
	return err
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func runDeleteDBCLI(options appruntime.Options) int {
	if options.Demo {
		fmt.Println("Demo mode does not use a database; nothing to delete.")
		return 0
	}
	removed, err := appruntime.DeleteSQLiteDatabase(options.Paths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to delete database %s: %v\n", options.Paths.SQLite, err)
		return 1
	}
	if len(removed) == 0 {
		fmt.Printf("No database found at %s.\n", options.Paths.SQLite)
		return 0
	}
	fmt.Printf("Deleted database %s", options.Paths.SQLite)
	if len(removed) > 1 {
		fmt.Printf(" and %d SQLite sidecar files", len(removed)-1)
	}
	fmt.Println(".")
	return 0
}

func printHelp() {
	fmt.Print(renderHelp())
}

func renderHelp() string {
	return fmt.Sprintf(`%s is a terminal job-search tracker.

%s:
  jobscout [options]
  jobscout [options] <command> [command options]

%s:
  --demo                  Run with in-memory demo data; read/write no user config or database
  -d, --debug             Show additional fetch and Company Health details
  --sources <list>        Use selected active fetch sources: rss, site, llm, llm_web, all
  --sources=<list>        Same as --sources <list>
                            llm_web is an opt-in experimental source
  --config <path>         Use an alternate config file
  --config=<path>         Same as --config <path>
  -h, --help              Show this help
  -v, --version           Show version information

%s:
  --fetch-dry-run [--json]       Fetch jobs without saving them
  --export-json [path|-]         Export saved jobs as JSON
  --import, -i                   Import jobs from stdin or editor JSON
  --delete-db                    Delete the SQLite database and exit
  --repair-job-identity          Repair missing company identity data
  --bench-llm [options]          Run LLM benchmark cases
    --list                       List embedded benchmark cases and exit
    --task <task|case>           Run only a benchmark task or case ID
    --task=<task|case>           Same as --task <task|case>
    --provider <name>            Override the configured LLM provider
    --provider=<name>            Same as --provider <name>
    --model <name>               Override the configured model
    --model=<name>               Same as --model <name>
    --all-models                 Run all discoverable provider models
    --json                       Print run records as JSON after saving them
    tasks: llm_job_search, llm_job_filtering, llm_company_health, job_identity, resume_to_criteria
  --bench-report [options]       Summarize saved LLM benchmark results
    --latest                     Only report the newest benchmark file
    --format <text|md|json>      Select report output format
    --format=<text|md|json>      Same as --format <text|md|json>
    --json                       Print saved benchmark records as JSON

Runtime files default to your operating system's user config directory. Use jobscout --demo to try the app without touching them.
`,
		cliui.Style("jobscout", cliui.Cyan, cliui.Bold),
		cliui.Style("Usage", cliui.Cyan, cliui.Bold),
		cliui.Style("Options", cliui.Cyan, cliui.Bold),
		cliui.Style("Commands", cliui.Cyan, cliui.Bold),
	)
}
