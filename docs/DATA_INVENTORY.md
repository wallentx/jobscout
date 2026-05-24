# Data Inventory

This is a working inventory of the data `jobscout` already renders, searches,
scores, stores, or should preserve when it is cheaply available from a page the
app already fetched.

The goal is not to scrape every possible field. The goal is to avoid throwing
away useful company, job, source, or evidence facts that can make later
enrichment and Company Health checks cheaper and more accurate.

## Capture Rule

When a source adapter, browser pass, structured-data parser, or LLM result has
already paid the cost to visit a URL, keep any fact that can later help one of
these tasks:

- render clearer job or company details
- match, filter, sort, or deduplicate jobs
- resolve company identity
- narrow a search query
- reject unrelated health evidence
- score health or employment risk
- explain why a result was accepted, rejected, or uncertain

Do not preserve raw page noise as first-class fields. If the value is not useful
for a task above, keep it out of the durable model.

## Durable Shapes Today

### Jobs

Stored by `internal/domain/job.go` and `internal/storage/sqlite_store.go`.

| Field | Used For | Notes |
| --- | --- | --- |
| `company` | table, details, dedupe, search, health identity | Required. |
| `title` | table, details, dedupe, search, filtering | Required. |
| `company_website` | details, health cache key, health evidence filtering, profile lookup | Most important company identity field. |
| `company_summary` | details, health evidence filtering, LLM health context | Useful context, but can be polluted by bad source text. |
| `company_industry` | details, deterministic filters, health evidence filtering | Currently a single string; many sources expose multiple industries. |
| `company_identity` | identity evidence for website, summary, industry | Rigid typed buckets today. |
| `remote` | table/detail text, deterministic filters | Source labels vary. |
| `compensation` | details, minimum salary filter, enrichment target | Free-form string. |
| `source` | details, fetch reporting, debugging | Human-readable source label. |
| `apply_url` | details, open URL, posting validation, dedupe guard | Usually posting URL; external apply URL is not preserved separately. |
| `why_matches` | details, LLM search/filter explanation | Optional. |
| `status` | table, filters, status actions, posting validation scope | Values are fixed in TUI. |
| `date_added` | table date, sorting | Often fetch/import time, not source posted date. |
| `date_discovered` | import compatibility only | Not stored separately. |
| `description` | filtering, enrichment, import/export | Often compressed source context, not full job text. |

### Company Identity

Stored by `internal/domain/company_identity.go` and the
`company_identities` tables.

| Field | Used For | Notes |
| --- | --- | --- |
| `key` | canonical identity key | Domain key when possible. |
| `display_name` | canonical display | Can differ from job row company. |
| `website` | cache key, health context, identity lookup | Should be official company domain only. |
| `summary` | health context | Helps reject same-name companies. |
| `industry` | health context | Should support multi-industry context. |
| `identity_evidence` | raw JSON evidence | Flexible storage exists, but normal code usually reads only website/summary/industry evidence. |
| `source_version` | invalidation/versioning | Useful when extraction rules change. |
| `created_at`, `updated_at` | cache freshness/debugging | Not user-facing today. |
| `name_aliases` | identity lookup, health searches | Loaded from lookup table. |
| `domain_aliases` | identity lookup | Loaded from lookup table. |

### Job Candidates

Stored by `internal/domain/job_candidate.go` and the `job_candidates` tables.
These rows are not shown on the board unless the fetch flow accepts them.

| Field | Used For | Notes |
| --- | --- | --- |
| `candidate_key` | candidate lookup, decision lookup, dedupe | Stable source identity from URL or company/title. |
| `source`, `source_key` | debug/source reporting | Human-readable source and source identity. |
| `apply_url`, `canonical_apply_url` | source reuse, posting checks | Keeps the discovered posting URL separate from normalized lookup. |
| `company`, `title` | lookup/debug | Mirrors the normalized job shape. |
| `job_json` | source-data reuse | Stores normalized candidate data so later fetches can reuse known fields. |
| `active` | posting lifecycle | Allows later active checks to mark stale candidates without erasing evidence. |
| `first_seen`, `last_seen` | pruning/freshness | Candidate cache retention is configurable. |

### Candidate Decisions

Stored by `internal/domain/job_candidate.go` and the
`job_candidate_decisions` table.

