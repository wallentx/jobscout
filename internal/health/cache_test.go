package health

import (
	"testing"
	"time"

	"github.com/wallentx/jobscout/internal/domain"
)

func TestCacheKeyForJobUsesDomain(t *testing.T) {
	job := domain.Job{
		Company:        "Circle",
		CompanyWebsite: "https://www.circle.com/careers",
	}

	if got := CacheKeyForJob(job); got != "domain:circle.com" {
		t.Fatalf("CacheKeyForJob() = %q, want domain:circle.com", got)
	}
}

func TestUniqueJobsFromJobsUsesDomainIdentity(t *testing.T) {
	jobs := []domain.Job{
		{Company: "Circle", Title: "Backend", CompanyWebsite: "https://circle.com/careers"},
		{Company: "Circle Internet Group", Title: "Frontend", CompanyWebsite: "https://www.circle.com/jobs"},
		{Company: "Acme", Title: "Infra", CompanyWebsite: "https://acme.example/jobs"},
	}

	got := UniqueJobsFromJobs(jobs)
	if len(got) != 2 {
		t.Fatalf("UniqueJobsFromJobs() len = %d, want 2: %#v", len(got), got)
	}
	if got[0].Company != "Acme" || got[1].Company != "Circle" {
		t.Fatalf("UniqueJobsFromJobs() = %#v, want sorted unique companies", got)
	}
}

func TestIsHealthCacheFreshKeepsStableResultsLongerThanOneDay(t *testing.T) {
	result := &domain.CompanyHealthResult{
		Confidence: "high",
		Score:      82,
		EmploymentRisk: &domain.EmploymentRisk{
			Score: 5,
			Level: "Low",
		},
	}

	if !IsHealthCacheFresh(time.Now().Add(-6*24*time.Hour), result) {
		t.Fatal("IsHealthCacheFresh(stable 6d old) = false, want true")
	}
	if IsHealthCacheFresh(time.Now().Add(-8*24*time.Hour), result) {
		t.Fatal("IsHealthCacheFresh(stable 8d old) = true, want false")
	}
}

func TestIsHealthCacheFreshKeepsVolatileResultsShortLived(t *testing.T) {
	result := &domain.CompanyHealthResult{
		Confidence: "high",
		Score:      45,
		Flags:      []string{"layoff_news_detected"},
		EmploymentRisk: &domain.EmploymentRisk{
			Score: 40,
			Level: "Medium",
		},
	}

	if !IsHealthCacheFresh(time.Now().Add(-23*time.Hour), result) {
		t.Fatal("IsHealthCacheFresh(volatile 23h old) = false, want true")
	}
	if IsHealthCacheFresh(time.Now().Add(-25*time.Hour), result) {
		t.Fatal("IsHealthCacheFresh(volatile 25h old) = true, want false")
	}
}
