package fetcher

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

const browserDebugSnapshotTimeout = 3 * time.Second

var debugFilenameUnsafePattern = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

type browserPageDebugMetadata struct {
	Scope          string   `json:"scope"`
	Status         string   `json:"status"`
	RequestedURL   string   `json:"requested_url"`
	FinalURL       string   `json:"final_url,omitempty"`
	Title          string   `json:"title,omitempty"`
	DurationMillis int64    `json:"duration_ms"`
	Error          string   `json:"error,omitempty"`
	CapturedAt     string   `json:"captured_at"`
	HTMLFile       string   `json:"html_file,omitempty"`
	TextFile       string   `json:"text_file,omitempty"`
	LinksFile      string   `json:"links_file,omitempty"`
	HTMLBytes      int      `json:"html_bytes,omitempty"`
	TextBytes      int      `json:"text_bytes,omitempty"`
	Links          int      `json:"links,omitempty"`
	CaptureErrors  []string `json:"capture_errors,omitempty"`
}

func captureBrowserPageDebugSnapshot(page *rod.Page, requestedURL string, scope string, duration time.Duration, runErr error) {
	enabled, debugPath := debugSettings()
	if !enabled || page == nil {
		return
	}

	status := "ok"
	errText := ""
	if runErr != nil {
		status = "error"
		errText = SimplifySiteSearchError(runErr).Error()
	}

	metadata := browserPageDebugMetadata{
		Scope:          strings.TrimSpace(scope),
		Status:         status,
		RequestedURL:   requestedURL,
		DurationMillis: duration.Milliseconds(),
		Error:          errText,
		CapturedAt:     time.Now().Format(time.RFC3339Nano),
	}

	snapshotPage := page.Timeout(browserDebugSnapshotTimeout)
	defer snapshotPage.CancelTimeout()

	var (
		html  string
		text  string
		links []pageLink
	)

	if err := rod.Try(func() {
		info, err := snapshotPage.Info()
		if err != nil {
			metadata.CaptureErrors = append(metadata.CaptureErrors, fmt.Sprintf("page info: %v", err))
		} else if info != nil {
			metadata.FinalURL = info.URL
			metadata.Title = info.Title
		}
	}); err != nil {
		metadata.CaptureErrors = append(metadata.CaptureErrors, fmt.Sprintf("page info: %v", err))
	}

	if err := rod.Try(func() {
		var err error
		html, err = snapshotPage.HTML()
		if err != nil {
			metadata.CaptureErrors = append(metadata.CaptureErrors, fmt.Sprintf("html: %v", err))
		}
	}); err != nil {
		metadata.CaptureErrors = append(metadata.CaptureErrors, fmt.Sprintf("html: %v", err))
	}

	if err := rod.Try(func() {
		body, err := snapshotPage.Element("body")
		if err != nil {
			metadata.CaptureErrors = append(metadata.CaptureErrors, fmt.Sprintf("body: %v", err))
			return
		}
		bodyText, err := body.Text()
		if err != nil {
			metadata.CaptureErrors = append(metadata.CaptureErrors, fmt.Sprintf("body text: %v", err))
			return
		}
		text = NormalizeWhitespace(bodyText)
	}); err != nil {
		metadata.CaptureErrors = append(metadata.CaptureErrors, fmt.Sprintf("body text: %v", err))
	}

	if err := rod.Try(func() {
		elements, err := snapshotPage.Elements("a[href]")
		if err != nil {
			metadata.CaptureErrors = append(metadata.CaptureErrors, fmt.Sprintf("links: %v", err))
			return
		}
		baseURL := requestedURL
		if metadata.FinalURL != "" {
			baseURL = metadata.FinalURL
		}
		links = extractDebugPageLinks(elements, baseURL)
	}); err != nil {
		metadata.CaptureErrors = append(metadata.CaptureErrors, fmt.Sprintf("links: %v", err))
	}

	prefix := debugBrowserPageArtifactPrefix(debugPath, requestedURL, status)
	if html != "" {
		path, err := writeDebugArtifact(prefix+".html", []byte(html))
		if err != nil {
			metadata.CaptureErrors = append(metadata.CaptureErrors, fmt.Sprintf("write html: %v", err))
		} else {
			metadata.HTMLFile = path
			metadata.HTMLBytes = len(html)
		}
	}
	if text != "" {
		path, err := writeDebugArtifact(prefix+".txt", []byte(text))
		if err != nil {
			metadata.CaptureErrors = append(metadata.CaptureErrors, fmt.Sprintf("write text: %v", err))
		} else {
			metadata.TextFile = path
			metadata.TextBytes = len(text)
		}
	}
	if len(links) > 0 {
		linkBytes, err := json.MarshalIndent(links, "", "  ")
		if err != nil {
			metadata.CaptureErrors = append(metadata.CaptureErrors, fmt.Sprintf("marshal links: %v", err))
		} else if path, err := writeDebugArtifact(prefix+".links.json", linkBytes); err != nil {
			metadata.CaptureErrors = append(metadata.CaptureErrors, fmt.Sprintf("write links: %v", err))
		} else {
			metadata.LinksFile = path
			metadata.Links = len(links)
		}
	}

	metaBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		logDebug("browser page snapshot metadata failed scope=%q status=%s url=%q error=%v", scope, status, requestedURL, err)
		return
	}
	metaPath, err := writeDebugArtifact(prefix+".json", metaBytes)
	if err != nil {
		logDebug("browser page snapshot write failed scope=%q status=%s url=%q error=%v", scope, status, requestedURL, err)
		return
	}
	logDebug(
		"browser page snapshot saved scope=%q status=%s url=%q metadata=%q html_chars=%d text_chars=%d links=%d capture_errors=%d",
		scope,
		status,
		requestedURL,
		metaPath,
		len(html),
		len(text),
		len(links),
		len(metadata.CaptureErrors),
	)
}

