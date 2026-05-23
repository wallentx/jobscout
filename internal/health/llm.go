package health

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/wallentx/jobscout/internal/config"
	"github.com/wallentx/jobscout/internal/domain"
	"github.com/wallentx/jobscout/internal/fetcher"
	llmpkg "github.com/wallentx/jobscout/internal/llm"
)

var (
	initConfiguredLLMForTask           = llmpkg.InitConfiguredLLMForTask
	enrichCompanyHealthIdentityWithLLM = llmpkg.EnrichCompanyHealthIdentityWithLLM
	evaluateCompanyHealthWithLLM       = llmpkg.EvaluateCompanyHealthWithLLM
)

const maxCompanyHealthLLMConcernStories = 3
const minCompanyHealthConcernArticleTextChars = 1000
const debugLLMHealthPreviewRunes = 160

func LLMCompanyHealthEnabled(appCfg *config.AppConfig) bool {
	return appCfg != nil && appCfg.LLM.Enabled && appCfg.LLM.CompanyHealth
}

func ApplyOptionalLLMCompanyIdentity(ctx context.Context, appCfg *config.AppConfig, identity domain.CompanyHealthContext) domain.CompanyHealthContext {
	if !LLMCompanyHealthEnabled(appCfg) {
		logDebug("llm company identity skipped company=%q: disabled in config", identity.Company)
		return identity
	}
	if !companyHealthIdentityNeedsLLM(identity) {
		logDebug("llm company identity skipped company=%q: identity already has website, summary, and industry", identity.Company)
		return identity
	}

	initStart := time.Now()
	logDebug("llm company identity init company=%q provider=%q", identity.Company, appCfg.LLM.Provider)
	llm, restoreAuth, err := initConfiguredLLMForTask(ctx, appCfg, "company_health")
	if err != nil {
		logDebug("llm company identity init failed company=%q duration=%s error=%v", identity.Company, time.Since(initStart).Round(time.Millisecond), err)
		return identity
	}
	defer restoreAuth()
	logDebug("timing company=%q step=llm_identity_init duration=%s provider=%q", identity.Company, time.Since(initStart).Round(time.Millisecond), appCfg.LLM.Provider)

	searchStart := time.Now()
	var result *llmpkg.CompanyIdentitySearchResult
	var usage llmpkg.LLMTokenUsage
	err = runThrottledHealthStep(ctx, companyHealthLLMSem, "llm company identity", identity.Company, func() error {
		var searchErr error
		result, usage, searchErr = enrichCompanyHealthIdentityWithLLM(ctx, llm, identity)
		return searchErr
	})
	if err != nil {
		logDebug("llm company identity failed company=%q duration=%s error=%v", identity.Company, time.Since(searchStart).Round(time.Millisecond), err)
		return identity
	}

	enriched := llmpkg.ApplyCompanyIdentitySearchResult(identity, result)
	canonicalName := ""
	sourceCount := 0
	if result != nil {
		canonicalName = result.CanonicalName
		sourceCount = len(result.Sources)
	}
	logDebug(
		"timing company=%q step=llm_identity duration=%s token_usage=%s canonical=%q website=%q industry=%q aliases=%d sources=%d",
		identity.Company,
		time.Since(searchStart).Round(time.Millisecond),
		debugLLMHealthTokenUsage(&usage),
		canonicalName,
		enriched.Website,
		enriched.Industry,
		len(enriched.Aliases),
		sourceCount,
	)
	return enriched
}

func companyHealthIdentityNeedsLLM(identity domain.CompanyHealthContext) bool {
	return strings.TrimSpace(identity.Company) != "" && (strings.TrimSpace(identity.Website) == "" ||
		strings.TrimSpace(identity.Summary) == "" ||
		strings.TrimSpace(identity.Industry) == "")
}

