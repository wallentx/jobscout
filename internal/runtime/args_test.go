package runtime

import (
	"path/filepath"
	"testing"
)

func TestParseArgs(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	options, err := ParseArgs([]string{
		"jobscout",
		"--fetch-dry-run",
		"--json",
	})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if options.Command != CommandFetchDryRun {
		t.Fatalf("ParseArgs(...).Command = %q; want %q", options.Command, CommandFetchDryRun)
	}
	if len(options.CommandArgs) != 1 || options.CommandArgs[0] != "--json" {
		t.Fatalf("ParseArgs(...).CommandArgs = %#v; want []string{\"--json\"}", options.CommandArgs)
	}
}

func TestParseArgsUsesUserConfigDirDefaults(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	options, err := ParseArgs([]string{"jobscout"})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

	runtimeDir := filepath.Join(configRoot, AppConfigDirName)
	tests := map[string]string{
		"Config":       options.Paths.Config,
		"SearchPrompt": options.Paths.SearchPrompt,
		"SQLite":       options.Paths.SQLite,
	}
	want := map[string]string{
		"Config":       filepath.Join(runtimeDir, ConfigFileName),
		"SearchPrompt": filepath.Join(runtimeDir, SearchPromptFileName),
		"SQLite":       filepath.Join(runtimeDir, SQLiteFileName),
	}
	for name, got := range tests {
		if got != want[name] {
			t.Fatalf("ParseArgs(...).Paths.%s = %q; want %q", name, got, want[name])
		}
	}
}

func TestParseArgsExportJSON(t *testing.T) {
	options, err := ParseArgs([]string{
		"jobscout",
		"--export-json",
		"exported.json",
	})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if options.Command != CommandExportJSON {
		t.Fatalf("ParseArgs(...).Command = %q; want %q", options.Command, CommandExportJSON)
	}
	if len(options.CommandArgs) != 1 || options.CommandArgs[0] != "exported.json" {
		t.Fatalf("ParseArgs(...).CommandArgs = %#v; want []string{\"exported.json\"}", options.CommandArgs)
	}
}

func TestParseArgsDeleteDB(t *testing.T) {
	options, err := ParseArgs([]string{
		"jobscout",
		"--delete-db",
	})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if options.Command != CommandDeleteDB {
		t.Fatalf("ParseArgs(...).Command = %q; want %q", options.Command, CommandDeleteDB)
	}
}

func TestParseArgsRejectsRemovedMigrateCommand(t *testing.T) {
	if _, err := ParseArgs([]string{"jobscout", "--migrate"}); err == nil {
		t.Fatal("ParseArgs(... --migrate) error = nil; want removed-command error")
	}
}

func TestParseArgsConfigOverride(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)

	options, err := ParseArgs([]string{
		"jobscout",
		"--config",
		"fixtures/jr.yaml",
		"--fetch-dry-run",
	})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if options.Paths.Config != "fixtures/jr.yaml" {
		t.Fatalf("ParseArgs(...).Paths.Config = %q; want fixtures/jr.yaml", options.Paths.Config)
	}
	if options.Command != CommandFetchDryRun {
		t.Fatalf("ParseArgs(...).Command = %q; want %q", options.Command, CommandFetchDryRun)
	}
	if len(options.CommandArgs) != 0 {
		t.Fatalf("ParseArgs(...).CommandArgs = %#v; want none", options.CommandArgs)
	}
}

func TestParseArgsDebugFlag(t *testing.T) {
	options, err := ParseArgs([]string{
		"jobscout",
		"-d",
		"--fetch-dry-run",
	})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if !options.Debug {
		t.Fatal("ParseArgs(...).Debug = false; want true")
	}
	if options.Command != CommandFetchDryRun {
		t.Fatalf("ParseArgs(...).Command = %q; want %q", options.Command, CommandFetchDryRun)
	}
	if len(options.CommandArgs) != 0 {
		t.Fatalf("ParseArgs(...).CommandArgs = %#v; want none", options.CommandArgs)
	}
}

func TestParseArgsDemoFlag(t *testing.T) {
	options, err := ParseArgs([]string{
		"jobscout",
		"--demo",
		"--fetch-dry-run",
	})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if !options.Demo {
		t.Fatal("ParseArgs(...).Demo = false; want true")
	}
	if options.Command != CommandFetchDryRun {
		t.Fatalf("ParseArgs(...).Command = %q; want %q", options.Command, CommandFetchDryRun)
	}
}

