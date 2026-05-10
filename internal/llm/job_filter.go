package llm

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/wallentx/jobscout/internal/domain"

	"github.com/tmc/langchaingo/llms"
)

const (
	maxJobFilterBatchSize        = 4
	maxJobFilterBatchPromptChars = 18000
)

func FilterJobsWithLLM(ctx context.Context, appCfg *AppConfig, criteriaCfg *CriteriaConfig, jobs []Job) ([]Job, []string) {
	if appCfg == nil || !appCfg.LLM.Enabled || !appCfg.LLM.JobFiltering || len(jobs) == 0 {
		return jobs, nil
	}

	selection := selectJobsForLLMFiltering(jobs, criteriaCfg)
	logDebug(
		"job filtering: selection total=%d selected=%d bypassed=%d sources=%s identity_complete=%d identity_weak=%d compensation_present=%d compensation_missing=%d description_usable=%d description_weak=%d reasons=%s",
		selection.stats.Total,
		len(selection.indexes),
		selection.stats.Total-len(selection.indexes),
		formatDebugCountMap(selection.stats.Sources),
		selection.stats.CompleteIdentity,
		selection.stats.WeakIdentity,
		selection.stats.CompensationPresent,
		selection.stats.CompensationMissing,
		selection.stats.UsableDescription,
		selection.stats.WeakDescription,
		formatDebugCountMap(selection.stats.Reasons),
	)
	if len(selection.indexes) == 0 {
		logDebug("job filtering: skipped; no ambiguous jobs require LLM evaluation")
		return jobs, nil
	}

	llm, restoreAuth, err := InitConfiguredLLMForTask(ctx, appCfg, llmTaskFiltering)
	if err != nil {
		logDebug("job filtering: skipped; init failed: %v", err)
		return jobs, []string{fmt.Sprintf("LLM job filtering skipped: %v", err)}
	}
	defer restoreAuth()

	return filterJobsWithLLMModel(ctx, llm, criteriaCfg, jobs, selection), nil
}

func JobFilterBypassGroups(appCfg *AppConfig, criteriaCfg *CriteriaConfig, jobs []Job) map[string][]Job {
	if appCfg == nil || !appCfg.LLM.Enabled || !appCfg.LLM.JobFiltering || len(jobs) == 0 {
		return nil
	}
	groups := make(map[string][]Job)
	for _, job := range jobs {
		shouldEvaluate, reason := shouldEvaluateJobWithLLM(job, criteriaCfg)
		if shouldEvaluate {
			continue
		}
		groups[reason] = append(groups[reason], job)
	}
	if len(groups) == 0 {
		return nil
	}
	return groups
}

type llmJobFilterSelection struct {
	indexes []int
	stats   llmJobFilterSelectionStats
}

type llmJobFilterSelectionStats struct {
	Total               int
	Sources             map[string]int
	Reasons             map[string]int
	CompleteIdentity    int
	WeakIdentity        int
	CompensationPresent int
	CompensationMissing int
	UsableDescription   int
	WeakDescription     int
}

func selectJobsForLLMFiltering(jobs []Job, criteriaCfg *CriteriaConfig) llmJobFilterSelection {
	selection := llmJobFilterSelection{
		stats: llmJobFilterSelectionStats{
			Total:   len(jobs),
			Sources: make(map[string]int),
			Reasons: make(map[string]int),
		},
	}
	for i, job := range jobs {
		source := jobSourceClass(job.Source)
		selection.stats.Sources[source]++
		if jobHasCompleteIdentity(job) {
			selection.stats.CompleteIdentity++
		} else {
			selection.stats.WeakIdentity++
		}
		if domain.JobCompensationMissing(job.Compensation) {
			selection.stats.CompensationMissing++
		} else {
			selection.stats.CompensationPresent++
		}
		if jobHasUsableDescription(job) {
			selection.stats.UsableDescription++
		} else {
			selection.stats.WeakDescription++
		}

		shouldEvaluate, reason := shouldEvaluateJobWithLLM(job, criteriaCfg)
		selection.stats.Reasons[reason]++
		if shouldEvaluate {
			selection.indexes = append(selection.indexes, i)
		}
	}
	return selection
}

func shouldEvaluateJobWithLLM(job Job, criteriaCfg *CriteriaConfig) (bool, string) {
	switch jobSourceClass(job.Source) {
	case "llm", "llm_web":
		return false, "llm_generated"
	}
	if !jobHasBasicIdentity(job) || looksLikeBadCompanyForLLMFilter(job.Company) {
		return false, "weak_identity"
	}
	if !jobHasUsableDescription(job) {
		return false, "weak_description"
	}
	if jobHasCompleteDeterministicFit(job, criteriaCfg) {
		return false, "deterministic_complete"
	}
	return true, "needs_fit_review"
}

