package domain

import (
	"strings"
)

func NormalizeTitlePrefixes(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		normalized := normalizeTitlePrefix(value)
		if normalized == "" {
			continue
		}
		key := strings.ToLower(normalized)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, normalized)
	}
	return out
}

func NormalizeTargetTitleNames(values []string, roleFamilies []RoleFamilyID) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		normalized := normalizeTargetTitleName(value, roleFamilies)
		if normalized == "" {
			continue
		}
		key := strings.ToLower(normalized)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, normalized)
	}
	if len(out) == 0 {
		for _, value := range defaultTargetTitleNamesForRoles(roleFamilies) {
			key := strings.ToLower(value)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, value)
		}
	}
	return out
}

func normalizeTitlePrefix(value string) string {
	value = strings.ToLower(strings.TrimSpace(strings.Trim(value, ". ")))
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, "-", " ")
	value = strings.Join(strings.Fields(value), " ")
	switch value {
	case "", "mid", "mid level", "midlevel", "dev", "developer", "eng", "engineer":
		return ""
	case "jr", "junior":
		return "Junior"
	case "sr", "senior":
		return "Senior"
	case "staff":
		return "Staff"
	case "principal":
		return "Principal"
	default:
		return titleCaseTitleTerm(value)
	}
}

func normalizeTargetTitleName(value string, roleFamilies []RoleFamilyID) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	tokens := strings.Fields(value)
	normalized := make([]string, 0, len(tokens)+1)
	for _, token := range tokens {
		token = strings.Trim(token, ".,;:()[]{}")
		if token == "" {
			continue
		}
		normalized = append(normalized, normalizeTitleToken(token))
	}
	title := strings.Join(normalized, " ")
	title = strings.Join(strings.Fields(title), " ")
	title = normalizeTitlePhrases(title)
	switch strings.ToLower(title) {
	case "", "dev", "developer", "eng", "engineer":
		return ""
	case "fullstack":
		title = "Full Stack"
	case "frontend":
		title = "Frontend"
	case "backend":
		title = "Backend"
	}
	title = appendImpliedTitleNoun(title, roleFamilies)
	return title
}

func normalizeTitleToken(token string) string {
	cleaned := strings.ToLower(strings.TrimSpace(token))
	cleaned = strings.ReplaceAll(cleaned, "_", "-")
	switch cleaned {
	case "dev", "dev.":
		return "Developer"
	case "eng", "eng.":
		return "Engineer"
	case "jr", "jr.":
		return "Junior"
	case "sr", "sr.":
		return "Senior"
	case "front-end", "frontend":
		return "Frontend"
	case "back-end", "backend":
		return "Backend"
	case "full-stack", "fullstack":
		return "Full Stack"
	case "devops":
		return "DevOps"
	case "sre":
		return "SRE"
	case "ai":
		return "AI"
	case "ml":
		return "ML"
	default:
		return titleCaseTitleTerm(cleaned)
	}
}

func normalizeTitlePhrases(title string) string {
	replacements := map[string]string{
		"Front End": "Frontend",
		"Back End":  "Backend",
		"Fullstack": "Full Stack",
	}
	for old, replacement := range replacements {
		title = strings.ReplaceAll(title, old, replacement)
	}
	return title
}

func appendImpliedTitleNoun(title string, roleFamilies []RoleFamilyID) string {
	if titleHasRoleNoun(title) {
		return title
	}
	switch impliedTitleNoun(roleFamilies) {
	case "Engineer":
		return title + " Engineer"
	case "Manager":
		return title + " Manager"
	case "Designer":
		return title + " Designer"
	default:
		return title
	}
}

func titleHasRoleNoun(title string) bool {
	lower := strings.ToLower(title)
	for _, noun := range []string{
		"administrator",
		"analyst",
		"architect",
		"consultant",
		"designer",
		"developer",
		"engineer",
		"manager",
		"scientist",
		"specialist",
		"sre",
	} {
		if strings.Contains(lower, noun) {
			return true
		}
	}
	return false
}

func impliedTitleNoun(roleFamilies []RoleFamilyID) string {
	roles := NormalizeRoleFamilies(roleFamilies)
	for _, role := range roles {
		switch role {
		case RoleProductManagement:
			return "Manager"
		case RoleDesign:
			return "Designer"
		}
	}
	for _, role := range roles {
		switch role {
		case RoleFrontendEngineering, RoleBackendEngineering, RoleFullStackEngineering, RoleDevOpsSRESystems, RoleAIMLEngineering, RoleData, RoleOtherSpecialized:
			return "Engineer"
		}
	}
	return ""
}

func defaultTargetTitleNamesForRoles(roleFamilies []RoleFamilyID) []string {
	var out []string
	for _, role := range NormalizeRoleFamilies(roleFamilies) {
		switch role {
		case RoleFrontendEngineering:
			out = append(out, "Frontend Engineer")
		case RoleBackendEngineering:
			out = append(out, "Backend Engineer")
		case RoleFullStackEngineering:
			out = append(out, "Full Stack Engineer")
		case RoleDevOpsSRESystems:
			out = append(out, "DevOps Engineer", "Site Reliability Engineer", "Platform Engineer")
		case RoleAIMLEngineering:
			out = append(out, "Machine Learning Engineer", "AI Engineer")
		case RoleData:
			out = append(out, "Data Engineer")
		case RoleDesign:
			out = append(out, "Product Designer")
		case RoleProductManagement:
			out = append(out, "Product Manager")
		}
	}
	return out
}

func titleCaseTitleTerm(value string) string {
	parts := strings.Fields(strings.ReplaceAll(value, "-", " "))
	for i, part := range parts {
		if part == "" {
			continue
		}
		switch strings.ToLower(part) {
		case "ai":
			parts[i] = "AI"
		case "ml":
			parts[i] = "ML"
		case "sre":
			parts[i] = "SRE"
		default:
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}
	return strings.Join(parts, " ")
}
