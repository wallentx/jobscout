package domain

import "strings"

type RoleFamilyID string

const (
	RoleFrontendEngineering  RoleFamilyID = "frontend"
	RoleBackendEngineering   RoleFamilyID = "backend"
	RoleFullStackEngineering RoleFamilyID = "fullstack"
	RoleDevOpsSRESystems     RoleFamilyID = "devops_sre_systems"
	RoleAIMLEngineering      RoleFamilyID = "ai_ml"
	RoleData                 RoleFamilyID = "data"
	RoleDesign               RoleFamilyID = "design"
	RoleProductManagement    RoleFamilyID = "product_management"
	RoleOtherSpecialized     RoleFamilyID = "other_specialized"
)

type RoleFamilySpec struct {
	ID    RoleFamilyID
	Label string
}

func RoleFamilySpecs() []RoleFamilySpec {
	return []RoleFamilySpec{
		{ID: RoleFrontendEngineering, Label: "Frontend Engineering"},
		{ID: RoleBackendEngineering, Label: "Backend Engineering"},
		{ID: RoleFullStackEngineering, Label: "Full-Stack Engineering"},
		{ID: RoleDevOpsSRESystems, Label: "DevOps / SRE / Systems"},
		{ID: RoleAIMLEngineering, Label: "AI / ML Engineering"},
		{ID: RoleData, Label: "Data"},
		{ID: RoleDesign, Label: "Design"},
		{ID: RoleProductManagement, Label: "Product Management"},
		{ID: RoleOtherSpecialized, Label: "Other / Specialized"},
	}
}

func AllRoleFamilyIDs() []RoleFamilyID {
	specs := RoleFamilySpecs()
	ids := make([]RoleFamilyID, 0, len(specs))
	for _, spec := range specs {
		ids = append(ids, spec.ID)
	}
	return ids
}

func RoleFamilyLabel(id RoleFamilyID) string {
	for _, spec := range RoleFamilySpecs() {
		if spec.ID == id {
			return spec.Label
		}
	}
	return string(id)
}

func ParseRoleFamilyValue(value string) (RoleFamilyID, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "", false
	}

	for _, spec := range RoleFamilySpecs() {
		if value == string(spec.ID) {
			return spec.ID, true
		}
		if value == strings.ToLower(spec.Label) {
			return spec.ID, true
		}
	}

	return "", false
}

func ParseRoleFamilyCSV(value string) ([]RoleFamilyID, error) {
	parts := strings.Split(value, ",")
	out := make([]RoleFamilyID, 0, len(parts))
	seen := make(map[RoleFamilyID]bool, len(parts))
	for _, part := range parts {
		role, ok := ParseRoleFamilyValue(part)
		if !ok {
			if strings.TrimSpace(part) == "" {
				continue
			}
			return nil, &InvalidRoleFamilyError{Value: strings.TrimSpace(part)}
		}
		if seen[role] {
			continue
		}
		seen[role] = true
		out = append(out, role)
	}
	return out, nil
}

type InvalidRoleFamilyError struct {
	Value string
}

func (e *InvalidRoleFamilyError) Error() string {
	return "unknown role family: " + e.Value
}

func FormatRoleFamilyIDs(ids []RoleFamilyID) string {
	ids = NormalizeRoleFamilies(ids)
	if len(ids) == 0 {
		return ""
	}

	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, string(id))
	}
	return strings.Join(values, ", ")
}

func FormatRoleFamilyLabels(ids []RoleFamilyID) string {
	ids = NormalizeRoleFamilies(ids)
	if len(ids) == 0 {
		return "none"
	}

	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, RoleFamilyLabel(id))
	}
	return strings.Join(values, ", ")
}

func NormalizeRoleFamilies(values []RoleFamilyID) []RoleFamilyID {
	valid := make(map[RoleFamilyID]bool, len(RoleFamilySpecs()))
	for _, spec := range RoleFamilySpecs() {
		valid[spec.ID] = true
	}

	seen := make(map[RoleFamilyID]bool, len(values))
	out := make([]RoleFamilyID, 0, len(values))
	for _, value := range values {
		value = RoleFamilyID(strings.TrimSpace(string(value)))
		if value == "" || !valid[value] || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
