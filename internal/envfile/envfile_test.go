package envfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xcke/envref/internal/parser"
)

// writeFile is a test helper that creates a file with the given content.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()

	t.Run("loads simple env file", func(t *testing.T) {
		path := writeFile(t, dir, ".env", "FOO=bar\nBAZ=qux\n")
		env, err := Load(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if env.Len() != 2 {
			t.Fatalf("expected 2 entries, got %d", env.Len())
		}
		entry, ok := env.Get("FOO")
		if !ok {
			t.Fatal("expected key FOO to exist")
		}
		if entry.Value != "bar" {
			t.Errorf("FOO: got %q, want %q", entry.Value, "bar")
		}
		entry, ok = env.Get("BAZ")
		if !ok {
			t.Fatal("expected key BAZ to exist")
		}
		if entry.Value != "qux" {
			t.Errorf("BAZ: got %q, want %q", entry.Value, "qux")
		}
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		_, err := Load(filepath.Join(dir, "nonexistent"))
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("returns error for parse error", func(t *testing.T) {
		path := writeFile(t, dir, ".env.bad", "FOO='unterminated")
		_, err := Load(path)
		if err == nil {
			t.Fatal("expected error for parse error")
		}
	})

	t.Run("handles duplicate keys (last wins)", func(t *testing.T) {
		path := writeFile(t, dir, ".env.dup", "FOO=first\nFOO=second\n")
		env, err := Load(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if env.Len() != 1 {
			t.Fatalf("expected 1 entry, got %d", env.Len())
		}
		entry, _ := env.Get("FOO")
		if entry.Value != "second" {
			t.Errorf("FOO: got %q, want %q", entry.Value, "second")
		}
	})

	t.Run("preserves key order", func(t *testing.T) {
		path := writeFile(t, dir, ".env.order", "CHARLIE=3\nALPHA=1\nBRAVO=2\n")
		env, err := Load(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		keys := env.Keys()
		want := []string{"CHARLIE", "ALPHA", "BRAVO"}
		if len(keys) != len(want) {
			t.Fatalf("expected %d keys, got %d", len(want), len(keys))
		}
		for i, k := range want {
			if keys[i] != k {
				t.Errorf("keys[%d]: got %q, want %q", i, keys[i], k)
			}
		}
	})

	t.Run("loads env file with comments and blank lines", func(t *testing.T) {
		content := "# Database\nDB_HOST=localhost\n\n# App\nAPP_NAME='My App'\n"
		path := writeFile(t, dir, ".env.comments", content)
		env, err := Load(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if env.Len() != 2 {
			t.Fatalf("expected 2 entries, got %d", env.Len())
		}
	})

	t.Run("loads env file with ref:// values", func(t *testing.T) {
		content := "API_KEY=ref://secrets/api_key\nDB_PASS=ref://keychain/db_pass\n"
		path := writeFile(t, dir, ".env.refs", content)
		env, err := Load(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		entry, _ := env.Get("API_KEY")
		if entry.Value != "ref://secrets/api_key" {
			t.Errorf("API_KEY: got %q, want %q", entry.Value, "ref://secrets/api_key")
		}
	})
}

func TestLoadOptional(t *testing.T) {
	dir := t.TempDir()

	t.Run("returns empty env for missing file", func(t *testing.T) {
		env, err := LoadOptional(filepath.Join(dir, ".env.local"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if env.Len() != 0 {
			t.Fatalf("expected 0 entries, got %d", env.Len())
		}
	})

	t.Run("loads existing file", func(t *testing.T) {
		path := writeFile(t, dir, ".env.local", "SECRET=value\n")
		env, err := LoadOptional(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if env.Len() != 1 {
			t.Fatalf("expected 1 entry, got %d", env.Len())
		}
		entry, _ := env.Get("SECRET")
		if entry.Value != "value" {
			t.Errorf("SECRET: got %q, want %q", entry.Value, "value")
		}
	})

	t.Run("returns error for parse error", func(t *testing.T) {
		path := writeFile(t, dir, ".env.local.bad", "KEY='unterminated")
		_, err := LoadOptional(path)
		if err == nil {
			t.Fatal("expected error for parse error")
		}
	})
}

func TestMerge(t *testing.T) {
	t.Run("overlay overrides base", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "FOO", Value: "base", Line: 1})
		base.Set(parser.Entry{Key: "BAR", Value: "base", Line: 2})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "FOO", Value: "override", Line: 1})

		result := Merge(base, overlay)

		if result.Len() != 2 {
			t.Fatalf("expected 2 entries, got %d", result.Len())
		}
		entry, _ := result.Get("FOO")
		if entry.Value != "override" {
			t.Errorf("FOO: got %q, want %q", entry.Value, "override")
		}
		entry, _ = result.Get("BAR")
		if entry.Value != "base" {
			t.Errorf("BAR: got %q, want %q", entry.Value, "base")
		}
	})

	t.Run("overlay adds new keys", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "FOO", Value: "base", Line: 1})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "NEW_KEY", Value: "new", Line: 1})

		result := Merge(base, overlay)

		if result.Len() != 2 {
			t.Fatalf("expected 2 entries, got %d", result.Len())
		}
		entry, _ := result.Get("NEW_KEY")
		if entry.Value != "new" {
			t.Errorf("NEW_KEY: got %q, want %q", entry.Value, "new")
		}
	})

	t.Run("preserves base order with overlay additions appended", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "ALPHA", Value: "1", Line: 1})
		base.Set(parser.Entry{Key: "BRAVO", Value: "2", Line: 2})
		base.Set(parser.Entry{Key: "CHARLIE", Value: "3", Line: 3})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "BRAVO", Value: "override", Line: 1})
		overlay.Set(parser.Entry{Key: "DELTA", Value: "4", Line: 2})

		result := Merge(base, overlay)

		keys := result.Keys()
		want := []string{"ALPHA", "BRAVO", "CHARLIE", "DELTA"}
		if len(keys) != len(want) {
			t.Fatalf("expected %d keys, got %d: %v", len(want), len(keys), keys)
		}
		for i, k := range want {
			if keys[i] != k {
				t.Errorf("keys[%d]: got %q, want %q", i, keys[i], k)
			}
		}
	})

	t.Run("does not modify base", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "FOO", Value: "original", Line: 1})

		overlay := NewEnv()
		overlay.Set(parser.Entry{Key: "FOO", Value: "override", Line: 1})

		_ = Merge(base, overlay)

		entry, _ := base.Get("FOO")
		if entry.Value != "original" {
			t.Errorf("base was modified: FOO got %q, want %q", entry.Value, "original")
		}
	})

	t.Run("multiple overlays applied in order", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "FOO", Value: "base", Line: 1})

		overlay1 := NewEnv()
		overlay1.Set(parser.Entry{Key: "FOO", Value: "first", Line: 1})
		overlay1.Set(parser.Entry{Key: "FROM_1", Value: "yes", Line: 2})

		overlay2 := NewEnv()
		overlay2.Set(parser.Entry{Key: "FOO", Value: "second", Line: 1})
		overlay2.Set(parser.Entry{Key: "FROM_2", Value: "yes", Line: 2})

		result := Merge(base, overlay1, overlay2)

		entry, _ := result.Get("FOO")
		if entry.Value != "second" {
			t.Errorf("FOO: got %q, want %q", entry.Value, "second")
		}
		if result.Len() != 3 {
			t.Fatalf("expected 3 entries, got %d", result.Len())
		}
	})

	t.Run("empty overlay does not change base", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "FOO", Value: "bar", Line: 1})

		overlay := NewEnv()

		result := Merge(base, overlay)

		if result.Len() != 1 {
			t.Fatalf("expected 1 entry, got %d", result.Len())
		}
		entry, _ := result.Get("FOO")
		if entry.Value != "bar" {
			t.Errorf("FOO: got %q, want %q", entry.Value, "bar")
		}
	})

	t.Run("merge with no overlays copies base", func(t *testing.T) {
		base := NewEnv()
		base.Set(parser.Entry{Key: "FOO", Value: "bar", Line: 1})

		result := Merge(base)

		if result.Len() != 1 {
			t.Fatalf("expected 1 entry, got %d", result.Len())
		}
		entry, _ := result.Get("FOO")
		if entry.Value != "bar" {
			t.Errorf("FOO: got %q, want %q", entry.Value, "bar")
		}
	})
}

