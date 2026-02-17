package envfile

import (
	"os"
	"testing"

	"github.com/xcke/envref/internal/parser"
)

// FuzzInterpolate exercises the variable interpolation engine with arbitrary
// key-value pairs to catch panics, infinite loops, or unexpected crashes.
func FuzzInterpolate(f *testing.F) {
	// Seed with representative interpolation patterns.
	// Each seed is a pair: (key, value).
	type seed struct {
		key   string
		value string
	}
	seeds := []seed{
		{"FOO", "bar"},
		{"URL", "https://${HOST}:${PORT}/path"},
		{"HOST", "localhost"},
		{"PORT", "5432"},
		{"REF", "$FOO"},
		{"ESCAPED", "$$literal"},
		{"UNTERM", "${unclosed"},
		{"EMPTY", ""},
		{"DOLLAR", "$"},
		{"TRAIL", "value$"},
		{"NESTED", "${${FOO}}"},
		{"BRACES", "${}"},
		{"SPECIAL", "$0invalid"},
		{"VALID", "$_under"},
		{"MULTI", "$A$B$C"},
		{"COMBO", "pre${X}mid${Y}post"},
	}

	for _, s := range seeds {
		f.Add(s.key, s.value)
	}

	f.Fuzz(func(t *testing.T, key, value string) {
		// Skip empty keys since Env.Set requires non-empty key for Entry.
		if key == "" {
			return
		}

		env := NewEnv()
		env.Set(parser.Entry{
			Key:   "BASE",
			Value: "base_value",
			Line:  1,
			Quote: parser.QuoteNone,
		})
		env.Set(parser.Entry{
			Key:   key,
			Value: value,
			Line:  2,
			Quote: parser.QuoteNone,
		})

		// Interpolate should never panic.
		Interpolate(env)

		// After interpolation, all entries should still be present.
		if env.Len() < 1 {
			t.Error("Env is empty after interpolation")
		}

		// The key should still exist.
		if _, ok := env.Get(key); !ok {
			t.Errorf("key %q lost after interpolation", key)
		}
	})
}

// FuzzExpandVars exercises the expandVars function directly with arbitrary
// input strings and variable values.
func FuzzExpandVars(f *testing.F) {
	seeds := []struct {
		input    string
		varName  string
		varValue string
	}{
		{"${FOO}", "FOO", "bar"},
		{"$FOO", "FOO", "bar"},
		{"$$", "FOO", "bar"},
		{"${}", "FOO", "bar"},
		{"${FOO", "FOO", "bar"},
		{"no vars", "FOO", "bar"},
		{"$", "FOO", "bar"},
		{"", "FOO", "bar"},
		{"${A}${B}", "A", "1"},
		{"$A$B", "A", "1"},
		{"pre$Apost", "A", "1"},
		{"$0", "FOO", "bar"},
		{"$_VAR", "_VAR", "val"},
	}

	for _, s := range seeds {
		f.Add(s.input, s.varName, s.varValue)
	}

	f.Fuzz(func(t *testing.T, input, varName, varValue string) {
		lookup := map[string]string{}
		if varName != "" {
			lookup[varName] = varValue
		}

		// Should never panic.
		_ = expandVars(input, lookup)
	})
}

// FuzzLoadParse exercises the full Load path with arbitrary .env file content.
func FuzzLoadParse(f *testing.F) {
	seeds := []string{
		"FOO=bar\nBAZ=qux\n",
		"# comment\nKEY=value\n",
		"MULTI=\"line1\nline2\"\n",
		"SINGLE='literal'\n",
		"REF=ref://secrets/key\n",
		"INTERP=${FOO}\nFOO=bar\n",
		"export VAR=val\n",
		"\xEF\xBB\xBFBOM=yes\n",
		"DUP=first\nDUP=second\n",
		"EMPTY=\n",
		"URL=https://host:8080/path?q=1&a=2\n",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data string) {
		dir := t.TempDir()
		path := dir + "/.env"
		if err := writeTestFile(path, data); err != nil {
			t.Skip("could not write temp file")
		}

		// Load should never panic.
		env, _, err := Load(path)
		if err != nil {
			return // parse errors are fine
		}

		// Basic invariants on successful load.
		keys := env.Keys()
		for _, k := range keys {
			entry, ok := env.Get(k)
			if !ok {
				t.Errorf("key %q in Keys() but not found via Get()", k)
			}
			if entry.Key != k {
				t.Errorf("entry.Key = %q, expected %q", entry.Key, k)
			}
		}

		// All() should return same count as Keys().
		if len(env.All()) != len(keys) {
			t.Errorf("All() returned %d entries, Keys() returned %d", len(env.All()), len(keys))
		}

		// Interpolate should not panic on loaded env.
		Interpolate(env)
	})
}

// writeTestFile writes content to a file path.
func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)}
