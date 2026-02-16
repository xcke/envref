package suggest

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b     string
		expected int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "a", 1},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"abc", "abcd", 1},
		{"abc", "ab", 1},
		{"kitten", "sitting", 3},
		{"saturday", "sunday", 3},
		{"api_key", "api_kye", 2},
		{"DB_HOST", "DB_PORT", 2},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			got := levenshtein(tt.a, tt.b)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestKeys(t *testing.T) {
	candidates := []string{
		"API_KEY",
		"API_SECRET",
		"DB_HOST",
		"DB_PORT",
		"DB_NAME",
		"LOG_LEVEL",
	}

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "exact match excluded",
			input:    "API_KEY",
			expected: nil,
		},
		{
			name:     "close typo",
			input:    "API_KYE",
			expected: []string{"API_KEY"},
		},
		{
			name:     "case insensitive",
			input:    "api_kye",
			expected: []string{"API_KEY"},
		},
		{
			name:     "missing underscore",
			input:    "APIKEY",
			expected: []string{"API_KEY"},
		},
		{
			name:     "DB confusion",
			input:    "DB_HOTS",
			expected: []string{"DB_HOST", "DB_PORT"},
		},
		{
			name:     "multiple close matches",
			input:    "DB_HORT",
			expected: []string{"DB_HOST", "DB_PORT"},
		},
		{
			name:     "too far away",
			input:    "COMPLETELY_DIFFERENT",
			expected: nil,
		},
		{
			name:     "empty input",
			input:    "",
			expected: nil,
		},
		{
			name:     "empty candidates",
			input:    "API_KEY",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cands := candidates
			if tt.name == "empty candidates" {
				cands = nil
			}
			got := Keys(tt.input, cands)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestKeys_LimitsToMaxSuggestions(t *testing.T) {
	// Create many candidates that are all close to the input.
	candidates := []string{"AA", "AB", "AC", "AD", "AE"}
	got := Keys("A", candidates)
	assert.LessOrEqual(t, len(got), maxSuggestions)
}

func TestFormatSuggestion(t *testing.T) {
	tests := []struct {
		name        string
		suggestions []string
		expected    string
	}{
		{
			name:        "no suggestions",
			suggestions: nil,
			expected:    "",
		},
		{
			name:        "one suggestion",
			suggestions: []string{"API_KEY"},
			expected:    "; did you mean API_KEY?",
		},
		{
			name:        "multiple suggestions",
			suggestions: []string{"DB_HOST", "DB_PORT"},
			expected:    "; did you mean one of: DB_HOST, DB_PORT?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSuggestion(tt.suggestions)
			assert.Equal(t, tt.expected, got)
		})
	}
}
