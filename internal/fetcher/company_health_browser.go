package fetcher

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/wallentx/jobscout/internal/domain"

	"github.com/go-rod/rod"
)

var ErrBrowserNotInstalled = errors.New("browser binary (Chrome/Chromium/Edge) not found")

const (
	companyProfileBrowserTimeout = 25 * time.Second
	companyProfileTextLimit      = 24000
)

type companySiteCandidate struct {
	Query string
	Title string
	URL   string
	Score int
}

type pageLink struct {
	Text string
	URL  string
}

var employerReviewRatingPattern = regexp.MustCompile(`(?i)\b([1-5](?:\.\d)?)\s*(?:out of|/)\s*5\b`)

func FetchBrowserCompanySiteProfile(company string) (*domain.CompanySiteProfile, error) {
	company = strings.TrimSpace(company)
	if company == "" {
		return nil, nil
	}
	if FindSiteSearchBrowserBinary() == "" {
		return nil, ErrBrowserNotInstalled
	}

	browser, cleanup, err := NewSiteSearchBrowser()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), companyProfileBrowserTimeout)
	defer cancel()

	return discoverCompanySiteProfile(ctx, browser, company), nil
}

func FetchBrowserEmployerReviewSignals(company string) ([]domain.EmployerReviewSignal, error) {
	company = strings.TrimSpace(company)
	if company == "" {
		return nil, nil
	}
	if FindSiteSearchBrowserBinary() == "" {
		return nil, ErrBrowserNotInstalled
	}

	browser, cleanup, err := NewSiteSearchBrowser()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), companyProfileBrowserTimeout)
	defer cancel()

	return discoverEmployerReviewSignals(ctx, browser, company), nil
}

func discoverCompanySiteProfile(ctx context.Context, browser *rod.Browser, company string) *domain.CompanySiteProfile {
	if err := ctx.Err(); err != nil {
		return nil
	}
	queries := []string{
		fmt.Sprintf(`"%s" official site`, company),
		fmt.Sprintf(`"%s" company`, company),
	}

	var best *companySiteCandidate
	for _, query := range queries {
		if err := ctx.Err(); err != nil {
			return nil
		}
		candidates, err := searchCompanySiteCandidates(ctx, browser, company, query)
		if err != nil || len(candidates) == 0 {
			continue
		}
		if best == nil || candidates[0].Score > best.Score {
			candidate := candidates[0]
			best = &candidate
		}
		if best.Score >= 12 {
			break
		}
	}
	if best == nil {
		return nil
	}

	profile := &domain.CompanySiteProfile{
		SearchQuery: best.Query,
		SearchURL:   companySearchURL(best.Query),
		WebsiteURL:  canonicalCompanySiteURL(best.URL),
	}
	if profile.WebsiteURL == "" {
		return nil
	}

	websiteText, links, err := extractBrowserPageContent(ctx, browser, profile.WebsiteURL)
	if err != nil {
		return profile
	}
	profile.WebsiteText = websiteText

	if aboutURL := chooseCompanyAboutURL(profile.WebsiteURL, links); aboutURL != "" {
		profile.AboutURL = aboutURL
		aboutText, _, err := extractBrowserPageContent(ctx, browser, aboutURL)
		if err == nil {
			profile.AboutText = aboutText
		}
	}

	return profile
}

func discoverEmployerReviewSignals(ctx context.Context, browser *rod.Browser, company string) []domain.EmployerReviewSignal {
	queries := []string{
		fmt.Sprintf(`"%s" Glassdoor reviews rating`, company),
		fmt.Sprintf(`"%s" Indeed company reviews rating`, company),
	}

	var signals []domain.EmployerReviewSignal
	for _, query := range queries {
		searchURL := companySearchURL(query)
		pageText, links, err := extractBrowserPageContent(ctx, browser, searchURL)
		if err != nil {
			continue
		}
		for _, link := range links {
			source := employerReviewSource(link.URL)
			if source == "" {
				continue
			}
			signal := employerReviewSignalFromSearchResult(source, pageText, link)
			if signal.Title == "" {
				continue
			}
			signals = append(signals, signal)
			break
		}
	}

	return dedupeEmployerReviewSignals(signals)
}

