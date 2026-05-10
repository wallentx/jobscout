package tuiapp

import (
	"errors"
	"testing"
)

func TestURLOpenCommand(t *testing.T) {
	tests := []struct {
		name      string
		goos      string
		available map[string]bool
		want      string
	}{
		{
			name:      "termux opener wins",
			goos:      "darwin",
			available: map[string]bool{"termux-open-url": true, "open": true},
			want:      "termux-open-url",
		},
		{
			name:      "macOS open",
			goos:      "darwin",
			available: map[string]bool{"open": true},
			want:      "open",
		},
		{
			name:      "Linux xdg-open",
			goos:      "linux",
			available: map[string]bool{"xdg-open": true},
			want:      "xdg-open",
		},
		{
			name:      "Linux ignores open",
			goos:      "linux",
			available: map[string]bool{"open": true},
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookPath := func(name string) (string, error) {
				if tt.available[name] {
					return "/usr/bin/" + name, nil
				}
				return "", errors.New("not found")
			}
			if got := urlOpenCommand(tt.goos, lookPath); got != tt.want {
				t.Fatalf("urlOpenCommand(%q, lookPath) = %q; want %q", tt.goos, got, tt.want)
			}
		})
	}
}
