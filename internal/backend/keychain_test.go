package backend

import (
	"errors"
	"fmt"
	"testing"

	"github.com/zalando/go-keyring"
)

// setupMockKeyring replaces the keyring provider with an in-memory mock
// and returns a cleanup function that restores the original provider.
func setupMockKeyring() func() {
	origSet := keyringProvider.Set
	origGet := keyringProvider.Get
	origDelete := keyringProvider.Delete

	keyring.MockInit()

	keyringProvider.Set = keyring.Set
	keyringProvider.Get = keyring.Get
	keyringProvider.Delete = keyring.Delete

	return func() {
		keyringProvider.Set = origSet
		keyringProvider.Get = origGet
		keyringProvider.Delete = origDelete
	}
}

func TestKeychainBackend_Interface(t *testing.T) {
	var _ Backend = NewKeychainBackend()
}

func TestKeychainBackend_Name(t *testing.T) {
	kb := NewKeychainBackend()
	if kb.Name() != "keychain" {
		t.Fatalf("Name: got %q, want %q", kb.Name(), "keychain")
	}
}

func TestKeychainBackend_GetNotFound(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	kb := NewKeychainBackend()

	_, err := kb.Get("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(nonexistent): got %v, want ErrNotFound", err)
	}
}

func TestKeychainBackend_SetAndGet(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	kb := NewKeychainBackend()

	// Set a secret.
	if err := kb.Set("api_key", "secret123"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Get the secret back.
	val, err := kb.Get("api_key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "secret123" {
		t.Fatalf("Get: got %q, want %q", val, "secret123")
	}
}

func TestKeychainBackend_SetOverwrite(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	kb := NewKeychainBackend()

	_ = kb.Set("api_key", "original")
	if err := kb.Set("api_key", "updated"); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}

	val, err := kb.Get("api_key")
	if err != nil {
		t.Fatalf("Get after overwrite: %v", err)
	}
	if val != "updated" {
		t.Fatalf("Get after overwrite: got %q, want %q", val, "updated")
	}
}

func TestKeychainBackend_Delete(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	kb := NewKeychainBackend()

	_ = kb.Set("api_key", "secret")

	if err := kb.Delete("api_key"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := kb.Get("api_key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete: got %v, want ErrNotFound", err)
	}
}

func TestKeychainBackend_DeleteNotFound(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	kb := NewKeychainBackend()

	err := kb.Delete("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete(missing): got %v, want ErrNotFound", err)
	}
}

func TestKeychainBackend_List(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	kb := NewKeychainBackend()

	// Empty list.
	keys, err := kb.List()
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("List empty: got %v, want empty", keys)
	}

	// Add some keys.
	_ = kb.Set("zebra", "val1")
	_ = kb.Set("alpha", "val2")
	_ = kb.Set("middle", "val3")

	keys, err = kb.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	// Keys should be sorted.
	want := []string{"alpha", "middle", "zebra"}
	if len(keys) != len(want) {
		t.Fatalf("List: got %v, want %v", keys, want)
	}
	for i, k := range keys {
		if k != want[i] {
			t.Fatalf("List[%d]: got %q, want %q", i, k, want[i])
		}
	}
}

func TestKeychainBackend_ListAfterDelete(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	kb := NewKeychainBackend()

	_ = kb.Set("alpha", "1")
	_ = kb.Set("beta", "2")
	_ = kb.Set("gamma", "3")

	_ = kb.Delete("beta")

	keys, err := kb.List()
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}

	want := []string{"alpha", "gamma"}
	if len(keys) != len(want) {
		t.Fatalf("List after delete: got %v, want %v", keys, want)
	}
	for i, k := range keys {
		if k != want[i] {
			t.Fatalf("List[%d]: got %q, want %q", i, k, want[i])
		}
	}
}

func TestKeychainBackend_SetIdempotentIndex(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	kb := NewKeychainBackend()

	// Set the same key multiple times — index should not have duplicates.
	_ = kb.Set("api_key", "v1")
	_ = kb.Set("api_key", "v2")
	_ = kb.Set("api_key", "v3")

	keys, err := kb.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("List: got %v, want [api_key]", keys)
	}
	if keys[0] != "api_key" {
		t.Fatalf("List[0]: got %q, want %q", keys[0], "api_key")
	}
}

