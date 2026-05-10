package fetcher

import "testing"

func TestExtractHTMLHrefsUsesParsedHTML(t *testing.T) {
	rawHTML := `<div><a data-kind="apply" href=https://example.com/apply?x=1&amp;y=2>Apply</a></div>`

	got := extractHTMLHrefs(rawHTML)
	if len(got) != 1 || got[0] != "https://example.com/apply?x=1&y=2" {
		t.Fatalf("extractHTMLHrefs(%q) = %#v; want decoded apply URL", rawHTML, got)
	}
}

func TestExtractMetaContentHandlesAttributeOrder(t *testing.T) {
	rawHTML := `<html><head><meta content="Acme builds deployment tools." name="description"></head></html>`

	got := extractMetaContent(rawHTML, "description")
	want := "Acme builds deployment tools."
	if got != want {
		t.Fatalf("extractMetaContent(%q, description) = %q; want %q", rawHTML, got, want)
	}
}

func TestNormalizeHTMLTextSeparatesBlockText(t *testing.T) {
	rawHTML := `<section><div>Industry</div><div>Developer Tools</div></section>`

	got := normalizeHTMLText(rawHTML)
	want := "Industry Developer Tools"
	if got != want {
		t.Fatalf("normalizeHTMLText(%q) = %q; want %q", rawHTML, got, want)
	}
}

func TestExtractStructuredJobPostingsHandlesScriptTypeCase(t *testing.T) {
	rawHTML := `<script TYPE="APPLICATION/LD+JSON">{"@type":"JobPosting","title":"Software Engineer"}</script>`

	got := extractStructuredJobPostings(rawHTML)
	if len(got) != 1 {
		t.Fatalf("extractStructuredJobPostings(%q) len = %d; want 1", rawHTML, len(got))
	}
	if got[0]["title"] != "Software Engineer" {
		t.Fatalf("extractStructuredJobPostings(%q)[0][title] = %#v; want Software Engineer", rawHTML, got[0]["title"])
	}
}
