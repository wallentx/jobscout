package domain

import (
	"fmt"
	"testing"
	"time"
)

func TestParseYearFromTextRequiresFoundedYearEvidence(t *testing.T) {
	futureYear := time.Now().Year() + 1
	tests := []struct {
		name string
		text string
	}{
		{
			name: "unrelated historical year",
			text: "Our mission is inspired by the 1969 moon landing and the teams that made it possible.",
		},
		{
			name: "legal copyright year",
			text: "Copyright 2024 Acme, Inc. All rights reserved. Terms, privacy, and accessibility.",
		},
		{
			name: "generic page date",
			text: "Last updated March 12, 2025. Subscribe for product news and customer stories.",
		},
		{
			name: "future year",
			text: fmt.Sprintf("Roadmap preview: new platform availability planned for %d.", futureYear),
		},
		{
			name: "future year with founded wording",
			text: fmt.Sprintf("Founded in %d to build tools for modern teams.", futureYear),
		},
		{
			name: "distant founded cue without year",
			text: "Founded by engineers who previously shipped software for enterprise teams. Copyright 2024.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseYearFromText(tt.text); got != nil {
				t.Fatalf("parseYearFromText(%q) = %d, want nil", tt.text, *got)
			}
		})
	}
}

func TestParseYearFromTextExplicitFoundedYearEvidence(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{
			name: "founded wording",
			text: "Acme was founded in 2012 to make better anvils.",
			want: 2012,
		},
		{
			name: "incorporated wording",
			text: "Acme Robotics, Inc. was incorporated on 2018 after three years of research.",
			want: 2018,
		},
		{
			name: "established wording",
			text: "Established in 1998, Acme supports customers around the world.",
			want: 1998,
		},
		{
			name: "since wording",
			text: "Trusted by teams since 2007.",
			want: 2007,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseYearFromText(tt.text)
			if got == nil || *got != tt.want {
				t.Fatalf("parseYearFromText(%q) = %#v, want %d", tt.text, got, tt.want)
			}
		})
	}
}
