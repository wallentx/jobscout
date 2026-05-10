package fetcher

import "testing"

func TestJobMatchesWorkSettings(t *testing.T) {
	tests := []struct {
		name     string
		job      Job
		settings WorkSettingsConfig
		want     bool
	}{
		{
			name: "remote field wins over incidental description text",
			job: Job{
				Remote:      "Remote",
				Description: "Experience with hybrid cloud systems and in office collaboration tools.",
			},
			settings: WorkSettingsConfig{Remote: true},
			want:     true,
		},
		{
			name: "in office or remote matches remote-only criteria",
			job: Job{
				Remote:      "In-Office or Remote",
				Description: "Candidate can work remotely in the US.",
			},
			settings: WorkSettingsConfig{Remote: true},
			want:     true,
		},
		{
			name: "remote or hybrid matches remote-only criteria",
			job: Job{
				Remote:      "Remote or NYC - Hybrid",
				Description: "If based in NYC, the role is hybrid.",
			},
			settings: WorkSettingsConfig{Remote: true},
			want:     true,
		},
		{
			name: "hybrid-only does not match remote-only criteria",
			job: Job{
				Remote:      "Hybrid",
				Description: "Hybrid role in Austin.",
			},
			settings: WorkSettingsConfig{Remote: true},
			want:     false,
		},
		{
			name: "not remote field does not match remote-only criteria",
			job: Job{
				Remote:      "Not remote",
				Description: "Experience coordinating remote release workflows.",
			},
			settings: WorkSettingsConfig{Remote: true},
			want:     false,
		},
		{
			name: "remote false field does not match remote-only criteria",
			job: Job{
				Remote:      "remote:false",
				Description: "Role partners with remote teams.",
			},
			settings: WorkSettingsConfig{Remote: true},
			want:     false,
		},
		{
			name: "not remote description does not match remote-only criteria",
			job: Job{
				Description: "This role is not remote.",
			},
			settings: WorkSettingsConfig{Remote: true},
			want:     false,
		},
		{
			name: "real remote field still matches remote-only criteria",
			job: Job{
				Remote:      "Remote - United States",
				Description: "Distributed engineering team.",
			},
			settings: WorkSettingsConfig{Remote: true},
			want:     true,
		},
		{
			name: "hybrid cloud alone is not a work setting",
			job: Job{
				Description: "Build and operate hybrid cloud infrastructure.",
			},
			settings: WorkSettingsConfig{Remote: true},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := jobMatchesWorkSettings(&tt.job, tt.settings); got != tt.want {
				t.Fatalf("jobMatchesWorkSettings(%+v, %+v) = %t; want %t", tt.job, tt.settings, got, tt.want)
			}
		})
	}
}