func TestEnv(t *testing.T) {
	t.Run("Get returns false for missing key", func(t *testing.T) {
		env := NewEnv()
		_, ok := env.Get("MISSING")
		if ok {
			t.Error("expected ok to be false for missing key")
		}
	})

	t.Run("Set updates existing key in place", func(t *testing.T) {
		env := NewEnv()
		env.Set(parser.Entry{Key: "FOO", Value: "first", Line: 1})
		env.Set(parser.Entry{Key: "BAR", Value: "second", Line: 2})
		env.Set(parser.Entry{Key: "FOO", Value: "updated", Line: 3})

		if env.Len() != 2 {
			t.Fatalf("expected 2 entries, got %d", env.Len())
		}
		keys := env.Keys()
		if keys[0] != "FOO" || keys[1] != "BAR" {
			t.Errorf("order changed: got %v", keys)
		}
		entry, _ := env.Get("FOO")
		if entry.Value != "updated" {
			t.Errorf("FOO: got %q, want %q", entry.Value, "updated")
		}
	})

	t.Run("All returns entries in order", func(t *testing.T) {
		env := NewEnv()
		env.Set(parser.Entry{Key: "B", Value: "2", Line: 1})
		env.Set(parser.Entry{Key: "A", Value: "1", Line: 2})
		env.Set(parser.Entry{Key: "C", Value: "3", Line: 3})

		all := env.All()
		if len(all) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(all))
		}
		wantKeys := []string{"B", "A", "C"}
		for i, key := range wantKeys {
			if all[i].Key != key {
				t.Errorf("all[%d].Key: got %q, want %q", i, all[i].Key, key)
			}
		}
	})
}

