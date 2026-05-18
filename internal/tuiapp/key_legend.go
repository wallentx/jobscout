package tuiapp

import "strings"

func keyLegendMessage() string {
	return strings.Join([]string{
		"Main view",
		"",
		"↑/↓ or j/k: Move selection",
		"Enter: Show job details",
		"h: Show selected company health",
		"H: Refresh health for all companies",
		"l: Explain health symbols and colors",
		"?: Show this key legend",
		"s: Change status",
		"m: Mark selected job viewed",
		"r: Fetch jobs",
		"U: Update missing job and company details",
		"V: Check active postings",
		"c: Configure Jobscout",
		"D: Delete selected job",
		"E: Edit selected job",
		"/: Search",
		"1-5: Sort",
		"f: Filter statuses",
		"t: Show or hide background task",
		"q: Quit",
		"",
		"Detail window",
		"",
		"u: Update selected job and company details",
		"o: Open posting URL",
		"Enter/Esc: Return",
		"",
		"Health window",
		"",
		"h: Refresh current company health",
		"Enter/Esc: Return",
	}, "\n")
}