func filterJobsWithLLMModel(ctx context.Context, llm llms.Model, criteriaCfg *CriteriaConfig, jobs []Job, selection llmJobFilterSelection) []Job {
	logDebug("job filtering: evaluating %d of %d jobs", len(selection.indexes), len(jobs))
	results := make(map[int]Job, len(selection.indexes))
	kept := make(map[int]bool, len(selection.indexes))
	selected := make(map[int]bool, len(selection.indexes))
	for _, index := range selection.indexes {
		selected[index] = true
	}
	stats := evaluateSelectedJobsForFiltering(ctx, llm, criteriaCfg, jobs, selection.indexes, results, kept)

	filteredJobs := make([]Job, 0, len(jobs))
	for i, job := range jobs {
		if !selected[i] {
			filteredJobs = append(filteredJobs, job)
			continue
		}
		if kept[i] {
			filteredJobs = append(filteredJobs, results[i])
		}
	}
	selectedKept := 0
	for _, index := range selection.indexes {
		if kept[index] {
			selectedKept++
		}
	}
	logDebug(
		"job filtering: run summary single_calls=%d batch_calls=%d selected=%d evaluated=%d kept=%d dropped=%d token_usage %s",
		stats.SingleCalls,
		stats.BatchCalls,
		len(selection.indexes),
		len(selection.indexes),
		selectedKept,
		len(selection.indexes)-selectedKept,
		formatTokenUsageForLog(stats.TokenUsage),
	)
	logDebug("job filtering: kept %d of %d jobs", len(filteredJobs), len(jobs))
	return filteredJobs
}

type jobFilterRunStats struct {
	SingleCalls int
	BatchCalls  int
	TokenUsage  LLMTokenUsage
}

func evaluateSelectedJobsForFiltering(ctx context.Context, llm llms.Model, criteriaCfg *CriteriaConfig, jobs []Job, indexes []int, results map[int]Job, kept map[int]bool) jobFilterRunStats {
	var stats jobFilterRunStats
	groups := jobFilterBatchesBySource(jobs, indexes)
	groups = splitOversizedJobFilterBatches(criteriaCfg, jobs, groups)
	logDebug("job filtering: grouped %d selected jobs into %d same-source batches", len(indexes), len(groups))
	for _, batch := range groups {
		if len(batch) == 1 {
			evaluateSingleJobForFiltering(ctx, llm, criteriaCfg, jobs[batch[0]], batch[0], results, kept, &stats)
			continue
		}
		if !evaluateJobFilterBatch(ctx, llm, criteriaCfg, jobs, batch, results, kept, &stats) {
			for _, index := range batch {
				evaluateSingleJobForFiltering(ctx, llm, criteriaCfg, jobs[index], index, results, kept, &stats)
			}
		}
	}
	return stats
}

func evaluateJobFilterBatch(ctx context.Context, llm llms.Model, criteriaCfg *CriteriaConfig, jobs []Job, indexes []int, results map[int]Job, kept map[int]bool, stats *jobFilterRunStats) bool {
	input := buildJobFilterBatchInput(criteriaCfg, jobs, indexes)

	logDebug("job filtering: batch generation start source=%q jobs=%d", input.Source, len(input.Jobs))
	if stats != nil {
		stats.BatchCalls++
	}
	output, err := evaluateJobFilterBatchWithLLM(ctx, llm, input)
	if err != nil {
		logDebug("job filtering: batch failed source=%q jobs=%d error=%v", input.Source, len(input.Jobs), err)
		return false
	}
	if stats != nil && output.TokenUsage != nil {
		addTokenUsage(&stats.TokenUsage, *output.TokenUsage)
	}
	for _, index := range indexes {
		id := strconv.Itoa(index)
		res, ok := output.Results[id]
		if !ok {
			logDebug("job filtering: batch missing result source=%q id=%s company=%q title=%q", input.Source, id, jobs[index].Company, jobs[index].Title)
			return false
		}
		applyJobFilterResult(jobs[index], index, &res, results, kept)
	}
	logDebug("job filtering: batch completed source=%q jobs=%d", input.Source, len(input.Jobs))
	return true
}

