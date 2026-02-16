package ref

import (
	"testing"
)

// FuzzParse exercises the ref:// URI parser with arbitrary input to catch
// panics or invariant violations. The parser should never panic.
func FuzzParse(f *testing.F) {
	seeds := []string{
		// Valid refs.
		"ref://secrets/api_key",
		"ref://keychain/db_pass",
		"ref://ssm/prod/db/password",
		"ref://vault/secret/data/myapp/db_password",
		"ref://1password/my-vault/item",
		// Edge cases.
		"ref://",
		"ref:///",
		"ref:///path",
		"ref://backend/",
		"ref://backend",
		// Not ref:// at all.
		"",
		"hello",
		"https://example.com",
		"ref:/missing-slash",
		"ref//broken",
		"REF://uppercase",
		// Special characters.
		"ref://backend/path with spaces",
		"ref://backend/path\twith\ttabs",
		"ref://backend/path\nwith\nnewlines",
		"ref://backend/unicode-\u4F60\u597D",
		"ref://backend/" + string([]byte{0x00, 0x01, 0x02}),
		// Long paths.
		"ref://backend/" + "a/b/c/d/e/f/g/h/i/j/k/l/m/n/o/p",
		// Multiple slashes.
		"ref://backend//double",
		"ref:///backend/path",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data string) {
		r, err := Parse(data)

		if err == nil {
			// Successful parse — verify invariants.

			// Raw must equal the input.
			if r.Raw != data {
				t.Errorf("Parse(%q).Raw = %q, want %q", data, r.Raw, data)
			}

			// Backend must be non-empty.
			if r.Backend == "" {
				t.Errorf("Parse(%q) succeeded with empty Backend", data)
			}

			// Path must be non-empty.
			if r.Path == "" {
				t.Errorf("Parse(%q) succeeded with empty Path", data)
			}

			// String() round-trip: should produce a valid ref:// URI.
			s := r.String()
			if !IsRef(s) {
				t.Errorf("Reference.String() = %q, not a ref:// URI", s)
			}

			// Re-parse the String() output — should succeed with same backend/path.
			r2, err2 := Parse(s)
			if err2 != nil {
				t.Errorf("Parse(String()) failed: %v (String = %q)", err2, s)
			} else {
				if r2.Backend != r.Backend {
					t.Errorf("round-trip Backend: got %q, want %q", r2.Backend, r.Backend)
				}
				if r2.Path != r.Path {
					t.Errorf("round-trip Path: got %q, want %q", r2.Path, r.Path)
				}
			}
		}

		// IsRef must be consistent with Parse success for ref:// prefixed input.
		isRef := IsRef(data)
		if err == nil && !isRef {
			t.Errorf("Parse(%q) succeeded but IsRef returned false", data)
		}
	})
}

// FuzzIsRef exercises the IsRef function with arbitrary input.
func FuzzIsRef(f *testing.F) {
	seeds := []string{
		"ref://secrets/key",
		"ref://",
		"ref:/",
		"ref:",
		"ref",
		"",
		"hello",
		"REF://uppercase",
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data string) {
		result := IsRef(data)

		// IsRef should be true if and only if data starts with "ref://".
		expected := len(data) >= len(Prefix) && data[:len(Prefix)] == Prefix
		if result != expected {
			t.Errorf("IsRef(%q) = %v, want %v", data, result, expected)
		}
	})
}
