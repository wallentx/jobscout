# Contributing

This project uses the Makefile as the local contract. Before opening a pull
request, run:

```sh
make all
```

`make all` applies mechanical fixes, verifies modules, checks formatting and
imports, runs static analysis, runs tests, runs the race detector where the
platform supports it, and builds the CLI.

Use `make full-check` before a release when you also want the heavier security
checks:

```sh
make full-check
```

## Development

Requirements:

- Go 1.26.1 or newer
- `make`
- network access the first time the Makefile installs local Go tool helpers

Useful targets:

```sh
make fix
make check
make test
make build
make install
```

Runtime data is intentionally ignored by git. Normal app usage writes under
the OS-specific user config directory by default, and local runtime files such
as `config.yaml`, `SEARCH_PROMPT.md`, `jobscout.db`, and JSON exports should not
be committed.

Test fixtures live beside the packages that need them. For example, config
criteria coverage uses `internal/config/testdata/criteria-sample.yaml`.

## LLM Configuration

LLM features are optional. Do not commit provider tokens, local prompts,
databases, caches, or exported job data. Hosted provider tokens should be
passed through environment variables or commands, not literal config values.
