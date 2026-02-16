package backend

import (
	"errors"
	"testing"
)

func TestNewNamespacedBackend(t *testing.T) {
	inner := newMemoryBackend("keychain")

	t.Run("valid", func(t *testing.T) {
		nb, err := NewNamespacedBackend(inner, "myapp")
		if err != nil {
			t.Fatalf("NewNamespacedBackend: %v", err)
		}
		if nb.Name() != "keychain" {
			t.Fatalf("Name: got %q, want %q", nb.Name(), "keychain")
		}
		if nb.Project() != "myapp" {
			t.Fatalf("Project: got %q, want %q", nb.Project(), "myapp")
		}
	})

	t.Run("empty project", func(t *testing.T) {
		_, err := NewNamespacedBackend(inner, "")
		if err == nil {
			t.Fatal("NewNamespacedBackend with empty project: expected error")
		}
	})

	t.Run("nil inner", func(t *testing.T) {
		_, err := NewNamespacedBackend(nil, "myapp")
		if err == nil {
			t.Fatal("NewNamespacedBackend with nil inner: expected error")
		}
	})
}

func TestNamespacedBackend_Interface(t *testing.T) {
	inner := newMemoryBackend("keychain")
	nb, _ := NewNamespacedBackend(inner, "myapp")
	var _ Backend = nb
}

func TestNamespacedBackend_GetSet(t *testing.T) {
	inner := newMemoryBackend("keychain")
	nb, _ := NewNamespacedBackend(inner, "myapp")

	// Set via namespaced backend.
	if err := nb.Set("api_key", "secret123"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Get via namespaced backend.
	val, err := nb.Get("api_key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "secret123" {
		t.Fatalf("Get: got %q, want %q", val, "secret123")
	}

	// Verify the underlying backend stores the namespaced key.
	val, err = inner.Get("myapp/api_key")
	if err != nil {
		t.Fatalf("inner.Get(myapp/api_key): %v", err)
	}
	if val != "secret123" {
		t.Fatalf("inner.Get: got %q, want %q", val, "secret123")
	}

	// Direct access with the un-prefixed key should fail.
	_, err = inner.Get("api_key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("inner.Get(api_key): got %v, want ErrNotFound", err)
	}
}

func TestNamespacedBackend_GetNotFound(t *testing.T) {
	inner := newMemoryBackend("keychain")
	nb, _ := NewNamespacedBackend(inner, "myapp")

	_, err := nb.Get("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(missing): got %v, want ErrNotFound", err)
	}
}

func TestNamespacedBackend_Delete(t *testing.T) {
	inner := newMemoryBackend("keychain")
	nb, _ := NewNamespacedBackend(inner, "myapp")

	_ = nb.Set("api_key", "secret")

	if err := nb.Delete("api_key"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := nb.Get("api_key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete: got %v, want ErrNotFound", err)
	}

	// Underlying backend should also have it removed.
	_, err = inner.Get("myapp/api_key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("inner.Get after Delete: got %v, want ErrNotFound", err)
	}
}

func TestNamespacedBackend_DeleteNotFound(t *testing.T) {
	inner := newMemoryBackend("keychain")
	nb, _ := NewNamespacedBackend(inner, "myapp")

	err := nb.Delete("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete(missing): got %v, want ErrNotFound", err)
	}
}

func TestNamespacedBackend_List(t *testing.T) {
	inner := newMemoryBackend("keychain")

	// Set up keys from two different projects in the same backend.
	_ = inner.Set("myapp/api_key", "secret1")
	_ = inner.Set("myapp/db_pass", "secret2")
	_ = inner.Set("otherapp/api_key", "other_secret")
	_ = inner.Set("unscoped_key", "bare")

	nb, _ := NewNamespacedBackend(inner, "myapp")

	keys, err := nb.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	// Should only see myapp's keys, without the prefix.
	want := []string{"api_key", "db_pass"}
	if len(keys) != len(want) {
		t.Fatalf("List: got %v, want %v", keys, want)
	}
	for i, k := range keys {
		if k != want[i] {
			t.Fatalf("List[%d]: got %q, want %q", i, k, want[i])
		}
	}
}

func TestNamespacedBackend_ListEmpty(t *testing.T) {
	inner := newMemoryBackend("keychain")
	nb, _ := NewNamespacedBackend(inner, "myapp")

	keys, err := nb.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("List: got %v, want empty", keys)
	}
}

func TestNamespacedBackend_ListOtherProjectOnly(t *testing.T) {
	inner := newMemoryBackend("keychain")
	_ = inner.Set("otherapp/key1", "val1")

	nb, _ := NewNamespacedBackend(inner, "myapp")

	keys, err := nb.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("List: got %v, want empty", keys)
	}
}

