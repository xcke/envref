package parser

import (
	"bufio"
	"strings"
	"testing"
)

// FuzzParse exercises the .env parser with arbitrary input to catch panics,
// infinite loops, or unexpected crashes. The parser should never panic
// regardless of input.
func FuzzParse(f *testing.F) {
	// Seed corpus with representative .env patterns.
	seeds := []string{
		// Basic key-value.
		"FOO=bar",
		"FOO=bar\nBAZ=qux",
		// Empty value.
		"EMPTY=",
		"EMPTY=  ",
		// Quoted values.
		"SINGLE='hello world'",
		`DOUBLE="hello world"`,
		"BACKTICK=`hello world`",
		// Multiline double-quoted.
		"MULTI=\"line1\nline2\nline3\"",
		// Multiline backtick-quoted.
		"MULTI=`line1\nline2\nline3`",
		// Escape sequences in double quotes.
		`ESCAPED="hello\nworld\t!"`,
		`ESCAPED="slash\\"`,
		`ESCAPED="quote\""`,
		// Comments.
		"# this is a comment\nFOO=bar",
		"FOO=bar # inline comment",
		"FOO=bar#notacomment",
		// Export prefix.
		"export FOO=bar",
		"export  FOO=bar",
		// BOM.
		"\xEF\xBB\xBFFOO=bar",
		// CRLF.
		"FOO=bar\r\nBAZ=qux\r\n",
		// Duplicate keys.
		"FOO=first\nFOO=second",
		// ref:// values.
		"SECRET=ref://secrets/api_key",
		"DB=ref://keychain/db_pass",
		// No = sign (should be ignored).
		"NOEQUALSSIGN",
		// Whitespace around key/value.
		"  FOO  =  bar  ",
		// Empty key.
		"=value",
		// Unterminated quotes.
		"FOO='unterminated",
		`FOO="unterminated`,
		"FOO=`unterminated",
		// Special characters.
		"URL=https://example.com/path?q=1&b=2",
		"JSON={\"key\":\"value\"}",
		// Unicode.
		"EMOJI=\U0001F600",
		"CJK=\u4F60\u597D",
		// Empty input.
		"",
		"\n\n\n",
		"# only comments\n# more comments",
		// Deeply nested escapes.
		`DEEP="\\\\\\\\value"`,
		// Large key.
		strings.Repeat("A", 1000) + "=value",
		// Large value.
		"KEY=" + strings.Repeat("x", 10000),
		// Many entries.
		strings.Repeat("K=V\n", 100),
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data string) {
		// Parse should never panic. Errors are acceptable.
		entries, warnings, err := Parse(strings.NewReader(data))

		// Basic invariant checks on successful parse.
		if err == nil {
			for _, e := range entries {
				// Key must be non-empty.
				if e.Key == "" {
					t.Error("parsed entry has empty key")
				}
				// Line must be positive.
				if e.Line < 1 {
					t.Errorf("parsed entry %q has invalid line number %d", e.Key, e.Line)
				}
				// IsRef must match value prefix.
				if e.IsRef != strings.HasPrefix(e.Value, RefPrefix) {
					t.Errorf("entry %q: IsRef=%v but value=%q", e.Key, e.IsRef, e.Value)
				}
				// Quote must be a valid enum value.
				if e.Quote < QuoteNone || e.Quote > QuoteBacktick {
					t.Errorf("entry %q: invalid QuoteStyle %d", e.Key, e.Quote)
				}
			}

			// Warnings should have valid line numbers.
			for _, w := range warnings {
				if w.Line < 1 {
					t.Errorf("warning has invalid line number %d: %s", w.Line, w.Message)
				}
			}
		}
	})
}

// FuzzParseValue exercises value parsing with various quote styles and content.
// Uses the internal parseValue function directly.
func FuzzParseValue(f *testing.F) {
	seeds := []string{
		"bar",
		"'single quoted'",
		`"double quoted"`,
		"`backtick quoted`",
		"  spaced  ",
		"value # comment",
		`"multi\nline"`,
		`"escaped\"quote"`,
		"'unmatched",
		`"unmatched`,
		"`unmatched",
		"",
		"ref://secrets/key",
		`"ref://secrets/key"`,
		"   'hello'  ",
		`"\n\t\r\\\""`,
		"value with spaces",
		"no#comment",
		"yes #comment",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data string) {
		// parseValue should never panic. It takes raw value, scanner, lineNum.
		// We pass a scanner over empty input since single-line values won't read it.
		scanner := newTestScanner("")
		_, _, lineNum, quote, _ := parseValue(data, scanner, 1)

		// Line number must not decrease.
		if lineNum < 1 {
			t.Errorf("parseValue returned invalid lineNum %d", lineNum)
		}

		// Quote must be valid.
		if quote < QuoteNone || quote > QuoteBacktick {
			t.Errorf("parseValue returned invalid QuoteStyle %d", quote)
		}
	})
}

// FuzzStripInlineComment exercises the inline comment stripping logic.
func FuzzStripInlineComment(f *testing.F) {
	seeds := []string{
		"value",
		"value # comment",
		"value#notcomment",
		"# all comment",
		"",
		"a",
		"a #",
		"a # b # c",
		strings.Repeat("x", 1000),
		strings.Repeat("x #", 100),
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data string) {
		result := stripInlineComment(data)

		// Result must be a prefix of input (or identical).
		if !strings.HasPrefix(data, result) {
			t.Errorf("stripInlineComment(%q) = %q, not a prefix of input", data, result)
		}
	})
}

// FuzzProcessDoubleQuoteEscapes exercises escape processing.
func FuzzProcessDoubleQuoteEscapes(f *testing.F) {
	seeds := []string{
		"hello",
		`hello\nworld`,
		`hello\tworld`,
		`hello\\world`,
		`hello\"world`,
		`hello\rworld`,
		`trailing\`,
		`\\\\`,
		`\n\t\r\\\"`,
		"",
		`\x`,
		`\`,
		`\\`,
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data string) {
		// Should never panic.
		_ = processDoubleQuoteEscapes(data)
	})
}

// newTestScanner creates a scanner over the given content for use in fuzz tests.
func newTestScanner(content string) *bufio.Scanner {
	return bufio.NewScanner(strings.NewReader(content))
}
