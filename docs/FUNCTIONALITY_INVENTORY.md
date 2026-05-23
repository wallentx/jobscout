# Functionality Inventory

This is a mechanical inventory of what Jobscout currently supports in code. It
is not marketing copy. Use it as source material when writing release notes,
submission text, and user-facing feature descriptions.

Last reviewed: 2026-05-23.

## Setup And Criteria

- First run opens setup automatically when config, criteria, prompt, or LLM auth
  needs attention.
- Users can reopen configuration later with `c`.
- Criteria profile fields include city, state, country code, years of
  experience, role families, required title prefixes, target title names,
  excluded title terms, work settings, minimum base USD, and priority signals.
- Role families are frontend, backend, fullstack, DevOps/SRE/systems, AI/ML,
  data, design, product management, and other specialized.
- Work settings are remote, hybrid, and on-site.
- Setup can review and edit the generated `SEARCH_PROMPT.md`.
- Setup can run a preview fetch before saving.
- Resume-assisted setup extracts resume text and asks the configured LLM to
  prefill criteria.
- Resume inputs include text-like files, Markdown, JSON, YAML, CSV, DOCX, ODT,
  RTF, PDF with `pdftotext` or `mutool`, and legacy DOC with `antiword` or
  `catdoc`.
- LLM setup supports Gemini, OpenAI, Anthropic, OpenRouter, and Ollama.
- LLM auth supports environment-variable and command-based auth in setup.
  Literal token mode exists for legacy/config compatibility.
- Model setup discovers provider models when possible, filters unsupported,
  deprecated, snapshot, and non-text models, shows aliases where metadata
  exists, and allows manual entry.
- Per-task model overrides exist for job search, job filtering, job identity,
  Company Health, and resume-to-criteria.

## Job Sources And Fetching

- Fetch groups are `rss`, `site`, `llm`, `llm_web`, and `api`.
- `rss` reads built-in catalog feeds plus user-configured feeds.
- `site` uses direct browser-backed site search targets plus the built-in site
  catalog.
- `llm` reads `SEARCH_PROMPT.md` and asks the configured model for JSON jobs.
- `llm_web` is an experimental provider-backed `site:` web-search path and is
  disabled by default.
- `api` reads configured structured APIs. Currently only Remotive is
  implemented and APIs are disabled unless selected.
- Built-in RSS sources are Remotive, We Work Remotely, and Real Work From
  Anywhere, with role-family-specific feeds.
- Built-in site sources are Indeed, LinkedIn, Y Combinator, Kube Careers, Built
  In Remote, Built In general, and Built In regional sites.
- Default configured site targets are Indeed, LinkedIn, Y Combinator, and Built
  In Remote.
- Default `llm_web` targets cover Greenhouse, Lever, Workday, Ashby,
  SmartRecruiters, iCIMS, and BambooHR via `site:` queries.
- Criteria affects source selection: role families choose RSS/specialty sources,
  Kube Careers requires DevOps/SRE/systems, and Built In regional targets are
  selected from candidate location and work settings.
- Site source order is randomized, site candidate evaluation is capped per
  source, and accepted fetch results can be capped.
- Fetching validates URLs, filters weak/non-job/listing results, skips saved
  duplicates, dedupes fetched results, and shows a fetch review before saving.
- Optional LLM job filtering runs after deterministic filtering and before
  review.
- Accepted jobs trigger background enrichment after save.

## Job And Company Data

- Stored job fields are company, company website, company summary, company
  industry, identity evidence, title, remote, compensation, source, apply URL,
  match reasons, status, date added, and description.
- Import accepts JSON aliases such as `company_name`, `company_url`, `website`,
  `about_company`, `industry`, `job_title`, `url`, `link`, `applyLink`,
  `salary`, and `pay`.
- Duplicate merge key is company plus title. Duplicate imports and fetches can
  fill missing website, summary, industry, compensation, and description.
- Company identity evidence stores source, URL, confidence, provisional flag,
  and reason for website, summary, and industry.
- Enrichment can parse JSON-LD `JobPosting`, known job boards, compensation
  text, company website links, summaries, industry labels, inferred industry,
  and page text.
- Board-specific enrichment exists for Built In, Y Combinator, Greenhouse, We
  Work Remotely, and Real Work From Anywhere.
- Public profile lookup can use LinkedIn, Glassdoor, and Indeed profile pages
  for health/profile evidence.
- Job detail view shows company, website, summary, industry, title, remote,
  compensation, source, apply URL, and match notes.
