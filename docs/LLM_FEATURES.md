# LLM Features

LLM support is optional. `jobscout` can run from RSS feeds, site-search targets,
and built-in source catalogs without provider credentials.

When LLM features are enabled, the app uses your configured provider only for
the feature being run.

## Providers

Supported provider names:

- `gemini`
- `openai`
- `anthropic`
- `openrouter`
- `ollama`

Hosted providers usually need a provider token. Use an environment variable or
a command that prints the token. The setup flow does not store literal provider
tokens in config. Ollama uses a local endpoint by default.

## What LLMs Can Do

- Discover available models during setup.
- Prefill search criteria from a resume you choose.
- Search for jobs from `SEARCH_PROMPT.md` when LLM job search is enabled.
- Review ambiguous job matches when LLM filtering is enabled.
- Fill missing company identity details from supplied page text.
- Summarize deterministic Company Health evidence.
- Run model benchmarks to compare quality, latency, token use, and failures.

LLM failures should not stop normal non-LLM fetching. The app records a notice
and continues with the deterministic sources that are available.

## Resume-Assisted Setup

During first-run setup, `jobscout` can read a resume file and ask your selected
LLM provider to infer starter search criteria.

Supported inputs include text, Markdown, JSON, YAML, CSV, DOCX, ODT, RTF, PDF
when `pdftotext` or `mutool` is installed, and legacy DOC when `antiword` or
`catdoc` is installed.

## Models

Setup fetches the provider's model list when it can. If that fails, it falls
back to a curated list and still allows manual model entry.

Provider model lists are filtered to avoid known incompatible, deprecated,
snapshot, and non-text models. When provider metadata exposes aliases, setup
shows the relationship and saves the runnable model ID.

For model comparisons, see [Benchmark Reports](BENCHMARKS.md).

<details>
<summary>Task-specific model config</summary>

Task-specific model entries are optional. When a task model is missing,
`jobscout` falls back to the provider's normal model.

```yaml
llm:
  enabled: true
  provider: gemini
  llm_job_search: true
  llm_job_filtering: true
  llm_company_health: true
  providers:
    gemini:
      model: gemini-3.1-flash-lite
      max_tokens: 4096
      models:
        llm_job_search: gemini-3.1-flash-lite
        llm_company_health: gemini-3.1-flash-lite
        llm_job_filtering: gemini-3.1-flash-lite
        job_identity: gemini-3.1-flash-lite
        resume_to_criteria: gemini-3.1-flash-lite
      auth:
        mode: env
        env_var: GEMINI_API_KEY
    openai:
      model: gpt-4o-mini
      max_tokens: 4096
      models:
        llm_job_search: gpt-4o-mini
        llm_company_health: gpt-4o-mini
        llm_job_filtering: gpt-5.4-mini
        job_identity: gpt-4o-mini
        resume_to_criteria: gpt-4o-mini
      auth:
        mode: env
        env_var: OPENAI_API_KEY
    anthropic:
      model: claude-3-5-haiku-latest
      auth:
        mode: env
        env_var: ANTHROPIC_API_KEY
    ollama:
      model: llama3
      endpoint: http://localhost:11434
      auth:
        none: true
```

Supported task keys:

- `llm_job_search`
- `llm_job_filtering`
- `job_identity`
- `resume_to_criteria`
- `llm_company_health`

`max_tokens` is an optional provider-level ceiling for generated output. It
caps every LLM task for that provider without raising lower per-call limits.

</details>

<details>
<summary>How each LLM feature is used</summary>

- Model discovery runs after a provider and auth method are configured.
- Resume-assisted setup sends extracted resume text to your configured provider
  only when you opt in.
- LLM job search reads `SEARCH_PROMPT.md`, asks the LLM for jobs as JSON,
  validates the returned jobs, and merges them with regular sources.
- LLM filtering runs after deterministic fetch filtering and only reviews jobs
  that still need fit review.
- Job identity enrichment calls the LLM only when company identity fields are
  missing or weak after deterministic parsing.
- Company Health runs deterministic checks first, then asks the LLM to summarize
  the gathered evidence when enabled.
- Benchmarks use synthetic, public-safe cases against the real provider and may
  incur provider costs.

LLM web-search sources are experimental and disabled during normal refreshes.
Use `--sources llm_web` when intentionally testing that path.

</details>

<details>
<summary>Developer reference</summary>

- Provider/model/auth setup: `internal/llm/llm_auth.go`,
  `internal/llm/llm_models.go`, and the setup flow in `internal/tuiapp`.
- Provider initialization: `llm.InitConfiguredLLMForTask`.
- Resume-assisted setup: `internal/llm/resume_criteria.go`.
- LLM job search: `fetcher.FetchAllJobs` when `llm.llm_job_search` is true;
  `internal/tuiapp/fetch_flow.go` wires in `llm.ExecuteLLMSearch`.
- LLM job filtering: `llm.FilterJobsWithLLM` from
  `internal/tuiapp/fetch_flow.go`.
- LLM job identity enrichment: `llm.EnrichJobIdentityWithLLM`, wired into
  `fetcher.FetchAllJobs` and repair flows.
- LLM Company Health review: `health.ApplyOptionalLLMCompanyHealth`, which
  calls `llm.EvaluateCompanyHealthWithLLM`.

</details>