func ApplyOptionalLLMCompanyHealth(ctx context.Context, appCfg *config.AppConfig, result *domain.CompanyHealthResult) {
	switch {
	case result == nil:
		logDebug("llm company health skipped: result is nil")
		return
	case result.LLMAssessment != nil:
		logDebug("llm company health skipped company=%q: assessment already present", result.Company)
		return
	case !LLMCompanyHealthEnabled(appCfg):
		logDebug("llm company health skipped company=%q: disabled in config", result.Company)
		return
	}

	initStart := time.Now()
	logDebug("llm company health init company=%q provider=%q", result.Company, appCfg.LLM.Provider)
	llm, restoreAuth, err := initConfiguredLLMForTask(ctx, appCfg, "company_health")
	if err != nil {
		logDebug("llm company health init failed company=%q duration=%s error=%v", result.Company, time.Since(initStart).Round(time.Millisecond), err)
		return
	}
	defer restoreAuth()
	logDebug("timing company=%q step=llm_init duration=%s provider=%q", result.Company, time.Since(initStart).Round(time.Millisecond), appCfg.LLM.Provider)

	if len(result.EmployerReviews) == 0 {
		reviewStart := time.Now()
		if shouldSkipEmployerReviewLookup(result) {
			logDebug("employer review lookup skipped company=%q reason=strong_existing_health duration=%s", result.Company, time.Since(reviewStart).Round(time.Millisecond))
		} else {
			searchCompany := result.Company
			if result.DiscoveredName != "" {
				searchCompany = result.DiscoveredName
			}
			logDebug("employer review lookup start company=%q search_company=%q", result.Company, searchCompany)
			signals, err := fetchBrowserEmployerReviewSignalsThrottled(ctx, searchCompany)
			if err == nil && len(signals) > 0 {
				result.EmployerReviews = signals
				result.SignalsUsed = append(result.SignalsUsed, "browser_employer_reviews")
				if result.Sources == nil {
					result.Sources = make(map[string]any)
				}
				result.Sources["employer_reviews"] = signals
				logDebug("employer review lookup succeeded company=%q signals=%d duration=%s", result.Company, len(signals), time.Since(reviewStart).Round(time.Millisecond))
			} else if errors.Is(err, fetcher.ErrBrowserNotInstalled) {
				result.Notices = append(result.Notices, "Install Chrome or Chromium to enable Glassdoor/Indeed review signals.")
				logDebug("employer review lookup skipped company=%q reason=browser_not_installed duration=%s", result.Company, time.Since(reviewStart).Round(time.Millisecond))
			} else if err != nil {
				logDebug("employer review lookup failed company=%q duration=%s error=%v", result.Company, time.Since(reviewStart).Round(time.Millisecond), err)
			} else {
				logDebug("employer review lookup returned no signals company=%q duration=%s", result.Company, time.Since(reviewStart).Round(time.Millisecond))
			}
		}
		logDebug("timing company=%q step=browser_employer_reviews duration=%s signals=%d", result.Company, time.Since(reviewStart).Round(time.Millisecond), len(result.EmployerReviews))
	}

	articleStart := time.Now()
	attachConcernStoryArticles(ctx, result)
	if len(result.ConcernStoryArticles) > 0 {
		logDebug("timing company=%q step=browser_concern_articles duration=%s articles=%d", result.Company, time.Since(articleStart).Round(time.Millisecond), len(result.ConcernStoryArticles))
	}

	assessmentStart := time.Now()
	logDebug("llm company health assessment start company=%q signals=%d rejected_evidence=%d", result.Company, len(result.SignalsUsed), len(result.RejectedEvidence))
	var assessment *domain.LLMCompanyHealthAssessment
	err = runThrottledHealthStep(ctx, companyHealthLLMSem, "llm company health assessment", result.Company, func() error {
		var evalErr error
		assessment, evalErr = evaluateCompanyHealthWithLLM(ctx, llm, result)
		return evalErr
	})
	if err != nil {
		logDebug("llm company health assessment failed company=%q duration=%s error=%v", result.Company, time.Since(assessmentStart).Round(time.Millisecond), err)
		return
	}
	applyLLMCompanyHealthScoreModifier(result, assessment)
	result.LLMAssessment = assessment
	logDebug("timing company=%q step=llm_assessment duration=%s token_usage=%s", result.Company, time.Since(assessmentStart).Round(time.Millisecond), debugLLMHealthTokenUsage(assessment.TokenUsage))
	logDebug(
		"llm company health assessment complete company=%q risk=%q positive=%d concerns=%d followups=%d recommendation_len=%d duration=%s",
		result.Company,
		assessment.RiskLevel,
		len(assessment.PositiveSignals),
		len(assessment.Concerns),
		len(assessment.FollowUpQuestions),
		len(assessment.Recommendation),
		time.Since(assessmentStart).Round(time.Millisecond),
	)
}

