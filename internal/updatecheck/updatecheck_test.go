package updatecheck

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestComparableCurrentVersion(t *testing.T) {
	tests := []struct {
		name    string
		current string
		want    string
		wantOK  bool
	}{
		{name: "release", current: "v1.2.3", want: "v1.2.3", wantOK: true},
		{name: "dev build", current: "v1.2.3-abcdef0", want: "v1.2.3", wantOK: true},
		{name: "no tag dev build", current: "v0.0.0-abcdef0", want: "v0.0.0", wantOK: true},
		{name: "prerelease", current: "v1.2.3-rc.1", want: "v1.2.3-rc.1", wantOK: true},
		{name: "no v prefix", current: "1.2.3", want: "v1.2.3", wantOK: true},
		{name: "dev", current: "dev", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := comparableCurrentVersion(tt.current)
			if ok != tt.wantOK {
				t.Fatalf("comparableCurrentVersion(%q) ok = %t; want %t", tt.current, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("comparableCurrentVersion(%q) = %q; want %q", tt.current, got, tt.want)
			}
		})
	}
}

func TestCheckLatestReleaseDetectsAvailableUpdate(t *testing.T) {
	var gotUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v1.3.0","html_url":"https://github.com/wallentx/jobscout/releases/tag/v1.3.0"}`))
	}))
	defer server.Close()

	result, err := CheckLatestReleaseWithClient(context.Background(), "v1.2.3-abcdef0", server.URL, server.Client())
	if err != nil {
		t.Fatalf("CheckLatestReleaseWithClient() error = %v", err)
	}
	if !result.Available {
		t.Fatal("CheckLatestReleaseWithClient().Available = false; want true")
	}
	if result.LatestVersion != "v1.3.0" {
		t.Fatalf("LatestVersion = %q; want v1.3.0", result.LatestVersion)
	}
	if gotUserAgent != userAgent {
		t.Fatalf("User-Agent = %q; want %q", gotUserAgent, userAgent)
	}
}

func TestCheckLatestReleaseDoesNotFlagSameTagDevBuild(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.3","html_url":"https://github.com/wallentx/jobscout/releases/tag/v1.2.3"}`))
	}))
	defer server.Close()

	result, err := CheckLatestReleaseWithClient(context.Background(), "v1.2.3-abcdef0", server.URL, server.Client())
	if err != nil {
		t.Fatalf("CheckLatestReleaseWithClient() error = %v", err)
	}
	if result.Available {
		t.Fatal("CheckLatestReleaseWithClient().Available = true; want false")
	}
}

func TestCheckLatestReleaseSkipsUncomparableCurrentVersion(t *testing.T) {
	result, err := CheckLatestReleaseWithClient(context.Background(), "dev", "https://example.invalid", nil)
	if !errors.Is(err, ErrVersionNotComparable) {
		t.Fatalf("CheckLatestReleaseWithClient() error = %v; want ErrVersionNotComparable", err)
	}
	if result.Available {
		t.Fatal("CheckLatestReleaseWithClient().Available = true; want false")
	}
}
