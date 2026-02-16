package parser

import (
	"fmt"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []Entry
		wantErr bool
	}{
		{
			name:  "simple key=value",
			input: "FOO=bar",
			want: []Entry{
				{Key: "FOO", Value: "bar", Raw: "bar", Line: 1},
			},
		},
		{
			name:  "multiple pairs",
			input: "FOO=bar\nBAZ=qux",
			want: []Entry{
				{Key: "FOO", Value: "bar", Raw: "bar", Line: 1},
				{Key: "BAZ", Value: "qux", Raw: "qux", Line: 2},
			},
		},
		{
			name:  "empty value",
			input: "FOO=",
			want: []Entry{
				{Key: "FOO", Value: "", Raw: "", Line: 1},
			},
		},
		{
			name:  "skip empty lines",
			input: "\nFOO=bar\n\nBAZ=qux\n",
			want: []Entry{
				{Key: "FOO", Value: "bar", Raw: "bar", Line: 2},
				{Key: "BAZ", Value: "qux", Raw: "qux", Line: 4},
			},
		},
		{
			name:  "skip comment lines",
			input: "# this is a comment\nFOO=bar\n# another comment\nBAZ=qux",
			want: []Entry{
				{Key: "FOO", Value: "bar", Raw: "bar", Line: 2},
				{Key: "BAZ", Value: "qux", Raw: "qux", Line: 4},
			},
		},
		{
			name:  "inline comment on unquoted value",
			input: "FOO=bar # this is a comment",
			want: []Entry{
				{Key: "FOO", Value: "bar", Raw: "bar # this is a comment", Line: 1},
			},
		},
		{
			name:  "hash without preceding space is not a comment",
			input: "FOO=bar#baz",
			want: []Entry{
				{Key: "FOO", Value: "bar#baz", Raw: "bar#baz", Line: 1},
			},
		},
		{
			name:  "export prefix",
			input: "export FOO=bar",
			want: []Entry{
				{Key: "FOO", Value: "bar", Raw: "bar", Line: 1},
			},
		},
		{
			name:  "whitespace around key and value",
			input: "  FOO  =  bar  ",
			want: []Entry{
				{Key: "FOO", Value: "bar", Raw: "  bar", Line: 1},
			},
		},
		{
			name:  "single-quoted value preserves content literally",
			input: "FOO='bar baz'",
			want: []Entry{
				{Key: "FOO", Value: "bar baz", Raw: "'bar baz'", Line: 1},
			},
		},
		{
			name:  "single-quoted value preserves special characters",
			input: `FOO='hello\nworld $VAR'`,
			want: []Entry{
				{Key: "FOO", Value: `hello\nworld $VAR`, Raw: `'hello\nworld $VAR'`, Line: 1},
			},
		},
		{
			name:  "double-quoted value processes escapes",
			input: `FOO="hello\nworld"`,
			want: []Entry{
				{Key: "FOO", Value: "hello\nworld", Raw: `"hello\nworld"`, Line: 1},
			},
		},
		{
			name:  "double-quoted value with tab escape",
			input: `FOO="col1\tcol2"`,
			want: []Entry{
				{Key: "FOO", Value: "col1\tcol2", Raw: `"col1\tcol2"`, Line: 1},
			},
		},
		{
			name:  "double-quoted value with escaped quote",
			input: `FOO="say \"hello\""`,
			want: []Entry{
				{Key: "FOO", Value: `say "hello"`, Raw: `"say \"hello\""`, Line: 1},
			},
		},
		{
			name:  "double-quoted value with escaped backslash",
			input: `FOO="path\\to\\file"`,
			want: []Entry{
				{Key: "FOO", Value: `path\to\file`, Raw: `"path\\to\\file"`, Line: 1},
			},
		},
		{
			name:  "double-quoted multiline value",
			input: "FOO=\"line1\nline2\nline3\"",
			want: []Entry{
				{Key: "FOO", Value: "line1\nline2\nline3", Raw: "\"line1\nline2\nline3\"", Line: 1},
			},
		},
		{
			name:  "backtick-quoted value",
			input: "FOO=`bar baz`",
			want: []Entry{
				{Key: "FOO", Value: "bar baz", Raw: "`bar baz`", Line: 1},
			},
		},
		{
			name:  "backtick-quoted value preserves escapes literally",
			input: "FOO=`hello\\nworld`",
			want: []Entry{
				{Key: "FOO", Value: `hello\nworld`, Raw: "`hello\\nworld`", Line: 1},
			},
		},
		{
			name:  "backtick-quoted multiline value",
			input: "FOO=`line1\nline2`",
			want: []Entry{
				{Key: "FOO", Value: "line1\nline2", Raw: "`line1\nline2`", Line: 1},
			},
		},
		{
			name:    "unterminated single quote",
			input:   "FOO='bar",
			wantErr: true,
		},
		{
			name:    "unterminated double quote",
			input:   "FOO=\"bar",
			wantErr: true,
		},
		{
			name:    "unterminated backtick quote",
			input:   "FOO=`bar",
			wantErr: true,
		},
		{
			name:  "line without equals sign is skipped",
			input: "JUSTAKEYNOVALUE\nFOO=bar",
			want: []Entry{
				{Key: "FOO", Value: "bar", Raw: "bar", Line: 2},
			},
		},
		{
			name:  "value with equals sign",
			input: "DATABASE_URL=postgres://user:pass@host:5432/db?sslmode=require",
			want: []Entry{
				{Key: "DATABASE_URL", Value: "postgres://user:pass@host:5432/db?sslmode=require", Raw: "postgres://user:pass@host:5432/db?sslmode=require", Line: 1},
			},
		},
		{
			name:  "ref:// value",
			input: "API_KEY=ref://secrets/api_key",
			want: []Entry{
				{Key: "API_KEY", Value: "ref://secrets/api_key", Raw: "ref://secrets/api_key", Line: 1, IsRef: true},
			},
		},
		{
			name:  "mixed entries",
			input: "# Database config\nDB_HOST=localhost\nDB_PORT=5432\nDB_PASS=ref://secrets/db_pass\n\n# App config\nAPP_NAME='My App'\nDEBUG=true",
			want: []Entry{
				{Key: "DB_HOST", Value: "localhost", Raw: "localhost", Line: 2},
				{Key: "DB_PORT", Value: "5432", Raw: "5432", Line: 3},
				{Key: "DB_PASS", Value: "ref://secrets/db_pass", Raw: "ref://secrets/db_pass", Line: 4, IsRef: true},
				{Key: "APP_NAME", Value: "My App", Raw: "'My App'", Line: 7},
				{Key: "DEBUG", Value: "true", Raw: "true", Line: 8},
			},
		},
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name:  "only comments and blank lines",
			input: "# comment\n\n# another comment\n",
			want:  nil,
		},
		{
			name:  "value with spaces in double quotes",
			input: `FOO="hello world"`,
			want: []Entry{
				{Key: "FOO", Value: "hello world", Raw: `"hello world"`, Line: 1},
			},
		},
		{
			name:  "double-quoted empty value",
			input: `FOO=""`,
			want: []Entry{
				{Key: "FOO", Value: "", Raw: `""`, Line: 1},
			},
		},
		{
			name:  "single-quoted empty value",
			input: `FOO=''`,
			want: []Entry{
				{Key: "FOO", Value: "", Raw: `''`, Line: 1},
			},
		},
		{
			name:  "double-quoted value with carriage return escape",
			input: `FOO="line1\r\nline2"`,
			want: []Entry{
				{Key: "FOO", Value: "line1\r\nline2", Raw: `"line1\r\nline2"`, Line: 1},
			},
		},
		{
			name:  "double-quoted unknown escape kept literally",
			input: `FOO="hello\xworld"`,
			want: []Entry{
				{Key: "FOO", Value: `hello\xworld`, Raw: `"hello\xworld"`, Line: 1},
			},
		},
		{
			name:  "key with underscores and numbers",
			input: "MY_VAR_123=value",
			want: []Entry{
				{Key: "MY_VAR_123", Value: "value", Raw: "value", Line: 1},
			},
		},
		{
			name:  "entry after multiline double-quoted value",
			input: "FOO=\"line1\nline2\"\nBAR=baz",
			want: []Entry{
				{Key: "FOO", Value: "line1\nline2", Raw: "\"line1\nline2\"", Line: 1},
				{Key: "BAR", Value: "baz", Raw: "baz", Line: 3},
			},
		},
		{
			name:  "comment with leading whitespace",
			input: "  # indented comment\nFOO=bar",
			want: []Entry{
				{Key: "FOO", Value: "bar", Raw: "bar", Line: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Parse(strings.NewReader(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("expected %d entries, got %d: %+v", len(tt.want), len(got), got)
			}

			for i, want := range tt.want {
				g := got[i]
				if g.Key != want.Key {
					t.Errorf("entry[%d] Key: got %q, want %q", i, g.Key, want.Key)
				}
				if g.Value != want.Value {
					t.Errorf("entry[%d] Value: got %q, want %q", i, g.Value, want.Value)
				}
				if g.Raw != want.Raw {
					t.Errorf("entry[%d] Raw: got %q, want %q", i, g.Raw, want.Raw)
				}
				if g.Line != want.Line {
					t.Errorf("entry[%d] Line: got %d, want %d", i, g.Line, want.Line)
				}
				if g.IsRef != want.IsRef {
					t.Errorf("entry[%d] IsRef: got %v, want %v", i, g.IsRef, want.IsRef)
				}
			}
		})
	}
}

