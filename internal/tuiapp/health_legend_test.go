package tuiapp

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestHealthLegendUsesSharedScoreBands(t *testing.T) {
	previousBands := append([]healthScoreBand(nil), healthScoreBands...)
	healthScoreBands = []healthScoreBand{
		{Min: 42, Range: "42-100", Label: "Review", Description: "custom shared band", SampleScore: 50, Color: lipgloss.Color("46")},
	}
	t.Cleanup(func() {
		healthScoreBands = previousBands
	})

	message := ansi.Strip(healthLegendMessage())

	for _, band := range healthScoreBands {
		want := fmt.Sprintf("%s: %s", band.Range, band.Label)
		if !strings.Contains(message, want) {
			t.Fatalf("health legend missing shared score band %q:\n%s", want, message)
		}
		if got := getHealthColor(band.SampleScore); got != band.Color {
			t.Fatalf("getHealthColor(%d) = %q; want shared band color %q", band.SampleScore, got, band.Color)
		}
	}
	if strings.Contains(message, "75-100") {
		t.Fatalf("health legend used stale hard-coded score bands:\n%s", message)
	}
}
