package tuiapp

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) currentDetailText(innerW int) string {
	if len(m.filteredJobs) == 0 || m.cursor < 0 || m.cursor >= len(m.filteredJobs) {
		return ""
	}

	selectedJob := m.filteredJobs[m.cursor]

	var content strings.Builder
	writeDetailField := func(label string, value string) {
		if strings.TrimSpace(value) == "" {
			value = "N/A"
		}
		prefix := label + ": "
		prefixWidth := lipgloss.Width(prefix)
		valueWidth := innerW - prefixWidth
		if valueWidth < 10 {
			valueWidth = 10
		}
		lines := wrapANSIWords(value, valueWidth)
		if len(lines) == 0 {
			lines = []string{value}
		}
		content.WriteString(detailLabelStyle.Render(prefix))
		content.WriteString(detailValueStyle.Render(lines[0]))
		content.WriteString("\n")
		continuationPrefix := strings.Repeat(" ", prefixWidth)
		for _, line := range lines[1:] {
			content.WriteString(continuationPrefix)
			content.WriteString(detailValueStyle.Render(line))
			content.WriteString("\n")
		}
	}
	writeEnrichedDetailField := func(label string, value string, field string) {
		if strings.TrimSpace(value) == "" && m.pendingField(selectedJob, field) {
			value = enrichmentLoadingText
		}
		writeDetailField(label, value)
	}

	writeDetailField("Company", selectedJob.Company)
	writeEnrichedDetailField("Company Website", selectedJob.CompanyWebsite, "company_website")
	writeEnrichedDetailField("Company Summary", selectedJob.CompanySummary, "company_summary")
	writeEnrichedDetailField("Company Industry", selectedJob.CompanyIndustry, "company_industry")
	writeDetailField("Title", selectedJob.Title)
	writeDetailField("Remote", selectedJob.Remote)
	writeEnrichedDetailField("Compensation", selectedJob.Compensation, "compensation")
	writeDetailField("Source", selectedJob.Source)
	writeDetailField("Apply", selectedJob.ApplyURL)

	content.WriteString("\n")
	content.WriteString(detailSectionStyle.Render("Why it matches"))
	content.WriteString("\n")

	for _, match := range selectedJob.WhyMatches {
		lines := wrapANSIWords(match, innerW-4)
		if len(lines) == 0 {
			lines = []string{match}
		}
		content.WriteString(detailBulletStyle.Render("  • "))
		content.WriteString(detailValueStyle.Render(lines[0]))
		content.WriteString("\n")
		for _, line := range lines[1:] {
			content.WriteString("    ")
			content.WriteString(detailValueStyle.Render(line))
			content.WriteString("\n")
		}
	}
	if len(selectedJob.WhyMatches) == 0 {
		content.WriteString(detailValueStyle.Render("  No match notes available."))
	}

	return strings.TrimRight(content.String(), "\n")
}