- `o` opens the job posting URL, not the company website.

## Company Health

- `h` on a selected job shows cached health if available, otherwise fetches it.
- `H` refreshes health for all unique jobs and clears the health cache first.
- In a health window, `h` refreshes the currently displayed company from
  scratch.
- `:health <company> [--aka <name>] [--website <url>]` fetches health for an
  arbitrary company.
- Health cache keys prefer company website domain, then company name.
- Freshness is 24 hours for volatile results and 7 days otherwise.
- Volatile means layoffs, employment risk, negative news, or SEC risk flags.
- Evidence sources include job identity, Wikipedia/Wikidata, RDAP/domain age,
  SEC EDGAR, company/profile pages, Hacker News, Google News RSS, Layoffs.fyi,
  Yahoo Finance stock history, and employer reviews.
- Health score starts at 50 and clamps to 0-100.
- Positive signals include company age, recent SEC filings, exchange listing,
  private-company age, clean/positive news, positive Hacker News signals, and
  strong employer reviews.
- Negative signals include SEC risk terms, negative news, negative Hacker News
  signals, stock distress, employer review concerns, and employment risk.
- Layoffs feed employment risk rather than directly double-penalizing the score.
- Employment risk is separately scored 0-100 as Low, Medium, High, or Critical.
- High and Critical employment risk cap the visible health score.
- Optional LLM Company Health summarizes deterministic evidence into positives,
  concerns, recommendation, risk level, and follow-up questions. When recent
  concern articles are available, it can review related article text and apply a
  constrained score modifier for novel evidence.
- Health UI can show score, risk, stock chart, notices, LLM review, company
  info, layoffs, Hacker News signals, employer reviews, rejected evidence,
  flags, notes, and debug evidence.

## Board Workflow

- Main table shows company, title, status, date, and health/status marker.
- Sort keys are `1` health, `2` company, `3` title, `4` status, and `5` date.
  Pressing the same sort key toggles direction.
- `/` searches company and title.
- Statuses are Unopened, Viewed, Applied, Interviewing, Rejected, Ignore, and
  Expired.
- `s` changes status.
- `m` marks the selected job viewed.
- `f` filters by status and can save default filter statuses.
- `D` deletes the selected job.
- `E` opens the selected job JSON in `$EDITOR`, defaulting to `nano`.
- `u` in detail view updates missing fields for that job.
- `U` updates missing fields across saved jobs.
- `V` checks Unopened and Viewed posting URLs and marks inactive postings
  Expired.
- `l` opens the health-symbol legend.
- `?` opens the key legend.
- `:` opens operator command mode with `debug`, `health`, and `help`.
- `t` expands or minimizes active background task, fetch, and health overlays.

## CLI And Utilities

- `jobscout` opens the TUI.
- `--demo` uses in-memory config, prompt, jobs, health, and identity storage.
- `--debug` writes additional fetch and Company Health logs. Browser fetches
  can also save page snapshots under `debug-pages/`.
- `--sources <list>` selects active fetch groups from `rss`, `site`, `llm`,
  `llm_web`, and `all`.
- `--candidate-limit <n>` caps site candidates evaluated per source.
- `--accepted-limit <n>` caps accepted jobs returned from a fetch.
- `--fetch-dry-run [--json]` fetches without saving.
- `--export-json [path|-]` exports saved jobs.
- `--import <path>` imports jobs from a JSON file only.
- `--delete-db` removes SQLite DB and sidecar files.
- `--repair-job-identity` enriches missing identity fields and can mark
  unusable jobs Expired.
- `--bench-llm` runs LLM benchmark cases.
- `--bench-report` summarizes benchmarks with text, Markdown, or JSON output.
- `--version` prints the app version.
- On startup, the app checks the latest GitHub release unless
  `JOBSCOUT_DISABLE_UPDATE_CHECK` is set.

## Current Gaps And Follow-Ups

- `api` is accepted by `--sources`, but current help/error text omits it.
- Docs use a coarser health band model than the live TUI legend.
- Employee range, headquarters, revenue, review metrics, and public profile URLs
  are health/profile data, not normal job-detail fields.
- The persistent company identity cache is internal and not directly exposed in
  the TUI.

## Commit Guidance

This file is useful if the project wants a public, auditable feature inventory
or a stable source for marketing copy. If it is only a temporary release-prep
scratchpad, leave it uncommitted or move it to a local ignored note.
