package tuiapp

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestMergeAndSaveJobsSavesDuplicateEnrichment(t *testing.T) {
	prevJobStore := runtimeJobStore
	fakeStore := &fakeJobStore{}
	runtimeJobStore = fakeStore
	t.Cleanup(func() {
		runtimeJobStore = prevJobStore
	})

	existing := []Job{
		{
			Company: "Acme",
			Title:   "Platform Engineer",
			Status:  "Viewed",
		},
	}
	incoming := []Job{
		{
			Company:        "Acme",
			Title:          "Platform Engineer",
			CompanyWebsite: "https://www.acme.com",
			Description:    "Build and operate Acme's deployment platform.",
		},
	}
	data, err := json.Marshal(incoming)
	if err != nil {
		t.Fatalf("json.Marshal(%#v) error = %v", incoming, err)
	}

	added, err := mergeAndSaveJobs(existing, data)
	if err != nil {
		t.Fatalf("mergeAndSaveJobs(existing, data) error = %v", err)
	}

	if added != 0 {
		t.Fatalf("mergeAndSaveJobs(existing, data) added = %d; want 0", added)
	}
	if len(fakeStore.saved) != 1 {
		t.Fatalf("saved jobs len = %d; want enriched duplicate saved", len(fakeStore.saved))
	}
	saved := fakeStore.saved[0]
	if saved.CompanyWebsite != "https://www.acme.com" {
		t.Fatalf("saved CompanyWebsite = %q; want imported website", saved.CompanyWebsite)
	}
	if saved.Description != "Build and operate Acme's deployment platform." {
		t.Fatalf("saved Description = %q; want imported description", saved.Description)
	}
	if saved.Status != "Viewed" {
		t.Fatalf("saved Status = %q; want existing status preserved", saved.Status)
	}
}

func TestURLOpenCommand(t *testing.T) {
	tests := []struct {
		name      string
		goos      string
		available map[string]bool
		want      string
	}{
		{
			name:      "termux opener wins",
			goos:      "darwin",
			available: map[string]bool{"termux-open-url": true, "open": true},
			want:      "termux-open-url",
		},
		{
			name:      "macOS open",
			goos:      "darwin",
			available: map[string]bool{"open": true},
			want:      "open",
		},
		{
			name:      "Linux xdg-open",
			goos:      "linux",
			available: map[string]bool{"xdg-open": true},
			want:      "xdg-open",
		},
		{
			name:      "Linux ignores open",
			goos:      "linux",
			available: map[string]bool{"open": true},
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookPath := func(name string) (string, error) {
				if tt.available[name] {
					return "/usr/bin/" + name, nil
				}
				return "", errors.New("not found")
			}
			if got := urlOpenCommand(tt.goos, lookPath); got != tt.want {
				t.Fatalf("urlOpenCommand(%q, lookPath) = %q; want %q", tt.goos, got, tt.want)
			}
		})
	}
}