func TestLoadAndMergeIntegration(t *testing.T) {
	dir := t.TempDir()

	// Create a .env file with base config.
	envContent := `# Database config
DB_HOST=localhost
DB_PORT=5432
DB_USER=devuser
DB_PASS=ref://secrets/db_pass

# App config
APP_NAME='My App'
DEBUG=false
`
	envPath := writeFile(t, dir, ".env", envContent)

	// Create a .env.local with personal overrides.
	localContent := `# Local overrides
DB_HOST=127.0.0.1
DEBUG=true
EXTRA_VAR=local_only
`
	localPath := writeFile(t, dir, ".env.local", localContent)

	base, err := Load(envPath)
	if err != nil {
		t.Fatalf("loading .env: %v", err)
	}

	local, err := Load(localPath)
	if err != nil {
		t.Fatalf("loading .env.local: %v", err)
	}

	merged := Merge(base, local)

	// Verify overrides.
	entry, ok := merged.Get("DB_HOST")
	if !ok {
		t.Fatal("expected DB_HOST")
	}
	if entry.Value != "127.0.0.1" {
		t.Errorf("DB_HOST: got %q, want %q", entry.Value, "127.0.0.1")
	}

	entry, ok = merged.Get("DEBUG")
	if !ok {
		t.Fatal("expected DEBUG")
	}
	if entry.Value != "true" {
		t.Errorf("DEBUG: got %q, want %q", entry.Value, "true")
	}

	// Verify base values preserved.
	entry, ok = merged.Get("DB_PORT")
	if !ok {
		t.Fatal("expected DB_PORT")
	}
	if entry.Value != "5432" {
		t.Errorf("DB_PORT: got %q, want %q", entry.Value, "5432")
	}

	entry, ok = merged.Get("DB_PASS")
	if !ok {
		t.Fatal("expected DB_PASS")
	}
	if entry.Value != "ref://secrets/db_pass" {
		t.Errorf("DB_PASS: got %q, want %q", entry.Value, "ref://secrets/db_pass")
	}

	// Verify new key from overlay.
	entry, ok = merged.Get("EXTRA_VAR")
	if !ok {
		t.Fatal("expected EXTRA_VAR")
	}
	if entry.Value != "local_only" {
		t.Errorf("EXTRA_VAR: got %q, want %q", entry.Value, "local_only")
	}

	// Verify total count: 6 from base + 1 new from overlay = 7.
	if merged.Len() != 7 {
		t.Errorf("expected 7 entries, got %d", merged.Len())
	}

	// Verify order: base keys first, then new keys from overlay.
	keys := merged.Keys()
	wantKeys := []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_PASS", "APP_NAME", "DEBUG", "EXTRA_VAR"}
	if len(keys) != len(wantKeys) {
		t.Fatalf("expected %d keys, got %d: %v", len(wantKeys), len(keys), keys)
	}
	for i, k := range wantKeys {
		if keys[i] != k {
			t.Errorf("keys[%d]: got %q, want %q", i, keys[i], k)
		}
	}
}

func TestRefs(t *testing.T) {
	t.Run("returns only ref entries", func(t *testing.T) {
		env := NewEnv()
		env.Set(parser.Entry{Key: "DB_HOST", Value: "localhost", Line: 1})
		env.Set(parser.Entry{Key: "DB_PASS", Value: "ref://secrets/db_pass", Line: 2, IsRef: true})
		env.Set(parser.Entry{Key: "API_KEY", Value: "ref://keychain/api_key", Line: 3, IsRef: true})
		env.Set(parser.Entry{Key: "DEBUG", Value: "true", Line: 4})

		refs := env.Refs()
		if len(refs) != 2 {
			t.Fatalf("expected 2 refs, got %d", len(refs))
		}
		if refs[0].Key != "DB_PASS" {
			t.Errorf("refs[0].Key: got %q, want %q", refs[0].Key, "DB_PASS")
		}
		if refs[1].Key != "API_KEY" {
			t.Errorf("refs[1].Key: got %q, want %q", refs[1].Key, "API_KEY")
		}
	})

	t.Run("returns empty for no refs", func(t *testing.T) {
		env := NewEnv()
		env.Set(parser.Entry{Key: "FOO", Value: "bar", Line: 1})

		refs := env.Refs()
		if len(refs) != 0 {
			t.Fatalf("expected 0 refs, got %d", len(refs))
		}
	})
}

