package domain

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// calculateEmploymentRisk computes risk score and level
func calculateEmploymentRisk(layoffs []LayoffSignal, stockHistory []float64, secRiskTerms, newsNeg, newsPos int, estimatedEmployees *int) *EmploymentRisk {
	risk := &EmploymentRisk{
		Score:   0,
		Factors: []string{},
	}

	// 1. Layoff signals (scaled by impact)
	if len(layoffs) > 0 {
		// Use provided logic: Base 25, Multipliers for scale/recency
		baseScore := 25.0
		scaleMultiplier := 1.0
		recencyMultiplier := 1.0

		now := time.Now()
		mostRecentLayoff := (*LayoffSignal)(nil)
		largestLayoff := (*LayoffSignal)(nil)
		maxEmployeeCount := 0

		for i := range layoffs {
			l := &layoffs[i]
			if l.EmployeeCount != nil && *l.EmployeeCount > maxEmployeeCount {
				maxEmployeeCount = *l.EmployeeCount
				largestLayoff = l
			}
			if l.Date != nil {
				if mostRecentLayoff == nil || l.Date.After(*mostRecentLayoff.Date) {
					mostRecentLayoff = l
				}
			}
		}

		// Calculate Scale
		if largestLayoff != nil && largestLayoff.EmployeeCount != nil {
			count := *largestLayoff.EmployeeCount
			percent := 0.0
			if estimatedEmployees != nil && *estimatedEmployees > 0 {
				percent = (float64(count) / float64(*estimatedEmployees)) * 100.0
			}

			if percent >= 20.0 {
				scaleMultiplier = 2.5
				risk.Factors = append(risk.Factors, fmt.Sprintf("Massive cut: ~%.1f%% workforce", percent))
			} else if percent >= 10.0 {
				scaleMultiplier = 1.8
				risk.Factors = append(risk.Factors, fmt.Sprintf("Major cut: ~%.1f%% workforce", percent))
			} else if count >= 1000 {
				scaleMultiplier = 2.0
				risk.Factors = append(risk.Factors, fmt.Sprintf("Large scale: %d affected", count))
			} else if count >= 100 {
				scaleMultiplier = 1.2
			}
		}

		// Calculate Recency
		if mostRecentLayoff != nil && mostRecentLayoff.Date != nil {
			daysAgo := int(now.Sub(*mostRecentLayoff.Date).Hours() / 24)
			risk.LastLayoffDate = mostRecentLayoff.Date
			if daysAgo < 30 {
				recencyMultiplier = 1.5
				risk.Factors = append(risk.Factors, "Very recent (<30 days)")
			} else if daysAgo < 90 {
				recencyMultiplier = 1.3
				risk.Factors = append(risk.Factors, "Recent (<90 days)")
			}
		}

		layoffScore := int(baseScore * scaleMultiplier * recencyMultiplier)
		risk.Score += min(75, layoffScore) // Cap layoff contribution
	}

	// 2. Stock performance (Continuous Scaling)
	if len(stockHistory) > 200 {
		start := stockHistory[0]
		end := stockHistory[len(stockHistory)-1]

		// Find 52-week High
		yearHigh := 0.0
		for _, price := range stockHistory {
			if price > yearHigh {
				yearHigh = price
			}
		}

		if start > 0 && yearHigh > 0 {
			// Metric A: Drop from 52-week High (Distress)
			dropFromHigh := (yearHigh - end) / yearHigh // 0.0 to 1.0

			// Metric B: Year-over-Year Change (Trend)
			yoyChange := (end - start) / start // e.g. -0.40

			// We use the worse of the two metrics to capture different types of failure
			stockRisk := 0

			if dropFromHigh > 0.30 {
				// Continuous: 30% drop = 15pts ... 80% drop = 40pts
				// Formula: (Drop - 0.20) * 50
				pts := int((dropFromHigh - 0.20) * 50)
				stockRisk = max(stockRisk, pts)
				risk.Factors = append(risk.Factors, fmt.Sprintf("Stock down %.0f%% from high", dropFromHigh*100))
			}

			if yoyChange < -0.20 {
				// Continuous: -20% = 10pts ... -60% = 30pts
				// Formula: abs(Change) * 50
				pts := int(math.Abs(yoyChange) * 50)
				stockRisk = max(stockRisk, pts)
				// Don't duplicate factor if we already logged the high drop
				if dropFromHigh <= 0.30 {
					risk.Factors = append(risk.Factors, fmt.Sprintf("Stock dropped %.0f%% YoY", math.Abs(yoyChange)*100))
				}
			}

			risk.Score += min(40, stockRisk) // Cap stock risk at 40
		}
	}

	// 3. SEC risk terms
	if secRiskTerms > 0 {
		risk.Score += min(20, secRiskTerms*5)
		risk.Factors = append(risk.Factors, "SEC filings contain financial risk keywords")
	}

	// 4. News sentiment
	if newsNeg > newsPos+2 {
		risk.Score += 10
		risk.Factors = append(risk.Factors, "Negative news sentiment outweighs positive")
	}

	// Cap score at 100
	if risk.Score > 100 {
		risk.Score = 100
	}

	updateEmploymentRiskLevel(risk)
	return risk
}

