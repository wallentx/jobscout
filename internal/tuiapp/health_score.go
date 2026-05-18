package tuiapp

import "github.com/charmbracelet/lipgloss"

type healthScoreBand struct {
	Min         int
	Range       string
	Label       string
	Description string
	SampleScore int
	Color       lipgloss.Color
}

var healthScoreBands = []healthScoreBand{
	{Min: 75, Range: "75-100", Label: "Strong", Description: "good stability signals", SampleScore: 80, Color: lipgloss.Color("46")},
	{Min: 60, Range: "60-74", Label: "Stable", Description: "mostly positive signals", SampleScore: 65, Color: lipgloss.Color("77")},
	{Min: 45, Range: "45-59", Label: "Watch", Description: "mixed or limited signals", SampleScore: 50, Color: lipgloss.Color("220")},
	{Min: 30, Range: "30-44", Label: "Risk", Description: "meaningful caution signals", SampleScore: 35, Color: lipgloss.Color("208")},
	{Min: 0, Range: "0-29", Label: "Critical", Description: "serious health concerns", SampleScore: 15, Color: lipgloss.Color("196")},
}

func getHealthColor(score int) lipgloss.Color {
	for _, band := range healthScoreBands {
		if score >= band.Min {
			return band.Color
		}
	}
	return healthScoreBands[len(healthScoreBands)-1].Color
}
