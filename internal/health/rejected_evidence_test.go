package health

import (
	"testing"

	"github.com/wallentx/jobscout/internal/domain"
)

func TestDebugRejectedEvidenceSummaryGroupsBySourceAndReason(t *testing.T) {
	evidence := []domain.CompanyHealthEvidence{
		{Source: "google_news_rss", Reason: "company mismatch"},
		{Source: "google_news_rss", Reason: "company mismatch"},
		{Source: "hacker_news", Reason: "industry mismatch"},
		{Source: "hacker_news"},
	}

	got := debugRejectedEvidenceSummary(evidence)
	want := "google_news_rss/company mismatch=2, hacker_news/industry mismatch=1, hacker_news/unspecified=1"
	if got != want {
		t.Fatalf("debugRejectedEvidenceSummary() = %q, want %q", got, want)
	}
}

func TestDebugRejectedEvidenceSummaryHandlesEmpty(t *testing.T) {
	if got := debugRejectedEvidenceSummary(nil); got != "[]" {
		t.Fatalf("debugRejectedEvidenceSummary(nil) = %q, want []", got)
	}
}
