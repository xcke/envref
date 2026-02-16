package envfile

import (
	"testing"

	"github.com/xcke/envref/internal/parser"
)

func TestInterpolate(t *testing.T) {
	tests := []struct {
		name    string
		entries []parser.Entry
		want    map[string]string
	}{
		{
			name: "simple ${VAR} reference",
			entries: []parser.Entry{
				{Key: "HOST", Value: "localhost", Quote: parser.QuoteNone},
				{Key: "URL", Value: "http://${HOST}/api", Quote: parser.QuoteNone},
			},
			want: map[string]string{
				"HOST": "localhost",
				"URL":  "http://localhost/api",
			},
		},
		{
			name: "simple $VAR reference",
			entries: []parser.Entry{
				{Key: "HOST", Value: "localhost", Quote: parser.QuoteNone},
				{Key: "URL", Value: "http://$HOST/api", Quote: parser.QuoteNone},
			},
			want: map[string]string{
				"HOST": "localhost",
				"URL":  "http://localhost/api",
			},
		},
		{
			name: "multiple references in one value",
			entries: []parser.Entry{
				{Key: "DB_HOST", Value: "localhost", Quote: parser.QuoteNone},
				{Key: "DB_PORT", Value: "5432", Quote: parser.QuoteNone},
				{Key: "DB_URL", Value: "postgres://${DB_HOST}:${DB_PORT}/app", Quote: parser.QuoteNone},
			},
			want: map[string]string{
				"DB_HOST": "localhost",
				"DB_PORT": "5432",
				"DB_URL":  "postgres://localhost:5432/app",
			},
		},
		{
			name: "chained references",
			entries: []parser.Entry{
				{Key: "A", Value: "hello", Quote: parser.QuoteNone},
				{Key: "B", Value: "${A} world", Quote: parser.QuoteNone},
				{Key: "C", Value: "${B}!", Quote: parser.QuoteNone},
			},
			want: map[string]string{
				"A": "hello",
				"B": "hello world",
				"C": "hello world!",
			},
		},
		{
			name: "undefined variable expands to empty string",
			entries: []parser.Entry{
				{Key: "URL", Value: "http://${UNDEFINED_HOST}/api", Quote: parser.QuoteNone},
			},
			want: map[string]string{
				"URL": "http:///api",
			},
		},
		{
			name: "single-quoted values are not interpolated",
			entries: []parser.Entry{
				{Key: "HOST", Value: "localhost", Quote: parser.QuoteNone},
				{Key: "LITERAL", Value: "${HOST} is literal", Quote: parser.QuoteSingle},
			},
			want: map[string]string{
				"HOST":    "localhost",
				"LITERAL": "${HOST} is literal",
			},
		},
		{
			name: "backtick-quoted values are not interpolated",
			entries: []parser.Entry{
				{Key: "HOST", Value: "localhost", Quote: parser.QuoteNone},
				{Key: "LITERAL", Value: "${HOST} is literal", Quote: parser.QuoteBacktick},
			},
			want: map[string]string{
				"HOST":    "localhost",
				"LITERAL": "${HOST} is literal",
			},
		},
		{
			name: "double-quoted values are interpolated",
			entries: []parser.Entry{
				{Key: "HOST", Value: "localhost", Quote: parser.QuoteNone},
				{Key: "URL", Value: "http://${HOST}/api", Quote: parser.QuoteDouble},
			},
			want: map[string]string{
				"HOST": "localhost",
				"URL":  "http://localhost/api",
			},
		},
		{
			name: "escaped dollar sign",
			entries: []parser.Entry{
				{Key: "PRICE", Value: "$$5.00", Quote: parser.QuoteNone},
			},
			want: map[string]string{
				"PRICE": "$5.00",
			},
		},
		{
			name: "no interpolation needed",
			entries: []parser.Entry{
				{Key: "SIMPLE", Value: "just a value", Quote: parser.QuoteNone},
			},
			want: map[string]string{
				"SIMPLE": "just a value",
			},
		},
		{
			name: "trailing dollar sign kept literal",
			entries: []parser.Entry{
				{Key: "VAL", Value: "end$", Quote: parser.QuoteNone},
			},
			want: map[string]string{
				"VAL": "end$",
			},
		},
		{
			name: "unterminated brace kept literal",
			entries: []parser.Entry{
				{Key: "VAL", Value: "${UNCLOSED", Quote: parser.QuoteNone},
			},
			want: map[string]string{
				"VAL": "${UNCLOSED",
			},
		},
		{
			name: "dollar followed by non-identifier kept literal",
			entries: []parser.Entry{
				{Key: "VAL", Value: "cost is $5", Quote: parser.QuoteNone},
			},
			want: map[string]string{
				"VAL": "cost is $5",
			},
		},
		{
			name: "self-reference does not expand (not yet defined at point of use)",
			entries: []parser.Entry{
				{Key: "FOO", Value: "${FOO}bar", Quote: parser.QuoteNone},
			},
			want: map[string]string{
				"FOO": "bar",
			},
		},
		{
			name: "ref:// values are not touched by interpolation",
			entries: []parser.Entry{
				{Key: "SECRET", Value: "ref://secrets/key", IsRef: true, Quote: parser.QuoteNone},
			},
			want: map[string]string{
				"SECRET": "ref://secrets/key",
			},
		},
		{
			name: "${ref://...} is preserved literally for nested resolution",
			entries: []parser.Entry{
				{Key: "URL", Value: "postgres://${ref://secrets/db_user}:${ref://secrets/db_pass}@host/db", Quote: parser.QuoteNone},
			},
			want: map[string]string{
				"URL": "postgres://ref://secrets/db_user:ref://secrets/db_pass@host/db",
			},
		},
		{
			name: "${ref://...} mixed with regular vars",
			entries: []parser.Entry{
				{Key: "HOST", Value: "localhost", Quote: parser.QuoteNone},
				{Key: "URL", Value: "postgres://${ref://secrets/db_user}@${HOST}/db", Quote: parser.QuoteNone},
			},
			want: map[string]string{
				"HOST": "localhost",
				"URL":  "postgres://ref://secrets/db_user@localhost/db",
			},
		},
		{
			name: "indirect ref via variable expansion",
			entries: []parser.Entry{
				{Key: "DB_USER", Value: "ref://secrets/db_user", IsRef: true, Quote: parser.QuoteNone},
				{Key: "URL", Value: "postgres://${DB_USER}@host/db", Quote: parser.QuoteNone},
			},
			want: map[string]string{
				"DB_USER": "ref://secrets/db_user",
				"URL":     "postgres://ref://secrets/db_user@host/db",
			},
		},
		{
			name: "mixed $VAR and ${VAR} in one value",
			entries: []parser.Entry{
				{Key: "USER", Value: "admin", Quote: parser.QuoteNone},
				{Key: "HOST", Value: "db.example.com", Quote: parser.QuoteNone},
				{Key: "DSN", Value: "$USER@${HOST}", Quote: parser.QuoteNone},
			},
			want: map[string]string{
				"USER": "admin",
				"HOST": "db.example.com",
				"DSN":  "admin@db.example.com",
			},
		},
		{
			name: "variable with underscores and numbers",
			entries: []parser.Entry{
				{Key: "MY_VAR_2", Value: "value", Quote: parser.QuoteNone},
				{Key: "REF", Value: "${MY_VAR_2}", Quote: parser.QuoteNone},
			},
			want: map[string]string{
				"MY_VAR_2": "value",
				"REF":      "value",
			},
		},
		{
			name: "empty value with reference",
			entries: []parser.Entry{
				{Key: "EMPTY", Value: "", Quote: parser.QuoteNone},
				{Key: "REF", Value: "[${EMPTY}]", Quote: parser.QuoteNone},
			},
			want: map[string]string{
				"EMPTY": "",
				"REF":   "[]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewEnv()
			for _, e := range tt.entries {
				env.Set(e)
			}

			Interpolate(env)

			for key, wantVal := range tt.want {
				entry, ok := env.Get(key)
				if !ok {
					t.Errorf("key %q not found after interpolation", key)
					continue
				}
				if entry.Value != wantVal {
					t.Errorf("%s: got %q, want %q", key, entry.Value, wantVal)
				}
			}
		})
	}
}

