# Contributing

Before opening a pull request, run:

```sh
make all
```

`make all` formats the code, verifies modules, runs static analysis, runs
tests, runs the race detector where the platform supports it, and builds the
CLI.

Most `make` targets silently install the repository's managed pre-commit hook
when it is missing. The hook accepts a current local check stamp from `make fix`,
`make check`, or `make all`; if the stamp is missing or stale, it runs
`make check` before allowing the commit. Use `git commit --no-verify` only when
you intentionally need to bypass local checks.

## Requirements

- Go 1.26.1 or newer
- `make`
- network access the first time the Makefile installs local Go tool helpers

## Useful Commands

```sh
make fix
make check
make test
make build
make install
```

Use `make full-check` before a release when you also want heavier security
checks.

## What Not To Commit

Runtime data is intentionally ignored by git. Do not commit provider tokens,
local prompts, databases, caches, exported job data, or local benchmark record
files.

Normal app usage writes under the OS-specific user config directory by default.
Test fixtures live beside the packages that need them, such as
`internal/config/testdata/criteria-sample.yaml`.

See [Benchmark Reports](docs/BENCHMARKS.md) for benchmark output guidance.