func TestParseBOM(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Entry
	}{
		{
			name:  "strips UTF-8 BOM from first line",
			input: "\xEF\xBB\xBFFOO=bar",
			want: []Entry{
				{Key: "FOO", Value: "bar", Raw: "bar", Line: 1},
			},
		},
		{
			name:  "BOM with multiple entries",
			input: "\xEF\xBB\xBFFOO=bar\nBAZ=qux",
			want: []Entry{
				{Key: "FOO", Value: "bar", Raw: "bar", Line: 1},
				{Key: "BAZ", Value: "qux", Raw: "qux", Line: 2},
			},
		},
		{
			name:  "BOM before comment",
			input: "\xEF\xBB\xBF# comment\nFOO=bar",
			want: []Entry{
				{Key: "FOO", Value: "bar", Raw: "bar", Line: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d entries, got %d: %+v", len(tt.want), len(got), got)
			}
			for i, want := range tt.want {
				if got[i].Key != want.Key {
					t.Errorf("entry[%d] Key: got %q, want %q", i, got[i].Key, want.Key)
				}
				if got[i].Value != want.Value {
					t.Errorf("entry[%d] Value: got %q, want %q", i, got[i].Value, want.Value)
				}
				if got[i].Line != want.Line {
					t.Errorf("entry[%d] Line: got %d, want %d", i, got[i].Line, want.Line)
				}
			}
		})
	}
}