func TestNamespacedBackend_ProjectIsolation(t *testing.T) {
	inner := newMemoryBackend("keychain")
	app1, _ := NewNamespacedBackend(inner, "app1")
	app2, _ := NewNamespacedBackend(inner, "app2")

	// Set the same key name in both projects.
	_ = app1.Set("api_key", "secret_for_app1")
	_ = app2.Set("api_key", "secret_for_app2")

	// Each project sees its own value.
	val1, err := app1.Get("api_key")
	if err != nil {
		t.Fatalf("app1.Get: %v", err)
	}
	if val1 != "secret_for_app1" {
		t.Fatalf("app1.Get: got %q, want %q", val1, "secret_for_app1")
	}

	val2, err := app2.Get("api_key")
	if err != nil {
		t.Fatalf("app2.Get: %v", err)
	}
	if val2 != "secret_for_app2" {
		t.Fatalf("app2.Get: got %q, want %q", val2, "secret_for_app2")
	}

	// Deleting from one project doesn't affect the other.
	_ = app1.Delete("api_key")

	_, err = app1.Get("api_key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("app1.Get after delete: got %v, want ErrNotFound", err)
	}

	val2, err = app2.Get("api_key")
	if err != nil {
		t.Fatalf("app2.Get after app1 delete: %v", err)
	}
	if val2 != "secret_for_app2" {
		t.Fatalf("app2.Get after app1 delete: got %q, want %q", val2, "secret_for_app2")
	}
}

func TestNamespacedBackend_ListViaNamespacedSet(t *testing.T) {
	inner := newMemoryBackend("keychain")
	nb, _ := NewNamespacedBackend(inner, "myapp")

	// Set keys via the namespaced backend.
	_ = nb.Set("alpha", "1")
	_ = nb.Set("beta", "2")
	_ = nb.Set("gamma", "3")

	keys, err := nb.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	want := []string{"alpha", "beta", "gamma"}
	if len(keys) != len(want) {
		t.Fatalf("List: got %v, want %v", keys, want)
	}
	for i, k := range keys {
		if k != want[i] {
			t.Fatalf("List[%d]: got %q, want %q", i, k, want[i])
		}
	}
}

// ---------------------------------------------------------------------------
// Profile-scoped NamespacedBackend Tests
// ---------------------------------------------------------------------------

func TestNewProfileNamespacedBackend(t *testing.T) {
	inner := newMemoryBackend("keychain")

	t.Run("valid", func(t *testing.T) {
		nb, err := NewProfileNamespacedBackend(inner, "myapp", "staging")
		if err != nil {
			t.Fatalf("NewProfileNamespacedBackend: %v", err)
		}
		if nb.Name() != "keychain" {
			t.Fatalf("Name: got %q, want %q", nb.Name(), "keychain")
		}
		if nb.Project() != "myapp" {
			t.Fatalf("Project: got %q, want %q", nb.Project(), "myapp")
		}
		if nb.Profile() != "staging" {
			t.Fatalf("Profile: got %q, want %q", nb.Profile(), "staging")
		}
	})

	t.Run("empty project", func(t *testing.T) {
		_, err := NewProfileNamespacedBackend(inner, "", "staging")
		if err == nil {
			t.Fatal("expected error for empty project")
		}
	})

	t.Run("empty profile", func(t *testing.T) {
		_, err := NewProfileNamespacedBackend(inner, "myapp", "")
		if err == nil {
			t.Fatal("expected error for empty profile")
		}
	})

	t.Run("nil inner", func(t *testing.T) {
		_, err := NewProfileNamespacedBackend(nil, "myapp", "staging")
		if err == nil {
			t.Fatal("expected error for nil inner")
		}
	})
}

