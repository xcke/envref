// Package suggest provides fuzzy key matching for "did you mean?" suggestions
// in error messages.
package suggest

import (
	"sort"
	"strings"
)

// maxDistance is the maximum Levenshtein distance for a key to be considered
// a plausible suggestion. Keys further away are not suggested.
const maxDistance = 3

// maxSuggestions is the maximum number of suggestions to return.
const maxSuggestions = 3

// Keys returns up to maxSuggestions keys from candidates that are similar to
// the input key, sorted by edit distance (closest first). Keys with a
// Levenshtein distance greater than maxDistance are excluded.
//
// Comparison is case-insensitive, but original candidate strings are returned.
func Keys(input string, candidates []string) []string {
	if len(candidates) == 0 {
		return nil
	}

	inputLower := strings.ToLower(input)

	type scored struct {
		key      string
		distance int
	}

	var matches []scored
	for _, c := range candidates {
		d := levenshtein(inputLower, strings.ToLower(c))
		if d > 0 && d <= maxDistance {
			matches = append(matches, scored{key: c, distance: d})
		}
	}

	if len(matches) == 0 {
		return nil
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].distance != matches[j].distance {
			return matches[i].distance < matches[j].distance
		}
		return matches[i].key < matches[j].key
	})

	n := len(matches)
	if n > maxSuggestions {
		n = maxSuggestions
	}

	result := make([]string, n)
	for i := 0; i < n; i++ {
		result[i] = matches[i].key
	}
	return result
}

// FormatSuggestion returns a "did you mean ...?" string for the given
// suggestions, or an empty string if there are no suggestions.
func FormatSuggestion(suggestions []string) string {
	switch len(suggestions) {
	case 0:
		return ""
	case 1:
		return "; did you mean " + suggestions[0] + "?"
	default:
		return "; did you mean one of: " + strings.Join(suggestions, ", ") + "?"
	}
}

// levenshtein computes the Levenshtein edit distance between two strings.
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Use two rows instead of a full matrix for O(min(m,n)) space.
	if len(a) > len(b) {
		a, b = b, a
	}

	prev := make([]int, len(a)+1)
	curr := make([]int, len(a)+1)

	for i := range prev {
		prev[i] = i
	}

	for j := 1; j <= len(b); j++ {
		curr[0] = j
		for i := 1; i <= len(a); i++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[i] = min3(
				curr[i-1]+1,   // insertion
				prev[i]+1,     // deletion
				prev[i-1]+cost, // substitution
			)
		}
		prev, curr = curr, prev
	}

	return prev[len(a)]
}

// min3 returns the smallest of three integers.
func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