func TestParseCRLF(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Entry
	}{
		{
			name:  "CRLF line endings",
			input: "FOO=bar\r\nBAZ=qux\r\n",
			want: []Entry{
				{Key: "FOO", Value: "bar", Raw: "bar", Line: 1},
				{Key: "BAZ", Value: "qux", Raw: "qux", Line: 2},
			},
		},
		{
			name:  "mixed LF and CRLF",
			input: "FOO=bar\r\nBAZ=qux\n",
			want: []Entry{
				{Key: "FOO", Value: "bar", Raw: "bar", Line: 1},
				{Key: "BAZ", Value: "qux", Raw: "qux", Line: 2},
			},
		},
		{
			name:  "CRLF with quoted value",
			input: "FOO=\"hello world\"\r\nBAR=baz\r\n",
			want: []Entry{
				{Key: "FOO", Value: "hello world", Raw: `"hello world"`, Line: 1},
				{Key: "BAR", Value: "baz", Raw: "baz", Line: 2},
			},
		},
		{
			name:  "CRLF with trailing whitespace",
			input: "FOO=bar  \r\nBAZ=qux\r\n",
			want: []Entry{
				{Key: "FOO", Value: "bar", Raw: "bar", Line: 1},
				{Key: "BAZ", Value: "qux", Raw: "qux", Line: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d entries, got %d: %+v", len(tt.want), len(got), got)
			}
			for i, want := range tt.want {
				if got[i].Key != want.Key {
					t.Errorf("entry[%d] Key: got %q, want %q", i, got[i].Key, want.Key)
				}
				if got[i].Value != want.Value {
					t.Errorf("entry[%d] Value: got %q, want %q", i, got[i].Value, want.Value)
				}
			}
		})
	}
}

