package fetcher

import (
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	nethtml "golang.org/x/net/html"
)

func extractDirectApplyURLFromHTML(rawHTML string, currentApplyURL string) string {
	for _, paragraph := range htmlParagraphs(rawHTML) {
		text := strings.ToLower(normalizeHTMLText(paragraph.HTML))
		if !strings.Contains(text, "apply") {
			continue
		}
		for _, href := range extractHTMLHrefs(paragraph.HTML) {
			if looksLikeDirectApplyURL(href, currentApplyURL) {
				return href
			}
		}
	}
	return ""
}

func looksLikeDirectApplyURL(candidate string, currentApplyURL string) bool {
	parsed, err := url.Parse(candidate)
	if err != nil || parsed.Host == "" {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	if port := parsed.Port(); port != "" && port != "80" && port != "443" {
		return false
	}
	current, err := url.Parse(currentApplyURL)
	if err == nil && strings.EqualFold(parsed.Host, current.Host) {
		return false
	}
	return true
}

func extractHTMLHrefs(rawHTML string) []string {
	doc, err := newHTMLDocument(rawHTML)
	if err != nil {
		return nil
	}
	var hrefs []string
	doc.Find("[href]").Each(func(_ int, selection *goquery.Selection) {
		href, ok := selection.Attr("href")
		if !ok {
			return
		}
		href = strings.TrimSpace(html.UnescapeString(href))
		if href == "" {
			return
		}
		hrefs = appendUniqueString(hrefs, href)
	})
	return hrefs
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if strings.EqualFold(strings.TrimSpace(existing), value) {
			return values
		}
	}
	return append(values, value)
}

func extractJobPostingDescriptionHTML(rawHTML string) string {
	pattern := regexp.MustCompile(`(?s)"description"\s*:\s*("(?:\\.|[^"\\])*")`)
	for _, match := range pattern.FindAllStringSubmatch(rawHTML, -1) {
		if len(match) < 2 {
			continue
		}
		unquoted, err := strconv.Unquote(match[1])
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(unquoted), "about us") || strings.Contains(strings.ToLower(unquoted), "about the company") {
			return html.UnescapeString(unquoted)
		}
	}
	return ""
}

func extractMetaContent(rawHTML string, key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	doc, err := newHTMLDocument(rawHTML)
	if err != nil {
		return ""
	}
	value := ""
	doc.Find("meta").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		name, _ := selection.Attr("name")
		if strings.TrimSpace(name) == "" {
			name, _ = selection.Attr("property")
		}
		name = strings.ToLower(strings.TrimSpace(html.UnescapeString(name)))
		if name != key && name != "og:"+key {
			return true
		}
		content, _ := selection.Attr("content")
		value = normalizeHTMLText(html.UnescapeString(content))
		return false
	})
	return value
}

func normalizeHTMLText(rawHTML string) string {
	text := rawHTML
	if doc, err := newHTMLDocument(rawHTML); err == nil {
		doc.Find("script, style, noscript").Remove()
		text = selectionTextWithSpaces(doc.Selection)
	}
	text = html.UnescapeString(text)
	text = strings.ReplaceAll(text, "\u00a0", " ")
	return strings.TrimSpace(whitespaceRe.ReplaceAllString(text, " "))
}

type htmlParagraph struct {
	HTML string
	Text string
}

func htmlParagraphs(rawHTML string) []htmlParagraph {
	doc, err := newHTMLDocument(rawHTML)
	if err != nil {
		return nil
	}
	var paragraphs []htmlParagraph
	doc.Find("p").Each(func(_ int, selection *goquery.Selection) {
		innerHTML, err := selection.Html()
		if err != nil {
			return
		}
		text := strings.TrimSpace(selectionTextWithSpaces(selection))
		paragraphs = append(paragraphs, htmlParagraph{
			HTML: innerHTML,
			Text: text,
		})
	})
	return paragraphs
}

func extractJSONLDScripts(rawHTML string) []string {
	doc, err := newHTMLDocument(rawHTML)
	if err != nil {
		return nil
	}
	var scripts []string
	doc.Find("script").Each(func(_ int, selection *goquery.Selection) {
		scriptType, _ := selection.Attr("type")
		if !strings.EqualFold(strings.TrimSpace(scriptType), "application/ld+json") {
			return
		}
		content := strings.TrimSpace(selection.Text())
		if content == "" {
			return
		}
		scripts = append(scripts, html.UnescapeString(content))
	})
	return scripts
}

func newHTMLDocument(rawHTML string) (*goquery.Document, error) {
	return goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
}

func selectionTextWithSpaces(selection *goquery.Selection) string {
	var builder strings.Builder
	for _, node := range selection.Nodes {
		appendHTMLNodeText(&builder, node)
	}
	return builder.String()
}

func appendHTMLNodeText(builder *strings.Builder, node *nethtml.Node) {
	switch node.Type {
	case nethtml.TextNode:
		writeHTMLTextPart(builder, node.Data)
	case nethtml.ElementNode:
		if htmlElementSeparatesText(node.Data) {
			ensureHTMLTextSpace(builder)
			defer ensureHTMLTextSpace(builder)
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		appendHTMLNodeText(builder, child)
	}
}

func writeHTMLTextPart(builder *strings.Builder, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	ensureHTMLTextSpace(builder)
	builder.WriteString(text)
}

func ensureHTMLTextSpace(builder *strings.Builder) {
	if builder.Len() == 0 {
		return
	}
	text := builder.String()
	if strings.HasSuffix(text, " ") {
		return
	}
	builder.WriteByte(' ')
}

func htmlElementSeparatesText(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "address", "article", "aside", "blockquote", "br", "dd", "div", "dl", "dt", "figcaption", "footer", "h1", "h2", "h3", "h4", "h5", "h6", "header", "hr", "li", "main", "nav", "ol", "p", "pre", "section", "table", "td", "th", "tr", "ul":
		return true
	default:
		return false
	}
}

func truncateAtSentence(text string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(text))
	if maxRunes <= 0 || len(runes) <= maxRunes {
		return string(runes)
	}
	for i := maxRunes - 1; i >= 0; i-- {
		switch runes[i] {
		case '.', '!', '?':
			if i > 80 {
				return strings.TrimSpace(string(runes[:i+1]))
			}
		}
	}
	if maxRunes > 1 {
		return strings.TrimSpace(string(runes[:maxRunes-1])) + "..."
	}
	return string(runes[:maxRunes])
}
