package fetcher

import (
	"context"
	"reflect"
	"strings"
	"time"

	"github.com/wallentx/jobscout/internal/domain"
	"github.com/wallentx/jobscout/internal/storage"
)

const (
	defaultCandidateCacheDays = 30
)

var deterministicCandidateDecisionReasons = map[string]struct{}{
	"title excludes":     {},
	"title requirements": {},
	"title includes":     {},
	"industry excludes":  {},
	"pay":                {},
	"work setting":       {},
}

func candidateCacheRetention(appCfg *AppConfig) time.Duration {
	days := defaultCandidateCacheDays
	if appCfg != nil && appCfg.Fetch.CandidateCacheDays > 0 {
		days = appCfg.Fetch.CandidateCacheDays
	}
	return time.Duration(days) * 24 * time.Hour
}

func pruneCandidateCache(ctx context.Context, store storage.CandidateStore, appCfg *AppConfig) {
	if store == nil {
		return
	}
	cutoff := time.Now().Add(-candidateCacheRetention(appCfg))
	removed, err := store.PruneJobCandidateCache(ctx, cutoff)
	if err != nil {
		logDebug("candidate cache: prune failed: %v", err)
		return
	}
	if removed > 0 {
		logDebug("candidate cache: pruned %d candidates older than %s", removed, cutoff.Format(time.RFC3339))
	}
}

func upsertJobCandidates(ctx context.Context, store storage.CandidateStore, jobs []Job) {
	upsertJobCandidatesWithActive(ctx, store, jobs, true)
}