func evaluateSingleJobForFiltering(ctx context.Context, llm llms.Model, criteriaCfg *CriteriaConfig, job Job, index int, results map[int]Job, kept map[int]bool, stats *jobFilterRunStats) {
	if stats != nil {
		stats.SingleCalls++
	}
	res, err := EvaluateJobWithLLM(ctx, llm, job, criteriaCfg)
	if err != nil {
		logDebug("job filtering: keeping %s - %s after evaluation error: %v", job.Company, job.Title, err)
		results[index] = job
		kept[index] = true
		return
	}
	if stats != nil && res != nil && res.TokenUsage != nil {
		addTokenUsage(&stats.TokenUsage, *res.TokenUsage)
	}
	applyJobFilterResult(job, index, res, results, kept)
}

func addTokenUsage(total *LLMTokenUsage, usage LLMTokenUsage) {
	if total == nil {
		return
	}
	total.InputTokens = addTokenUsageField(total.InputTokens, usage.InputTokens)
	total.OutputTokens = addTokenUsageField(total.OutputTokens, usage.OutputTokens)
	total.TotalTokens = addTokenUsageField(total.TotalTokens, usage.TotalTokens)
	total.CachedTokens = addTokenUsageField(total.CachedTokens, usage.CachedTokens)
	total.ReasoningTokens = addTokenUsageField(total.ReasoningTokens, usage.ReasoningTokens)
	total.ThinkingTokens = addTokenUsageField(total.ThinkingTokens, usage.ThinkingTokens)
}

func addTokenUsageField(total *int, value *int) *int {
	if value == nil {
		return total
	}
	if total == nil {
		return intPtr(*value)
	}
	return intPtr(*total + *value)
}

func applyJobFilterResult(job Job, index int, res *LLMEvaluationResult, results map[int]Job, kept map[int]bool) {
	if res == nil {
		results[index] = job
		kept[index] = true
		return
	}
	original := job
	if res.Matches {
		if compensation := strings.TrimSpace(res.CompensationExtracted); compensation != "" {
			job.Compensation = compensation
		}
		if remote := strings.TrimSpace(res.RemoteEligibility); remote != "" {
			job.Remote = remote
		}
		if len(res.WhyItMatches) > 0 {
			job.WhyMatches = res.WhyItMatches
		}
		logDebug(
			"job filtering: kept %s - %s remote=%q compensation=%q reasons=%q diff=%q",
			job.Company,
			job.Title,
			job.Remote,
			job.Compensation,
			debugReasonList(job.WhyMatches),
			jobFilterPersistedDiff(original, job),
		)
		results[index] = job
		kept[index] = true
		return
	}
	logDebug(
		"job filtering: dropped %s - %s remote=%q compensation=%q reasons=%q diff=%q",
		job.Company,
		job.Title,
		res.RemoteEligibility,
		res.CompensationExtracted,
		debugReasonList(res.WhyRejected),
		jobFilterRejectedDiff(original, res),
	)
}

func jobFilterPersistedDiff(before Job, after Job) string {
	changes := make([]string, 0, 3)
	if before.Compensation != after.Compensation {
		changes = append(changes, debugFieldChange("compensation", before.Compensation, after.Compensation))
	}
	if before.Remote != after.Remote {
		changes = append(changes, debugFieldChange("remote", before.Remote, after.Remote))
	}
	if !equalStringSlices(before.WhyMatches, after.WhyMatches) {
		changes = append(changes, fmt.Sprintf("why_matches: %d -> %d", len(before.WhyMatches), len(after.WhyMatches)))
	}
	if len(changes) == 0 {
		return "no persisted field changes"
	}
	return strings.Join(changes, "; ")
}

func jobFilterRejectedDiff(job Job, res *LLMEvaluationResult) string {
	if res == nil {
		return "no model result"
	}
	parts := make([]string, 0, 3)
	if compensation := strings.TrimSpace(res.CompensationExtracted); compensation != "" && compensation != strings.TrimSpace(job.Compensation) {
		parts = append(parts, debugFieldChange("compensation_extracted", job.Compensation, compensation))
	}
	if remote := strings.TrimSpace(res.RemoteEligibility); remote != "" && remote != strings.TrimSpace(job.Remote) {
		parts = append(parts, debugFieldChange("remote_eligibility", job.Remote, remote))
	}
	if len(res.WhyRejected) > 0 {
		parts = append(parts, fmt.Sprintf("why_rejected: %d", len(res.WhyRejected)))
	}
	if len(parts) == 0 {
		return "no extracted field changes"
	}
	return strings.Join(parts, "; ")
}

