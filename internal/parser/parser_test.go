package parser

import (
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
			got, err := Parse(strings.NewReader(tt.input))
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

func TestParseError(t *testing.T) {
	_, err := Parse(strings.NewReader("FOO='unterminated"))
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