func upsertJobCandidatesWithActive(ctx context.Context, store storage.CandidateStore, jobs []Job, active bool) {
	if store == nil || len(jobs) == 0 {
		return
	}
	candidates := make([]domain.JobCandidate, 0, len(jobs))
	now := time.Now()
	for _, job := range jobs {
		candidate, ok := jobCandidateFromJob(job, now, active)
		if ok {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 0 {
		return
	}
	if err := store.UpsertJobCandidates(ctx, candidates); err != nil {
		logDebug("candidate cache: failed to store %d candidates: %v", len(candidates), err)
		return
	}
	logDebug("candidate cache: stored %d candidates", len(candidates))
}

func RecordAcceptedJobCandidates(ctx context.Context, store storage.CandidateStore, jobs []Job) {
	upsertJobCandidates(ctx, store, jobs)
}

func RecordInactiveJobCandidates(ctx context.Context, store storage.CandidateStore, jobs []Job) {
	upsertJobCandidatesWithActive(ctx, store, jobs, false)
}

func jobCandidateFromJob(job Job, now time.Time, active bool) (domain.JobCandidate, bool) {
	key := strings.TrimSpace(fetchedJobDedupeKey(job))
	if key == "" {
		return domain.JobCandidate{}, false
	}
	sourceKey := key
	canonicalApplyURL := ""
	if strings.HasPrefix(key, "url:") {
		canonicalApplyURL = strings.TrimPrefix(key, "url:")
	} else if strings.TrimSpace(job.ApplyURL) != "" {
		canonicalApplyURL = canonicalURLVisitKey(job.ApplyURL)
	}
	return domain.JobCandidate{
		Key:               key,
		Source:            strings.TrimSpace(job.Source),
		SourceKey:         sourceKey,
		ApplyURL:          strings.TrimSpace(job.ApplyURL),
		CanonicalApplyURL: canonicalApplyURL,
		Company:           strings.TrimSpace(job.Company),
		Title:             strings.TrimSpace(job.Title),
		Job:               job,
		Active:            active,
		FirstSeen:         now,
		LastSeen:          now,
	}, true
}

func hydrateJobsFromCandidateCache(ctx context.Context, store storage.CandidateStore, jobs []Job) []Job {
	if store == nil || len(jobs) == 0 {
		return jobs
	}
	hits := 0
	for i := range jobs {
		key := strings.TrimSpace(fetchedJobDedupeKey(jobs[i]))
		if key == "" {
			continue
		}
		candidate, err := store.GetJobCandidate(ctx, key)
		if err != nil {
			logDebug("candidate cache: lookup failed key=%q: %v", key, err)
			continue
		}
		if candidate == nil {
			continue
		}
		before := jobs[i]
		domain.MergeJobIdentityFields(&jobs[i], candidate.Job)
		if jobChangedByCandidateHydration(before, jobs[i]) {
			hits++
		}
	}
	if hits > 0 {
		logDebug("candidate cache: hydrated %d jobs from cached normalized source data", hits)
	}
	return jobs
}

func jobChangedByCandidateHydration(before Job, after Job) bool {
	return before.CompanyWebsite != after.CompanyWebsite ||
		before.CompanySummary != after.CompanySummary ||
		before.CompanyIndustry != after.CompanyIndustry ||
		before.Compensation != after.Compensation ||
		before.Description != after.Description ||
		!reflect.DeepEqual(before.Metadata, after.Metadata)
}

func applyCachedDeterministicRejects(ctx context.Context, store storage.CandidateStore, criteria *CriteriaConfig, jobs []Job, summary *FetchSummary) []Job {
	key := deterministicCandidateDecisionKey(criteria)
	kept, rejected := filterCachedRejectedJobs(ctx, store, key, jobs)
	if len(rejected) == 0 {
		return jobs
	}
	if summary != nil {
		if summary.Filtered == nil {
			summary.Filtered = make(map[string][]Job)
		}
		summary.Filtered["cached rejection"] = append(summary.Filtered["cached rejection"], rejected...)
	}
	logDebug("candidate cache: skipped %d candidates with cached deterministic rejections", len(rejected))
	return kept
}

func deterministicCandidateDecisionKey(criteria *CriteriaConfig) domain.JobCandidateDecisionKey {
	return domain.JobCandidateDecisionKey{
		CriteriaHash:    domain.CriteriaHash(criteria),
		Stage:           domain.CandidateDecisionStageDeterministic,
		DecisionVersion: domain.CandidateDecisionVersionDeterministic,
	}
}

func cachedDeterministicRejectedJob(ctx context.Context, store storage.CandidateStore, criteria *CriteriaConfig, job Job) (Job, bool) {
	if store == nil {
		return Job{}, false
	}
	key := deterministicCandidateDecisionKey(criteria)
	key.CandidateKey = strings.TrimSpace(fetchedJobDedupeKey(job))
	if key.CandidateKey == "" {
		return Job{}, false
	}
	decision, err := store.GetJobCandidateDecision(ctx, key)
	if err != nil {
		logDebug("candidate cache: decision lookup failed key=%q stage=%q: %v", key.CandidateKey, key.Stage, err)
		return Job{}, false
	}
	if decision == nil || decision.Decision != domain.CandidateDecisionRejected {
		return Job{}, false
	}
	cached := job
	if !jobLooksUsableForCachedRejection(cached) {
		if candidate, err := store.GetJobCandidate(ctx, key.CandidateKey); err == nil && candidate != nil {
			cached = candidate.Job
		} else if err != nil {
			logDebug("candidate cache: candidate lookup failed key=%q: %v", key.CandidateKey, err)
		}
	}
	if !jobLooksUsableForCachedRejection(cached) && jobLooksUsableForCachedRejection(decision.Job) {
		cached = decision.Job
	}
	if strings.TrimSpace(cached.Source) == "" {
		cached.Source = strings.TrimSpace(job.Source)
	}
	if strings.TrimSpace(cached.ApplyURL) == "" {
		cached.ApplyURL = strings.TrimSpace(job.ApplyURL)
	}
	if reason := strings.TrimSpace(decision.Reason); reason != "" {
		cached.WhyMatches = append(cached.WhyMatches, "Cached rejection: "+reason)
	}
	return cached, true
}

func jobLooksUsableForCachedRejection(job Job) bool {
	return strings.TrimSpace(job.Company) != "" &&
		!siteSearchCompanyMissingOrInvalid(job.Company) &&
		strings.TrimSpace(job.Title) != ""
}

func recordDeterministicCandidateDecisions(ctx context.Context, store storage.CandidateStore, criteria *CriteriaConfig, accepted []Job, filtered map[string][]Job) {
	if store == nil {
		return
	}
	criteriaHash := domain.CriteriaHash(criteria)
	decisions := make([]domain.JobCandidateDecision, 0, len(accepted)+countFilteredJobs(filtered))
	for _, job := range accepted {
		if decision, ok := jobCandidateDecision(job, criteriaHash, domain.CandidateDecisionStageDeterministic, domain.CandidateDecisionVersionDeterministic, "", "", domain.CandidateDecisionAccepted, ""); ok {
			decisions = append(decisions, decision)
		}
	}
	for reason, jobs := range filtered {
		if _, ok := deterministicCandidateDecisionReasons[reason]; !ok {
			continue
		}
		for _, job := range jobs {
			if decision, ok := jobCandidateDecision(job, criteriaHash, domain.CandidateDecisionStageDeterministic, domain.CandidateDecisionVersionDeterministic, "", "", domain.CandidateDecisionRejected, reason); ok {
				decisions = append(decisions, decision)
			}
		}
	}
	upsertJobCandidateDecisions(ctx, store, decisions, "deterministic")
}

func ApplyCachedLLMFilterDecisions(ctx context.Context, store storage.CandidateStore, appCfg *AppConfig, criteria *CriteriaConfig, jobs []Job, summary *FetchSummary) []Job {
	signature, ok := llmFilteringDecisionSignature(appCfg)
	if !ok {
		return jobs
	}
	key := domain.JobCandidateDecisionKey{
		CriteriaHash:    domain.CriteriaHash(criteria),
		Stage:           domain.CandidateDecisionStageLLMFiltering,
		DecisionVersion: domain.CandidateDecisionVersionLLMFiltering,
		LLMProvider:     signature.provider,
		LLMModel:        signature.model,
	}
	kept, rejected := filterCachedRejectedJobs(ctx, store, key, jobs)
	if len(rejected) == 0 {
		return jobs
	}
	if summary != nil {
		if summary.Filtered == nil {
			summary.Filtered = make(map[string][]Job)
		}
		summary.Filtered["cached llm rejection"] = append(summary.Filtered["cached llm rejection"], rejected...)
	}
	logDebug("candidate cache: skipped %d candidates with cached LLM filter rejections", len(rejected))
	return kept
}

func RecordLLMFilterCandidateDecisions(ctx context.Context, store storage.CandidateStore, appCfg *AppConfig, criteria *CriteriaConfig, before []Job, after []Job, notices []string) {
	if store == nil || len(before) == 0 || len(notices) > 0 {
		return
	}
	signature, ok := llmFilteringDecisionSignature(appCfg)
	if !ok {
		return
	}
	criteriaHash := domain.CriteriaHash(criteria)
	afterKeys := make(map[string]struct{}, len(after))
	for _, job := range after {
		if key := strings.TrimSpace(fetchedJobDedupeKey(job)); key != "" {
			afterKeys[key] = struct{}{}
		}
	}
	decisions := make([]domain.JobCandidateDecision, 0, len(before))
	for _, job := range before {
		key := strings.TrimSpace(fetchedJobDedupeKey(job))
		if key == "" {
			continue
		}
		decision := domain.CandidateDecisionRejected
		reason := "llm job filtering"
		if _, ok := afterKeys[key]; ok {
			decision = domain.CandidateDecisionAccepted
			reason = ""
		}
		if candidateDecision, ok := jobCandidateDecision(job, criteriaHash, domain.CandidateDecisionStageLLMFiltering, domain.CandidateDecisionVersionLLMFiltering, signature.provider, signature.model, decision, reason); ok {
			decisions = append(decisions, candidateDecision)
		}
	}
	upsertJobCandidateDecisions(ctx, store, decisions, "llm filtering")
}

func filterCachedRejectedJobs(ctx context.Context, store storage.CandidateStore, key domain.JobCandidateDecisionKey, jobs []Job) ([]Job, []Job) {
	if store == nil || len(jobs) == 0 {
		return jobs, nil
	}
	kept := make([]Job, 0, len(jobs))
	rejected := make([]Job, 0)
	for _, job := range jobs {
		candidateKey := strings.TrimSpace(fetchedJobDedupeKey(job))
		if candidateKey == "" {
			kept = append(kept, job)
			continue
		}
		key.CandidateKey = candidateKey
		decision, err := store.GetJobCandidateDecision(ctx, key)
		if err != nil {
			logDebug("candidate cache: decision lookup failed key=%q stage=%q: %v", candidateKey, key.Stage, err)
			kept = append(kept, job)
			continue
		}
		if decision != nil && decision.Decision == domain.CandidateDecisionRejected {
			if strings.TrimSpace(decision.Reason) != "" {
				job.WhyMatches = append(job.WhyMatches, "Cached rejection: "+decision.Reason)
			}
			rejected = append(rejected, job)
			continue
		}
		kept = append(kept, job)
	}
	return kept, rejected
}

func jobCandidateDecision(job Job, criteriaHash string, stage string, version string, provider string, model string, decision string, reason string) (domain.JobCandidateDecision, bool) {
	candidate, ok := jobCandidateFromJob(job, time.Now(), true)
	if !ok {
		return domain.JobCandidateDecision{}, false
	}
	return domain.JobCandidateDecision{
		JobCandidateDecisionKey: domain.JobCandidateDecisionKey{
			CandidateKey:    candidate.Key,
			CriteriaHash:    strings.TrimSpace(criteriaHash),
			Stage:           strings.TrimSpace(stage),
			DecisionVersion: strings.TrimSpace(version),
			LLMProvider:     strings.ToLower(strings.TrimSpace(provider)),
			LLMModel:        strings.TrimSpace(model),
		},
		Decision:  strings.TrimSpace(decision),
		Reason:    strings.TrimSpace(reason),
		Job:       job,
		DecidedAt: time.Now(),
	}, true
}

func upsertJobCandidateDecisions(ctx context.Context, store storage.CandidateStore, decisions []domain.JobCandidateDecision, label string) {
	if store == nil || len(decisions) == 0 {
		return
	}
	candidates := make([]domain.JobCandidate, 0, len(decisions))
	now := time.Now()
	for _, decision := range decisions {
		candidate, ok := jobCandidateFromJob(decision.Job, now, true)
		if ok {
			candidates = append(candidates, candidate)
		}
	}
	if err := store.UpsertJobCandidates(ctx, candidates); err != nil {
		logDebug("candidate cache: failed to store %s candidates before decisions: %v", label, err)
		return
	}
	if err := store.UpsertJobCandidateDecisions(ctx, decisions); err != nil {
		logDebug("candidate cache: failed to store %s decisions: %v", label, err)
		return
	}
	logDebug("candidate cache: stored %d %s decisions", len(decisions), label)
}

type llmFilteringSignature struct {
	provider string
	model    string
}

func llmFilteringDecisionSignature(appCfg *AppConfig) (llmFilteringSignature, bool) {
	if appCfg == nil || !appCfg.LLM.Enabled || !appCfg.LLM.JobFiltering {
		return llmFilteringSignature{}, false
	}
	provider := strings.ToLower(strings.TrimSpace(appCfg.LLM.Provider))
	if provider == "" {
		return llmFilteringSignature{}, false
	}
	model := strings.TrimSpace(appCfg.LLM.Model)
	if appCfg.LLM.Providers != nil {
		providerCfg := appCfg.LLM.Providers[provider]
		providerModel := strings.TrimSpace(providerCfg.Model)
		if providerCfg.Models != nil {
			providerModel = firstNonEmptyString(providerCfg.Models["llm_job_filtering"], providerModel)
		}
		model = firstNonEmptyString(providerModel, model)
	}
	if appCfg.LLM.Models != nil {
		model = firstNonEmptyString(appCfg.LLM.Models["llm_job_filtering"], model)
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return llmFilteringSignature{}, false
	}
	return llmFilteringSignature{
		provider: provider,
		model:    model,
	}, true
}