func TestParseDuplicateKeys(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantEntries  []Entry
		wantWarnings int
	}{
		{
			name:  "duplicate key last wins",
			input: "FOO=first\nFOO=second",
			wantEntries: []Entry{
				{Key: "FOO", Value: "first", Raw: "first", Line: 1},
				{Key: "FOO", Value: "second", Raw: "second", Line: 2},
			},
			wantWarnings: 1,
		},
		{
			name:  "triple duplicate",
			input: "FOO=first\nFOO=second\nFOO=third",
			wantEntries: []Entry{
				{Key: "FOO", Value: "first", Raw: "first", Line: 1},
				{Key: "FOO", Value: "second", Raw: "second", Line: 2},
				{Key: "FOO", Value: "third", Raw: "third", Line: 3},
			},
			wantWarnings: 2,
		},
		{
			name:  "no duplicates no warnings",
			input: "FOO=bar\nBAZ=qux",
			wantEntries: []Entry{
				{Key: "FOO", Value: "bar", Raw: "bar", Line: 1},
				{Key: "BAZ", Value: "qux", Raw: "qux", Line: 2},
			},
			wantWarnings: 0,
		},
		{
			name:  "duplicate with other keys between",
			input: "FOO=first\nBAR=middle\nFOO=second",
			wantEntries: []Entry{
				{Key: "FOO", Value: "first", Raw: "first", Line: 1},
				{Key: "BAR", Value: "middle", Raw: "middle", Line: 2},
				{Key: "FOO", Value: "second", Raw: "second", Line: 3},
			},
			wantWarnings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, warnings, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.wantEntries) {
				t.Fatalf("expected %d entries, got %d: %+v", len(tt.wantEntries), len(got), got)
			}
			for i, want := range tt.wantEntries {
				if got[i].Key != want.Key || got[i].Value != want.Value {
					t.Errorf("entry[%d]: got {%q, %q}, want {%q, %q}", i, got[i].Key, got[i].Value, want.Key, want.Value)
				}
			}
			if len(warnings) != tt.wantWarnings {
				t.Errorf("expected %d warnings, got %d: %v", tt.wantWarnings, len(warnings), warnings)
			}
		})
	}
}

func TestParseDuplicateWarningMessage(t *testing.T) {
	_, warnings, err := Parse(strings.NewReader("FOO=first\nFOO=second"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	w := warnings[0]
	if w.Line != 2 {
		t.Errorf("warning line: got %d, want 2", w.Line)
	}
	if !strings.Contains(w.Message, "duplicate key") {
		t.Errorf("warning message should contain 'duplicate key': got %q", w.Message)
	}
	if !strings.Contains(w.Message, "FOO") {
		t.Errorf("warning message should contain key name: got %q", w.Message)
	}
}

func TestParseBOMWithCRLF(t *testing.T) {
	input := "\xEF\xBB\xBFFOO=bar\r\nBAZ=qux\r\n"
	got, _, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d: %+v", len(got), got)
	}
	if got[0].Key != "FOO" || got[0].Value != "bar" {
		t.Errorf("entry[0]: got {%q, %q}, want {\"FOO\", \"bar\"}", got[0].Key, got[0].Value)
	}
	if got[1].Key != "BAZ" || got[1].Value != "qux" {
		t.Errorf("entry[1]: got {%q, %q}, want {\"BAZ\", \"qux\"}", got[1].Key, got[1].Value)
	}
}

func TestParseTrailingWhitespace(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValue string
	}{
		{
			name:      "trailing spaces on unquoted value",
			input:     "FOO=bar   ",
			wantValue: "bar",
		},
		{
			name:      "trailing tabs on unquoted value",
			input:     "FOO=bar\t\t",
			wantValue: "bar",
		},
		{
			name:      "trailing whitespace preserved in double quotes",
			input:     `FOO="bar   "`,
			wantValue: "bar   ",
		},
		{
			name:      "trailing whitespace preserved in single quotes",
			input:     `FOO='bar   '`,
			wantValue: "bar   ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(got))
			}
			if got[0].Value != tt.wantValue {
				t.Errorf("Value: got %q, want %q", got[0].Value, tt.wantValue)
			}
		})
	}
}

func TestParseError(t *testing.T) {
	_, _, err := Parse(strings.NewReader("FOO='unterminated"))
	if err == nil {
		t.Fatal("expected error for unterminated single quote")
	}

	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if pe.Line != 1 {
		t.Errorf("expected error on line 1, got line %d", pe.Line)
	}
}