func TestExpandVars(t *testing.T) {
	lookup := map[string]string{
		"HOST": "localhost",
		"PORT": "8080",
		"NAME": "app",
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no vars", "hello world", "hello world"},
		{"braced var", "${HOST}", "localhost"},
		{"bare var", "$HOST", "localhost"},
		{"braced in string", "http://${HOST}:${PORT}", "http://localhost:8080"},
		{"bare in string", "http://$HOST:$PORT", "http://localhost:8080"},
		{"undefined var", "${MISSING}", ""},
		{"escaped dollar", "$$HOST", "$HOST"},
		{"trailing dollar", "end$", "end$"},
		{"dollar digit", "$5", "$5"},
		{"unterminated brace", "${OPEN", "${OPEN"},
		{"empty braces", "${}", ""},
		{"consecutive vars", "${HOST}${PORT}", "localhost8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandVars(tt.input, lookup)
			if got != tt.want {
				t.Errorf("expandVars(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestInterpolateWithFileLoad(t *testing.T) {
	dir := t.TempDir()

	content := `DB_HOST=localhost
DB_PORT=5432
DB_NAME=myapp
DB_URL=postgres://${DB_HOST}:${DB_PORT}/${DB_NAME}
LITERAL='${DB_HOST} not expanded'
`
	path := writeFile(t, dir, ".env", content)

	env, _, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	Interpolate(env)

	entry, _ := env.Get("DB_URL")
	want := "postgres://localhost:5432/myapp"
	if entry.Value != want {
		t.Errorf("DB_URL: got %q, want %q", entry.Value, want)
	}

	entry, _ = env.Get("LITERAL")
	wantLiteral := "${DB_HOST} not expanded"
	if entry.Value != wantLiteral {
		t.Errorf("LITERAL: got %q, want %q", entry.Value, wantLiteral)
	}
}
