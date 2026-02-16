// Package envfile provides functions for loading .env files from disk
// and merging multiple env file layers with override semantics.
package envfile

import (
	"fmt"
	"os"
	"strings"

	"github.com/xcke/envref/internal/parser"
	"github.com/xcke/envref/internal/ref"
)

// Env represents a set of environment variables loaded from one or more files.
// Keys are stored in the order they were first encountered.
type Env struct {
	// entries maps keys to their parsed entries.
	entries map[string]parser.Entry
	// order preserves the insertion order of keys.
	order []string
	// refCount tracks the number of ref:// entries for O(1) HasRefs.
	refCount int
}

// NewEnv creates an empty Env.
func NewEnv() *Env {
	return &Env{
		entries: make(map[string]parser.Entry),
	}
}

// newEnvSized creates an Env pre-sized for the expected number of entries.
func newEnvSized(n int) *Env {
	return &Env{
		entries: make(map[string]parser.Entry, n),
		order:   make([]string, 0, n),
	}
}

// Set adds or replaces an entry. If the key already exists, it is updated
// in place (preserving order). New keys are appended.
func (e *Env) Set(entry parser.Entry) {
	if existing, exists := e.entries[entry.Key]; exists {
		// Update ref count if IsRef status changed.
		if existing.IsRef && !entry.IsRef {
			e.refCount--
		} else if !existing.IsRef && entry.IsRef {
			e.refCount++
		}
	} else {
		e.order = append(e.order, entry.Key)
		if entry.IsRef {
			e.refCount++
		}
	}
	e.entries[entry.Key] = entry
}

// Get returns the entry for the given key and whether it was found.
func (e *Env) Get(key string) (parser.Entry, bool) {
	entry, ok := e.entries[key]
	return entry, ok
}

// Keys returns all keys in insertion order.
func (e *Env) Keys() []string {
	result := make([]string, len(e.order))
	copy(result, e.order)
	return result
}

// Len returns the number of entries.
func (e *Env) Len() int {
	return len(e.entries)
}

// All returns all entries in insertion order.
func (e *Env) All() []parser.Entry {
	result := make([]parser.Entry, 0, len(e.order))
	for _, key := range e.order {
		result = append(result, e.entries[key])
	}
	return result
}

// Refs returns all entries whose values are ref:// references, in insertion order.
func (e *Env) Refs() []parser.Entry {
	var refs []parser.Entry
	for _, key := range e.order {
		entry := e.entries[key]
		if entry.IsRef {
			refs = append(refs, entry)
		}
	}
	return refs
}

// ResolvedRefs returns a map of parsed ref.Reference objects keyed by env key,
// for all entries that have valid ref:// values. Entries with malformed ref://
// URIs are skipped (use Refs() and parse individually to handle errors).
func (e *Env) ResolvedRefs() map[string]ref.Reference {
	result := make(map[string]ref.Reference)
	for _, key := range e.order {
		entry := e.entries[key]
		if !entry.IsRef {
			continue
		}
		parsed, err := ref.Parse(entry.Value)
		if err != nil {
			continue
		}
		result[key] = parsed
	}
	return result
}

// HasRefs reports whether the Env contains any ref:// references.
func (e *Env) HasRefs() bool {
	return e.refCount > 0
}

// HasAnyRefs reports whether the Env contains any ref:// references,
// including embedded ref:// URIs within non-ref values (nested references).
func (e *Env) HasAnyRefs() bool {
	if e.refCount > 0 {
		return true
	}
	for _, key := range e.order {
		if ref.ContainsRef(e.entries[key].Value) {
			return true
		}
	}
	return false
}

// Load reads a .env file from disk and returns an Env with all entries.
// Returns an error if the file cannot be opened or parsed.
// Parse warnings (e.g., duplicate keys) are returned as the second value.
func Load(path string) (*Env, []parser.Warning, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("opening %s: %w", path, err)
	}

	entries, warnings, parseErr := parser.Parse(f)
	closeErr := f.Close()
	if parseErr != nil {
		return nil, warnings, fmt.Errorf("parsing %s: %w", path, parseErr)
	}
	if closeErr != nil {
		return nil, warnings, fmt.Errorf("closing %s: %w", path, closeErr)
	}

	env := newEnvSized(len(entries))
	for _, entry := range entries {
		env.Set(entry)
	}
	return env, warnings, nil
}

// LoadOptional reads a .env file from disk, returning an empty Env if the
// file does not exist. Other errors (permission denied, parse errors) are
// still returned. Parse warnings are returned as the second value.
func LoadOptional(path string) (*Env, []parser.Warning, error) {
	env, warnings, err := Load(path)
	if err != nil && os.IsNotExist(unwrapPathError(err)) {
		return NewEnv(), nil, nil
	}
	return env, warnings, err
}

// Merge combines a base Env with one or more overlay Envs. Overlays are
// applied in order — later overlays win on key conflicts. The base Env
// is not modified; a new Env is returned.
func Merge(base *Env, overlays ...*Env) *Env {
	// Estimate capacity: base entries plus overlay entries (some may overlap).
	capacity := base.Len()
	for _, overlay := range overlays {
		capacity += overlay.Len()
	}
	result := newEnvSized(capacity)

	// Copy base entries.
	for _, key := range base.order {
		result.Set(base.entries[key])
	}

	// Apply overlays in order.
	for _, overlay := range overlays {
		for _, key := range overlay.order {
			result.Set(overlay.entries[key])
		}
	}

	return result
}

// Write serializes the Env to a .env formatted file at the given path.
// Entries are written in insertion order, one per line, as KEY=VALUE.
// Values that contain spaces, quotes, or newlines are double-quoted with
// appropriate escaping.
func (e *Env) Write(path string) error {
	var b strings.Builder
	for _, key := range e.order {
		entry := e.entries[key]
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(formatValue(entry.Value))
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// formatValue returns the value formatted for a .env file.
// Simple values are returned as-is. Values containing spaces, newlines,
// double quotes, or hash characters are wrapped in double quotes with escaping.
func formatValue(value string) string {
	if value == "" {
		return ""
	}
	needsQuoting := strings.ContainsAny(value, " \t\n\r\"'`#\\$")
	if !needsQuoting {
		return value
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, ch := range value {
		switch ch {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(ch)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// unwrapPathError extracts the underlying error from a wrapped path error,
// allowing os.IsNotExist to work through fmt.Errorf wrapping.
func unwrapPathError(err error) error {
	for {
		unwrapped, ok := err.(interface{ Unwrap() error })
		if !ok {
			return err
		}
		err = unwrapped.Unwrap()
	}
}
