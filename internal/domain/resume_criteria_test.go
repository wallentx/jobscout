package domain

import (
	"testing"
)

func TestNormalizeExtractedText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Removes excess whitespace",
			input:    "This   is \n\n a \t test.",
			expected: "This is \na test.",
		},
		{
			name:     "Trims edges",
			input:    "  Hello World  ",
			expected: "Hello World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeExtractedText(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeExtractedText() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTextFromXML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Extracts text nodes",
			input:    `<document><p>Hello <b>World</b></p><p>New paragraph</p></document>`,
			expected: "Hello World\nNew paragraph",
		},
		{
			name:     "Handles empty XML",
			input:    `<document></document>`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := textFromXML([]byte(tt.input))
			// Account for potential spacing differences based on actual implementation
			normalized := normalizeExtractedText(result)
			if normalized != tt.expected {
				t.Errorf("textFromXML() normalized = %q, want %q", normalized, tt.expected)
			}
		})
	}
}
