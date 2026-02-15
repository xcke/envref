// Package envfile — variable interpolation support.
package envfile

import (
	"strings"

	"github.com/xcke/envref/internal/parser"
)

// Interpolate expands ${VAR} and $VAR references within env values.
// Variables are resolved against the Env itself (earlier definitions are
// available to later ones, order-dependent). Undefined variables expand to
// an empty string.
//
// Single-quoted and backtick-quoted values are treated as literals and are
// not interpolated (consistent with shell behavior). Double-quoted and
// unquoted values are interpolated.
//
// The Env is modified in place. A new Env is not created.
func Interpolate(env *Env) {
	// Build a lookup map that grows as we process entries in order.
	// This means later entries can reference earlier ones.
	resolved := make(map[string]string, env.Len())

	for _, key := range env.order {
		entry := env.entries[key]

		// Single-quoted and backtick-quoted values are literal — skip.
		if entry.Quote == parser.QuoteSingle || entry.Quote == parser.QuoteBacktick {
			resolved[key] = entry.Value
			continue
		}

		// Expand variable references in the value.
		expanded := expandVars(entry.Value, resolved)
		if expanded != entry.Value {
			entry.Value = expanded
			env.entries[key] = entry
		}

		resolved[key] = entry.Value
	}
}

// expandVars replaces ${VAR} and $VAR patterns in s using values from the
// lookup map. Undefined variables expand to empty string. A literal $ can
// be written as $$ (which produces a single $). The ${VAR} form is preferred
// as it avoids ambiguity.
func expandVars(s string, lookup map[string]string) string {
	// Fast path: no $ in string means nothing to expand.
	if !strings.Contains(s, "$") {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))
	i := 0

	for i < len(s) {
		if s[i] != '$' {
			b.WriteByte(s[i])
			i++
			continue
		}

		// Found $. Check what follows.
		if i+1 >= len(s) {
			// Trailing $ — keep it literal.
			b.WriteByte('$')
			i++
			continue
		}

		next := s[i+1]

		// $$ → literal $
		if next == '$' {
			b.WriteByte('$')
			i += 2
			continue
		}

		// ${VAR} form
		if next == '{' {
			closeIdx := strings.IndexByte(s[i+2:], '}')
			if closeIdx < 0 {
				// Unterminated ${ — keep literal.
				b.WriteByte('$')
				i++
				continue
			}
			varName := s[i+2 : i+2+closeIdx]
			if val, ok := lookup[varName]; ok {
				b.WriteString(val)
			}
			// Undefined vars expand to empty string.
			i = i + 2 + closeIdx + 1
			continue
		}

		// $VAR form — collect valid identifier characters.
		if isVarStart(next) {
			j := i + 2
			for j < len(s) && isVarCont(s[j]) {
				j++
			}
			varName := s[i+1 : j]
			if val, ok := lookup[varName]; ok {
				b.WriteString(val)
			}
			i = j
			continue
		}

		// $ followed by something that's not a var reference — keep literal.
		b.WriteByte('$')
		i++
	}

	return b.String()
}

// isVarStart reports whether c is a valid first character for a variable name.
func isVarStart(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_'
}

// isVarCont reports whether c is a valid continuation character for a variable name.
func isVarCont(c byte) bool {
	return isVarStart(c) || (c >= '0' && c <= '9')
}