func attachConcernStoryArticles(ctx context.Context, result *domain.CompanyHealthResult) {
	if result == nil || len(result.ConcernStories) == 0 {
		return
	}
	stories := selectConcernStoriesForLLM(result.ConcernStories, maxCompanyHealthLLMConcernStories)
	if len(stories) == 0 {
		logDebug("browser concern article lookup skipped company=%q reason=no_recent_high_concern_stories candidates=%d", result.Company, len(result.ConcernStories))
		return
	}
	articles := make([]domain.CompanyHealthConcernArticle, 0, len(stories))
	for index, story := range stories {
		if strings.TrimSpace(story.URL) == "" {
			logDebug("browser concern article skipped company=%q story=%d reason=missing_url title=%q", result.Company, index+1, story.Title)
			continue
		}
		logDebug(
			"browser concern article selected company=%q story=%d/%d source=%q title=%q url_host=%q concern=%q date=%s",
			result.Company,
			index+1,
			len(stories),
			story.Source,
			story.Title,
			logURLHost(story.URL),
			story.Concern,
			formatOptionalTime(story.Date),
		)
		text, err := fetchBrowserConcernStoryTextThrottled(ctx, result.Company, story.URL)
		if err != nil {
			logDebug("browser concern article fetch failed company=%q url=%q error=%v", result.Company, story.URL, err)
			continue
		}
		text = strings.TrimSpace(text)
		if !concernArticleTextUseful(text) {
			logDebug(
				"browser concern article skipped company=%q source=%q title=%q url_host=%q reason=low_value_text text_chars=%d",
				result.Company,
				story.Source,
				story.Title,
				logURLHost(story.URL),
				len(text),
			)
			continue
		}
		logDebug(
			"browser concern article accepted company=%q source=%q title=%q url_host=%q text_chars=%d",
			result.Company,
			story.Source,
			story.Title,
			logURLHost(story.URL),
			len(text),
		)
		articles = append(articles, domain.CompanyHealthConcernArticle{
			Source:  story.Source,
			Title:   story.Title,
			URL:     story.URL,
			Date:    story.Date,
			Concern: story.Concern,
			Text:    text,
		})
	}
	result.ConcernStoryArticles = articles
}

func concernArticleTextUseful(text string) bool {
	text = strings.TrimSpace(text)
	if len(text) < minCompanyHealthConcernArticleTextChars {
		return false
	}
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "sign in to continue") && len(text) < minCompanyHealthConcernArticleTextChars*2:
		return false
	case strings.Contains(lower, "enable javascript") && len(text) < minCompanyHealthConcernArticleTextChars*2:
		return false
	}
	return true
}

func selectConcernStoriesForLLM(stories []domain.CompanyHealthConcernStory, limit int) []domain.CompanyHealthConcernStory {
	if limit <= 0 || len(stories) == 0 {
		return nil
	}
	selected := make([]domain.CompanyHealthConcernStory, 0, limit)
	seen := make(map[string]bool)
	for _, story := range stories {
		key := normalizedStoryURL(story.URL)
		if key == "" || seen[key] || story.Date == nil {
			continue
		}
		if time.Since(*story.Date) > 365*24*time.Hour || story.Date.After(time.Now().Add(24*time.Hour)) {
			continue
		}
		seen[key] = true
		selected = append(selected, story)
		if len(selected) >= limit {
			break
		}
	}
	return selected
}

func fetchBrowserConcernStoryTextThrottled(ctx context.Context, company string, rawURL string) (string, error) {
	var text string
	err := runThrottledHealthStep(ctx, companyHealthBrowserSem, "browser concern article", company, func() error {
		var fetchErr error
		if session := browserSessionFromContext(ctx); session != nil {
			text, fetchErr = session.FetchArticleText(ctx, rawURL)
		} else {
			text, fetchErr = fetcher.FetchBrowserArticleText(ctx, rawURL)
		}
		return fetchErr
	})
	return text, err
}

