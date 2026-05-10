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
	rawLines := strings.Split(strings.Trim(emptyStateLogo, "\n"), "\n")
	lines, versionLine := emptyStateLogoLinesWithVersion(rawLines, width, height, version)
	if len(lines) > height {
		lines = lines[:height]
	}

	topPadding := (height - len(lines)) / 2
	if topPadding < 0 {
		topPadding = 0
	}

	out := make([]string, 0, height)
	for len(out) < topPadding {
		out = append(out, lipgloss.NewStyle().Width(width).Render(""))
	}
	for _, line := range lines {
		styled := renderLogoLine(line.text, logoBlockGradientColor(line.logoIndex), logoGradientColor(emptyStateLogoShadowColors, line.logoIndex))
		if line.version != "" {
			styled += "  " + emptyStateVersionStyle.Render(line.version)
		}
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

type emptyStateLogoLine struct {
	text      string
	logoIndex int
	version   string
}

var emptyStateVersionStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("245")).
	Bold(true)

func emptyStateLogoLinesWithVersion(lines []string, width int, height int, version string) ([]emptyStateLogoLine, string) {
	renderLines := make([]emptyStateLogoLine, 0, len(lines))
	for i, line := range lines {
		renderLines = append(renderLines, emptyStateLogoLine{text: line, logoIndex: i})
	}

	version = strings.TrimSpace(version)
	if version == "" {
		return renderLines, ""
	}
	logoWidth := 0
	for _, line := range lines {
		if lineWidth := lipgloss.Width(strings.TrimRight(line, " ")); lineWidth > logoWidth {
			logoWidth = lineWidth
		}
	}
	if logoWidth+2+lipgloss.Width(version) <= width {
		idx := len(renderLines) / 2
		renderLines[idx].text = strings.TrimRight(renderLines[idx].text, " ")
		renderLines[idx].version = version
		return renderLines, ""
	}
	if height > len(lines) {
		return renderLines, version
	}
	return renderLines, ""
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
