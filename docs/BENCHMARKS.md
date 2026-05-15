# Benchmark Reports

Use benchmarks when choosing an LLM model for `jobscout`. The benchmark runner
uses synthetic, public-safe cases and saves local records. The report command
turns those records into task-by-task recommendations.

## Quick Use

```sh
jobscout --bench-llm
jobscout --bench-report --latest
```

To compare every discoverable model for the configured provider:

```sh
jobscout --bench-llm --all-models
jobscout --bench-report --latest
```

Saved records are written to `benchmarks/llm-bench-*.jsonl` under the
`jobscout` user config directory.

## How To Read A Report

Start with:

- **Task winners**: best model by task, plus latency, token use, and cost when
  cost data exists.
- **Not recommended models**: models that failed or scored poorly for a task.
- **Error summary**: repeated provider or parsing failures.
- **Task details**: the full per-model table when you need to inspect tradeoffs.

Use `--format md` when you want Markdown tables for issues, discussions, or
shared reports:

```sh
jobscout --bench-report --latest --format md > jobscout-bench-report.md
```

## Published Results

These checked-in reports show benchmark data used to choose initial defaults.
Provider lifecycle guidance still takes precedence over raw benchmark scores
when a provider documents that a model is deprecated or replaced.

| Provider | Report | Notes |
| --- | --- | --- |
| Google Gemini | [Google Gemini 2026-05-15](benchmarks/google-20260515-063703.md) | Full `--all-models` run used to evaluate Gemini defaults alongside Google model lifecycle guidance. |
| OpenAI | [OpenAI 2026-05-15](benchmarks/openai-20260515-042118.md) | Full `--all-models` run used to choose the initial OpenAI defaults and task recommendations. |

## Share Results

When sharing benchmark results, include:

- `jobscout --version`
- the command used to run the benchmark
- the text from `jobscout --bench-report --latest`
- the JSON output from `jobscout --bench-report --latest --json` only when
  deeper inspection is useful

Benchmark cases do not include provider credentials, but saved records contain
model output and timing/token metadata. Review JSON before sharing.

<details>
<summary>Benchmark options and scoring details</summary>

Useful benchmark options:

- `--list`: list embedded benchmark cases.
- `--task <task|case>`: run only one benchmark task or case ID.
- `--provider <name>`: run with a specific provider.
- `--model <name>`: run with a specific model.
- `--all-models`: run every discoverable, deduplicated model for the configured
  provider.
- `--json`: print the records from this run after saving them.

Examples:

```sh
jobscout --bench-llm --task llm_job_filtering --provider gemini --model gemini-3.1-flash-lite
jobscout --bench-report --json > jobscout-bench-records.json
```

Current synthetic cases cover supplied-snippet job search, job filtering,
same-source batch filtering, job identity extraction, Company Health summaries,
and resume parsing across multiple resume structures.

Benchmark scoring uses accuracy, JSON validity, grounding, speed, cost, and
stability categories. Cases can also define hallucination patterns; matching
one of those patterns subtracts accuracy points and caps the final score so
invented companies, URLs, or unsupported facts cannot win a task.

Reports include a "Not recommended models" section when a model fails at least
one run for a task or its average successful score for that task falls below
75.0. These advisories do not change the raw scores.

For OpenAI records, `jobscout` estimates USD cost from token usage with a
built-in pricing table sourced from OpenAI API pricing and last reviewed on
2026-05-15. Other providers may show `avgUSD` as `n/a` unless the provider
reports cost data or `jobscout` has a pricing table for that provider. Treat
estimated costs as report guidance and verify current provider pricing before
making cost-sensitive decisions.

</details>
