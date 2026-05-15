# jobscout

![jobscout terminal UI](docs/assets/jobscout-demo.gif)

`jobscout` is a terminal job-search tracker. It finds jobs from configured
sources, stores them locally, and helps review company fit from the terminal.

It works without an LLM provider. Optional LLM features can help with search,
filtering, resume-assisted setup, and company summaries.

## Quick Start

Install with Go 1.26.1 or newer:

```sh
go install github.com/wallentx/jobscout/cmd/jobscout@latest
```

Run the app:

```sh
jobscout
```

Try it without touching your normal config or database:

```sh
jobscout --demo
```

See [Demo Mode](docs/DEMO_MODE.md) for what the demo includes.

<details>
<summary>Build from a checkout</summary>

```sh
make build
make install
```

For contributor checks, see [Contributing](CONTRIBUTING.md).

</details>

## Privacy

`jobscout` does not collect your information, run analytics, or send your data
to the app author. Runtime data stays on your machine unless you enable an LLM
feature that sends the relevant prompt content to your selected provider.

<details>
<summary>Runtime files and update checks</summary>

By default, `jobscout` writes runtime files under the OS-specific user config
directory:

- `config.yaml`
- `SEARCH_PROMPT.md`
- `jobscout.db`

Common locations are:

- macOS: `~/Library/Application Support/jobscout/`
- Linux and most Unix systems: `$XDG_CONFIG_HOME/jobscout/` or `~/.config/jobscout/`
- Windows: `%AppData%\jobscout\`

On TUI startup, `jobscout` makes one unauthenticated request to GitHub releases
to check whether a newer version is available. The check is silent when it
fails or when the current build version cannot be compared. Set
`JOBSCOUT_DISABLE_UPDATE_CHECK=1` to disable it.

</details>

## LLM Features

LLM features are optional. During setup, choose non-LLM mode if you want all job
fetching and filtering to stay deterministic.

When enabled, LLM features can assist with:

- resume-assisted setup
- LLM job search
- LLM job filtering
- company identity enrichment
- company health summaries
- model benchmarks

Hosted provider tokens should be supplied through environment variables or
commands. The setup flow does not store literal provider tokens in config.

See [LLM Features](docs/LLM_FEATURES.md) for provider setup and model options.
See [Benchmark Reports](docs/BENCHMARKS.md) when choosing a model.

## Common Commands

```sh
jobscout                       # open the TUI
jobscout --demo                # try the app with in-memory demo data
jobscout --fetch-dry-run       # fetch without saving
jobscout --export-json jobs.json
jobscout --import < jobs.json
jobscout --help
jobscout --version
```

<details>
<summary>Full command-line help</summary>

```sh
jobscout is a terminal job-search tracker.

Usage:
  jobscout [options]
  jobscout [options] <command> [command options]

Options:
  --demo                  Run with in-memory demo data; read/write no user config or database
  -d, --debug             Show additional fetch and Company Health details
  --sources <list>        Use selected active fetch sources: rss, site, llm, llm_web, all
  --sources=<list>        Same as --sources <list>
                            llm_web is an opt-in experimental source
  --config <path>         Use an alternate config file
  --config=<path>         Same as --config <path>
  -h, --help              Show this help
  -v, --version           Show version information

Commands:
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
```

</details>

## More Documentation

- [LLM Features](docs/LLM_FEATURES.md)
- [Benchmark Reports](docs/BENCHMARKS.md)
- [Demo Mode](docs/DEMO_MODE.md)
- [Contributing](CONTRIBUTING.md)
