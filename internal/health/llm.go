package health

import (
	"context"
	"errors"
	"fmt"
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