func searchCompanySiteCandidates(ctx context.Context, browser *rod.Browser, company string, query string) ([]companySiteCandidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	searchURL := companySearchURL(query)
	pageText, links, err := extractBrowserPageContent(ctx, browser, searchURL)
	if err != nil {
		return nil, err
	}
	if pageText == "" && len(links) == 0 {
		return nil, nil
	}

	candidates := make([]companySiteCandidate, 0, len(links))
	for _, link := range links {
		score := scoreCompanySiteCandidate(company, link.Text, link.URL)
		if score <= 0 {
			continue
		}
		candidates = append(candidates, companySiteCandidate{
			Query: query,
			Title: link.Text,
			URL:   link.URL,
			Score: score,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].URL < candidates[j].URL
		}
		return candidates[i].Score > candidates[j].Score
	})

	if len(candidates) > 20 {
		candidates = candidates[:20]
	}
	return candidates, nil
}

func companySearchURL(query string) string {
	return "https://duckduckgo.com/html/?q=" + url.QueryEscape(query)
}

func extractBrowserPageContent(ctx context.Context, browser *rod.Browser, pageURL string) (string, []pageLink, error) {
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}

	var (
		pageText string
		links    []pageLink
	)

	err := rod.Try(func() {
		timeout := SiteSearchBrowserTimeout
		if deadline, ok := ctx.Deadline(); ok {
			if remaining := time.Until(deadline); remaining <= 0 {
				panic(ctx.Err())
			} else if remaining < timeout {
				timeout = remaining
			}
		}

		page := browser.Timeout(timeout).MustPage(pageURL)
		defer page.MustClose()

		page.MustWaitLoad()
		time.Sleep(SiteSearchSettleDelay)

		info := page.MustInfo()
		baseURL, err := url.Parse(info.URL)
		if err != nil {
			baseURL, err = url.Parse(pageURL)
			if err != nil {
				panic(err)
			}
		}

		body := page.MustElement("body")
		pageText = truncateBrowserText(NormalizeWhitespace(body.MustText()))

		elements := page.MustElements("a[href]")
		rawLinks := make([]pageLink, 0, len(elements))
		for _, element := range elements {
			text := NormalizeWhitespace(element.MustText())
			href := strings.TrimSpace(DerefString(element.MustAttribute("href")))
			if href == "" {
				continue
			}

			resolved, err := baseURL.Parse(href)
			if err != nil {
				continue
			}

			linkURL := decodeSearchResultURL(resolved.String())
			if linkURL == "" {
				continue
			}

			rawLinks = append(rawLinks, pageLink{
				Text: text,
				URL:  linkURL,
			})
		}

		links = dedupePageLinks(rawLinks)
	})
	if err != nil {
		return "", nil, SimplifySiteSearchError(err)
	}

	return pageText, links, nil
}

func dedupePageLinks(links []pageLink) []pageLink {
	seen := make(map[string]pageLink, len(links))
	order := make([]string, 0, len(links))

	for _, link := range links {
		key := link.URL
		if key == "" {
			continue
		}
		existing, ok := seen[key]
		if ok {
			if existing.Text == "" && link.Text != "" {
				seen[key] = link
			}
			continue
		}
		seen[key] = link
		order = append(order, key)
	}

	out := make([]pageLink, 0, len(order))
	for _, key := range order {
		out = append(out, seen[key])
	}
	return out
}

func employerReviewSource(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}

	host := strings.ToLower(parsed.Hostname())
	switch {
	case host == "glassdoor.com" || strings.HasSuffix(host, ".glassdoor.com"):
		return "glassdoor"
	case host == "indeed.com" || strings.HasSuffix(host, ".indeed.com"):
		return "indeed"
	default:
		return ""
	}
}

