# jobscout

<img width="1520" height="2260" alt="1000035900" src="https://github.com/user-attachments/assets/b9e95064-b348-4293-8537-fd948b29d96c" />

`jobscout` is a terminal job-search tracker. It can run from RSS feeds, site-search targets, and built-in source catalogs without any LLM provider, and it can optionally use an LLM to expand or refine results.

## Install

Requires Go 1.26.1 or newer.

```sh
go install github.com/wallentx/jobscout/cmd/jobscout@latest
```

From a checkout, you can also build or install with the Makefile:

```sh
make build
make install
```

## Privacy

`jobscout` does not collect your information, run analytics, or send your data to the app author. Runtime data stays on your machine under the OS-specific user config directory unless you explicitly configure an LLM feature that sends the relevant prompt content to your selected provider.

On TUI startup, `jobscout` makes one unauthenticated request to GitHub releases
to check whether a newer version is available. The check is silent when it
fails or when the current build version cannot be compared. Set
`JOBSCOUT_DISABLE_UPDATE_CHECK=1` to disable it.

## Deterministic Mode

The app can fully function with, or without LLM enhancements. When fetching jobs, it uses configured sources such as RSS feeds, site-search targets, and built-in source catalogs. Company Health also uses deterministic source checks such as SEC, Wikipedia/Wikidata, Google News RSS, Hacker News, and stock-history signals.

Runtime files are created under the OS-specific user config directory by
default. Choose "No, use non-LLM sources only" during setup, or disable LLM
behavior in `config.yaml`:

```yaml
llm:
  enabled: false
  llm_job_filtering: false
  llm_job_search: false
  llm_company_health: false
```

## Optional LLM Enhancements

LLM usage is optional. When enabled, `jobscout` can use an LLM for model
discovery, resume-assisted setup, LLM job search, LLM job filtering, identity
enrichment, and Company Health review. Hosted providers should use environment
variables or commands for tokens; the setup flow does not store literal provider
tokens in config.

See [LLM features](docs/LLM_FEATURES.md) for provider setup, task-specific
models, and the current LLM integration map.

## Runtime Files

By default, `jobscout` writes runtime files under the OS-specific user config
directory:

- `config.yaml`
- `SEARCH_PROMPT.md`
- `jobscout.db`

Common locations are:

- macOS: `~/Library/Application Support/jobscout/`
- Linux and most Unix systems: `$XDG_CONFIG_HOME/jobscout/` or `~/.config/jobscout/`
- Windows: `%AppData%\jobscout\`

Personal config, prompts, databases, exports, and caches are ignored so normal
app usage does not dirty the checkout.

## Demo Mode

Run `jobscout --demo` to explore the app without reading or writing your normal
config, prompt, database, jobs, or health cache.

See [Demo mode](docs/DEMO_MODE.md) for the in-memory profile and LLM behavior.

## CLI Helpers

Most of these are only useful during development.
```sh
Usage:
  jobscout [options]
  jobscout [options] <command> [command options]

Options:
  --demo                  Run with in-memory demo data; read/write no user config or database
  -d, --debug             Show additional fetch and Company Health details
  --sources <list>        Use only selected fetch sources for this run: rss, site, llm, llm_web
                            llm_web is an opt-in experimental source
  --config <path>         Use an alternate config file
  -h, --help              Show this help
  -v, --version           Show version information

Commands:
  --fetch-dry-run [--json]       Fetch jobs without saving them
  --export-json [path|-]         Export saved jobs as JSON
  --import, -i                   Import jobs from stdin or editor JSON
  --delete-db                    Delete the SQLite database and exit
  --repair-job-identity          Repair missing company identity data
  --bench-llm                   Run LLM benchmark cases
  --bench-report                Summarize saved LLM benchmark results
```

## Development

Use the Makefile as the local release gate:

```sh
make all
```

See [Contributing](CONTRIBUTING.md) for development commands and
[Release](docs/RELEASE.md) for versioned build notes.
