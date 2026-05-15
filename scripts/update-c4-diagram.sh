#!/bin/sh
set -eu

mode=write
target=docs/README.md

case "${1:-}" in
	--check)
		mode=check
		shift
		;;
esac

if [ "${1:-}" != "" ]; then
	target=$1
fi

begin='<!-- BEGIN GENERATED C4 COMPONENT VIEW -->'
end='<!-- END GENERATED C4 COMPONENT VIEW -->'

if ! grep -Fxq "$begin" "$target"; then
	printf 'missing generated diagram marker: %s\n' "$begin" >&2
	exit 1
fi
if ! grep -Fxq "$end" "$target"; then
	printf 'missing generated diagram marker: %s\n' "$end" >&2
	exit 1
fi

tmpdir=$(mktemp -d "${TMPDIR:-/tmp}/jobscout-c4.XXXXXX")
trap 'rm -rf "$tmpdir"' EXIT

block=$tmpdir/c4.md
updated=$tmpdir/README.md

cat >"$block" <<'EOF'
```mermaid
C4Component
title jobscout component view

Person(user, "Job seeker", "Runs jobscout in a terminal")
System_Ext(jobSources, "Job sources", "RSS feeds, job boards, configured source targets")
System_Ext(llmProviders, "LLM providers", "Gemini, OpenAI, Anthropic, OpenRouter, Ollama")
System_Ext(healthSources, "Company health data", "Wikipedia/Wikidata, SEC, Google News RSS, Hacker News, stock history, Layoffs.fyi, company and review pages")
System_Ext(githubReleases, "GitHub Releases", "Optional update check")

System_Boundary(jobscout, "jobscout") {
  Component(entry, "CLI entrypoint", "cmd/jobscout + internal/jobscout", "Parses commands, configures runtime, starts the TUI or one-shot commands")
  Component(runtime, "Runtime", "internal/runtime", "Resolves config, prompt, and SQLite paths")
  Component(config, "Config and setup", "internal/config + internal/setup", "Loads defaults, criteria, provider settings, and source selection")
  Component(tui, "Terminal UI", "internal/tuiapp + internal/tui + internal/cliui", "Main job tracker, setup flow, fetch review, and command output")
  Component(fetcher, "Fetch pipeline", "internal/fetcher", "Resolves source catalogs, fetches jobs, filters, validates, deduplicates, and enriches identity")
  Component(llm, "LLM adapter", "internal/llm", "Provider auth, model discovery, optional LLM search/filtering/enrichment, and benchmarks")
  Component(health, "Company Health", "internal/health + internal/domain", "Loads cached health, gathers deterministic signals, scores risk, and optionally asks an LLM to summarize")
  Component(domain, "Domain model", "internal/domain", "Jobs, criteria, role families, company identity, scoring rules, and merge logic")
  ComponentDb(store, "Local storage", "internal/storage", "SQLite job, health, and company identity stores")
  Component(update, "Update check", "internal/updatecheck", "Checks the latest GitHub release at startup unless disabled")
EOF

known='cliui config domain fetcher health jobscout llm runtime setup storage tui tuiapp updatecheck'
for dir in internal/*; do
	[ -d "$dir" ] || continue
	name=${dir#internal/}
	case " $known " in
		*" $name "*) continue ;;
	esac
	id=$(printf '%s' "$name" | tr -c '[:alnum:]_' '_')
	printf '  Component(%s, "Internal package: %s", "internal/%s", "Generated placeholder for an unclassified internal package")\n' "$id" "$name" "$name" >>"$block"
done

cat >>"$block" <<'EOF'
}

Rel(user, entry, "Runs commands")
Rel(entry, runtime, "Gets runtime paths")
Rel(entry, config, "Loads app config")
Rel(entry, tui, "Starts interactive app")
Rel(tui, fetcher, "Requests job refreshes")
Rel(fetcher, config, "Reads criteria and source settings")
Rel(fetcher, jobSources, "Fetches RSS, configured APIs, and site-search pages")
Rel(fetcher, llm, "Uses optional LLM search, filtering, and identity enrichment")
Rel(llm, llmProviders, "Calls configured provider")
Rel(tui, health, "Requests Company Health")
Rel(health, healthSources, "Collects deterministic evidence")
Rel(health, llm, "Optionally summarizes evidence")
Rel(tui, store, "Reads and writes jobs")
Rel(fetcher, store, "Checks existing jobs and identity cache")
Rel(health, store, "Reads and writes health cache")
Rel(update, githubReleases, "Checks latest release")
```
EOF

awk -v begin="$begin" -v end="$end" -v block="$block" '
	$0 == begin {
		print
		while ((getline line < block) > 0) {
			print line
		}
		close(block)
		skip = 1
		next
	}
	$0 == end {
		skip = 0
		print
		next
	}
	!skip {
		print
	}
' "$target" >"$updated"

if cmp -s "$target" "$updated"; then
	printf 'C4 diagram is up to date: %s\n' "$target"
	exit 0
fi

if [ "$mode" = check ]; then
	printf 'C4 diagram is out of date: %s\n' "$target" >&2
	diff -u "$target" "$updated" >&2 || true
	exit 1
fi

cp "$updated" "$target"
printf 'Updated C4 diagram: %s\n' "$target"