func employerReviewSignalFromSearchResult(source string, pageText string, link pageLink) domain.EmployerReviewSignal {
	title := NormalizeWhitespace(link.Text)
	snippet := employerReviewSnippet(pageText, title, source)
	rating := extractEmployerReviewRating(strings.Join([]string{title, snippet}, " "))

	return domain.EmployerReviewSignal{
		Source:  source,
		Title:   title,
		URL:     link.URL,
		Rating:  rating,
		Snippet: snippet,
		Flags:   employerReviewFlags(strings.Join([]string{title, snippet}, " ")),
	}
}

func employerReviewSnippet(pageText string, title string, source string) string {
	pageText = NormalizeWhitespace(pageText)
	if pageText == "" {
		return ""
	}

	index := -1
	for _, needle := range []string{title, source} {
		needle = strings.TrimSpace(needle)
		if needle == "" {
			continue
		}
		index = strings.Index(strings.ToLower(pageText), strings.ToLower(needle))
		if index >= 0 {
			break
		}
	}
	if index < 0 {
		return ""
	}

	start := max(0, index-80)
	end := min(len(pageText), index+360)
	return strings.TrimSpace(pageText[start:end])
}

func extractEmployerReviewRating(text string) string {
	match := employerReviewRatingPattern.FindStringSubmatch(text)
	if len(match) != 2 {
		return ""
	}
	return match[1] + "/5"
}

func employerReviewFlags(text string) []string {
	text = strings.ToLower(text)
	flagTerms := map[string]string{
		"burnout":       "burnout",
		"toxic":         "toxic culture",
		"poor culture":  "poor culture",
		"long hours":    "long hours",
		"low pay":       "low pay",
		"layoff":        "layoff mentions",
		"work life":     "work-life balance",
		"wlb":           "work-life balance",
		"great culture": "positive culture",
		"good culture":  "positive culture",
	}

	flags := []string{}
	seen := map[string]bool{}
	for term, flag := range flagTerms {
		if !strings.Contains(text, term) || seen[flag] {
			continue
		}
		seen[flag] = true
		flags = append(flags, flag)
	}
	sort.Strings(flags)
	return flags
}

func dedupeEmployerReviewSignals(signals []domain.EmployerReviewSignal) []domain.EmployerReviewSignal {
	seen := make(map[string]bool, len(signals))
	out := make([]domain.EmployerReviewSignal, 0, len(signals))
	for _, signal := range signals {
		key := signal.Source + "|" + strings.ToLower(signal.URL)
		if key == "|" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, signal)
	}
	return out
}

func truncateBrowserText(text string) string {
	text = strings.TrimSpace(text)
	if len(text) <= companyProfileTextLimit {
		return text
	}
	return strings.TrimSpace(text[:companyProfileTextLimit])
}

func decodeSearchResultURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	host := strings.ToLower(parsed.Hostname())
	if strings.Contains(host, "duckduckgo.") && parsed.Path == "/l/" {
		target := strings.TrimSpace(parsed.Query().Get("uddg"))
		if target != "" {
			return target
		}
	}

	parsed.Fragment = ""
	return parsed.String()
}

