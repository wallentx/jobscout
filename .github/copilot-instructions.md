# Copilot instructions for `jobscout`

## Build, test, lint

This repo is Go (`go 1.26.1` per `go.mod`) and should be driven through the `Makefile`.

Preferred gates:
- `make all` — run mechanical fixes, then the full local merge gate
- `make check` — verify modules, formatting checks, `go mod tidy` check, static analysis, tests, race detector (skips on Android), and build
- `make fix` — apply `gofmt`, `goimports`, and `go mod tidy`

Other useful targets:
- `make test` — `go test -timeout 300s ./...`
- `make lint` — `go vet` + `staticcheck` + `golangci-lint` + `errcheck`
- `make build` — `go build` with version injection via `-X github.com/wallentx/jobscout/internal/jobscout.version=...`
- `make full-check` — `make check` + `gosec` + `govulncheck`

Single-test examples:
- `go test ./internal/updatecheck -run '^TestCheckLatestReleaseDetectsAvailableUpdate$'`
- `go test ./internal/config -run '^TestDefaultArtifacts$'`

Tooling:
- Prefer `make ...` over globally installed tools
- `make` installs repo-managed helper tools into `./.tools/bin` when missing

## Pull request titles and bodies

When creating or editing PR titles and bodies, be direct and brief.

### PR titles

- Use one clear sentence fragment in imperative or noun style
- Prefer a scope when it helps: `ci: classify PR release labels`, `docs: add benchmark sharing notes`, `fix: keep setup inputs editable`
- Do not use vague titles like `updates`, `misc fixes`, `cleanup`, or `improvements`

### Copilot-generated PR summaries

These rules are mandatory unless the user explicitly asks for a different format.

- MUST use a terse bullet list only
- MUST NOT start with a prose paragraph, overview sentence, or preamble
- MUST NOT use section headings like `Summary`, `Testing`, `Verification`, `Notes`, or similar
- MUST keep the entire summary to 3–5 bullets total when possible
- MUST keep each bullet to one line when possible
- MUST use sentence fragments, not explanatory paragraphs
- MUST focus only on observable behavior changes, meaningful tests, and notable risk
- MUST avoid formal phrases like `This PR`, `This pull request`, `This change`, `This update`, or `The purpose of this PR`
- If the first draft is prose, rewrite it as bullets instead
- If there is little to say, prefer fewer bullets

Preferred shape:
- Short top-level behavior change
  - Key detail
  - Key detail
- Test coverage update

Good:
- Changes overlay close behavior in the TUI
  - Allows `Enter` and `Esc` to close job detail overlays
  - Allows `Enter` and `Esc` to close company health overlays
  - Updates overlay hints and tests for the new behavior

Bad:
- This pull request updates the overlay behavior...
- Summary:
- Testing:

## High-level architecture

Canonical overview: `docs/README.md`.

At a high level:
- Entrypoint: `cmd/jobscout/main.go` → `internal/jobscout.Run`
- Command dispatch: `internal/jobscout` handles one-shot commands (for example `--fetch-dry-run`, `--export-json`, `--import`, `--bench-*`) and otherwise starts the Bubble Tea TUI in `internal/tuiapp`
- Runtime paths and stores: `internal/runtime` resolves OS user config paths and opens either SQLite-backed stores or in-memory stores for `--demo`
- Config/setup: `internal/config` loads and sanitizes `config.yaml`, criteria, source selection, and LLM provider config
- Fetch pipeline: `internal/fetcher` resolves effective sources, fetches jobs from supported sources, normalizes them, filters/validates/dedupes them, and enriches company identity when possible
- LLM integration: `internal/llm` provides provider auth, model discovery, task-specific runners, and benchmark/report CLIs
- Company health: `internal/health` and `internal/domain` compute a deterministic score from evidence sources; optional LLM output supplements but does not replace deterministic scoring
- Storage: `internal/storage` is SQLite (`modernc.org/sqlite`) with migrations for jobs, health cache, and persistent company identities
- Update check: `internal/updatecheck` optionally hits GitHub Releases at startup; disable with `JOBSCOUT_DISABLE_UPDATE_CHECK=1`

## Repo conventions

- Prefer small, targeted changes; do not refactor unrelated code opportunistically
- Follow existing naming, package boundaries, and test patterns before introducing new structure
- Keep runtime data out of git; normal app usage writes under the OS user config directory, while `--demo` stays fully in memory
- Do not persist provider tokens; config save logic intentionally avoids writing literal API keys to disk
- Treat experimental sources as opt-in; keep `llm_web` and API sources inactive unless explicitly selected
- `docs/README.md` contains a generated Mermaid block between `BEGIN/END GENERATED C4 COMPONENT VIEW`; update it with `make c4-diagram` when needed
- Many `make` targets install the repo-managed pre-commit hook via `scripts/git-hooks/install-pre-commit.sh`; do not remove or bypass this workflow casually

## Change guidance

- Fix the root cause when practical, not just the immediate symptom
- Preserve user-visible behavior unless the change intentionally updates it
- Update tests whenever behavior changes or regressions are possible
- Prefer deterministic logic over heuristic or LLM-dependent behavior in core flows
- Keep optional/demo/in-memory paths working when touching runtime or storage code
- When changing CLI flags, config, or user-visible workflows, update related docs and help text

## Verification expectations

Before proposing work as complete, prefer the smallest relevant validation that gives confidence.

- For broad or risky changes, prefer `make check`
- For narrow changes, run the most relevant package tests first
- If you could not run validation, say so plainly
- Do not claim success without naming what was verified

## Response style

- Be concise by default
- Lead with the answer or result, then supporting detail if needed
- Prefer concrete file paths, commands, and observable effects over generalities
- When summarizing code changes, emphasize what changed, why it matters, and how it was verified
