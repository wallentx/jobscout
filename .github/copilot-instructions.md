# Copilot instructions for `jobscout`

## Build, test, lint

This repo is Go (`go 1.26.1` per `go.mod`) and is intended to be driven via the `Makefile` targets.

**Common gates (preferred):**

- `make all` — run mechanical fixes then the full local merge gate (this is what CONTRIBUTING asks before opening a PR).
- `make check` — verify modules, formatting checks, `go mod tidy` check, static analysis, tests, race detector (skips on Android), and build.
- `make fix` — apply gofmt + goimports + `go mod tidy`.

**Individual targets:**

- `make test` — `go test -timeout 300s ./...`
- `make lint` — `go vet` + `staticcheck` + `golangci-lint` + `errcheck`
- `make build` — `go build` (injects version via `-X github.com/wallentx/jobscout/internal/jobscout.version=...`)
- `make full-check` — `make check` + `gosec` + `govulncheck`

**Run a single test:**

- By package + test name:
  - `go test ./internal/updatecheck -run '^TestCheckLatestReleaseDetectsAvailableUpdate$'`
  - `go test ./internal/config -run '^TestDefaultArtifacts$'`

**Tooling note:**

`make` will install repo-managed helper tools into `./.tools/bin` when missing (goimports/staticcheck/golangci-lint/errcheck/gosec/govulncheck). Prefer `make …` over requiring globally-installed linters.

## Pull request titles and bodies

When creating or editing PR titles and bodies, be direct and brief. Do not write
marketing copy, apology text, motivational language, or long narrative
summaries. Optimize for a maintainer who is scanning the PR list.

**Title:**

- Use one clear sentence fragment in imperative or noun style.
- Prefer a scope when it helps: `ci: classify PR release labels`,
  `docs: add benchmark sharing notes`, `fix: keep setup inputs editable`.
- Do not use vague titles like `updates`, `misc fixes`, `cleanup`, or
  `improvements`.

**Body:**

Use this shape by default, omitting sections that do not add useful information:

```md
Summary:
- What changed
- Why it changed

Verification:
- `make all`
- Any focused command or manual check that matters

Notes:
- Risks, follow-ups, or intentionally deferred work
```

Keep bullets concrete and easy to read. Mention changed commands, workflows,
config files, user-visible behavior, and test results. Do not restate every file
that changed unless the file list itself is important.

## High-level architecture (big picture)

The canonical overview is `docs/README.md` (includes a generated C4 component diagram and a package map). At a high level:

- **Entrypoint:** `cmd/jobscout/main.go` → `internal/jobscout.Run`.
- **Command dispatch:** `internal/jobscout` handles one-shot commands (e.g. `--fetch-dry-run`, `--export-json`, `--import`, `--bench-*`) and otherwise starts the **Bubble Tea** TUI (`internal/tuiapp`).
- **Runtime paths & stores:** `internal/runtime` resolves the OS user config directory paths (config, prompt, sqlite db) and opens either **SQLite-backed stores** or **in-memory stores** for `--demo`.
- **Config/setup:** `internal/config` loads/sanitizes `config.yaml`, criteria, source selection, and LLM provider config (and keeps experimental sources inert unless explicitly selected).
- **Fetch pipeline:** `internal/fetcher` resolves effective sources (built-in + user configured), fetches from:
  - RSS feeds
  - site-search targets (static parsing + a Rod-backed browser probe when needed)
  - configured APIs (currently `type: remotive`)
  - optional LLM job search / optional LLM web search
  Then normalizes jobs into a single shape, applies deterministic filtering/validation/deduping, enriches company identity when possible, and produces a reviewable summary.
- **LLM integration:** `internal/llm` provides provider auth/model discovery, task-specific LLM runners (search/filtering/identity/company health), and benchmark/report CLIs.
- **Company Health:** `internal/health` + `internal/domain` compute a deterministic score from evidence sources and (optionally) ask an LLM to summarize evidence; the LLM does not replace the deterministic score.
- **Storage:** `internal/storage` is SQLite (`modernc.org/sqlite`) with migrations; it stores jobs, health cache, and persistent company identities.
- **Update check:** `internal/updatecheck` optionally hits GitHub Releases at startup; can be disabled via `JOBSCOUT_DISABLE_UPDATE_CHECK=1`.

## Key repo conventions (non-obvious)

- **Repo-managed pre-commit hook:** most `make` targets run `scripts/git-hooks/install-pre-commit.sh` to install a managed `pre-commit` hook. The hook enforces `make check` unless a `.checks` stamp matches the current HEAD + tree; `make fix/check/all` write that stamp via `scripts/git-hooks/stamp-checks.sh`.
- **Runtime data stays out of git:** normal app usage writes under the OS user config directory (e.g. `config.yaml`, `SEARCH_PROMPT.md`, `jobscout.db`); `--demo` runs fully in-memory (no user config/db writes).
- **Do not persist provider tokens:** config save logic intentionally sanitizes auth—if a user enters a literal API key, it is not saved; auth is reset to env-var mode so secrets aren’t written to disk.
- **Experimental sources are opt-in:** `llm_web` and API sources are kept disabled during normal runs unless explicitly selected (e.g. via `--sources`), even if present in config.
- **Generated architecture diagram:** `docs/README.md` contains a generated mermaid block between `BEGIN/END GENERATED C4 COMPONENT VIEW`; update with `make c4-diagram` (and CI can check it with `make c4-diagram-check`).