func extractDebugPageLinks(elements rod.Elements, baseRawURL string) []pageLink {
	baseURL, err := url.Parse(baseRawURL)
	if err != nil {
		baseURL = nil
	}
	rawLinks := make([]pageLink, 0, len(elements))
	for _, element := range elements {
		text, err := element.Text()
		if err != nil {
			text = ""
		}
		href, err := element.Attribute("href")
		if err != nil || href == nil || strings.TrimSpace(*href) == "" {
			continue
		}
		resolved := strings.TrimSpace(*href)
		if baseURL != nil {
			if parsed, err := baseURL.Parse(resolved); err == nil {
				resolved = parsed.String()
			}
		}
		resolved = decodeSearchResultURL(resolved)
		if resolved == "" {
			continue
		}
		rawLinks = append(rawLinks, pageLink{
			Text: NormalizeWhitespace(text),
			URL:  resolved,
		})
	}
	return dedupePageLinks(rawLinks)
}

func debugBrowserPageArtifactPrefix(debugPath string, requestedURL string, status string) string {
	dir := debugBrowserPageArtifactDir(debugPath)
	host := "page"
	if parsed, err := url.Parse(requestedURL); err == nil && parsed.Hostname() != "" {
		host = parsed.Hostname()
	}
	sum := sha1.Sum([]byte(requestedURL))
	hash := hex.EncodeToString(sum[:])[:10]
	name := fmt.Sprintf(
		"%s-%s-%s-%s",
		time.Now().UTC().Format("20060102T150405.000000000Z"),
		status,
		debugFilenamePart(host),
		hash,
	)
	return filepath.Join(dir, name)
}

func debugBrowserPageArtifactDir(debugPath string) string {
	debugPath = strings.TrimSpace(debugPath)
	if debugPath == "" {
		debugPath = "debug.log"
	}
	base := filepath.Base(debugPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if name == "" || name == "." {
		name = "debug"
	}
	return filepath.Join(filepath.Dir(debugPath), name+"-pages")
}

func debugFilenamePart(value string) string {
	value = strings.Trim(debugFilenameUnsafePattern.ReplaceAllString(value, "-"), "-")
	if value == "" {
		return "page"
	}
	if len(value) > 80 {
		return value[:80]
	}
	return value
}

func writeDebugArtifact(path string, content []byte) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, content, 0600); err != nil {
		return "", err
	}
	return path, nil
}