func applyLLMCompanyHealthScoreModifier(result *domain.CompanyHealthResult, assessment *domain.LLMCompanyHealthAssessment) {
	if result == nil || assessment == nil {
		return
	}
	logLLMCompanyHealthArticleReviews(result.Company, assessment)
	allowed := relatedArticleURLSet(result.ConcernStoryArticles, assessment.ArticleReviews)
	if len(allowed) == 0 {
		logDebug("llm company health modifier rejected company=%q reason=no_related_articles article_reviews=%d", result.Company, len(assessment.ArticleReviews))
		clearArticleDerivedAssessmentFields(assessment)
		return
	}
	if strings.TrimSpace(assessment.StoryInsight) == "" && assessment.ScoreModifier == nil {
		logDebug("llm company health modifier skipped company=%q reason=no_story_insight_or_modifier related_articles=%d", result.Company, len(allowed))
		return
	}
	if assessment.ScoreModifier == nil || *assessment.ScoreModifier == 0 {
		logDebug("llm company health modifier skipped company=%q reason=no_modifier story_insight=%t related_articles=%d", result.Company, strings.TrimSpace(assessment.StoryInsight) != "", len(allowed))
		clearArticleScoreModifierFields(assessment)
		return
	}
	if !modifierUsesOnlyAllowedSources(assessment.ScoreModifierSources, allowed) {
		logDebug(
			"llm company health modifier rejected company=%q reason=modifier_sources_not_related modifier=%d sources=%q related_articles=%d",
			result.Company,
			*assessment.ScoreModifier,
			strings.Join(assessment.ScoreModifierSources, ", "),
			len(allowed),
		)
		clearArticleScoreModifierFields(assessment)
		return
	}
	if !novelModifierFactValid(assessment.ScoreModifierNovelFact, result) {
		logDebug(
			"llm company health modifier rejected company=%q reason=missing_or_non_novel_fact modifier=%d fact_len=%d",
			result.Company,
			*assessment.ScoreModifier,
			len(strings.TrimSpace(assessment.ScoreModifierNovelFact)),
		)
		clearArticleScoreModifierFields(assessment)
		return
	}

	delta := *assessment.ScoreModifier
	if delta > 10 {
		delta = 10
	}
	if delta < -10 {
		delta = -10
	}
	oldScore := result.Score
	newScore := clampHealthScoreForRisk(oldScore+delta, result.EmploymentRisk)
	actualDelta := newScore - oldScore
	if actualDelta == 0 {
		logDebug("llm company health modifier rejected company=%q reason=no_effect_after_caps requested=%d old_score=%d risk_score=%d", result.Company, delta, oldScore, healthRiskScore(result.EmploymentRisk))
		clearArticleScoreModifierFields(assessment)
		return
	}
	result.Score = newScore
	assessment.ScoreModifier = &actualDelta
	reason := strings.TrimSpace(assessment.ScoreModifierReason)
	if reason == "" {
		reason = strings.TrimSpace(assessment.ScoreModifierNovelFact)
	}
	result.Notes = append(result.Notes, fmt.Sprintf("LLM article review adjusted score %+d: %s", actualDelta, reason))
	novelFact := strings.TrimSpace(assessment.ScoreModifierNovelFact)
	logDebug(
		"llm company health modifier accepted company=%q requested=%d applied=%d old_score=%d new_score=%d reason_len=%d reason_preview=%q novel_fact_len=%d novel_fact_preview=%q sources=%q",
		result.Company,
		delta,
		actualDelta,
		oldScore,
		newScore,
		debugTextLen(reason),
		debugTextPreview(reason),
		debugTextLen(novelFact),
		debugTextPreview(novelFact),
		strings.Join(assessment.ScoreModifierSources, ", "),
	)
}

func clearArticleDerivedAssessmentFields(assessment *domain.LLMCompanyHealthAssessment) {
	if assessment == nil {
		return
	}
	assessment.ArticleReviews = nil
	assessment.StoryInsight = ""
	clearArticleScoreModifierFields(assessment)
}

func clearArticleScoreModifierFields(assessment *domain.LLMCompanyHealthAssessment) {
	if assessment == nil {
		return
	}
	assessment.ScoreModifier = nil
	assessment.ScoreModifierReason = ""
	assessment.ScoreModifierNovelFact = ""
	assessment.ScoreModifierSources = nil
}

func logLLMCompanyHealthArticleReviews(company string, assessment *domain.LLMCompanyHealthAssessment) {
	if assessment == nil {
		return
	}
	if insight := strings.TrimSpace(assessment.StoryInsight); insight != "" {
		logDebug("llm company health story insight company=%q chars=%d insight_preview=%q", company, debugTextLen(insight), debugTextPreview(insight))
	}
	for index, review := range assessment.ArticleReviews {
		relevanceReason := strings.TrimSpace(review.RelevanceReason)
		novelFact := strings.TrimSpace(review.NovelFact)
		logDebug(
			"llm company health article review company=%q article=%d source=%q url_host=%q related=%t relevance_len=%d relevance_preview=%q novel_fact_len=%d novel_fact_preview=%q",
			company,
			index+1,
			review.Source,
			logURLHost(review.URL),
			review.Related,
			debugTextLen(relevanceReason),
			debugTextPreview(relevanceReason),
			debugTextLen(novelFact),
			debugTextPreview(novelFact),
		)
	}
}