| Field | Used For | Notes |
| --- | --- | --- |
| `candidate_key` | candidate lookup | References the candidate row. |
| `criteria_hash` | cache invalidation | Criteria edits naturally produce different decisions. |
| `stage`, `decision_version` | matching invalidation | Separates deterministic filters from LLM filtering and lets rules change safely. |
| `llm_provider`, `llm_model` | LLM decision invalidation | LLM decisions are reused only for the same provider/model path. |
| `decision`, `reason` | fetch skipping/reporting | Rejected candidates can be skipped before repeating fit work. |
| `job_json` | audit/debug context | Captures the job shape that produced the decision. |
| `decided_at`, `expires_at` | freshness | Expiry is available for future decision TTLs. |

### Company Health

Stored as `health_cache.payload_json`, so it can already retain rich structured
data without schema changes.

| Field | Used For | Notes |
| --- | --- | --- |
| `company` | health title/cache display | Input or resolved company name. |
| `score`, `confidence` | table marker, health report, sorting | Score and confidence are separate. |
| `public` | health report/scoring | Derived from SEC lookup. |
| `founded_year`, `age_years` | health report/scoring | Wikipedia, Wikidata, domain age, company site, public profiles. |
| `estimated_employees` | health report/scoring | Used to scale layoff risk. |
| `signals_used` | debug/explainability | Not shown prominently. |
| `flags`, `notes`, `notices` | health report | User-visible explanation. |
| `discovered_ticker`, `discovered_name` | health report, SEC/stock lookup | Useful input for future health passes. |
| `layoff_signals` | employment risk, report | Title, URL, date, employee count, percentage. |
| `hn_signals` | sentiment/risk, report | Title, URL, points, comments, date. |
| `employer_reviews` | score/risk, report | Source, rating, review count, recommend, CEO approval, snippet, URL. |
| `employment_risk` | score cap, report | Level, score, factors, last layoff date. |
| `llm_assessment` | health report | Summary, recommendation, risk level, positives, concerns, follow-up questions, token usage. |
| `rejected_evidence` | debug/report explainability | Critical for showing why bad evidence was ignored. |
| `sources` | raw structured source payloads | Flexible bucket for stock history, SEC, company site, public profiles, news titles, identity, etc. |
| `field_assessments` | debug evidence | Tracks evidence candidates for facts such as employees and founded year. |

## Data Used By Search And Scoring

These are the inputs that determine how expensive downstream work is.

| Input | Current Use | Current Gap |
| --- | --- | --- |
| company name | all source labels, health searches, SEC, Wikipedia, HN, layoffs | Same-name collisions are common. |
| aliases | Google News filtering/search, health command input, identity cache | HN and layoff searches do not search aliases. |
| official website/domain | cache key, identity resolution, public profile searches, evidence acceptance | Some adapters only keep it as evidence URL, not as reusable source context. |
| summary | health context terms, LLM prompts | Bad summaries can poison matching. |
| industry | filters, health conflict rejection, LLM prompts | Single-value field loses important context like fintech/payments/crypto. |
| ticker | SEC and stock history | Jobs do not store it; health can rediscover it. |
| public/private status | health report/scoring | Stored only in health result. |
| founded year | age score, report | Stored only in health result. |
| employee count/range | layoff scaling, report | Stored only in health result/public profile structs. |
| review ratings/counts | health score/risk | Stored only in health result. |
| source profile URL | profile enrichment, evidence trace | Often discarded or only embedded in evidence URL. |
| source posted date | date sorting/freshness | Often discarded; `date_added` is not the same fact. |
| source expiration/valid-through | posting validation/freshness | Structured JobPosting parser does not preserve it. |
| external apply URL | opening the actual application page | Built In detail can see `howToApply`, but keeps Built In URL as `apply_url` and normalizes `howToApply` into website. |
| location/workplace | filtering, display | Stored as `remote` string, not structured locations. |
| skills | fit filtering, detail display | Built In listing compresses top skills into description. |
| full job description | filtering, display, enrichment | Detail parsing does not replace a short listing description. |

## Source Adapter Inventory

### Built In

Current listing extraction captures company, title, Built In job URL, work
setting, compensation, short description, one industry, top skills in
description text, company profile URL, and sometimes website evidence.

Current detail extraction captures company name, title, structured JobPosting
facts, compensation, industry, summary, and Built In `howToApply` as company
website evidence.

Useful facts Built In can expose but the durable job model currently loses or
compresses:

