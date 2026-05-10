package domain

import (
	"fmt"
	"time"
)

const (
	fieldStatusGap       = "gap"
	fieldStatusEstimated = "estimated"
	fieldStatusConfirmed = "confirmed"
	fieldStatusConflict  = "conflict"
)

func initCompanyHealthAssessments(result *CompanyHealthResult) {
	if result == nil {
		return
	}
	if result.FieldAssessments == nil {
		result.FieldAssessments = make(map[string]*CompanyHealthFieldAssessment)
	}
	ensureCompanyHealthField(result, "founded_year")
	ensureCompanyHealthField(result, "estimated_employees")
}

func ensureCompanyHealthField(result *CompanyHealthResult, field string) *CompanyHealthFieldAssessment {
	if result.FieldAssessments == nil {
		result.FieldAssessments = make(map[string]*CompanyHealthFieldAssessment)
	}
	assessment := result.FieldAssessments[field]
	if assessment == nil {
		assessment = &CompanyHealthFieldAssessment{Status: fieldStatusGap}
		result.FieldAssessments[field] = assessment
	}
	return assessment
}

func observeFoundedYear(
	result *CompanyHealthResult,
	year int,
	source string,
	sourceURL string,
	confidence string,
	reason string,
) bool {
	assessment := ensureCompanyHealthField(result, "founded_year")
	accepted := false

	if result.FoundedYear == nil {
		result.FoundedYear = new(year)
		ageYears := time.Now().Year() - year
		result.AgeYears = &ageYears
		accepted = true
		setAcceptedFieldState(assessment, confidence, source, sourceURL, fieldStatusForConfidence(confidence))
		if reason != "" {
			assessment.Notes = append(assessment.Notes, reason)
		}
	} else if foundedYearConflicts(*result.FoundedYear, year) {
		assessment.Status = fieldStatusConflict
		assessment.Notes = append(assessment.Notes, fmt.Sprintf(
			"Conflicting founded year candidate from %s: %d; current accepted value is %d.",
			source,
			year,
			*result.FoundedYear,
		))
		if confidenceRank(confidence) > confidenceRank(assessment.Confidence) && confidenceRank(confidence) >= confidenceRank("medium") {
			result.FoundedYear = new(year)
			ageYears := time.Now().Year() - year
			result.AgeYears = &ageYears
			accepted = true
			setAcceptedFieldState(assessment, confidence, source, sourceURL, fieldStatusForConfidence(confidence))
			assessment.Notes = append(assessment.Notes, "Accepted the higher-confidence founded year candidate.")
		}
	} else if confidenceRank(confidence) > confidenceRank(assessment.Confidence) {
		result.FoundedYear = new(year)
		ageYears := time.Now().Year() - year
		result.AgeYears = &ageYears
		setAcceptedFieldState(assessment, confidence, source, sourceURL, fieldStatusForConfidence(confidence))
		accepted = true
	}

	assessment.Evidence = append(assessment.Evidence, CompanyHealthEvidence{
		Value:      fmt.Sprintf("%d", year),
		Source:     source,
		URL:        sourceURL,
		Confidence: confidence,
		Accepted:   accepted,
		Reason:     reason,
	})

	return accepted
}

func ObserveFoundedYear(
	result *CompanyHealthResult,
	year int,
	source string,
	sourceURL string,
	confidence string,
	reason string,
) bool {
	return observeFoundedYear(result, year, source, sourceURL, confidence, reason)
}

func observeEmployeeCount(
	result *CompanyHealthResult,
	count int,
	source string,
	sourceURL string,
	confidence string,
	reason string,
) bool {
	assessment := ensureCompanyHealthField(result, "estimated_employees")
	accepted := false

	if result.EstimatedEmployees == nil {
		result.EstimatedEmployees = new(count)
		accepted = true
		setAcceptedFieldState(assessment, confidence, source, sourceURL, fieldStatusForConfidence(confidence))
		if reason != "" {
			assessment.Notes = append(assessment.Notes, reason)
		}
	} else if employeeCountConflicts(*result.EstimatedEmployees, count) {
		assessment.Status = fieldStatusConflict
		assessment.Notes = append(assessment.Notes, fmt.Sprintf(
			"Conflicting employee-count candidate from %s: %d; current accepted value is %d.",
			source,
			count,
			*result.EstimatedEmployees,
		))
		if confidenceRank(confidence) > confidenceRank(assessment.Confidence) && confidenceRank(confidence) >= confidenceRank("medium") {
			result.EstimatedEmployees = new(count)
			accepted = true
			setAcceptedFieldState(assessment, confidence, source, sourceURL, fieldStatusForConfidence(confidence))
			assessment.Notes = append(assessment.Notes, "Accepted the higher-confidence employee-count candidate.")
		}
	} else if confidenceRank(confidence) > confidenceRank(assessment.Confidence) {
		result.EstimatedEmployees = new(count)
		setAcceptedFieldState(assessment, confidence, source, sourceURL, fieldStatusForConfidence(confidence))
		accepted = true
	}

	assessment.Evidence = append(assessment.Evidence, CompanyHealthEvidence{
		Value:      fmt.Sprintf("%d", count),
		Source:     source,
		URL:        sourceURL,
		Confidence: confidence,
		Accepted:   accepted,
		Reason:     reason,
	})

	return accepted
}

func ObserveEmployeeCount(
	result *CompanyHealthResult,
	count int,
	source string,
	sourceURL string,
	confidence string,
	reason string,
) bool {
	return observeEmployeeCount(result, count, source, sourceURL, confidence, reason)
}

func noteFieldGap(result *CompanyHealthResult, field string, note string) {
	assessment := ensureCompanyHealthField(result, field)
	if note == "" {
		return
	}
	assessment.Notes = append(assessment.Notes, note)
}

func NoteFieldGap(result *CompanyHealthResult, field string, note string) {
	noteFieldGap(result, field, note)
}

func finalizeCompanyHealthAssessments(result *CompanyHealthResult) {
	if result == nil {
		return
	}

	founded := ensureCompanyHealthField(result, "founded_year")
	if result.FoundedYear == nil && founded.Status != fieldStatusConflict {
		founded.Status = fieldStatusGap
		if len(founded.Notes) == 0 {
			founded.Notes = append(founded.Notes, "No trustworthy founded-year evidence was found.")
		}
	}

	employees := ensureCompanyHealthField(result, "estimated_employees")
	if result.EstimatedEmployees == nil && employees.Status != fieldStatusConflict {
		employees.Status = fieldStatusGap
		if len(employees.Notes) == 0 {
			employees.Notes = append(employees.Notes, "No trustworthy employee-count evidence was found.")
		}
	}
}

func setAcceptedFieldState(
	assessment *CompanyHealthFieldAssessment,
	confidence string,
	source string,
	sourceURL string,
	status string,
) {
	assessment.Status = status
	assessment.Confidence = confidence
	assessment.Source = source
	assessment.URL = sourceURL
}

func fieldStatusForConfidence(confidence string) string {
	if confidenceRank(confidence) >= confidenceRank("high") {
		return fieldStatusConfirmed
	}
	return fieldStatusEstimated
}

func confidenceRank(confidence string) int {
	switch confidence {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func foundedYearConflicts(current int, candidate int) bool {
	diff := current - candidate
	if diff < 0 {
		diff = -diff
	}
	return diff > 2
}

func employeeCountConflicts(current int, candidate int) bool {
	high := max(current, candidate)
	low := min(current, candidate)
	if high == 0 || low == 0 {
		return high != low
	}
	return high >= int(float64(low)*1.5) && high-low >= 200
}
