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

func LLMCompanyHealthEnabled(appCfg *config.AppConfig) bool {
	return appCfg != nil && appCfg.LLM.Enabled && appCfg.LLM.CompanyHealth
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
	llm, restoreAuth, err := llmpkg.InitConfiguredLLMForTask(ctx, appCfg, "company_health")
	if err != nil {
		logDebug("llm company health init failed company=%q duration=%s error=%v", result.Company, time.Since(initStart).Round(time.Millisecond), err)
		return
	}
	defer restoreAuth()
	logDebug("timing company=%q step=llm_init duration=%s provider=%q", result.Company, time.Since(initStart).Round(time.Millisecond), appCfg.LLM.Provider)

	if len(result.EmployerReviews) == 0 {
		reviewStart := time.Now()
		searchCompany := result.Company
		if result.DiscoveredName != "" {
			searchCompany = result.DiscoveredName
		}
		logDebug("employer review lookup start company=%q search_company=%q", result.Company, searchCompany)
		var signals []domain.EmployerReviewSignal
		err := runThrottledHealthStep(ctx, companyHealthBrowserSem, "browser employer reviews", result.Company, func() error {
			var fetchErr error
			signals, fetchErr = fetcher.FetchBrowserEmployerReviewSignals(searchCompany)
			return fetchErr
		})
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
		logDebug("timing company=%q step=browser_employer_reviews duration=%s signals=%d", result.Company, time.Since(reviewStart).Round(time.Millisecond), len(result.EmployerReviews))
	}

	assessmentStart := time.Now()
	logDebug("llm company health assessment start company=%q signals=%d rejected_evidence=%d", result.Company, len(result.SignalsUsed), len(result.RejectedEvidence))
	var assessment *domain.LLMCompanyHealthAssessment
	err = runThrottledHealthStep(ctx, companyHealthLLMSem, "llm company health assessment", result.Company, func() error {
		var evalErr error
		assessment, evalErr = llmpkg.EvaluateCompanyHealthWithLLM(ctx, llm, result)
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