func TestWarningString(t *testing.T) {
	w := Warning{Line: 5, Message: "duplicate key \"FOO\""}
	got := w.String()
	want := `line 5: duplicate key "FOO"`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestParseQuoteStyle verifies that the Quote field is set correctly for
// each quoting style.
func TestParseQuoteStyle(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantQuote QuoteStyle
	}{
		{
			name:      "unquoted value",
			input:     "FOO=bar",
			wantQuote: QuoteNone,
		},
		{
			name:      "single-quoted value",
			input:     "FOO='bar'",
			wantQuote: QuoteSingle,
		},
		{
			name:      "double-quoted value",
			input:     `FOO="bar"`,
			wantQuote: QuoteDouble,
		},
		{
			name:      "backtick-quoted value",
			input:     "FOO=`bar`",
			wantQuote: QuoteBacktick,
		},
		{
			name:      "empty unquoted value",
			input:     "FOO=",
			wantQuote: QuoteNone,
		},
		{
			name:      "empty double-quoted value",
			input:     `FOO=""`,
			wantQuote: QuoteDouble,
		},
		{
			name:      "empty single-quoted value",
			input:     "FOO=''",
			wantQuote: QuoteSingle,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(got))
			}
			if got[0].Quote != tt.wantQuote {
				t.Errorf("Quote: got %d, want %d", got[0].Quote, tt.wantQuote)
			}
		})
	}
}

// TestParseExportVariations tests different forms of the export prefix.
func TestParseExportVariations(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantKey   string
		wantValue string
	}{
		{
			name:      "export with single space",
			input:     "export FOO=bar",
			wantKey:   "FOO",
			wantValue: "bar",
		},
		{
			name:      "export with multiple spaces",
			input:     "export   FOO=bar",
			wantKey:   "FOO",
			wantValue: "bar",
		},
		{
			name:      "export with leading whitespace",
			input:     "  export FOO=bar",
			wantKey:   "FOO",
			wantValue: "bar",
		},
		{
			name:      "export with quoted value",
			input:     `export FOO="bar baz"`,
			wantKey:   "FOO",
			wantValue: "bar baz",
		},
		{
			name:      "export as key prefix not stripped",
			input:     "exportFOO=bar",
			wantKey:   "exportFOO",
			wantValue: "bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 entry, got %d: %+v", len(got), got)
			}
			if got[0].Key != tt.wantKey {
				t.Errorf("Key: got %q, want %q", got[0].Key, tt.wantKey)
			}
			if got[0].Value != tt.wantValue {
				t.Errorf("Value: got %q, want %q", got[0].Value, tt.wantValue)
			}
		})
	}
}

// TestParseSpecialKeyNames tests keys with dots, hyphens, and other
// characters that commonly appear in .env files.
func TestParseSpecialKeyNames(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantKey string
	}{
		{
			name:    "key with dots",
			input:   "spring.datasource.url=jdbc:mysql://localhost/db",
			wantKey: "spring.datasource.url",
		},
		{
			name:    "key with hyphens",
			input:   "my-var=value",
			wantKey: "my-var",
		},
		{
			name:    "key with mixed characters",
			input:   "MY_VAR.sub-key_123=value",
			wantKey: "MY_VAR.sub-key_123",
		},
		{
			name:    "single character key",
			input:   "A=1",
			wantKey: "A",
		},
		{
			name:    "lowercase key",
			input:   "lowercase=value",
			wantKey: "lowercase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(got))
			}
			if got[0].Key != tt.wantKey {
				t.Errorf("Key: got %q, want %q", got[0].Key, tt.wantKey)
			}
		})
	}
}

// TestParseInlineCommentEdgeCases covers various inline comment scenarios.
func TestParseInlineCommentEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValue string
	}{
		{
			name:      "tab before hash is not an inline comment",
			input:     "FOO=bar\t#baz",
			wantValue: "bar\t#baz",
		},
		{
			name:      "multiple hashes after space",
			input:     "FOO=bar ## comment",
			wantValue: "bar",
		},
		{
			name:      "hash at start of value",
			input:     "FOO=#notacomment",
			wantValue: "#notacomment",
		},
		{
			name:      "double-quoted value with hash",
			input:     `FOO="bar # not a comment"`,
			wantValue: "bar # not a comment",
		},
		{
			name:      "single-quoted value with hash",
			input:     `FOO='bar # not a comment'`,
			wantValue: "bar # not a comment",
		},
		{
			name:      "value is only a hash",
			input:     "FOO= #",
			wantValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 entry, got %d: %+v", len(got), got)
			}
			if got[0].Value != tt.wantValue {
				t.Errorf("Value: got %q, want %q", got[0].Value, tt.wantValue)
			}
		})
	}
}

