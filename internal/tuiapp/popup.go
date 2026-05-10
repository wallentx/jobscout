package tuiapp

import (
	"strings"

	"github.com/wallentx/jobscout/internal/fetcher"
	"github.com/wallentx/jobscout/internal/tui"

	"github.com/charmbracelet/lipgloss"
)

type popupLineStyler func(string) string

type popupViewport struct {
	content   string
	maxOffset int
}

type popupBodyKind int

const (
	popupBodyNone popupBodyKind = iota
	popupBodyText
	popupBodyScrollableText
	popupBodyForm
)

type popupBodySection struct {
	kind    popupBodyKind
	content string
}

type popupSpec struct {
	width     int
	x         *int
	y         *int
	title     string
	titleView string
	header    string
	body      popupBodySection
	menu      []popupMenuItem
	footer    string
}

type popupMenuItem struct {
	Prefix   string
	Label    string
	Detail   string
	Disabled bool
	Selected bool
}

type popupDropdownSpec struct {
	Label       string
	Value       string
	Items       []string
	LabelWidth  int
	Width       int
	MaxOpenRows int
	SelectedIdx int
	Open        bool
	Focused     bool
}

type popupDropdownOverlay struct {
	content string
	width   int
	height  int
}

func clampPopupWidth(termWidth int, minWidth int, maxWidth int) int {
	width := termWidth - 8
	if maxWidth > 0 && width > maxWidth {
		width = maxWidth
	}
	if width < minWidth {
		width = minWidth
	}
	return width
}

func popupMaxViewportLines(termHeight int, minLines int) int {
	return popupMaxViewportLinesWithChrome(termHeight, minLines, 0)
}

func popupMaxViewportLinesWithChrome(termHeight int, minLines int, chromeLines int) int {
	lines := calculateTableHeight(termHeight) + 2 - chromeLines
	if lines < minLines {
		lines = minLines
	}
	if lines < 1 {
		lines = 1
	}
	return lines
}

func fitPopupWidth(termWidth int, minWidth int, maxWidth int, sections ...string) int {
	width := minWidth
	for _, section := range sections {
		for _, line := range strings.Split(section, "\n") {
			lineWidth := lipgloss.Width(line) + 6
			if lineWidth > width {
				width = lineWidth
			}
		}
	}

	maxAllowed := clampPopupWidth(termWidth, minWidth, maxWidth)
	if width > maxAllowed {
		return maxAllowed
	}
	return width
}

func renderPopupWithTitleAndBackgroundDimming(baseView string, termWidth int, termHeight int, width int, x *int, y *int, titleView string, title string, content string, dimmedBackground bool) string {
	var body strings.Builder
	switch {
	case strings.TrimSpace(titleView) != "":
		body.WriteString(titleView)
		body.WriteString("\n\n")
	case strings.TrimSpace(title) != "":
		body.WriteString(popupTitleStyle.Render(title))
		body.WriteString("\n\n")
	}
	body.WriteString(content)

	dialog := lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2).
		Render(body.String())
	if dimmedBackground {
		dialog = popupDialogForDimmedBackground(dialog)
	}

	posX := termWidth/2 - lipgloss.Width(dialog)/2
	posY := termHeight/2 - lipgloss.Height(dialog)/2 - 2
	if x != nil {
		posX = *x
	}
	if y != nil {
		posY = *y
	}
	if posY < 0 {
		posY = 0
	}
	return lipgloss.PlaceOverlay(
		posX,
		posY,
		dialog,
		baseView,
	)
}

func renderPopupSpec(baseView string, termWidth int, termHeight int, spec popupSpec) string {
	return renderPopupSpecWithBackgroundDimming(baseView, termWidth, termHeight, spec, false)
}

func renderPopupSpecWithBackgroundDimming(baseView string, termWidth int, termHeight int, spec popupSpec, dimmedBackground bool) string {
	var content strings.Builder
	contentWidth := spec.width - 6
	if contentWidth < 10 {
		contentWidth = 10
	}
	if strings.TrimSpace(spec.header) != "" {
		content.WriteString(spec.header)
	}
	if strings.TrimSpace(spec.body.content) != "" {
		if content.Len() > 0 {
			content.WriteString("\n\n")
		}
		content.WriteString(renderPopupBody(spec.body, contentWidth))
	}
	if len(spec.menu) > 0 {
		if content.Len() > 0 {
			content.WriteString("\n\n")
		}
		content.WriteString(renderPopupMenu(spec.menu))
	}
	if strings.TrimSpace(spec.footer) != "" {
		if content.Len() > 0 {
			content.WriteString("\n\n\n")
		}
		content.WriteString(renderPopupWrappedContent(spec.footer, contentWidth))
	}
	return renderPopupWithTitleAndBackgroundDimming(baseView, termWidth, termHeight, spec.width, spec.x, spec.y, spec.titleView, spec.title, content.String(), dimmedBackground)
}

