package tuiapp

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const emptyStateLogo = `
      ██╗ ██████╗ ██████╗ ███████╗ ██████╗ ██████╗ ██╗   ██╗████████╗ 
      ██║██╔═══██╗██╔══██╗██╔════╝██╔════╝██╔═══██╗██║   ██║╚══██╔══╝ 
      ██║██║   ██║██████╔╝███████╗██║     ██║   ██║██║   ██║   ██║    
 ██   ██║██║   ██║██╔══██╗╚════██║██║     ██║   ██║██║   ██║   ██║    
 ╚█████╔╝╚██████╔╝██████╔╝███████║╚██████╗╚██████╔╝╚██████╔╝   ██║    
  ╚════╝  ╚═════╝ ╚═════╝ ╚══════╝ ╚═════╝ ╚═════╝  ╚═════╝    ╚═╝    
`

type rgbColor struct {
	R int
	G int
	B int
}

var emptyStateLogoBlockColors = []rgbColor{
	{R: 0xC6, G: 0xD8, B: 0xFF},
	{R: 0xB0, G: 0xC7, B: 0xFF},
	{R: 0x98, G: 0xB0, B: 0xF2},
	{R: 0x7E, G: 0x95, B: 0xDD},
	{R: 0x68, G: 0x7C, B: 0xC5},
	{R: 0x52, G: 0x62, B: 0xA6},
}

var emptyStateLogoShadowColors = []rgbColor{
	{R: 0x9A, G: 0x9A, B: 0x9A},
	{R: 0x80, G: 0x80, B: 0x80},
	{R: 0x66, G: 0x66, B: 0x66},
	{R: 0x4C, G: 0x4C, B: 0x4C},
	{R: 0x32, G: 0x32, B: 0x32},
	{R: 0x1E, G: 0x1E, B: 0x1E},
}

func renderEmptyTableLogo(width int, height int, version string) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	version = strings.TrimSpace(version)
	return renderEmptyTableLogoWithTopPadding(width, height, version, 1)
}

func renderSetupEmptyTable(width int, height int, savedJobs int, version string) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	messageLines := setupTableMessageLines(width, savedJobs)
	if len(messageLines) > height {
		messageLines = messageLines[:height]
	}
	logoHeight := height - len(messageLines)
	if logoHeight < 0 {
		logoHeight = 0
	}

	out := splitRenderedLines(renderEmptyTableLogoWithTopPadding(width, logoHeight, version, 1))
	for len(out) < logoHeight {
		out = append(out, lipgloss.NewStyle().Width(width).Render(""))
	}
	out = append(out, messageLines...)
	for len(out) < height {
		out = append(out, lipgloss.NewStyle().Width(width).Render(""))
	}
	if len(out) > height {
		out = out[:height]
	}
	return strings.Join(out, "\n")
}

func setupTableMessageLines(width int, savedJobs int) []string {
	lines := strings.Split(strings.TrimRight(tempSetupTableMessage(savedJobs), "\n"), "\n")
	out := make([]string, 0, len(lines))
	style := lipgloss.NewStyle().Width(width).Foreground(lipgloss.Color("244"))
	for _, line := range lines {
		out = append(out, style.Render(truncate(line, width)))
	}
	return out
}

func renderEmptyTableLogoWithTopPadding(width int, height int, version string, topPadding int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	rawLines := strings.Split(strings.Trim(emptyStateLogo, "\n"), "\n")
	version = strings.TrimSpace(version)
	versionLine := ""
	if version != "" && height > len(rawLines) {
		versionLine = version
	}
	lines := emptyStateLogoLines(rawLines)
	if len(lines) > height {
		lines = lines[:height]
	}
	contentHeight := len(lines)
	if versionLine != "" {
		contentHeight++
	}
	if contentHeight > height {
		versionLine = ""
		contentHeight = len(lines)
	}
	maxTopPadding := height - contentHeight
	if topPadding > maxTopPadding {
		topPadding = maxTopPadding
	}
	if topPadding < 0 {
		topPadding = 0
	}

	out := make([]string, 0, height)
	for len(out) < topPadding {
		out = append(out, lipgloss.NewStyle().Width(width).Render(""))
	}
	for _, line := range lines {
		styled := renderLogoLine(line.text, logoBlockGradientColor(line.logoIndex), logoGradientColor(emptyStateLogoShadowColors, line.logoIndex))
		out = append(out, lipgloss.PlaceHorizontal(width, lipgloss.Center, styled))
	}
	if versionLine != "" && len(out) < height {
		out = append(out, lipgloss.PlaceHorizontal(width, lipgloss.Center, emptyStateVersionStyle.Render(versionLine)))
	}
	for len(out) < height {
		out = append(out, lipgloss.NewStyle().Width(width).Render(""))
	}
	return strings.Join(out, "\n")
}

func splitRenderedLines(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, "\n")
}

type emptyStateLogoLine struct {
	text      string
	logoIndex int
}

var emptyStateVersionStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("245")).
	Bold(true)

func emptyStateLogoLines(lines []string) []emptyStateLogoLine {
	renderLines := make([]emptyStateLogoLine, 0, len(lines))
	for i, line := range lines {
		renderLines = append(renderLines, emptyStateLogoLine{text: line, logoIndex: i})
	}
	return renderLines
}

func logoBlockGradientColor(idx int) rgbColor {
	if idx < len(emptyStateLogoBlockColors) {
		return emptyStateLogoBlockColors[idx]
	}
	return emptyStateLogoBlockColors[len(emptyStateLogoBlockColors)-1]
}

func logoGradientColor(colors []rgbColor, idx int) rgbColor {
	if idx < len(colors) {
		return colors[idx]
	}
	return colors[len(colors)-1]
}

func renderLogoLine(line string, blockColor rgbColor, shadowColor rgbColor) string {
	var out strings.Builder
	current := ""
	for _, char := range line {
		switch char {
		case ' ':
			out.WriteRune(char)
			continue
		case '█':
			next := renderRGBForegroundCode(blockColor)
			if current != next {
				out.WriteString(next)
				current = next
			}
		default:
			next := renderRGBForegroundCode(shadowColor)
			if current != next {
				out.WriteString(next)
				current = next
			}
		}
		out.WriteRune(char)
	}
	if current != "" {
		out.WriteString("\x1b[39m")
	}
	return out.String()
}

func renderRGBForegroundCode(color rgbColor) string {
	return "\x1b[38;2;" + intString(color.R) + ";" + intString(color.G) + ";" + intString(color.B) + "m"
}

func intString(value int) string {
	return strconv.Itoa(value)
}
