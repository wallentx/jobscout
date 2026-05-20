package fetcher

import (
	"context"
	"net/url"
	"sync"
	"testing"
	"time"
)

func TestDecodeSearchResultURLDuckDuckGoRedirect(t *testing.T) {
	raw := "https://duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fabout%3Fref%3Dsearch"
	if got, want := decodeSearchResultURL(raw), "https://example.com/about?ref=search"; got != want {
		t.Fatalf("decodeSearchResultURL(%q) = %q; want %q", raw, got, want)
	}
}

func TestScoreCompanySiteCandidatePrefersOfficialDomain(t *testing.T) {
	official := scoreCompanySiteCandidate("Acme", "Acme Official Site", "https://www.acme.com/")
	directory := scoreCompanySiteCandidate("Acme", "Acme on LinkedIn", "https://www.linkedin.com/company/acme/")

	if official <= 0 {
		t.Fatalf("scoreCompanySiteCandidate(%q, %q, %q) = %d; want positive score", "Acme", "Acme Official Site", "https://www.acme.com/", official)
	}
	if directory != 0 {
		t.Fatalf("scoreCompanySiteCandidate(%q, %q, %q) = %d; want 0 for excluded host", "Acme", "Acme on LinkedIn", "https://www.linkedin.com/company/acme/", directory)
	}
}

func TestScoreCompanySiteCandidateRejectsDirectoryHostTitleMatch(t *testing.T) {
	got := scoreCompanySiteCandidate("Loft Orbital Solutions", "Loft Orbital Solutions | Aviation Week Marketplace", "https://marketplace.aviationweek.com/company/loft-orbital")
	if got != 0 {
		t.Fatalf("scoreCompanySiteCandidate(%q, %q, %q) = %d; want 0 for title-only directory match", "Loft Orbital Solutions", "Loft Orbital Solutions | Aviation Week Marketplace", "https://marketplace.aviationweek.com/company/loft-orbital", got)
	}
}

func TestChooseCompanyAboutURLPrefersSameHostAboutPage(t *testing.T) {
	links := []pageLink{
		{Text: "Careers", URL: "https://example.com/careers"},
		{Text: "About Us", URL: "https://example.com/about"},
		{Text: "Team", URL: "https://blog.example.com/team"},
	}

	if got, want := chooseCompanyAboutURL("https://example.com/", links), "https://example.com/about"; got != want {
		t.Fatalf("chooseCompanyAboutURL(%q, %v) = %q; want %q", "https://example.com/", links, got, want)
	}
}

func TestCanonicalCompanySiteURLReturnsHostRoot(t *testing.T) {
	raw := "https://www.example.com/about?x=1"
	if got, want := canonicalCompanySiteURL(raw), "https://www.example.com/"; got != want {
		t.Fatalf("canonicalCompanySiteURL(%q) = %q; want %q", raw, got, want)
	}
}

func TestScoreCompanyAboutLinkRejectsExternalHosts(t *testing.T) {
	baseURL, err := url.Parse("https://example.com/")
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", "https://example.com/", err)
	}

	link := pageLink{
		Text: "About Example",
		URL:  "https://other.example.org/about",
	}
	if got := scoreCompanyAboutLink(baseURL, link); got != 0 {
		t.Fatalf("scoreCompanyAboutLink(%q, %v) = %d; want 0 for external host", baseURL.String(), link, got)
	}
}

func TestEmployerReviewSignalFromSearchResultExtractsRatingAndFlags(t *testing.T) {
	signal := employerReviewSignalFromSearchResult("glassdoor", "Acme Reviews | Glassdoor", pageLink{
		Text: "Acme Reviews: What Is It Like to Work At Acme?",
		URL:  "https://www.glassdoor.com/Reviews/Acme-Reviews-E123.htm",
	})

	if signal.Source != "glassdoor" {
		t.Fatalf("employerReviewSignalFromSearchResult().Source = %q; want glassdoor", signal.Source)
	}
	if signal.Rating != "" {
		t.Fatalf("employerReviewSignalFromSearchResult().Rating = %q; want empty without rating text", signal.Rating)
	}

	withSnippet := employerReviewSignalFromSearchResult(
		"indeed",
		"Search results Acme Employee Reviews 3.7 out of 5 employees mention good culture and long hours",
		pageLink{Text: "Acme Employee Reviews", URL: "https://www.indeed.com/cmp/Acme/reviews"},
	)
	if withSnippet.Rating != "3.7/5" {
		t.Fatalf("employerReviewSignalFromSearchResult().Rating = %q; want 3.7/5", withSnippet.Rating)
	}
	if len(withSnippet.Flags) == 0 {
		t.Fatalf("employerReviewSignalFromSearchResult().Flags = %v; want extracted flags", withSnippet.Flags)
	}
}

func TestEmployerReviewSourceRecognizesReviewHosts(t *testing.T) {
	tests := []struct {
		rawURL string
		want   string
	}{
		{rawURL: "https://www.glassdoor.com/Reviews/Acme-Reviews-E123.htm", want: "glassdoor"},
		{rawURL: "https://www.indeed.com/cmp/Acme/reviews", want: "indeed"},
		{rawURL: "https://example.com/reviews", want: ""},
	}

	for _, tt := range tests {
		if got := employerReviewSource(tt.rawURL); got != tt.want {
			t.Errorf("employerReviewSource(%q) = %q; want %q", tt.rawURL, got, tt.want)
		}
	}
}

func TestReusableBrowserPagePoolLimitsConfiguredConcurrency(t *testing.T) {
	pool := newReusableBrowserPagePool(nil, 2)
	ctx := context.Background()

	first, err := pool.acquire(ctx)
	if err != nil {
		t.Fatalf("first acquire error = %v", err)
	}
	second, err := pool.acquire(ctx)
	if err != nil {
		t.Fatalf("second acquire error = %v", err)
	}

	blocked := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		slot, err := pool.acquire(ctx)
		if err != nil {
			t.Errorf("third acquire error = %v", err)
			close(blocked)
			return
		}
		close(blocked)
		pool.release(slot)
	}()

	select {
	case <-blocked:
		t.Fatal("third acquire completed before a slot was released")
	case <-time.After(20 * time.Millisecond):
	}

	pool.release(first)
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("third acquire did not complete after a slot was released")
	}

	pool.release(second)
	wg.Wait()
}
