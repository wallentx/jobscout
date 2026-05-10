package tuiapp

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderPopupSpecWrapsBodyAndFooter(t *testing.T) {
	spec := popupSpec{
		width:  54,
		body:   popupTextBody(popupBodyStyle.Render("This body line is intentionally long enough to wrap inside the shared popup body renderer.")),
		footer: popupHintStyle.Render("This footer line is intentionally long enough to wrap inside the shared popup footer renderer."),
	}
	rendered := renderPopupSpec("", 90, 40, spec)
	stripped := ansi.Strip(rendered)

	for _, expected := range []string{
		"shared popup body",
		"shared popup footer",
	} {
		if !strings.Contains(stripped, expected) {
			t.Fatalf("rendered popup missing %q in:\n%s", expected, stripped)
		}
	}
}

func TestPopupMaxViewportLinesTracksTableHeight(t *testing.T) {
	if got, want := popupMaxViewportLinesWithChrome(80, 4, 11), calculateTableHeight(80)+2-11; got != want {
		t.Fatalf("popupMaxViewportLinesWithChrome(80, 4, 11) = %d; want table height plus two minus chrome = %d", got, want)
	}
	if got, want := popupMaxViewportLinesWithChrome(30, 4, 8), calculateTableHeight(30)+2-8; got != want {
		t.Fatalf("popupMaxViewportLinesWithChrome(30, 4, 8) = %d; want table height plus two minus chrome = %d", got, want)
	}
	if got := popupMaxViewportLinesWithChrome(10, 8, 11); got != 8 {
		t.Fatalf("popupMaxViewportLinesWithChrome(10, 8, 11) = %d; want minimum 8", got)
	}
}

func TestNoticePopupStylesOnlyURLSegment(t *testing.T) {
	line := "Searching site target: https://builtin.com/jobs/remote"

	ranges := popupURLRanges(line)

	if len(ranges) != 1 {
		t.Fatalf("popupURLRanges(%q) len = %d; want 1", line, len(ranges))
	}
	url := line[ranges[0].start:ranges[0].end]
	if url != "https://builtin.com/jobs/remote" {
		t.Fatalf("popupURLRanges(%q) URL segment = %q; want URL only", line, url)
	}
	prefix := line[:ranges[0].start]
	if prefix != "Searching site target: " {
		t.Fatalf("popupURLRanges(%q) prefix = %q; want non-URL text outside URL range", line, prefix)
	}
}

func TestNoticePopupStylesURLContinuationTokenOnly(t *testing.T) {
	if !looksLikeURLContinuationToken("country=USA") {
		t.Fatal("looksLikeURLContinuationToken(\"country=USA\") = false; want true")
	}
	if looksLikeURLContinuationToken("(1)") {
		t.Fatal("looksLikeURLContinuationToken(\"(1)\") = true; want false")
	}
}