- full industry list
- top skills as structured values
- Built In company profile URL
- external apply URL separate from source posting URL
- source posted or reposted date
- source location labels
- employment type
- structured `datePosted` and `validThrough`
- structured `jobLocation` and applicant location requirements
- company size or employee range from company/profile pages
- headquarters from company/profile pages
- founded year from company/profile pages
- ticker or public-company hints when present
- richer source evidence for each extracted fact

### Generic Site Search

The browser probe sees anchor text, href, parent/card context, image alt text,
page title, page body text, and a score. The persisted job keeps only the
minimal normalized job: company, title, URL, remote, source, and a small
description. The discarded context could help explain confidence, extract
location/posted-date/skills, or avoid later LLM/browser work.

### RSS

RSS extraction keeps title/company, link, description, remote inference, and
default compensation. Feed `Published`, `Updated`, categories, and feed-specific
metadata are not preserved.

### Structured JobPosting

The parser currently uses:

- `hiringOrganization.name`
- `hiringOrganization.sameAs` or `url`
- `title`
- `description`
- `baseSalary`
- `industry`

Useful structured fields currently not preserved:

- `datePosted`
- `validThrough`
- `employmentType`
- `jobLocation`
- `applicantLocationRequirements`
- direct apply URL if present
- richer organization fields beyond name/url

### Public Profiles And Company Site

The health/company-profile path already has richer structs for:

- profile source, URL, title, snippet
- industry
- employee range and estimated employees
- founded year
- headquarters
- revenue
- employer rating, review count, recommend percent, CEO approval
- website/about text and URLs

These facts feed health today, but they do not feed back into the job or company
identity model except for website, summary, and industry.

## Health Query Gaps

Current deterministic health checks do use some context, but several searches
start too broadly:

- Wikipedia searches only company name.
- SEC lookup uses company and ticker, but not website/domain or aliases.
- Google News searches company plus aliases, then filters with context.
- Layoff news searches only `"Company" layoffs`, then filters with context.
- Hacker News searches only company name, then filters with context.
- Company-site discovery uses website first, otherwise only company-name
  queries.
- Public-profile search is the strongest query today because it includes both
  company name and website domain.

For ambiguous companies, search construction should prefer the resolved identity
context we already have. For example, with company name, official domain, and
industry known, layoff/news searches should include those terms instead of
querying only the bare company name.

## Persistence Pressure

The current model has three different flexibility levels:

- `health_cache.payload_json` is flexible and already stores rich health
  evidence.
- `company_identities.identity_evidence_json` is flexible, but most code treats
  it like fixed website/summary/industry evidence.
- `job_candidates.job_json` preserves normalized source facts for candidates
  that may never become visible jobs.
- `jobs.metadata_json` and `jobs.company_identity_json` preserve structured job
  and identity metadata for accepted jobs.

This means source adapters can often discover useful facts with nowhere clean to
put them. Candidate and job metadata give adapters a better place to store
opportunistic facts, but source-specific fields should still be added only when
they are useful to render, filter, search, score, or debug.

Candidate buckets:

- job source facts: posting URL, external apply URL, posted date, expiry,
  locations, skills, employment type, source profile URL, raw source labels
- company facts: aliases, domains, industries, founded year, employee range,
  estimated employees, headquarters, revenue, public profiles, ticker, exchange
- evidence facts: source, URL, confidence, reason, accepted/rejected state,
  observed-at timestamp

## Suggested Implementation Split

### Built In Adapter

Improve Built In extraction without changing Company Health logic:

- preserve full industry list instead of collapsing to one value
- preserve top skills as structured metadata
- preserve Built In company profile URL
- preserve external apply URL separately from the Built In posting URL
- extract structured `datePosted`, `validThrough`, `employmentType`, and
  location fields when available
- promote company profile facts such as employee range, founded year, HQ, and
  ticker/public hints when available

### Health Context

Improve health query construction and evidence filtering without changing the
Built In adapter:

- include official domain and useful industry terms in ambiguous news/layoff
  searches
- search aliases for HN and layoff signals when available
- use ticker and known public-company hints before broad SEC lookup
- treat multi-industry context as signal, not as competing single labels
- show rejected evidence clearly enough to debug false positives

### Data Model

Add a clean place for opportunistic facts before adding many more one-off
columns. The model should make it easy for any adapter to attach structured
facts with evidence while still preserving the simple job table fields used by
the current UI.
