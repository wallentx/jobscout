package tuiapp

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPostingValidationTargetsOnlyUnopenedAndViewedJobs(t *testing.T) {
	jobs := []Job{
		{Company: "Acme", Title: "Unopened Role", ApplyURL: "https://jobs.example/acme", Status: "Unopened"},
		{Company: "Bravo", Title: "Viewed Role", ApplyURL: "https://jobs.example/bravo", Status: "Viewed"},
		{Company: "Charlie", Title: "Applied Role", ApplyURL: "https://jobs.example/charlie", Status: "Applied"},
		{Company: "Delta", Title: "Expired Role", ApplyURL: "https://jobs.example/delta", Status: "Expired"},
	}

	targets := postingValidationTargets(jobs)

	if len(targets) != 2 {
		t.Fatalf("postingValidationTargets(%#v) len = %d, want 2", jobs, len(targets))
	}
	if targets[0].key != backgroundJobKey(jobs[0]) || targets[1].key != backgroundJobKey(jobs[1]) {
		t.Fatalf("postingValidationTargets(%#v) keys = %#v, want first two jobs", jobs, targets)
	}
}

func TestPostingValidationCompletionExpiresOnlyDeadUnopenedAndViewedJobs(t *testing.T) {
	prevStore := runtimeJobStore
	fakeStore := &fakeJobStore{}
	runtimeJobStore = fakeStore
	t.Cleanup(func() {
		runtimeJobStore = prevStore
	})

	deadUnopened := Job{Company: "Acme", Title: "Unopened Role", ApplyURL: "https://jobs.example/acme", Status: "Unopened"}
	liveViewed := Job{Company: "Bravo", Title: "Viewed Role", ApplyURL: "https://jobs.example/bravo", Status: "Viewed"}
	appliedSameURL := Job{Company: "Acme", Title: "Applied Role", ApplyURL: deadUnopened.ApplyURL, Status: "Applied"}
	m := model{
		allJobs:       []Job{deadUnopened, liveViewed, appliedSameURL},
		filteredJobs:  []Job{deadUnopened, liveViewed, appliedSameURL},
		activeFilters: filterValuesFromStatuses(nil),
		backgroundTask: backgroundTaskState{
			active: true,
			id:     7,
		},
	}

	updated, cmd := m.Update(postingValidationCompleteMsg{
		taskID:  7,
		checked: 2,
		results: []postingValidationResult{
			{key: backgroundJobKey(deadUnopened), active: false, reason: "HTTP 404"},
			{key: backgroundJobKey(liveViewed), active: true},
		},
	})
	got := updated.(model)

	if cmd != nil {
		t.Fatalf("Update(postingValidationCompleteMsg) cmd = %v, want nil", cmd)
	}
	if got.allJobs[0].Status != "Expired" {
		t.Fatalf("dead unopened status = %q, want Expired", got.allJobs[0].Status)
	}
	if got.allJobs[1].Status != "Viewed" {
		t.Fatalf("live viewed status = %q, want Viewed", got.allJobs[1].Status)
	}
	if got.allJobs[2].Status != "Applied" {
		t.Fatalf("applied same-url status = %q, want Applied", got.allJobs[2].Status)
	}
	if got.backgroundTask.active {
		t.Fatal("backgroundTask.active = true after posting validation complete; want false")
	}
	if len(fakeStore.saved) != 3 || fakeStore.saved[0].Status != "Expired" {
		t.Fatalf("saved jobs = %#v, want expired first job persisted", fakeStore.saved)
	}
	if got.overlay.kind != overlayNotice || !strings.Contains(got.overlay.notice.message, "Marked 1 posting as Expired") {
		t.Fatalf("notice = kind %v message %q, want expired summary", got.overlay.kind, got.overlay.notice.message)
	}
}

func TestMainListPostingValidationKeyStartsBackgroundTask(t *testing.T) {
	jobs := []Job{
		{Company: "Acme", Title: "Unopened Role", ApplyURL: "https://jobs.example/acme", Status: "Unopened"},
		{Company: "Bravo", Title: "Viewed Role", ApplyURL: "https://jobs.example/bravo", Status: "Viewed"},
	}
	m := model{
		allJobs:       jobs,
		filteredJobs:  jobs,
		activeFilters: filterValuesFromStatuses(nil),
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("V")})
	got := updated.(model)

	if cmd == nil {
		t.Fatal("Update(V) cmd = nil, want posting validation command")
	}
	if !got.backgroundTask.active || !got.backgroundTask.expanded {
		t.Fatalf("backgroundTask = %#v, want active expanded validation task", got.backgroundTask)
	}
	if got.backgroundTask.title != "Checking Active Postings" {
		t.Fatalf("backgroundTask.title = %q, want Checking Active Postings", got.backgroundTask.title)
	}
	if !strings.Contains(got.backgroundTask.progress, "Checking 2 active postings") {
		t.Fatalf("backgroundTask.progress = %q, want active posting count", got.backgroundTask.progress)
	}
}
