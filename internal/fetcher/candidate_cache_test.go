package fetcher

import (
	"context"
	"testing"
	"time"

	"github.com/wallentx/jobscout/internal/domain"
	appruntime "github.com/wallentx/jobscout/internal/runtime"
)

func TestApplyCachedLLMFilterDecisionsSkipsRejectedCandidate(t *testing.T) {
	ctx := context.Background()
	store := appruntime.InMemoryStores().Candidates
	appCfg := defaultAppConfig()
	criteria := defaultCriteriaConfig()
	job := Job{
		Company:     "DropCo",
		Title:       "Platform Engineer",
		Source:      "Site Search: Example",
		ApplyURL:    "https://jobs.example/dropco/platform-engineer",
		Description: "Platform engineering role.",
	}
	candidate, ok := jobCandidateFromJob(job, time.Now(), true)
	if !ok {
		t.Fatal("jobCandidateFromJob() ok = false, want true")
	}
	if err := store.UpsertJobCandidate(ctx, candidate); err != nil {
		t.Fatalf("UpsertJobCandidate() error = %v", err)
	}
	signature, ok := llmFilteringDecisionSignature(&appCfg)
	if !ok {
		t.Fatal("llmFilteringDecisionSignature() ok = false, want true")
	}
	if err := store.UpsertJobCandidateDecision(ctx, domain.JobCandidateDecision{
		JobCandidateDecisionKey: domain.JobCandidateDecisionKey{
			CandidateKey:    candidate.Key,
			CriteriaHash:    domain.CriteriaHash(&criteria),
			Stage:           domain.CandidateDecisionStageLLMFiltering,
			DecisionVersion: domain.CandidateDecisionVersionLLMFiltering,
			LLMProvider:     signature.provider,
			LLMModel:        signature.model,
		},
		Decision:  domain.CandidateDecisionRejected,
		Reason:    "llm job filtering",
		Job:       job,
		DecidedAt: time.Now(),
	}); err != nil {
		t.Fatalf("UpsertJobCandidateDecision() error = %v", err)
	}
	summary := newFetchSummary()

	got := ApplyCachedLLMFilterDecisions(ctx, store, &appCfg, &criteria, []Job{job}, &summary)

	if len(got) != 0 {
		t.Fatalf("ApplyCachedLLMFilterDecisions() len = %d; want 0", len(got))
	}
	if len(summary.Filtered["cached llm rejection"]) != 1 {
		t.Fatalf("summary.Filtered[cached llm rejection] len = %d; want 1", len(summary.Filtered["cached llm rejection"]))
	}
}