func debugFieldChange(field string, before string, after string) string {
	return fmt.Sprintf("%s: %q -> %q", field, strings.TrimSpace(before), strings.TrimSpace(after))
}

func equalStringSlices(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func jobFilterBatchesBySource(jobs []Job, indexes []int) [][]int {
	if len(indexes) == 0 {
		return nil
	}
	orderedSources := make([]string, 0)
	bySource := make(map[string][]int)
	for _, index := range indexes {
		source := strings.TrimSpace(jobs[index].Source)
		if source == "" {
			source = "unknown"
		}
		if _, ok := bySource[source]; !ok {
			orderedSources = append(orderedSources, source)
		}
		bySource[source] = append(bySource[source], index)
	}

	batches := make([][]int, 0, len(indexes)/maxJobFilterBatchSize+len(orderedSources))
	for _, source := range orderedSources {
		group := bySource[source]
		for start := 0; start < len(group); start += maxJobFilterBatchSize {
			end := start + maxJobFilterBatchSize
			if end > len(group) {
				end = len(group)
			}
			batches = append(batches, group[start:end])
		}
	}
	return batches
}

func splitOversizedJobFilterBatches(criteriaCfg *CriteriaConfig, jobs []Job, groups [][]int) [][]int {
	if len(groups) == 0 {
		return nil
	}
	splitGroups := make([][]int, 0, len(groups))
	for _, group := range groups {
		split := splitJobFilterBatchByPromptSize(criteriaCfg, jobs, group)
		if len(split) > 1 {
			source := ""
			if len(group) > 0 {
				source = strings.TrimSpace(jobs[group[0]].Source)
			}
			logDebug(
				"job filtering: split oversized batch source=%q jobs=%d prompt_chars=%d into %d batches cap=%d",
				source,
				len(group),
				jobFilterBatchPromptChars(criteriaCfg, jobs, group),
				len(split),
				maxJobFilterBatchPromptChars,
			)
		}
		splitGroups = append(splitGroups, split...)
	}
	return splitGroups
}

func splitJobFilterBatchByPromptSize(criteriaCfg *CriteriaConfig, jobs []Job, indexes []int) [][]int {
	if len(indexes) <= 1 || jobFilterBatchPromptChars(criteriaCfg, jobs, indexes) <= maxJobFilterBatchPromptChars {
		return [][]int{indexes}
	}

	var split [][]int
	current := make([]int, 0, len(indexes))
	for _, index := range indexes {
		candidate := append(append([]int(nil), current...), index)
		if len(current) > 0 && jobFilterBatchPromptChars(criteriaCfg, jobs, candidate) > maxJobFilterBatchPromptChars {
			split = append(split, append([]int(nil), current...))
			current = []int{index}
			continue
		}
		current = candidate
	}
	if len(current) > 0 {
		split = append(split, current)
	}
	return split
}

func jobFilterBatchPromptChars(criteriaCfg *CriteriaConfig, jobs []Job, indexes []int) int {
	return len(buildJobFilterBatchPrompt(buildJobFilterBatchInput(criteriaCfg, jobs, indexes)))
}

func buildJobFilterBatchInput(criteriaCfg *CriteriaConfig, jobs []Job, indexes []int) benchmarkJobFilterBatchInput {
	input := benchmarkJobFilterBatchInput{
		Criteria: criteriaValue(criteriaCfg),
		Source:   strings.TrimSpace(jobs[indexes[0]].Source),
		Jobs:     make([]benchmarkJobFilterBatchEntry, 0, len(indexes)),
	}
	for _, index := range indexes {
		input.Jobs = append(input.Jobs, benchmarkJobFilterBatchEntry{
			ID:  strconv.Itoa(index),
			Job: jobs[index],
		})
	}
	return input
}

func criteriaValue(criteriaCfg *CriteriaConfig) CriteriaConfig {
	if criteriaCfg == nil {
		return CriteriaConfig{}
	}
	return *criteriaCfg
}

func jobHasCompleteDeterministicFit(job Job, criteriaCfg *CriteriaConfig) bool {
	if criteriaCfg == nil {
		return !domain.JobCompensationMissing(job.Compensation) && jobHasBasicIdentity(job)
	}
	if criteriaCfg.Filters.MinBaseUSD > 0 && domain.JobCompensationMissing(job.Compensation) {
		return false
	}
	if workSettingsConfigured(criteriaCfg.Filters.WorkSettings) && !jobMatchesConfiguredWorkSetting(job, criteriaCfg.Filters.WorkSettings) {
		return false
	}
	return jobHasBasicIdentity(job)
}

func jobHasBasicIdentity(job Job) bool {
	return strings.TrimSpace(job.Company) != "" &&
		!strings.EqualFold(strings.TrimSpace(job.Company), "unknown") &&
		strings.TrimSpace(job.Title) != "" &&
		strings.TrimSpace(job.ApplyURL) != ""
}

func jobHasCompleteIdentity(job Job) bool {
	return jobHasBasicIdentity(job) &&
		!domain.JobCompanyWebsiteMissingOrInvalid(job.CompanyWebsite) &&
		!domain.JobCompanySummaryMissingOrInvalid(job.CompanySummary, job.Company) &&
		strings.TrimSpace(job.CompanyIndustry) != "" &&
		!jobHasProvisionalIdentity(job)
}

func jobHasProvisionalIdentity(job Job) bool {
	if job.CompanyIdentity == nil {
		return false
	}
	return jobIdentityEvidenceProvisional(job.CompanyIdentity.Website) ||
		jobIdentityEvidenceProvisional(job.CompanyIdentity.Summary) ||
		jobIdentityEvidenceProvisional(job.CompanyIdentity.Industry)
}

func jobIdentityEvidenceProvisional(evidence *domain.JobIdentityEvidence) bool {
	return evidence != nil && evidence.Provisional
}

func jobHasUsableDescription(job Job) bool {
	description := strings.TrimSpace(job.Description)
	if len([]rune(description)) < 120 {
		return false
	}
	if parsed, err := url.Parse(description); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return false
	}
	return true
}

