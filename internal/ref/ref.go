// Package ref provides parsing and representation of ref:// secret references
// used in .env files to point to values stored in secret backends.
//
// A ref:// URI has the form:
//
//	ref://<backend>/<path>
//
// Examples:
//
//	ref://secrets/api_key        → backend "secrets", path "api_key"
//	ref://keychain/db_pass       → backend "keychain", path "db_pass"
//	ref://ssm/prod/db/password   → backend "ssm", path "prod/db/password"
package ref

import (
	"fmt"
	"strings"
)

// Prefix is the URI scheme prefix for secret references.
const Prefix = "ref://"

// Reference represents a parsed ref:// URI pointing to a secret in a backend.
type Reference struct {
	// Raw is the original ref:// string as it appeared in the .env file.
	Raw string
	// Backend is the backend identifier (e.g. "secrets", "keychain", "ssm").
	Backend string
	// Path is the key or path within the backend (e.g. "api_key", "prod/db/password").
	Path string
}

// String returns the canonical ref:// URI for this reference.
func (r Reference) String() string {
	return Prefix + r.Backend + "/" + r.Path
}

// IsRef reports whether the given value is a ref:// reference.
func IsRef(value string) bool {
	return strings.HasPrefix(value, Prefix)
}

// Parse parses a ref:// URI into a Reference.
// Returns an error if the value is not a valid ref:// URI.
func Parse(value string) (Reference, error) {
	if !IsRef(value) {
		return Reference{}, fmt.Errorf("not a ref:// URI: %q", value)
	}

	// Strip the ref:// prefix.
	rest := value[len(Prefix):]
	if rest == "" {
		return Reference{}, fmt.Errorf("empty ref:// URI: %q", value)
	}

	// Split into backend and path at the first slash.
	slashIdx := strings.IndexByte(rest, '/')
	if slashIdx < 0 {
		return Reference{}, fmt.Errorf("ref:// URI missing path: %q (expected ref://<backend>/<path>)", value)
	}

	backend := rest[:slashIdx]
	path := rest[slashIdx+1:]

	if backend == "" {
		return Reference{}, fmt.Errorf("ref:// URI has empty backend: %q", value)
	}
	if path == "" {
		return Reference{}, fmt.Errorf("ref:// URI has empty path: %q", value)
	}

	return Reference{
		Raw:     value,
		Backend: backend,
		Path:    path,
	}, nil
}

// ContainsRef reports whether s contains an embedded ref:// URI.
// Unlike IsRef, this checks for ref:// anywhere in the string.
func ContainsRef(s string) bool {
	return strings.Contains(s, Prefix)
}

// Embedded represents a ref:// URI found embedded within a larger string,
// along with its start and end positions.
type Embedded struct {
	// Ref is the parsed reference.
	Ref Reference
	// Start is the byte offset of "ref://" in the containing string.
	Start int
	// End is the byte offset just past the last character of the ref URI.
	End int
}

// FindAll returns all ref:// URIs embedded in s, in order of appearance.
// It extracts ref URIs by scanning for the "ref://" prefix and collecting
// valid URI characters (alphanumeric, slash, underscore, hyphen, dot) until
// a delimiter or end of string is reached. Invalid URIs (e.g., missing path)
// are skipped.
func FindAll(s string) []Embedded {
	var results []Embedded
	offset := 0
	for {
		idx := strings.Index(s[offset:], Prefix)
		if idx < 0 {
			break
		}
		start := offset + idx
		// Extract the ref:// URI by scanning forward for valid URI chars.
		end := start + len(Prefix)
		for end < len(s) && isRefChar(s[end]) {
			end++
		}
		raw := s[start:end]
		parsed, err := Parse(raw)
		if err == nil {
			results = append(results, Embedded{
				Ref:   parsed,
				Start: start,
				End:   end,
			})
		}
		// Move past this ref:// occurrence.
		offset = start + len(Prefix)
	}
	return results
}

// isRefChar reports whether c is a valid character within a ref:// URI
// (after the "ref://" prefix). Valid characters are alphanumeric, slash,
// underscore, hyphen, and dot.
func isRefChar(c byte) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '/' || c == '_' || c == '-' || c == '.'
}