func TestHasRefs(t *testing.T) {
	t.Run("true when refs exist", func(t *testing.T) {
		env := NewEnv()
		env.Set(parser.Entry{Key: "FOO", Value: "bar", Line: 1})
		env.Set(parser.Entry{Key: "SECRET", Value: "ref://secrets/key", Line: 2, IsRef: true})

		if !env.HasRefs() {
			t.Error("expected HasRefs() to return true")
		}
	})

	t.Run("false when no refs", func(t *testing.T) {
		env := NewEnv()
		env.Set(parser.Entry{Key: "FOO", Value: "bar", Line: 1})

		if env.HasRefs() {
			t.Error("expected HasRefs() to return false")
		}
	})

	t.Run("false for empty env", func(t *testing.T) {
		env := NewEnv()
		if env.HasRefs() {
			t.Error("expected HasRefs() to return false for empty env")
		}
	})
}

func TestResolvedRefs(t *testing.T) {
	t.Run("parses valid refs", func(t *testing.T) {
		env := NewEnv()
		env.Set(parser.Entry{Key: "DB_HOST", Value: "localhost", Line: 1})
		env.Set(parser.Entry{Key: "DB_PASS", Value: "ref://secrets/db_pass", Line: 2, IsRef: true})
		env.Set(parser.Entry{Key: "API_KEY", Value: "ref://keychain/api_key", Line: 3, IsRef: true})

		resolved := env.ResolvedRefs()
		if len(resolved) != 2 {
			t.Fatalf("expected 2 resolved refs, got %d", len(resolved))
		}

		dbRef, ok := resolved["DB_PASS"]
		if !ok {
			t.Fatal("expected DB_PASS in resolved refs")
		}
		if dbRef.Backend != "secrets" {
			t.Errorf("DB_PASS backend: got %q, want %q", dbRef.Backend, "secrets")
		}
		if dbRef.Path != "db_pass" {
			t.Errorf("DB_PASS path: got %q, want %q", dbRef.Path, "db_pass")
		}

		apiRef, ok := resolved["API_KEY"]
		if !ok {
			t.Fatal("expected API_KEY in resolved refs")
		}
		if apiRef.Backend != "keychain" {
			t.Errorf("API_KEY backend: got %q, want %q", apiRef.Backend, "keychain")
		}
		if apiRef.Path != "api_key" {
			t.Errorf("API_KEY path: got %q, want %q", apiRef.Path, "api_key")
		}
	})
}

func TestLoadRefsFromFile(t *testing.T) {
	dir := t.TempDir()
	content := "DB_HOST=localhost\nDB_PASS=ref://secrets/db_pass\nAPI_KEY=ref://keychain/api_key\nDEBUG=true\n"
	path := writeFile(t, dir, ".env", content)

	env, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !env.HasRefs() {
		t.Fatal("expected env to have refs")
	}

	refs := env.Refs()
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
	if refs[0].Key != "DB_PASS" {
		t.Errorf("refs[0].Key: got %q, want %q", refs[0].Key, "DB_PASS")
	}
	if refs[1].Key != "API_KEY" {
		t.Errorf("refs[1].Key: got %q, want %q", refs[1].Key, "API_KEY")
	}

	// Non-ref entries should have IsRef = false.
	entry, _ := env.Get("DB_HOST")
	if entry.IsRef {
		t.Error("DB_HOST should not be a ref")
	}
	entry, _ = env.Get("DEBUG")
	if entry.IsRef {
		t.Error("DEBUG should not be a ref")
	}
}

func TestLoadAndMergeOptionalLocal(t *testing.T) {
	dir := t.TempDir()

	// Only .env exists, no .env.local.
	envPath := writeFile(t, dir, ".env", "FOO=bar\n")

	base, err := Load(envPath)
	if err != nil {
		t.Fatalf("loading .env: %v", err)
	}

	local, err := LoadOptional(filepath.Join(dir, ".env.local"))
	if err != nil {
		t.Fatalf("loading optional .env.local: %v", err)
	}

	merged := Merge(base, local)

	if merged.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", merged.Len())
	}
	entry, _ := merged.Get("FOO")
	if entry.Value != "bar" {
		t.Errorf("FOO: got %q, want %q", entry.Value, "bar")
	}
}
