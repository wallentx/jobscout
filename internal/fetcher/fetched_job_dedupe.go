package fetcher

import (
	"net/url"
	"strings"

	"github.com/wallentx/jobscout/internal/domain"
)

func dedupeFetchedJobs(jobs []Job) ([]Job, []Job) {
	if len(jobs) < 2 {
		return jobs, nil
	}
	seen := make(map[string]int, len(jobs))
	deduped := make([]Job, 0, len(jobs))
	duplicates := make([]Job, 0)
	for _, job := range jobs {
		key := fetchedJobDedupeKey(job)
		if key == "" {
			deduped = append(deduped, job)
			continue
		}
		if existingIdx, ok := seen[key]; ok {
			domain.MergeJobIdentityFields(&deduped[existingIdx], job)
			duplicates = append(duplicates, job)
			continue
		}
		seen[key] = len(deduped)
		deduped = append(deduped, job)
	}
	return deduped, duplicates
}

type existingJobIndex struct {
	keys map[string]struct{}
}

func newExistingJobIndex(jobs []Job) *existingJobIndex {
	index := &existingJobIndex{keys: make(map[string]struct{}, len(jobs))}
	for _, job := range jobs {
		key := fetchedJobDedupeKey(job)
		if key == "" {
			continue
		}
		index.keys[key] = struct{}{}
	}
	return index
}

func (i *existingJobIndex) contains(job Job) bool {
	if i == nil || len(i.keys) == 0 {
		return false
	}
	key := fetchedJobDedupeKey(job)
	if key == "" {
		return false
	}
	_, ok := i.keys[key]
	return ok
}

func skipExistingFetchedJobs(jobs []Job, existing *existingJobIndex) ([]Job, []Job) {
	if existing == nil || len(jobs) == 0 {
		return jobs, nil
	}
	kept := make([]Job, 0, len(jobs))
	skipped := make([]Job, 0)
	for _, job := range jobs {
		if existing.contains(job) {
			skipped = append(skipped, job)
			continue
		}
		kept = append(kept, job)
	}
	return kept, skipped
}

func SkipExistingFetchedJobs(jobs []Job, existing []Job) ([]Job, []Job) {
	return skipExistingFetchedJobs(jobs, newExistingJobIndex(existing))
}

func fetchedJobDedupeKey(job Job) string {
	if key := builtInJobDedupeKey(job.ApplyURL); key != "" {
		return key
	}
	if key := canonicalApplyURLDedupeKeyForJob(job); key != "" {
		return key
	}
	key := domain.JobMergeKey(job)
	if key == "|" {
		return ""
	}
	return "job:" + key
}

func builtInJobDedupeKey(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" || !isBuiltInHost(parsed.Hostname()) {
		return ""
	}
	id := trailingNumericPathSegment(parsed.EscapedPath())
	if id == "" || !isBuiltInJobDetailPath(parsed.EscapedPath()) {
		return ""
	}
	return "builtin:" + id
}

func canonicalApplyURLDedupeKeyForJob(job Job) string {
	rawURL := job.ApplyURL
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if !canonicalApplyURLLooksJobSpecific(parsed, job) {
		return ""
	}
	return "url:" + parsed.String()
}

func canonicalApplyURLLooksJobSpecific(parsed *url.URL, job Job) bool {
	if parsed == nil {
		return false
	}
	path := strings.ToLower(strings.Trim(parsed.EscapedPath(), "/"))
	if path == "" {
		return false
	}
	if strings.Contains(path, "job") || strings.Contains(path, "career") || strings.Contains(path, "position") || strings.Contains(path, "opening") {
		return true
	}
	if looksLikeJobTitle(strings.NewReplacer("-", " ", "_", " ", "/", " ").Replace(path)) {
		return true
	}
	if slug := slugify(job.Title); slug != "" && strings.Contains(path, slug) {
		return true
	}
	host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
	return isIndeedHost(host) || isLinkedInHost(host) || isSharedATSDirectoryHost(host)
}

func trailingNumericPathSegment(rawPath string) string {
	parts := strings.Split(strings.Trim(rawPath, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	last := parts[len(parts)-1]
	if last == "" {
		return ""
	}
	for _, r := range last {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return last
}