// TestParseQuotedValuesWithOtherQuotes tests quotes nested inside other quote types.
func TestParseQuotedValuesWithOtherQuotes(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValue string
	}{
		{
			name:      "double quotes inside single quotes",
			input:     `FOO='say "hello"'`,
			wantValue: `say "hello"`,
		},
		{
			name:      "single quotes inside double quotes",
			input:     `FOO="it's a test"`,
			wantValue: `it's a test`,
		},
		{
			name:      "backticks inside double quotes",
			input:     "FOO=\"`command`\"",
			wantValue: "`command`",
		},
		{
			name:      "backticks inside single quotes",
			input:     "FOO='`command`'",
			wantValue: "`command`",
		},
		{
			name:      "double quotes inside backticks",
			input:     "FOO=`say \"hello\"`",
			wantValue: `say "hello"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(got))
			}
			if got[0].Value != tt.wantValue {
				t.Errorf("Value: got %q, want %q", got[0].Value, tt.wantValue)
			}
		})
	}
}

// TestParseRefInQuotedValues ensures ref:// detection works inside quoted values.
func TestParseRefInQuotedValues(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantRef bool
	}{
		{
			name:    "ref in unquoted value",
			input:   "KEY=ref://secrets/key",
			wantRef: true,
		},
		{
			name:    "ref in double-quoted value",
			input:   `KEY="ref://secrets/key"`,
			wantRef: true,
		},
		{
			name:    "ref in single-quoted value",
			input:   `KEY='ref://secrets/key'`,
			wantRef: true,
		},
		{
			name:    "ref in backtick-quoted value",
			input:   "KEY=`ref://secrets/key`",
			wantRef: true,
		},
		{
			name:    "partial ref prefix",
			input:   "KEY=ref:/not-a-ref",
			wantRef: false,
		},
		{
			name:    "ref in middle of value is not ref",
			input:   "KEY=some-ref://secrets/key",
			wantRef: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(got))
			}
			if got[0].IsRef != tt.wantRef {
				t.Errorf("IsRef: got %v, want %v", got[0].IsRef, tt.wantRef)
			}
		})
	}
}

// TestParseMultilineEdgeCases covers additional multiline scenarios.
func TestParseMultilineEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		wantFirst string
		wantLast  string
	}{
		{
			name:      "backtick multiline with entry after",
			input:     "FOO=`line1\nline2`\nBAR=baz",
			wantCount: 2,
			wantFirst: "line1\nline2",
			wantLast:  "baz",
		},
		{
			name:      "double-quoted multiline with entry after",
			input:     "FOO=\"line1\nline2\"\nBAR=baz",
			wantCount: 2,
			wantFirst: "line1\nline2",
			wantLast:  "baz",
		},
		{
			name:      "multiple multiline values",
			input:     "A=\"one\ntwo\"\nB=`three\nfour`\nC=five",
			wantCount: 3,
			wantFirst: "one\ntwo",
			wantLast:  "five",
		},
		{
			name:      "double-quoted with escaped newline in value",
			input:     "FOO=\"line1\\nline2\"",
			wantCount: 1,
			wantFirst: "line1\nline2",
			wantLast:  "line1\nline2",
		},
		{
			name:      "multiline double-quoted three lines",
			input:     "FOO=\"line1\nline2\nline3\"",
			wantCount: 1,
			wantFirst: "line1\nline2\nline3",
			wantLast:  "line1\nline2\nline3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.wantCount {
				t.Fatalf("expected %d entries, got %d: %+v", tt.wantCount, len(got), got)
			}
			if got[0].Value != tt.wantFirst {
				t.Errorf("first entry Value: got %q, want %q", got[0].Value, tt.wantFirst)
			}
			if got[len(got)-1].Value != tt.wantLast {
				t.Errorf("last entry Value: got %q, want %q", got[len(got)-1].Value, tt.wantLast)
			}
		})
	}
}

