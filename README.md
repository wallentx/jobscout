# jobscout

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

## Non-LLM Mode

The app can function without LLM access. In non-LLM mode it uses configured sources such as RSS feeds, site-search targets, and built-in source catalogs. Company Health also uses deterministic source checks such as SEC, Wikipedia/Wikidata, Google News RSS, Hacker News, and stock-history signals.

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

```sh
jobscout --demo
jobscout --help
jobscout --version
jobscout -v
jobscout --fetch-dry-run
jobscout --fetch-dry-run --json
jobscout --export-json jobs-export.json
jobscout --import < jobs-export.json
```

Use `--config` to point at an alternate config file, and `--debug` for
additional fetch and Company Health details. `--import` reads JSON from stdin;
when run interactively without stdin, it opens `$EDITOR` for JSON entry.

LLM web-search sources are inert during normal refreshes. Use
`--sources llm_web` when intentionally testing that path.

## Development

Use the Makefile as the local release gate:

```sh
make all
```

See [Contributing](CONTRIBUTING.md) for development commands and
[Release](docs/RELEASE.md) for versioned build notes.
