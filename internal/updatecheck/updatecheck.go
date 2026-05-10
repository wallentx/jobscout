package updatecheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	"golang.org/x/mod/semver"
)

const (
	LatestReleaseURL = "https://api.github.com/repos/wallentx/jobscout/releases/latest"
	userAgent        = "jobscout (+https://github.com/wallentx/jobscout)"
	requestTimeout   = 3 * time.Second
)

var ErrVersionNotComparable = errors.New("version is not comparable")

type Result struct {
	CurrentVersion string
	LatestVersion  string
	ReleaseURL     string
	Available      bool
}

type latestReleaseResponse struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func CheckLatestRelease(ctx context.Context, currentVersion string) (Result, error) {
	return CheckLatestReleaseWithClient(ctx, currentVersion, LatestReleaseURL, nil)
}

func CheckLatestReleaseWithClient(ctx context.Context, currentVersion string, endpoint string, client *http.Client) (Result, error) {
	currentComparable, ok := comparableCurrentVersion(currentVersion)
	result := Result{CurrentVersion: strings.TrimSpace(currentVersion)}
	if !ok {
		return result, ErrVersionNotComparable
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	if client == nil {
		client = &http.Client{}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return result, fmt.Errorf("build latest release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("fetch latest release: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNotFound {
		return result, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("fetch latest release: HTTP %d", resp.StatusCode)
	}

	var payload latestReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return result, fmt.Errorf("decode latest release: %w", err)
	}
	latestVersion := normalizeSemver(payload.TagName)
	if latestVersion == "" {
		return result, ErrVersionNotComparable
	}

	result.LatestVersion = payload.TagName
	if strings.TrimSpace(result.LatestVersion) == "" {
		result.LatestVersion = latestVersion
	}
	result.ReleaseURL = strings.TrimSpace(payload.HTMLURL)
	if result.ReleaseURL == "" {
		result.ReleaseURL = "https://github.com/wallentx/jobscout/releases/tag/" + latestVersion
	}
	result.Available = semver.Compare(currentComparable, latestVersion) < 0
	return result, nil
}

func comparableCurrentVersion(version string) (string, bool) {
	if base, ok := devBuildBaseVersion(version); ok {
		if normalized := normalizeSemver(base); normalized != "" {
			return normalized, true
		}
	}
	if normalized := normalizeSemver(version); normalized != "" {
		return normalized, true
	}
	return "", false
}

func devBuildBaseVersion(version string) (string, bool) {
	value := strings.TrimSpace(version)
	idx := strings.LastIndex(value, "-")
	if idx <= 0 || idx == len(value)-1 {
		return "", false
	}
	suffix := value[idx+1:]
	if len(suffix) < 7 || len(suffix) > 40 {
		return "", false
	}
	for _, r := range suffix {
		if !unicode.IsDigit(r) && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return "", false
		}
	}
	return value[:idx], true
}

func normalizeSemver(version string) string {
	value := strings.TrimSpace(version)
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "v") {
		value = "v" + value
	}
	return semver.Canonical(value)
}
