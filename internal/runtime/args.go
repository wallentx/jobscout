package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	AppConfigDirName     = "jobscout"
	ConfigFileName       = "config.yaml"
	SearchPromptFileName = "SEARCH_PROMPT.md"
	SQLiteFileName       = "jobscout.db"

	CommandImport            = "--import"
	CommandImportShort       = "-i"
	CommandFetchDryRun       = "--fetch-dry-run"
	CommandExportJSON        = "--export-json"
	CommandBenchLLM          = "--bench-llm"
	CommandBenchReport       = "--bench-report"
	CommandRepairJobIdentity = "--repair-job-identity"
	CommandDeleteDB          = "--delete-db"
	CommandDemo              = "--demo"
	CommandHelp              = "--help"
	CommandHelpShort         = "-h"
	CommandVersion           = "--version"
	CommandVersionShort      = "-v"
)

type Paths struct {
	Config       string
	SearchPrompt string
	SQLite       string
}

type Options struct {
	Paths           Paths
	Debug           bool
	Demo            bool
	SourceSelection []string
	Command         string
	CommandArgs     []string
}

func DefaultDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(configDir) == "" {
		if homeDir, homeErr := os.UserHomeDir(); homeErr == nil && strings.TrimSpace(homeDir) != "" {
			configDir = filepath.Join(homeDir, ".config")
		} else {
			configDir = "."
		}
	}
	return filepath.Join(configDir, AppConfigDirName)
}

func DefaultPath(filename string) string {
	return filepath.Join(DefaultDir(), filename)
}

func DefaultPaths() Paths {
	return Paths{
		Config:       DefaultPath(ConfigFileName),
		SearchPrompt: DefaultPath(SearchPromptFileName),
		SQLite:       DefaultPath(SQLiteFileName),
	}
}

func ParseArgs(args []string) (Options, error) {
	options := Options{Paths: DefaultPaths()}
	for i := 1; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == CommandHelp || arg == CommandHelpShort:
			options.Command = CommandHelp
		case arg == CommandVersion || arg == CommandVersionShort:
			options.Command = CommandVersion
		case arg == CommandDemo:
			options.Demo = true
		case arg == "-d" || arg == "--debug":
			options.Debug = true
		case strings.HasPrefix(arg, "--config="):
			path := strings.TrimSpace(strings.TrimPrefix(arg, "--config="))
			if path == "" {
				return Options{}, fmt.Errorf("--config requires a file path")
			}
			options.Paths.Config = path
		case strings.HasPrefix(arg, "--criteria="):
			return Options{}, fmt.Errorf("--criteria has been removed; put search criteria in config.yaml")
		case strings.HasPrefix(arg, "--sources="):
			sources, err := parseSourceSelection(strings.TrimPrefix(arg, "--sources="))
			if err != nil {
				return Options{}, err
			}
			options.SourceSelection = sources
		case arg == "--config":
			if i+1 >= len(args) {
				return Options{}, fmt.Errorf("--config requires a file path")
			}
			path := strings.TrimSpace(args[i+1])
			if path == "" {
				return Options{}, fmt.Errorf("--config requires a file path")
			}
			options.Paths.Config = path
			i++
		case arg == "--criteria":
			return Options{}, fmt.Errorf("--criteria has been removed; put search criteria in config.yaml")
		case arg == "--sources":
			if i+1 >= len(args) {
				return Options{}, fmt.Errorf("--sources requires a comma-separated source list")
			}
			sources, err := parseSourceSelection(args[i+1])
			if err != nil {
				return Options{}, err
			}
			options.SourceSelection = sources
			i++
		case arg == "--migrate":
			return Options{}, fmt.Errorf("--migrate has been removed; use --import to load JSON exports into the configured SQLite database")
		case isCommand(arg):
			options.Command = arg
		default:
			if options.Command != "" {
				options.CommandArgs = append(options.CommandArgs, arg)
			}
		}
	}
	return options, nil
}

func isCommand(arg string) bool {
	switch arg {
	case CommandImportShort, CommandImport, CommandFetchDryRun, CommandExportJSON, CommandBenchLLM, CommandBenchReport, CommandRepairJobIdentity, CommandDeleteDB, CommandHelp, CommandHelpShort, CommandVersion, CommandVersionShort:
		return true
	default:
		return false
	}
}

func parseSourceSelection(value string) ([]string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil, fmt.Errorf("--sources requires a comma-separated source list")
	}
	seen := make(map[string]bool)
	var sources []string
	for _, part := range strings.Split(raw, ",") {
		source := normalizeSourceName(part)
		if source == "" {
			continue
		}
		if source == "all" {
			return nil, nil
		}
		if !isValidSourceName(source) {
			return nil, fmt.Errorf("--sources includes unsupported source %q; valid sources are rss, site, llm, llm_web, all", strings.TrimSpace(part))
		}
		if seen[source] {
			continue
		}
		seen[source] = true
		sources = append(sources, source)
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("--sources requires at least one source")
	}
	return sources, nil
}

func normalizeSourceName(value string) string {
	source := strings.ToLower(strings.TrimSpace(value))
	source = strings.ReplaceAll(source, "-", "_")
	switch source {
	case "feed", "feeds":
		return "rss"
	case "apis":
		return "api"
	case "sites", "site_search", "site_searches":
		return "site"
	case "web_search", "llm_web_search":
		return "llm_web"
	case "llm_job_search", "llm_search":
		return "llm"
	default:
		return source
	}
}

func isValidSourceName(source string) bool {
	switch source {
	case "rss", "api", "site", "llm", "llm_web":
		return true
	default:
		return false
	}
}