// TestParseDoubleQuoteEscapeEdgeCases tests edge cases in escape processing.
func TestParseDoubleQuoteEscapeEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValue string
	}{
		{
			name:      "consecutive backslashes",
			input:     `FOO="a\\\\b"`,
			wantValue: `a\\b`,
		},
		{
			name:      "escaped backslash before closing quote",
			input:     `FOO="path\\"`,
			wantValue: `path\`,
		},
		{
			name:      "all escape sequences",
			input:     `FOO="\n\r\t\\\""`,
			wantValue: "\n\r\t\\\"",
		},
		{
			name:      "escape at start of value",
			input:     `FOO="\nhello"`,
			wantValue: "\nhello",
		},
		{
			name:      "multiple unknown escapes",
			input:     `FOO="\a\b\c"`,
			wantValue: `\a\b\c`,
		},
		{
			name:      "escaped quote inside value",
			input:     `FOO="say \"hi\" please"`,
			wantValue: `say "hi" please`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 entry, got %d: %+v", len(got), got)
			}
			if got[0].Value != tt.wantValue {
				t.Errorf("Value: got %q, want %q", got[0].Value, tt.wantValue)
			}
		})
	}
}

// TestParseWhitespaceOnlyLines ensures that lines containing only whitespace
// are treated as blank and skipped.
func TestParseWhitespaceOnlyLines(t *testing.T) {
	input := "FOO=bar\n   \n\t\t\nBAR=baz"
	got, _, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d: %+v", len(got), got)
	}
	if got[0].Key != "FOO" || got[0].Value != "bar" {
		t.Errorf("entry[0]: got {%q, %q}, want {\"FOO\", \"bar\"}", got[0].Key, got[0].Value)
	}
	if got[1].Key != "BAR" || got[1].Value != "baz" {
		t.Errorf("entry[1]: got {%q, %q}, want {\"BAR\", \"baz\"}", got[1].Key, got[1].Value)
	}
}

// TestParseLineNumbers verifies correct line numbering including across multiline values.
func TestParseLineNumbers(t *testing.T) {
	input := "# header comment\nFOO=bar\n\nMULTI=\"line1\nline2\nline3\"\nAFTER=value"
	got, _, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d: %+v", len(got), got)
	}
	if got[0].Line != 2 {
		t.Errorf("FOO line: got %d, want 2", got[0].Line)
	}
	if got[1].Line != 4 {
		t.Errorf("MULTI line: got %d, want 4", got[1].Line)
	}
	// MULTI spans lines 4-6, so AFTER is on line 7.
	if got[2].Line != 7 {
		t.Errorf("AFTER line: got %d, want 7", got[2].Line)
	}
}

// TestParseLargeInput ensures the parser handles a large number of entries.
func TestParseLargeInput(t *testing.T) {
	var b strings.Builder
	const count = 500
	for i := 0; i < count; i++ {
		fmt.Fprintf(&b, "KEY_%d=value_%d\n", i, i)
	}
	got, _, err := Parse(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != count {
		t.Fatalf("expected %d entries, got %d", count, len(got))
	}
	// Spot-check first and last entries.
	if got[0].Key != "KEY_0" || got[0].Value != "value_0" {
		t.Errorf("first entry: got {%q, %q}", got[0].Key, got[0].Value)
	}
	if got[count-1].Key != "KEY_499" || got[count-1].Value != "value_499" {
		t.Errorf("last entry: got {%q, %q}", got[count-1].Key, got[count-1].Value)
	}
}

// TestParseCRLFInsideDoubleQuotedMultiline ensures that CRLF inside a
// double-quoted multiline value is normalized.
func TestParseCRLFInsideDoubleQuotedMultiline(t *testing.T) {
	// The scanner splits on \n; each scanned line has \r stripped.
	// So a double-quoted multiline with CRLF should produce \n joins.
	input := "FOO=\"line1\r\nline2\r\nline3\""
	got, _, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	// The CRLF normalization strips \r from each scanned line, and the
	// multiline join uses \n, so the result should be clean \n-separated.
	want := "line1\nline2\nline3"
	if got[0].Value != want {
		t.Errorf("Value: got %q, want %q", got[0].Value, want)
	}
}

// TestParseEmptyKeySkipped ensures a line like "=value" is skipped.
func TestParseEmptyKeySkipped(t *testing.T) {
	input := "=value\nFOO=bar"
	got, _, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d: %+v", len(got), got)
	}
	if got[0].Key != "FOO" {
		t.Errorf("Key: got %q, want %q", got[0].Key, "FOO")
	}
}

// TestParseValuesWithEqualsSign tests that values containing = signs are
// correctly handled (only the first = is treated as the separator).
func TestParseValuesWithEqualsSign(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValue string
	}{
		{
			name:      "base64 value with padding",
			input:     "TOKEN=dGVzdA==",
			wantValue: "dGVzdA==",
		},
		{
			name:      "URL with query params",
			input:     "URL=https://example.com?foo=1&bar=2",
			wantValue: "https://example.com?foo=1&bar=2",
		},
		{
			name:      "value is just equals signs",
			input:     "SEP====",
			wantValue: "===",
		},
		{
			name:      "quoted value with equals",
			input:     `TOKEN="abc=def"`,
			wantValue: "abc=def",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(got))
			}
			if got[0].Value != tt.wantValue {
				t.Errorf("Value: got %q, want %q", got[0].Value, tt.wantValue)
			}
		})
	}
}

// TestParseErrorLineContext verifies that parse errors report the correct line.
func TestParseErrorLineContext(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantLine int
	}{
		{
			name:     "unterminated single quote on line 1",
			input:    "FOO='unterminated",
			wantLine: 1,
		},
		{
			name:     "unterminated double quote on line 3",
			input:    "A=1\nB=2\nC=\"unterminated",
			wantLine: 3,
		},
		{
			name:     "unterminated backtick on line 2",
			input:    "A=1\nB=`unterminated",
			wantLine: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Parse(strings.NewReader(tt.input))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			pe, ok := err.(*ParseError)
			if !ok {
				t.Fatalf("expected *ParseError, got %T: %v", err, err)
			}
			if pe.Line != tt.wantLine {
				t.Errorf("error line: got %d, want %d", pe.Line, tt.wantLine)
			}
		})
	}
}

