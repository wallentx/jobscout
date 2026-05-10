# LLM Features

LLM usage is optional. `jobscout` can run from RSS feeds, site-search targets,
and built-in source catalogs without provider credentials. When LLM features are enabled, the
app uses the configured provider only for the specific feature being run.

## Providers

`jobscout` supports these provider names:

- `gemini`
- `openai`
- `anthropic`
- `ollama`

Hosted providers usually need a provider token. Use an environment variable or
a command that prints the token. The setup flow does not store literal provider
tokens in config. Ollama uses a local endpoint by default.

## Current LLM Uses

1. Model discovery during setup

   After a provider and auth method are configured, setup calls the provider's
   model-list endpoint to show available models. If the request fails or returns
   unusable results, setup falls back to a curated static list and still allows
   manual model entry.

2. Resume-assisted setup

   During first-run setup, after provider auth succeeds and a provider model
   list is fetched, setup can ask whether you want to prefill search criteria
   from a resume. If you opt in, `jobscout` extracts text from the resume path
   you provide and sends that text to your configured LLM provider to infer
   starter criteria.

   Supported inputs include ASCII/UTF-8 text files, Markdown, JSON/YAML/CSV,
   DOCX, ODT, RTF, PDF when `pdftotext` or `mutool` is installed, and legacy DOC
   when `antiword` or `catdoc` is installed.

3. LLM job search

   When `llm.llm_job_search: true`, the fetch pipeline initializes the configured
   provider, reads `SEARCH_PROMPT.md` from the OS-specific user config
   directory, asks the LLM to return job postings as JSON, validates the returned
   jobs, and merges them with the regular configured sources. If LLM
   initialization or execution fails, the app records a notice and continues
   with non-LLM sources.

   LLM web-search sources are experimental and disabled during normal refreshes.
   Use `--sources llm_web` when intentionally testing that path.

4. Optional LLM job filtering

   When `llm.llm_job_filtering: true`, the app uses the configured LLM as a
   targeted ambiguity resolver after deterministic fetch filtering. Jobs with
   weak identity, weak descriptions, LLM-generated sources, or enough
   deterministic data to keep without review bypass the model. Only jobs that
   still need fit review are sent to the LLM.

   Matching reviewed jobs may get normalized compensation, remote eligibility,
   and `why_it_matches` details from the model. Reviewed jobs that the LLM marks
   as non-matches are dropped. If the LLM is unavailable, the original fetched
   jobs are preserved and a notice is shown.

5. Job identity enrichment

   When a job is missing company identity data, the deterministic enrichment
   path checks the apply page and related public profile pages. If the
   `job_identity` LLM task is configured, `jobscout` can ask the LLM to extract
   the company website, company summary, and industry from supplied page text.
   Deterministic parsing runs first. The LLM is called only when one of those
   identity fields is still missing or weak after parsing. Missing compensation
   alone does not trigger identity LLM work. LLM output still has to pass the
   same validation as deterministic output.

6. Optional Company Health review

   When LLM assistance is enabled for Company Health, the deterministic health
   check runs first, then the configured LLM summarizes the evidence into
   positive signals, concerns, and follow-up questions. If the LLM is
   unavailable, the deterministic Company Health report is still shown.

7. LLM benchmarks

   `jobscout --bench-llm` runs synthetic, public-safe benchmark cases against
   the configured provider/model. These calls use the real provider and may
   incur provider costs. Saved benchmark records are written under
   `benchmarks/` in the OS-specific user config directory.

   Current repeatable benchmark tasks cover Company Health summaries, job
   filtering, same-source batch filtering, job identity extraction from supplied
   page text, and resume-to-criteria extraction. Browser or live-search tasks
   should only be compared against models that can actually retrieve live data
   or use the required tools.

## Config Example

```yaml
llm:
  enabled: true
  provider: gemini
  llm_job_search: true
  llm_job_filtering: true
  llm_company_health: true
  providers:
    gemini:
      model: gemini-2.5-flash-lite
      models:
        llm_job_search: gemini-2.5-flash-lite
        llm_company_health: gemini-flash-lite-latest
        llm_job_filtering: gemini-2.5-flash-lite
        job_identity: gemini-2.5-flash-lite
        resume_to_criteria: gemini-2.5-flash-lite
      auth:
        mode: env
        env_var: GEMINI_API_KEY
    openai:
      model: gpt-4.1
      models:
        llm_job_search: gpt-4.1
        llm_company_health: gpt-4o-2024-11-20
        llm_job_filtering: gpt-4.1
        job_identity: gpt-4.1
        resume_to_criteria: gpt-5.3-chat-latest
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

Task-specific provider model entries are optional. Put them under the provider
they apply to, for example `llm.providers.gemini.models.llm_job_filtering`. When a task
model is missing, `jobscout` falls back to that provider's normal model.

Supported task keys currently include:

- `llm_job_search`
- `llm_job_filtering`
- `job_identity`
- `resume_to_criteria`
- `llm_company_health`
- `benchmark`

## Implementation Map

- Provider/model/auth setup: `internal/llm/llm_auth.go`,
  `internal/llm/llm_models.go`, and the setup flow in `internal/tuiapp`.
- Provider initialization: `llm.InitConfiguredLLMForTask`.
- Resume-assisted setup: `internal/llm/resume_criteria.go`.
- LLM job search: `fetcher.FetchAllJobs` when `llm.llm_job_search` is true;
  `internal/tuiapp/fetch_flow.go` wires in `llm.ExecuteLLMSearch`.
- LLM job filtering: `llm.FilterJobsWithLLM` from
  `internal/tuiapp/fetch_flow.go`; selection happens before provider
  initialization so unambiguous jobs bypass model calls.
- LLM job identity enrichment: `llm.EnrichJobIdentityWithLLM`, wired into
  `fetcher.FetchAllJobs` and repair flows.
- LLM Company Health review: `health.ApplyOptionalLLMCompanyHealth`, which
  calls `llm.EvaluateCompanyHealthWithLLM`.
- Setup preview and `--fetch-dry-run` reuse the same fetch/filter functions, so
  LLM behavior is consistent between preview, TUI fetch, and CLI dry runs.