func TestKeychainBackend_RegistryIntegration(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	kb := NewKeychainBackend()
	reg := NewRegistry()
	if err := reg.Register(kb); err != nil {
		t.Fatalf("Register: %v", err)
	}

	_ = kb.Set("db_pass", "keychain_secret")

	val, err := reg.Get("db_pass")
	if err != nil {
		t.Fatalf("Registry.Get: %v", err)
	}
	if val != "keychain_secret" {
		t.Fatalf("Registry.Get: got %q, want %q", val, "keychain_secret")
	}
}

func TestKeychainBackend_NamespacedIntegration(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	kb := NewKeychainBackend()
	nb, err := NewNamespacedBackend(kb, "myproject")
	if err != nil {
		t.Fatalf("NewNamespacedBackend: %v", err)
	}

	// Set via namespaced backend.
	if err := nb.Set("api_key", "proj_secret"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Get via namespaced backend.
	val, err := nb.Get("api_key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "proj_secret" {
		t.Fatalf("Get: got %q, want %q", val, "proj_secret")
	}

	// List via namespaced backend returns only project keys.
	keys, err := nb.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 1 || keys[0] != "api_key" {
		t.Fatalf("List: got %v, want [api_key]", keys)
	}

	// Delete via namespaced backend.
	if err := nb.Delete("api_key"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = nb.Get("api_key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete: got %v, want ErrNotFound", err)
	}
}

func TestKeychainBackend_MultipleProjectIsolation(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	kb := NewKeychainBackend()
	app1, _ := NewNamespacedBackend(kb, "app1")
	app2, _ := NewNamespacedBackend(kb, "app2")

	_ = app1.Set("secret", "app1_val")
	_ = app2.Set("secret", "app2_val")

	val1, err := app1.Get("secret")
	if err != nil {
		t.Fatalf("app1.Get: %v", err)
	}
	if val1 != "app1_val" {
		t.Fatalf("app1.Get: got %q, want %q", val1, "app1_val")
	}

	val2, err := app2.Get("secret")
	if err != nil {
		t.Fatalf("app2.Get: %v", err)
	}
	if val2 != "app2_val" {
		t.Fatalf("app2.Get: got %q, want %q", val2, "app2_val")
	}

	// Delete from app1 does not affect app2.
	_ = app1.Delete("secret")
	_, err = app1.Get("secret")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("app1.Get after delete: got %v, want ErrNotFound", err)
	}

	val2, err = app2.Get("secret")
	if err != nil {
		t.Fatalf("app2.Get after app1 delete: %v", err)
	}
	if val2 != "app2_val" {
		t.Fatalf("app2.Get after app1 delete: got %q, want %q", val2, "app2_val")
	}
}

func TestKeychainBackend_ErrorWrapping(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	kb := NewKeychainBackend()

	// Verify that ErrNotFound from Get is unwrappable.
	_, err := kb.Get("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// Verify that ErrNotFound from Delete is unwrappable.
	err = kb.Delete("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestKeychainBackend_SpecialCharacterKeys(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	kb := NewKeychainBackend()

	tests := []struct {
		key   string
		value string
	}{
		{"key-with-dashes", "val1"},
		{"key.with.dots", "val2"},
		{"key_with_underscores", "val3"},
		{"KEY_UPPER", "val4"},
		{"key/with/slashes", "val5"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if err := kb.Set(tt.key, tt.value); err != nil {
				t.Fatalf("Set(%q): %v", tt.key, err)
			}
			val, err := kb.Get(tt.key)
			if err != nil {
				t.Fatalf("Get(%q): %v", tt.key, err)
			}
			if val != tt.value {
				t.Fatalf("Get(%q): got %q, want %q", tt.key, val, tt.value)
			}
		})
	}
}

func TestKeychainBackend_LargeValue(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	kb := NewKeychainBackend()

	// Store a large value (e.g., a certificate or long token).
	largeVal := ""
	for i := 0; i < 1000; i++ {
		largeVal += fmt.Sprintf("segment-%d-", i)
	}

	if err := kb.Set("large_key", largeVal); err != nil {
		t.Fatalf("Set large value: %v", err)
	}

	val, err := kb.Get("large_key")
	if err != nil {
		t.Fatalf("Get large value: %v", err)
	}
	if val != largeVal {
		t.Fatalf("Get large value: length got %d, want %d", len(val), len(largeVal))
	}
}
