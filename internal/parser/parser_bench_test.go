package parser_test

import (
	"strings"
	"testing"

	"github.com/xcke/envref/internal/parser"
)

// generateEnvContent creates a .env file content string with n entries.
// Entries include a mix of simple values, quoted values, interpolation refs,
// and ref:// references to simulate realistic workloads.
func generateEnvContent(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		switch i % 5 {
		case 0:
			// Simple unquoted value.
			b.WriteString("KEY_")
			b.WriteString(strings.Repeat("A", 3))
			b.WriteString("_")
			b.WriteString(string(rune('0' + i%10)))
			b.WriteString("=value_")
			b.WriteString(string(rune('0' + i%10)))
			b.WriteByte('\n')
		case 1:
			// Double-quoted value with spaces.
			b.WriteString("QUOTED_")
			b.WriteString(string(rune('0' + i%10)))
			b.WriteString("=\"hello world ")
			b.WriteString(string(rune('0' + i%10)))
			b.WriteString("\"\n")
		case 2:
			// Value with interpolation syntax.
			b.WriteString("INTERP_")
			b.WriteString(string(rune('0' + i%10)))
			b.WriteString("=prefix_${KEY_AAA_")
			b.WriteString(string(rune('0' + (i-2)%10)))
			b.WriteString("}_suffix\n")
		case 3:
			// ref:// reference.
			b.WriteString("SECRET_")
			b.WriteString(string(rune('0' + i%10)))
			b.WriteString("=ref://keychain/api_key_")
			b.WriteString(string(rune('0' + i%10)))
			b.WriteByte('\n')
		case 4:
			// Comment followed by entry.
			b.WriteString("# This is a comment\n")
			b.WriteString("AFTER_COMMENT_")
			b.WriteString(string(rune('0' + i%10)))
			b.WriteString("=value\n")
		}
	}
	return b.String()
}

func BenchmarkParse10(b *testing.B) {
	content := generateEnvContent(10)
	b.ResetTimer()
	for b.Loop() {
		_, _, _ = parser.Parse(strings.NewReader(content))
	}
}

func BenchmarkParse50(b *testing.B) {
	content := generateEnvContent(50)
	b.ResetTimer()
	for b.Loop() {
		_, _, _ = parser.Parse(strings.NewReader(content))
	}
}

func BenchmarkParse100(b *testing.B) {
	content := generateEnvContent(100)
	b.ResetTimer()
	for b.Loop() {
		_, _, _ = parser.Parse(strings.NewReader(content))
	}
}

func BenchmarkParse500(b *testing.B) {
	content := generateEnvContent(500)
	b.ResetTimer()
	for b.Loop() {
		_, _, _ = parser.Parse(strings.NewReader(content))
	}
}