func relatedArticleURLSet(articles []domain.CompanyHealthConcernArticle, reviews []domain.LLMCompanyHealthArticleReview) map[string]bool {
	articleURLs := make(map[string]bool, len(articles))
	for _, article := range articles {
		if key := normalizedStoryURL(article.URL); key != "" {
			articleURLs[key] = true
		}
	}
	related := make(map[string]bool)
	for _, review := range reviews {
		key := normalizedStoryURL(review.URL)
		if key == "" || !review.Related || !articleURLs[key] {
			continue
		}
		related[key] = true
	}
	return related
}

func modifierUsesOnlyAllowedSources(sources []string, allowed map[string]bool) bool {
	if len(sources) == 0 || len(allowed) == 0 {
		return false
	}
	for _, source := range sources {
		key := normalizedStoryURL(source)
		if key == "" || !allowed[key] {
			return false
		}
	}
	return true
}

func novelModifierFactValid(fact string, result *domain.CompanyHealthResult) bool {
	fact = strings.TrimSpace(fact)
	if len(fact) < 20 {
		return false
	}
	foldedFact := strings.ToLower(fact)
	for _, story := range result.ConcernStories {
		title := strings.ToLower(strings.TrimSpace(story.Title))
		if title != "" && (foldedFact == title || strings.Contains(title, foldedFact)) {
			return false
		}
	}
	return true
}

func clampHealthScoreForRisk(score int, risk *domain.EmploymentRisk) int {
	if risk != nil {
		switch {
		case risk.Score >= 75 && score > 40:
			score = 40
		case risk.Score >= 50 && score > 55:
			score = 55
		}
	}
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func healthRiskScore(risk *domain.EmploymentRisk) int {
	if risk == nil {
		return 0
	}
	return risk.Score
}

func normalizedStoryURL(rawURL string) string {
	return strings.TrimRight(strings.ToLower(strings.TrimSpace(rawURL)), "/")
}

func debugTextPreview(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= debugLLMHealthPreviewRunes {
		return value
	}
	return string(runes[:debugLLMHealthPreviewRunes]) + "...[truncated]"
}

func debugTextLen(value string) int {
	return len([]rune(strings.TrimSpace(value)))
}

func logURLHost(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

func formatOptionalTime(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(time.RFC3339)
}

func shouldSkipEmployerReviewLookup(result *domain.CompanyHealthResult) bool {
	if result == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(result.Confidence), "high") {
		return false
	}
	if result.Score < 70 {
		return false
	}
	if result.EmploymentRisk != nil && result.EmploymentRisk.Score >= 25 {
		return false
	}
	if len(result.LayoffSignals) > 0 || len(result.Flags) > 0 {
		return false
	}

	sourceCount := 0
	hasAuthoritativeSource := false
	for source := range result.Sources {
		if source == "employer_reviews" || source == "company_site" {
			continue
		}
		sourceCount++
		switch source {
		case "sec", "stock_history", "wikidata":
			hasAuthoritativeSource = true
		}
	}
	return sourceCount >= 4 && hasAuthoritativeSource
}

func fetchBrowserEmployerReviewSignalsThrottled(ctx context.Context, company string) ([]domain.EmployerReviewSignal, error) {
	var signals []domain.EmployerReviewSignal
	err := runThrottledHealthStep(ctx, companyHealthBrowserSem, "browser employer reviews", company, func() error {
		var fetchErr error
		if session := browserSessionFromContext(ctx); session != nil {
			logDebug("employer review lookup using shared browser company=%q", company)
			signals, fetchErr = session.FetchEmployerReviewSignals(ctx, company)
		} else {
			signals, fetchErr = fetcher.FetchBrowserEmployerReviewSignals(company)
		}
		return fetchErr
	})
	return signals, err
}

func debugLLMHealthTokenUsage(usage *domain.LLMTokenUsage) string {
	if usage == nil || !usage.Available() {
		return "unavailable"
	}
	parts := make([]string, 0, 3)
	if usage.InputTokens != nil {
		parts = append(parts, fmt.Sprintf("input=%d", *usage.InputTokens))
	}
	if usage.OutputTokens != nil {
		parts = append(parts, fmt.Sprintf("output=%d", *usage.OutputTokens))
	}
	if usage.TotalTokens != nil {
		parts = append(parts, fmt.Sprintf("total=%d", *usage.TotalTokens))
	}
	if len(parts) == 0 {
		return "unavailable"
	}
	return strings.Join(parts, " ")
}
