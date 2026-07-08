package ui

import (
	"testing"
)

func TestParseYouTrackURL(t *testing.T) {
	tests := []struct {
		url          string
		expectedBase string
		expectedID   string
	}{
		{
			url:          "https://youtrack.example.com/issue/PROJ-12345",
			expectedBase: "https://youtrack.example.com",
			expectedID:   "PROJ-12345",
		},
		{
			url:          "https://youtrack.example.com/projects/PROJ/issues/PROJ-12346",
			expectedBase: "https://youtrack.example.com",
			expectedID:   "PROJ-12346",
		},
		{
			url:          "https://youtrack.example.com/issues/PROJ-12346",
			expectedBase: "https://youtrack.example.com",
			expectedID:   "PROJ-12346",
		},
		{
			url:          "http://localhost:8080/issue/PROJ-123?query=abc#hash",
			expectedBase: "http://localhost:8080",
			expectedID:   "PROJ-123",
		},
		{
			url:          "https://youtrack.example.com/projects/PROJ",
			expectedBase: "",
			expectedID:   "",
		},
		{
			url:          "invalid-url",
			expectedBase: "",
			expectedID:   "",
		},
	}

	for _, tt := range tests {
		base, id := parseYouTrackURL(tt.url)
		if base != tt.expectedBase || id != tt.expectedID {
			t.Errorf("parseYouTrackURL(%q) = (%q, %q); expected (%q, %q)",
				tt.url, base, id, tt.expectedBase, tt.expectedID)
		}
	}
}

func TestExtractIssueIDFromURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "https://youtrack.example.com/issue/PROJ-12345",
			expected: "PROJ-12345",
		},
		{
			input:    "https://youtrack.example.com/projects/PROJ/issues/PROJ-12346",
			expected: "PROJ-12346",
		},
		{
			input:    "https://youtrack.example.com/issues/PROJ-12346",
			expected: "PROJ-12346",
		},
		{
			input:    "PROJ-12345",
			expected: "PROJ-12345",
		},
	}

	for _, tt := range tests {
		actual := extractIssueIDFromURL(tt.input)
		if actual != tt.expected {
			t.Errorf("extractIssueIDFromURL(%q) = %q; expected %q",
				tt.input, actual, tt.expected)
		}
	}
}