func dimPopupBackground(view string) string {
	if view == "" {
		return view
	}
	dimmed := strings.ReplaceAll(view, "\x1b[0m", "\x1b[0m\x1b[2m")
	dimmed = strings.ReplaceAll(dimmed, "\x1b[22m", "\x1b[22m\x1b[2m")
	return "\x1b[2m" + dimmed + "\x1b[22m"
}

func popupDialogForDimmedBackground(dialog string) string {
	if dialog == "" {
		return dialog
	}
	lines := strings.Split(dialog, "\n")
	for i, line := range lines {
		lines[i] = "\x1b[22m" + line + "\x1b[2m"
	}
	return strings.Join(lines, "\n")
}

func renderPopupBody(body popupBodySection, width int) string {
	switch body.kind {
	case popupBodyText, popupBodyForm:
		return renderPopupWrappedContent(body.content, width)
	case popupBodyScrollableText:
		return body.content
	default:
		return renderPopupWrappedContent(body.content, width)
	}
}

func popupTextBody(content string) popupBodySection {
	return popupBodySection{kind: popupBodyText, content: content}
}

func popupScrollableTextBody(content string) popupBodySection {
	return popupBodySection{kind: popupBodyScrollableText, content: content}
}

func popupFormBody(content string) popupBodySection {
	return popupBodySection{kind: popupBodyForm, content: content}
}

func renderPopupWrappedContent(content string, width int) string {
	return tui.RenderPopupWrappedContent(content, width)
}

func renderPopupMenu(items []popupMenuItem) string {
	return tui.RenderPopupMenu(tuiPopupMenuItems(items))
}

func renderPopupDropdown(spec popupDropdownSpec) string {
	return tui.RenderPopupDropdown(tuiPopupDropdownSpec(spec))
}

func renderPopupDropdownOverlay(spec popupDropdownSpec) popupDropdownOverlay {
	overlay := tui.RenderPopupDropdownOverlay(tuiPopupDropdownSpec(spec))
	return popupDropdownOverlay{
		content: overlay.Content,
		width:   overlay.Width,
		height:  overlay.Height,
	}
}

func normalizeDropdownSelectedIdx(selectedIdx int, itemCount int) int {
	return tui.NormalizeDropdownSelectedIdx(selectedIdx, itemCount)
}

func padVisibleRight(text string, width int) string {
	return tui.PadVisibleRight(text, width)
}

func renderPopupMenuViewport(items []popupMenuItem, width int, maxLines int, selectedIdx int) popupViewport {
	return popupViewportFromTUI(tui.RenderPopupMenuViewport(tuiPopupMenuItems(items), width, maxLines, selectedIdx))
}

func popupMenuScrollOffset(selectedIdx int, itemCount int, maxLines int) int {
	return tui.PopupMenuScrollOffset(selectedIdx, itemCount, maxLines)
}

func tuiPopupMenuItems(items []popupMenuItem) []tui.PopupMenuItem {
	out := make([]tui.PopupMenuItem, 0, len(items))
	for _, item := range items {
		out = append(out, tuiPopupMenuItem(item))
	}
	return out
}

func tuiPopupMenuItem(item popupMenuItem) tui.PopupMenuItem {
	return tui.PopupMenuItem{
		Prefix:   item.Prefix,
		Label:    item.Label,
		Detail:   item.Detail,
		Disabled: item.Disabled,
		Selected: item.Selected,
	}
}

func tuiPopupDropdownSpec(spec popupDropdownSpec) tui.PopupDropdownSpec {
	return tui.PopupDropdownSpec{
		Label:       spec.Label,
		Value:       spec.Value,
		Items:       spec.Items,
		LabelWidth:  spec.LabelWidth,
		Width:       spec.Width,
		MaxOpenRows: spec.MaxOpenRows,
		SelectedIdx: spec.SelectedIdx,
		Open:        spec.Open,
		Focused:     spec.Focused,
	}
}