// TestParseUnicodeValues verifies that unicode characters in keys and values
// are handled correctly.
func TestParseUnicodeValues(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantKey   string
		wantValue string
	}{
		{
			name:      "unicode in unquoted value",
			input:     "GREETING=héllo wörld",
			wantKey:   "GREETING",
			wantValue: "héllo wörld",
		},
		{
			name:      "emoji in double-quoted value",
			input:     `EMOJI="🚀 launch"`,
			wantKey:   "EMOJI",
			wantValue: "🚀 launch",
		},
		{
			name:      "CJK characters in single-quoted value",
			input:     "MSG='日本語テスト'",
			wantKey:   "MSG",
			wantValue: "日本語テスト",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(got))
			}
			if got[0].Key != tt.wantKey {
				t.Errorf("Key: got %q, want %q", got[0].Key, tt.wantKey)
			}
			if got[0].Value != tt.wantValue {
				t.Errorf("Value: got %q, want %q", got[0].Value, tt.wantValue)
			}
		})
	}
}

// TestParseMixedRealWorld tests a realistic .env file with a variety of entry
// types, comments, blank lines, refs, and multiline values.
func TestParseMixedRealWorld(t *testing.T) {
	input := `# Application configuration
APP_NAME=my-service
APP_ENV=development
APP_PORT=3000

# Database
DATABASE_URL="postgres://user:pass@localhost:5432/mydb?sslmode=disable"
DB_POOL_SIZE=10

# Secrets (stored in OS keychain)
API_KEY=ref://secrets/api_key
JWT_SECRET=ref://keychain/jwt_secret

# Feature flags
ENABLE_CACHE=true
DEBUG_MODE=false

# Multi-line certificate
TLS_CERT="-----BEGIN CERTIFICATE-----
MIIBxTCCAWugAwIBAgIJAJfkXl8y
-----END CERTIFICATE-----"

export NODE_ENV=production
`

	got, _, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []struct {
		key   string
		value string
		isRef bool
	}{
		{"APP_NAME", "my-service", false},
		{"APP_ENV", "development", false},
		{"APP_PORT", "3000", false},
		{"DATABASE_URL", "postgres://user:pass@localhost:5432/mydb?sslmode=disable", false},
		{"DB_POOL_SIZE", "10", false},
		{"API_KEY", "ref://secrets/api_key", true},
		{"JWT_SECRET", "ref://keychain/jwt_secret", true},
		{"ENABLE_CACHE", "true", false},
		{"DEBUG_MODE", "false", false},
		{"TLS_CERT", "-----BEGIN CERTIFICATE-----\nMIIBxTCCAWugAwIBAgIJAJfkXl8y\n-----END CERTIFICATE-----", false},
		{"NODE_ENV", "production", false},
	}

	if len(got) != len(expected) {
		t.Fatalf("expected %d entries, got %d", len(expected), len(got))
	}

	for i, exp := range expected {
		if got[i].Key != exp.key {
			t.Errorf("entry[%d] Key: got %q, want %q", i, got[i].Key, exp.key)
		}
		if got[i].Value != exp.value {
			t.Errorf("entry[%d] Value: got %q, want %q", i, got[i].Value, exp.value)
		}
		if got[i].IsRef != exp.isRef {
			t.Errorf("entry[%d] IsRef: got %v, want %v", i, got[i].IsRef, exp.isRef)
		}
	}
}