func applyEmployerReviewHealthSignals(result *CompanyHealthResult) {
	if result == nil || len(result.EmployerReviews) == 0 {
		return
	}
	if result.EmploymentRisk == nil {
		result.EmploymentRisk = &EmploymentRisk{Factors: []string{}}
	}
	applied := false
	for _, signal := range result.EmployerReviews {
		rating := employerReviewRatingValue(signal.Rating)
		if rating > 0 {
			switch {
			case rating < 3.0:
				result.Score -= 8
				result.Flags = append(result.Flags, "low_employer_review_rating")
				result.EmploymentRisk.Score += 15
				result.EmploymentRisk.Factors = append(result.EmploymentRisk.Factors, fmt.Sprintf("%s rating %.1f/5", reviewSourceLabel(signal.Source), rating))
				applied = true
			case rating < 3.5:
				result.Score -= 3
				result.EmploymentRisk.Score += 7
				result.EmploymentRisk.Factors = append(result.EmploymentRisk.Factors, fmt.Sprintf("%s rating %.1f/5", reviewSourceLabel(signal.Source), rating))
				applied = true
			case rating >= 4.2:
				result.Score += 3
				result.Notes = append(result.Notes, fmt.Sprintf("%s rating %.1f/5 suggests generally positive employee reviews.", reviewSourceLabel(signal.Source), rating))
				applied = true
			}
		}
		if signal.RecommendPercent != nil {
			switch {
			case *signal.RecommendPercent < 50:
				result.Score -= 6
				result.Flags = append(result.Flags, "low_employer_recommendation")
				result.EmploymentRisk.Score += 12
				result.EmploymentRisk.Factors = append(result.EmploymentRisk.Factors, fmt.Sprintf("%s recommend-to-friend is %d%%", reviewSourceLabel(signal.Source), *signal.RecommendPercent))
				applied = true
			case *signal.RecommendPercent >= 80:
				result.Score += 2
				result.Notes = append(result.Notes, fmt.Sprintf("%s recommend-to-friend is %d%%.", reviewSourceLabel(signal.Source), *signal.RecommendPercent))
				applied = true
			}
		}
		if signal.CEOApprovalPercent != nil && *signal.CEOApprovalPercent < 50 {
			result.Score -= 3
			result.EmploymentRisk.Score += 5
			result.EmploymentRisk.Factors = append(result.EmploymentRisk.Factors, fmt.Sprintf("%s CEO approval is %d%%", reviewSourceLabel(signal.Source), *signal.CEOApprovalPercent))
			applied = true
		}
	}
	if !applied {
		return
	}
	if result.EmploymentRisk.Score > 100 {
		result.EmploymentRisk.Score = 100
	}
	updateEmploymentRiskLevel(result.EmploymentRisk)
	if !companyHealthSignalUsed(result.SignalsUsed, "employer_review_health") {
		result.SignalsUsed = append(result.SignalsUsed, "employer_review_health")
	}
}

func employerReviewRatingValue(rating string) float64 {
	rating = strings.TrimSpace(strings.TrimSuffix(rating, "/5"))
	if rating == "" {
		return 0
	}
	value, err := strconv.ParseFloat(rating, 64)
	if err != nil || value < 0 || value > 5 {
		return 0
	}
	return value
}

func reviewSourceLabel(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return "Employer review"
	}
	return strings.ToUpper(source[:1]) + source[1:]
}

func updateEmploymentRiskLevel(risk *EmploymentRisk) {
	if risk == nil {
		return
	}
	if risk.Score > 100 {
		risk.Score = 100
	}
	if risk.Score < 0 {
		risk.Score = 0
	}
	if risk.Score >= 75 {
		risk.Level = "Critical"
	} else if risk.Score >= 50 {
		risk.Level = "High"
	} else if risk.Score >= 25 {
		risk.Level = "Medium"
	} else {
		risk.Level = "Low"
	}
}