func renderScrollablePopupText(text string, width int, maxLines int, offset int, styler popupLineStyler) popupViewport {
	if styler == nil {
		styler = defaultPopupLineStyle
	}
	return popupViewportFromTUI(tui.RenderScrollablePopupText(text, width, maxLines, offset, tui.LineStyler(styler)))
}

func renderScrollablePopupTextWithInheritedStyle(text string, width int, maxLines int, offset int, stylerForOrigin func(string) popupLineStyler) popupViewport {
	if stylerForOrigin == nil {
		stylerForOrigin = func(string) popupLineStyler {
			return defaultPopupLineStyle
		}
	}
	return popupViewportFromTUI(tui.RenderScrollablePopupTextWithInheritedStyle(text, width, maxLines, offset, func(origin string) tui.LineStyler {
		styleLine := stylerForOrigin(origin)
		if styleLine == nil {
			styleLine = defaultPopupLineStyle
		}
		return tui.LineStyler(styleLine)
	}))
}

func structuredPopupLines(text string, width int) []string {
	return tui.StructuredPopupLines(text, width)
}

func defaultPopupLineStyle(line string) string {
	if line == "" {
		return ""
	}
	return popupBodyStyle.Render(line)
}

func wrapANSIWords(line string, width int) []string {
	return tui.WrapANSIWords(line, width)
}

func renderPopupViewport(lines []string, width int, maxLines int, offset int, styler popupLineStyler) popupViewport {
	if styler == nil {
		styler = defaultPopupLineStyle
	}
	return popupViewportFromTUI(tui.RenderPopupViewport(lines, width, maxLines, offset, tui.LineStyler(styler)))
}

func clampPopupScroll(offset int, maxOffset int) int {
	return tui.ClampPopupScroll(offset, maxOffset)
}

func padPopupViewportLine(line string, width int) string {
	return tui.PadPopupViewportLine(line, width)
}

func popupViewportFromTUI(viewport tui.PopupViewport) popupViewport {
	return popupViewport{
		content:   viewport.Content,
		maxOffset: viewport.MaxOffset,
	}
}

func noticePopupLineStyle(line string) string {
	return noticePopupLineStyleAs(line, line)
}

func noticePopupLineStylerForOrigin(origin string) popupLineStyler {
	return func(line string) string {
		return noticePopupLineStyleAs(origin, line)
	}
}

func noticePopupLineStyleAs(origin string, line string) string {
	trimmed := strings.TrimSpace(origin)
	render := func(style lipgloss.Style) string {
		return renderNoticePopupLineWithURLSegments(origin, line, style)
	}
	switch {
	case origin == "":
		return ""
	case origin == "Added":
		return render(lipgloss.NewStyle().Foreground(lipgloss.Color("114")).Bold(true))
	case origin == "Rejected":
		return render(lipgloss.NewStyle().Foreground(lipgloss.Color("209")).Bold(true))
	case origin == "Filtered":
		return render(lipgloss.NewStyle().Foreground(lipgloss.Color("221")).Bold(true))
	case origin == "Searches":
		return render(lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true))
	case origin == "Notes":
		return render(lipgloss.NewStyle().Foreground(lipgloss.Color("110")).Bold(true))
	case isFetchSearchHeading(trimmed):
		return render(lipgloss.NewStyle().Foreground(lipgloss.Color("110")).Bold(true))
	case strings.HasPrefix(origin, "        - "):
		return render(lipgloss.NewStyle().Foreground(lipgloss.Color("221")))
	case strings.HasPrefix(origin, "          ") && !strings.HasSuffix(origin, ")"):
		return render(lipgloss.NewStyle().Foreground(lipgloss.Color("221")))
	case strings.HasPrefix(origin, "      - "):
		return render(lipgloss.NewStyle().Foreground(lipgloss.Color("210")))
	case strings.HasPrefix(origin, "        ") && !strings.HasSuffix(origin, ")"):
		return render(lipgloss.NewStyle().Foreground(lipgloss.Color("210")))
	case strings.HasPrefix(origin, "    + "):
		return render(lipgloss.NewStyle().Foreground(lipgloss.Color("151")))
	case strings.HasPrefix(origin, "      ") && !strings.HasSuffix(origin, ")"):
		return render(lipgloss.NewStyle().Foreground(lipgloss.Color("151")))
	case strings.HasPrefix(origin, "    - "):
		return render(lipgloss.NewStyle().Foreground(lipgloss.Color("221")))
	case strings.HasPrefix(origin, "    skipped:"):
		return render(lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true))
	case strings.HasPrefix(origin, "    failed:"):
		return render(lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Italic(true))
	case strings.HasPrefix(origin, "    no "):
		return render(lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true))
	case strings.HasPrefix(origin, "    ") && strings.HasSuffix(origin, ")"):
		return render(lipgloss.NewStyle().Foreground(lipgloss.Color("216")).Bold(true))
	case strings.HasPrefix(origin, "  ") && strings.HasSuffix(origin, ")"):
		return render(lipgloss.NewStyle().Foreground(lipgloss.Color("216")).Bold(true))
	case strings.HasPrefix(origin, "  "):
		return render(lipgloss.NewStyle().Foreground(lipgloss.Color("248")))
	default:
		return render(popupBodyStyle)
	}
}

