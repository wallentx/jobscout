package tuiapp

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestPlaceOverlayPlacesForegroundAtCoordinates(t *testing.T) {
	bg := strings.Join([]string{
		"abcdef",
		"ghijkl",
		"mnopqr",
	}, "\n")

	got := placeOverlay(2, 1, "XX", bg)
	want := strings.Join([]string{
		"abcdef",
		"ghXXkl",
		"mnopqr",
	}, "\n")
	if got != want {
		t.Fatalf("placeOverlay() = %q; want %q", got, want)
	}
}

func TestPlaceOverlayClampsToBackground(t *testing.T) {
	bg := strings.Join([]string{
		"abcdef",
		"ghijkl",
		"mnopqr",
	}, "\n")

	got := placeOverlay(99, 99, "XX", bg)
	want := strings.Join([]string{
		"abcdef",
		"ghijkl",
		"mnopXX",
	}, "\n")
	if got != want {
		t.Fatalf("placeOverlay() = %q; want %q", got, want)
	}
}

func TestPlaceOverlayPreservesStyledForeground(t *testing.T) {
	bg := "abcdef"
	fg := "\x1b[31mXX\x1b[0m"

	got := placeOverlay(2, 0, fg, bg)
	if ansi.Strip(got) != "abXXef" {
		t.Fatalf("visible placeOverlay() = %q; want %q", ansi.Strip(got), "abXXef")
	}
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("placeOverlay() = %q; want ANSI styling preserved", got)
	}
}
