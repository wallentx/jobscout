package domain

import "testing"

func TestNormalizeTitlePrefixes(t *testing.T) {
	got := NormalizeTitlePrefixes([]string{"Jr.", "sr", "mid-level", "Engineer", "Staff", "sr."})
	want := []string{"Junior", "Senior", "Staff"}
	if len(got) != len(want) {
		t.Fatalf("NormalizeTitlePrefixes() len = %d; want %d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeTitlePrefixes()[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

func TestNormalizeTargetTitleNames(t *testing.T) {
	got := NormalizeTargetTitleNames(
		[]string{"frontend", "software dev", "platform eng", "Engineer", "Fullstack"},
		[]RoleFamilyID{RoleFrontendEngineering, RoleDevOpsSRESystems},
	)
	want := []string{"Frontend Engineer", "Software Developer", "Platform Engineer", "Full Stack Engineer"}
	if len(got) != len(want) {
		t.Fatalf("NormalizeTargetTitleNames() len = %d; want %d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeTargetTitleNames()[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

func TestNormalizeTargetTitleNamesDefaultsByRoleFamily(t *testing.T) {
	got := NormalizeTargetTitleNames([]string{"Engineer", "Developer"}, []RoleFamilyID{RoleBackendEngineering, RoleProductManagement})
	want := []string{"Backend Engineer", "Product Manager"}
	if len(got) != len(want) {
		t.Fatalf("NormalizeTargetTitleNames() len = %d; want %d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeTargetTitleNames()[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}