func TestProfileNamespacedBackend_GetSet(t *testing.T) {
	inner := newMemoryBackend("keychain")
	nb, _ := NewProfileNamespacedBackend(inner, "myapp", "staging")

	// Set via profile-scoped backend.
	if err := nb.Set("api_key", "staging-secret"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Get via profile-scoped backend.
	val, err := nb.Get("api_key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "staging-secret" {
		t.Fatalf("Get: got %q, want %q", val, "staging-secret")
	}

	// Verify the underlying backend stores the profile-namespaced key.
	val, err = inner.Get("myapp/staging/api_key")
	if err != nil {
		t.Fatalf("inner.Get(myapp/staging/api_key): %v", err)
	}
	if val != "staging-secret" {
		t.Fatalf("inner.Get: got %q, want %q", val, "staging-secret")
	}

	// Direct access with project-only prefix should fail.
	_, err = inner.Get("myapp/api_key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("inner.Get(myapp/api_key): got %v, want ErrNotFound", err)
	}
}

func TestProfileNamespacedBackend_Delete(t *testing.T) {
	inner := newMemoryBackend("keychain")
	nb, _ := NewProfileNamespacedBackend(inner, "myapp", "prod")

	_ = nb.Set("secret", "value")

	if err := nb.Delete("secret"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := nb.Get("secret")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete: got %v, want ErrNotFound", err)
	}
}

func TestProfileNamespacedBackend_List(t *testing.T) {
	inner := newMemoryBackend("keychain")

	// Set up keys from different scopes.
	_ = inner.Set("myapp/staging/api_key", "staging-secret")
	_ = inner.Set("myapp/staging/db_pass", "staging-db")
	_ = inner.Set("myapp/prod/api_key", "prod-secret")
	_ = inner.Set("myapp/api_key", "project-secret")
	_ = inner.Set("otherapp/staging/api_key", "other-secret")

	nb, _ := NewProfileNamespacedBackend(inner, "myapp", "staging")

	keys, err := nb.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	want := []string{"api_key", "db_pass"}
	if len(keys) != len(want) {
		t.Fatalf("List: got %v, want %v", keys, want)
	}
	for i, k := range keys {
		if k != want[i] {
			t.Fatalf("List[%d]: got %q, want %q", i, k, want[i])
		}
	}
}

func TestProfileNamespacedBackend_Isolation(t *testing.T) {
	inner := newMemoryBackend("keychain")

	// Create backends for different scopes on the same project.
	projectBackend, _ := NewNamespacedBackend(inner, "myapp")
	stagingBackend, _ := NewProfileNamespacedBackend(inner, "myapp", "staging")
	prodBackend, _ := NewProfileNamespacedBackend(inner, "myapp", "prod")

	// Set the same key in all three scopes.
	_ = projectBackend.Set("api_key", "project-value")
	_ = stagingBackend.Set("api_key", "staging-value")
	_ = prodBackend.Set("api_key", "prod-value")

	// Each scope sees its own value.
	val, _ := projectBackend.Get("api_key")
	if val != "project-value" {
		t.Fatalf("project: got %q, want %q", val, "project-value")
	}

	val, _ = stagingBackend.Get("api_key")
	if val != "staging-value" {
		t.Fatalf("staging: got %q, want %q", val, "staging-value")
	}

	val, _ = prodBackend.Get("api_key")
	if val != "prod-value" {
		t.Fatalf("prod: got %q, want %q", val, "prod-value")
	}

	// Deleting from one scope doesn't affect others.
	_ = stagingBackend.Delete("api_key")

	_, err := stagingBackend.Get("api_key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("staging after delete: got %v, want ErrNotFound", err)
	}

	val, _ = projectBackend.Get("api_key")
	if val != "project-value" {
		t.Fatalf("project after staging delete: got %q, want %q", val, "project-value")
	}

	val, _ = prodBackend.Get("api_key")
	if val != "prod-value" {
		t.Fatalf("prod after staging delete: got %q, want %q", val, "prod-value")
	}
}

func TestNamespacedBackend_ProfileReturnsEmpty(t *testing.T) {
	inner := newMemoryBackend("keychain")
	nb, _ := NewNamespacedBackend(inner, "myapp")
	if nb.Profile() != "" {
		t.Fatalf("Profile: got %q, want empty", nb.Profile())
	}
}

func TestNamespacedBackend_RegistryIntegration(t *testing.T) {
	// Verify that NamespacedBackend works correctly within a Registry.
	inner1 := newMemoryBackend("keychain")
	inner2 := newMemoryBackend("vault")
	nb1, _ := NewNamespacedBackend(inner1, "myapp")
	nb2, _ := NewNamespacedBackend(inner2, "myapp")

	reg := NewRegistry()
	_ = reg.Register(nb1)
	_ = reg.Register(nb2)

	// Set in vault (second backend).
	_ = nb2.Set("db_pass", "vault_secret")

	// Registry.Get falls through keychain (not found) to vault.
	val, err := reg.Get("db_pass")
	if err != nil {
		t.Fatalf("Registry.Get: %v", err)
	}
	if val != "vault_secret" {
		t.Fatalf("Registry.Get: got %q, want %q", val, "vault_secret")
	}

	// Set in keychain (first backend) — it should win.
	_ = nb1.Set("db_pass", "keychain_secret")
	val, err = reg.Get("db_pass")
	if err != nil {
		t.Fatalf("Registry.Get with keychain: %v", err)
	}
	if val != "keychain_secret" {
		t.Fatalf("Registry.Get with keychain: got %q, want %q", val, "keychain_secret")
	}
}