func scoreCompanySiteCandidate(company string, title string, rawURL string) int {
	candidateURL := decodeSearchResultURL(rawURL)
	if candidateURL == "" {
		return 0
	}

	parsed, err := url.Parse(candidateURL)
	if err != nil {
		return 0
	}
	host := strings.ToLower(parsed.Hostname())
	if companySearchHostExcluded(host) {
		return 0
	}

	titleLower := strings.ToLower(strings.TrimSpace(title))
	pathLower := strings.ToLower(strings.TrimSpace(parsed.EscapedPath()))
	companyTokens := companySearchTokens(company)
	score := 0

	if pathLower == "" || pathLower == "/" {
		score += 5
	}
	if len(pathLower) > 0 && pathLower != "/" && strings.Count(pathLower, "/") <= 1 {
		score += 1
	}
	if strings.Contains(titleLower, "official") {
		score += 3
	}
	if strings.Contains(titleLower, "about") || strings.Contains(pathLower, "about") {
		score += 2
	}
	if strings.Contains(pathLower, "careers") || strings.Contains(pathLower, "jobs") {
		score -= 3
	}
	if strings.Contains(pathLower, "news") || strings.Contains(pathLower, "blog") {
		score -= 2
	}

	for _, token := range companyTokens {
		if strings.Contains(titleLower, token) {
			score += 2
		}
		if strings.Contains(host, token) {
			score += 3
		}
	}

	hostSlug := strings.ReplaceAll(host, "-", "")
	companySlug := strings.ReplaceAll(Slugify(company), "-", "")
	if companySlug != "" && strings.Contains(hostSlug, companySlug) {
		score += 6
	}
	if !companyHostMatchesName(host, company) {
		if !strings.Contains(titleLower, "official") {
			return 0
		}
		score -= 6
	}

	return score
}

func companySearchTokens(company string) []string {
	cleaned := CleanCompanyName(company)
	parts := strings.FieldsFunc(cleaned, func(r rune) bool {
		return r == ' ' || r == '-' || r == '_' || r == '.'
	})

	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))
		if len(part) < 3 {
			continue
		}
		out = append(out, part)
	}
	return out
}

func companySearchHostExcluded(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return true
	}

	excluded := []string{
		"duckduckgo.com",
		"google.com",
		"bing.com",
		"linkedin.com",
		"wikipedia.org",
		"facebook.com",
		"instagram.com",
		"x.com",
		"twitter.com",
		"youtube.com",
		"glassdoor.com",
		"indeed.com",
		"crunchbase.com",
		"pitchbook.com",
		"news.google.com",
		"finance.yahoo.com",
		"sec.gov",
		"builtin.com",
		"greenhouse.io",
		"lever.co",
		"myworkdayjobs.com",
		"smartrecruiters.com",
	}
	for _, value := range excluded {
		if host == value || strings.HasSuffix(host, "."+value) {
			return true
		}
	}
	return false
}

func canonicalCompanySiteURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return ""
	}
	return (&url.URL{
		Scheme: parsed.Scheme,
		Host:   parsed.Host,
		Path:   "/",
	}).String()
}

func chooseCompanyAboutURL(siteURL string, links []pageLink) string {
	baseURL, err := url.Parse(siteURL)
	if err != nil {
		return ""
	}

	bestURL := ""
	bestScore := 0
	for _, link := range links {
		score := scoreCompanyAboutLink(baseURL, link)
		if score > bestScore {
			bestScore = score
			bestURL = link.URL
		}
	}
	if bestScore < 4 {
		return ""
	}
	return bestURL
}

func scoreCompanyAboutLink(baseURL *url.URL, link pageLink) int {
	parsed, err := url.Parse(link.URL)
	if err != nil {
		return 0
	}
	if !strings.EqualFold(parsed.Hostname(), baseURL.Hostname()) {
		return 0
	}

	pathLower := strings.ToLower(strings.TrimSpace(path.Clean(parsed.Path)))
	textLower := strings.ToLower(strings.TrimSpace(link.Text))
	if pathLower == "." {
		pathLower = "/"
	}

	for _, excluded := range []string{"jobs", "careers", "blog", "news", "privacy", "terms", "legal", "contact"} {
		if strings.Contains(pathLower, excluded) || strings.Contains(textLower, excluded) {
			return 0
		}
	}

	score := 0
	for _, keyword := range []string{"about", "about-us", "company", "our-story", "who-we-are", "team", "mission"} {
		if strings.Contains(pathLower, keyword) {
			score += 3
		}
		if strings.Contains(textLower, strings.ReplaceAll(keyword, "-", " ")) {
			score += 2
		}
	}
	if pathLower == "/about" || pathLower == "/about-us" {
		score += 4
	}
	if len(pathLower) > 0 && strings.Count(pathLower, "/") <= 2 {
		score++
	}
	return score
}