func TestParseArgsSourcesFlag(t *testing.T) {
	options, err := ParseArgs([]string{
		"jobscout",
		"--sources",
		"rss,llm,site",
		"--fetch-dry-run",
	})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	want := []string{"rss", "llm", "site"}
	if len(options.SourceSelection) != len(want) {
		t.Fatalf("ParseArgs(...).SourceSelection = %#v; want %#v", options.SourceSelection, want)
	}
	for i := range want {
		if options.SourceSelection[i] != want[i] {
			t.Fatalf("ParseArgs(...).SourceSelection = %#v; want %#v", options.SourceSelection, want)
		}
	}
}

func TestParseArgsSourcesFlagWithEqualsAndAliases(t *testing.T) {
	options, err := ParseArgs([]string{
		"jobscout",
		"--sources=feeds,site-search,llm-web,llm_job_search,api",
	})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	want := []string{"rss", "site", "llm_web", "llm", "api"}
	if len(options.SourceSelection) != len(want) {
		t.Fatalf("ParseArgs(...).SourceSelection = %#v; want %#v", options.SourceSelection, want)
	}
	for i := range want {
		if options.SourceSelection[i] != want[i] {
			t.Fatalf("ParseArgs(...).SourceSelection = %#v; want %#v", options.SourceSelection, want)
		}
	}
}

func TestParseArgsSourcesFlagRejectsUnsupportedSource(t *testing.T) {
	if _, err := ParseArgs([]string{"jobscout", "--sources", "rss,google"}); err == nil {
		t.Fatal("ParseArgs(... --sources rss,google) error = nil; want error")
	}
}

func TestParseArgsHelpAndVersion(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{
			args: []string{"jobscout", "-h"},
			want: CommandHelp,
		},
		{
			args: []string{"jobscout", "--help"},
			want: CommandHelp,
		},
		{
			args: []string{"jobscout", "--version"},
			want: CommandVersion,
		},
		{
			args: []string{"jobscout", "-v"},
			want: CommandVersion,
		},
	}

	for _, tt := range tests {
		options, err := ParseArgs(tt.args)
		if err != nil {
			t.Fatalf("ParseArgs(%#v) error = %v", tt.args, err)
		}
		if options.Command != tt.want {
			t.Fatalf("ParseArgs(%#v).Command = %q; want %q", tt.args, options.Command, tt.want)
		}
	}
}

func TestParseArgsSupportsEqualsOverrides(t *testing.T) {
	options, err := ParseArgs([]string{
		"jobscout",
		"--config=fixtures/jr.yaml",
		"--fetch-dry-run",
	})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if options.Paths.Config != "fixtures/jr.yaml" {
		t.Fatalf("ParseArgs(...).Paths.Config = %q; want fixtures/jr.yaml", options.Paths.Config)
	}
}

func TestParseArgsMissingPathErrors(t *testing.T) {
	tests := [][]string{
		{"jobscout", "--config"},
		{"jobscout", "--config="},
	}

	for _, args := range tests {
		if _, err := ParseArgs(args); err == nil {
			t.Fatalf("ParseArgs(%#v) error = nil; want error", args)
		}
	}
}

func TestParseArgsRejectsRemovedCriteriaFlag(t *testing.T) {
	tests := [][]string{
		{"jobscout", "--criteria", "old-search.yaml"},
		{"jobscout", "--criteria=old-search.yaml"},
	}

	for _, args := range tests {
		if _, err := ParseArgs(args); err == nil {
			t.Fatalf("ParseArgs(%#v) error = nil; want removed-criteria error", args)
		}
	}
}

func TestParseArgsIgnoresUnknownArgsBeforeCommand(t *testing.T) {
	options, err := ParseArgs([]string{
		"jobscout",
		"--ignored",
		"--fetch-dry-run",
		"--json",
	})
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if options.Command != CommandFetchDryRun {
		t.Fatalf("ParseArgs(...).Command = %q; want %q", options.Command, CommandFetchDryRun)
	}
	if len(options.CommandArgs) != 1 || options.CommandArgs[0] != "--json" {
		t.Fatalf("ParseArgs(...).CommandArgs = %#v; want []string{\"--json\"}", options.CommandArgs)
	}
}