func renderNoticePopupLineWithURLSegments(origin string, line string, baseStyle lipgloss.Style) string {
	ranges := popupURLRanges(line)
	if len(ranges) == 0 {
		if popupOriginStartsWithURL(origin) {
			return renderURLContinuationLine(line, baseStyle)
		}
		return renderToken(baseStyle, line)
	}

	urlStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("210"))
	var out strings.Builder
	last := 0
	for _, r := range ranges {
		if r.start > last {
			out.WriteString(renderToken(baseStyle, line[last:r.start]))
		}
		out.WriteString(renderToken(urlStyle, line[r.start:r.end]))
		last = r.end
	}
	if last < len(line) {
		out.WriteString(renderToken(baseStyle, line[last:]))
	}
	return out.String()
}

type popupURLRange struct {
	start int
	end   int
}

func popupURLRanges(line string) []popupURLRange {
	var ranges []popupURLRange
	for pos := 0; pos < len(line); {
		start := nextPopupURLStart(line, pos)
		if start < 0 {
			break
		}
		end := start
		for end < len(line) && !isPopupURLTerminator(line[end]) {
			end++
		}
		if end > start {
			ranges = append(ranges, popupURLRange{start: start, end: end})
		}
		pos = end
	}
	return ranges
}

func nextPopupURLStart(line string, pos int) int {
	httpIdx := strings.Index(line[pos:], "http://")
	httpsIdx := strings.Index(line[pos:], "https://")
	switch {
	case httpIdx < 0 && httpsIdx < 0:
		return -1
	case httpIdx < 0:
		return pos + httpsIdx
	case httpsIdx < 0:
		return pos + httpIdx
	case httpIdx < httpsIdx:
		return pos + httpIdx
	default:
		return pos + httpsIdx
	}
}

func isPopupURLTerminator(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func popupOriginStartsWithURL(origin string) bool {
	trimmed := strings.TrimSpace(origin)
	return strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://")
}

func renderURLContinuationLine(line string, baseStyle lipgloss.Style) string {
	indentLen := len(line) - len(strings.TrimLeft(line, " "))
	if indentLen >= len(line) {
		return renderToken(baseStyle, line)
	}
	rest := line[indentLen:]
	tokenEnd := strings.IndexAny(rest, " \t")
	if tokenEnd < 0 {
		tokenEnd = len(rest)
	}
	token := rest[:tokenEnd]
	if !looksLikeURLContinuationToken(token) {
		return renderToken(baseStyle, line)
	}

	urlStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("210"))
	var out strings.Builder
	out.WriteString(renderToken(baseStyle, line[:indentLen]))
	out.WriteString(renderToken(urlStyle, token))
	if tokenEnd < len(rest) {
		out.WriteString(renderToken(baseStyle, rest[tokenEnd:]))
	}
	return out.String()
}

func looksLikeURLContinuationToken(token string) bool {
	return strings.ContainsAny(token, "./?&=%+#:-_")
}

func isFetchSearchHeading(line string) bool {
	for _, searchType := range fetcher.FetchSearchKinds() {
		if line == searchType || strings.HasPrefix(line, searchType+" ") || strings.HasPrefix(line, searchType+":") {
			return true
		}
	}
	return false
}
