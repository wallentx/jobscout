package domain

import "testing"

func TestParseCSVListAcceptsCommasAndNewlines(t *testing.T) {
	got := ParseCSVList("Kubernetes\nGo, reliability")
	want := []string{"Kubernetes", "Go", "reliability"}
	if len(got) != len(want) {
		t.Fatalf("ParseCSVList() len = %d; want %d (%#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ParseCSVList()[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

func TestParseWorkSettingsAcceptsNewlineSeparatedValues(t *testing.T) {
	got := ParseWorkSettings("remote\non-site")
	if !got.Remote {
		t.Fatal("ParseWorkSettings(...).Remote = false; want true")
	}
	if !got.Onsite {
		t.Fatal("ParseWorkSettings(...).Onsite = false; want true")
	}
	if got.Hybrid {
		t.Fatal("ParseWorkSettings(...).Hybrid = true; want false")
	}
}