func jobMatchesConfiguredWorkSetting(job Job, settings domain.WorkSettingsConfig) bool {
	text := strings.ToLower(strings.TrimSpace(job.Remote) + " " + strings.TrimSpace(job.Description))
	mentionsNotRemote := strings.Contains(text, "not remote") || strings.Contains(text, "non-remote")
	isHybrid := strings.Contains(text, "hybrid")
	isOnsite := mentionsNotRemote ||
		strings.Contains(text, "on-site") ||
		strings.Contains(text, "onsite") ||
		strings.Contains(text, "in office") ||
		strings.Contains(text, "office based")
	isRemote := !mentionsNotRemote && strings.Contains(text, "remote")

	switch {
	case isHybrid:
		return settings.Hybrid
	case isOnsite:
		return settings.Onsite
	case isRemote:
		return settings.Remote
	default:
		return false
	}
}

func workSettingsConfigured(settings domain.WorkSettingsConfig) bool {
	return settings.Remote || settings.Hybrid || settings.Onsite
}

func jobSourceClass(source string) string {
	source = strings.ToLower(strings.TrimSpace(source))
	switch {
	case strings.HasPrefix(source, "llm web:"):
		return "llm_web"
	case strings.HasPrefix(source, "llm:"):
		return "llm"
	case strings.HasPrefix(source, "rss:"):
		return "rss"
	case strings.HasPrefix(source, "api:"):
		return "api"
	case strings.HasPrefix(source, "site search:"):
		return "site_search"
	case source == "":
		return "unknown"
	default:
		return "other"
	}
}

func looksLikeBadCompanyForLLMFilter(company string) bool {
	lower := strings.ToLower(strings.TrimSpace(company))
	switch lower {
	case "", "unknown", "easily apply", "easy apply", "new", "today", "featured", "urgently hiring":
		return true
	}
	badSubstrings := []string{
		"company logo",
		"easily apply",
		"easy apply",
		"hiring multiple candidates",
		"image:",
		"often replies in ",
		"often responds within ",
		"transit information",
		"view similar jobs with this employer",
	}
	for _, bad := range badSubstrings {
		if strings.Contains(lower, bad) {
			return true
		}
	}
	return false
}

func formatDebugCountMap(values map[string]int) string {
	if len(values) == 0 {
		return "<none>"
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, values[key]))
	}
	return strings.Join(parts, ",")
}

func debugReasonList(reasons []string) string {
	if len(reasons) == 0 {
		return "<none>"
	}
	cleaned := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		reason = strings.TrimSpace(reason)
		if reason != "" {
			cleaned = append(cleaned, reason)
		}
	}
	if len(cleaned) == 0 {
		return "<none>"
	}
	text := strings.Join(cleaned, "; ")
	if len(text) > 300 {
		text = text[:300] + "..."
	}
	return text
}
