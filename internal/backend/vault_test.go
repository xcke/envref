package backend

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// testVault creates a VaultBackend in a temporary directory for testing.
func testVault(t *testing.T) *VaultBackend {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test-vault.db")
	v, err := NewVaultBackend("test-passphrase", WithVaultPath(dbPath))
	if err != nil {
		t.Fatalf("NewVaultBackend: %v", err)
	}
	t.Cleanup(func() {
		_ = v.Close()
	})
	return v
}

func TestVaultBackend_Interface(t *testing.T) {
	v := testVault(t)
	var _ Backend = v
}

func TestVaultBackend_Name(t *testing.T) {
	v := testVault(t)
	if v.Name() != "vault" {
		t.Fatalf("Name: got %q, want %q", v.Name(), "vault")
	}
}

func TestVaultBackend_GetSetDelete(t *testing.T) {
	v := testVault(t)

	// Get on missing key returns ErrNotFound.
	_, err := v.Get("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(missing): got %v, want ErrNotFound", err)
	}

	// Set and Get.
	if err := v.Set("api_key", "secret123"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := v.Get("api_key")
	if err != nil {
		t.Fatalf("Get after Set: %v", err)
	}
	if val != "secret123" {
		t.Fatalf("Get after Set: got %q, want %q", val, "secret123")
	}

	// Overwrite.
	if err := v.Set("api_key", "updated"); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	val, err = v.Get("api_key")
	if err != nil {
		t.Fatalf("Get after overwrite: %v", err)
	}
	if val != "updated" {
		t.Fatalf("Get after overwrite: got %q, want %q", val, "updated")
	}

	// Delete.
	if err := v.Delete("api_key"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = v.Get("api_key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete: got %v, want ErrNotFound", err)
	}

	// Delete on missing key returns ErrNotFound.
	err = v.Delete("api_key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete(missing): got %v, want ErrNotFound", err)
	}
}

func TestVaultBackend_List(t *testing.T) {
	v := testVault(t)

	// Empty vault.
	keys, err := v.List()
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("List empty: got %v, want empty", keys)
	}

	// Add some keys.
	for _, k := range []string{"zebra", "alpha", "middle"} {
		if err := v.Set(k, "val-"+k); err != nil {
			t.Fatalf("Set(%s): %v", k, err)
		}
	}

	keys, err = v.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
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

func TestVaultBackend_EncryptionRoundtrip(t *testing.T) {
	v := testVault(t)

	tests := []struct {
		name  string
		key   string
		value string
	}{
		{"simple", "k1", "hello"},
		{"empty value", "k2", ""},
		{"unicode", "k3", "こんにちは世界"},
		{"multiline", "k4", "line1\nline2\nline3"},
		{"special chars", "k5", `{"key": "va!@#$%^&*()_+={}<>?/\\"}`},
		{"long value", "k6", string(make([]byte, 10000))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := v.Set(tt.key, tt.value); err != nil {
				t.Fatalf("Set: %v", err)
			}
			got, err := v.Get(tt.key)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got != tt.value {
				t.Fatalf("roundtrip: got %q, want %q", got, tt.value)
			}
		})
	}
}

func TestVaultBackend_WrongPassphrase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test-vault.db")

	// Create vault and store a secret.
	v1, err := NewVaultBackend("correct-password", WithVaultPath(dbPath))
	if err != nil {
		t.Fatalf("NewVaultBackend v1: %v", err)
	}

	if err := v1.Set("api_key", "secret123"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := v1.Close(); err != nil {
		t.Fatalf("Close v1: %v", err)
	}

	// Open with wrong passphrase — Get should fail on decrypt.
	v2, err := NewVaultBackend("wrong-password", WithVaultPath(dbPath))
	if err != nil {
		t.Fatalf("NewVaultBackend v2: %v", err)
	}
	defer func() { _ = v2.Close() }()

	_, err = v2.Get("api_key")
	if err == nil {
		t.Fatal("Get with wrong passphrase should fail")
	}
}

func TestVaultBackend_Persistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test-vault.db")

	// Create vault, store secrets, and close.
	v1, err := NewVaultBackend("pass123", WithVaultPath(dbPath))
	if err != nil {
		t.Fatalf("NewVaultBackend v1: %v", err)
	}
	if err := v1.Set("key1", "value1"); err != nil {
		t.Fatalf("Set key1: %v", err)
	}
	if err := v1.Set("key2", "value2"); err != nil {
		t.Fatalf("Set key2: %v", err)
	}
	if err := v1.Close(); err != nil {
		t.Fatalf("Close v1: %v", err)
	}

	// Reopen with same passphrase — secrets should persist.
	v2, err := NewVaultBackend("pass123", WithVaultPath(dbPath))
	if err != nil {
		t.Fatalf("NewVaultBackend v2: %v", err)
	}
	defer func() { _ = v2.Close() }()

	val, err := v2.Get("key1")
	if err != nil {
		t.Fatalf("Get key1: %v", err)
	}
	if val != "value1" {
		t.Fatalf("Get key1: got %q, want %q", val, "value1")
	}

	keys, err := v2.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("List: got %d keys, want 2", len(keys))
	}
}

func TestVaultBackend_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c")
	dbPath := filepath.Join(nested, "vault.db")

	v, err := NewVaultBackend("pass", WithVaultPath(dbPath))
	if err != nil {
		t.Fatalf("NewVaultBackend: %v", err)
	}
	defer func() { _ = v.Close() }()

	// Trigger database creation.
	if err := v.Set("k", "v"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Verify directory was created.
	if _, err := os.Stat(nested); os.IsNotExist(err) {
		t.Fatalf("nested directory was not created")
	}
}

func TestVaultBackend_DBPath(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "custom.db")

	v, err := NewVaultBackend("pass", WithVaultPath(dbPath))
	if err != nil {
		t.Fatalf("NewVaultBackend: %v", err)
	}
	defer func() { _ = v.Close() }()

	if v.DBPath() != dbPath {
		t.Fatalf("DBPath: got %q, want %q", v.DBPath(), dbPath)
	}
}

func TestNewVaultBackend_EmptyPassphrase(t *testing.T) {
	_, err := NewVaultBackend("")
	if err == nil {
		t.Fatal("NewVaultBackend with empty passphrase should fail")
	}
}

func TestVaultBackend_CloseIdempotent(t *testing.T) {
	v := testVault(t)

	// Store something to trigger db open.
	if err := v.Set("k", "v"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Close multiple times should not panic or error.
	if err := v.Close(); err != nil {
		t.Fatalf("Close 1: %v", err)
	}
	if err := v.Close(); err != nil {
		t.Fatalf("Close 2: %v", err)
	}
}

func TestVaultBackend_Initialize(t *testing.T) {
	v := testVault(t)

	// Vault starts uninitialized.
	initialized, err := v.IsInitialized()
	if err != nil {
		t.Fatalf("IsInitialized: %v", err)
	}
	if initialized {
		t.Fatal("new vault should not be initialized")
	}

	// Initialize the vault.
	if err := v.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// Vault is now initialized.
	initialized, err = v.IsInitialized()
	if err != nil {
		t.Fatalf("IsInitialized after init: %v", err)
	}
	if !initialized {
		t.Fatal("vault should be initialized after Initialize()")
	}

	// Double-initialize should fail.
	err = v.Initialize()
	if err == nil {
		t.Fatal("double Initialize should fail")
	}
}

func TestVaultBackend_VerifyPassphrase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "vault.db")

	// Create and initialize a vault.
	v1, err := NewVaultBackend("correct-pass", WithVaultPath(dbPath))
	if err != nil {
		t.Fatalf("NewVaultBackend: %v", err)
	}
	if err := v1.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// Verify with correct passphrase.
	if err := v1.VerifyPassphrase(); err != nil {
		t.Fatalf("VerifyPassphrase (correct): %v", err)
	}
	if err := v1.Close(); err != nil {
		t.Fatalf("Close v1: %v", err)
	}

	// Open with wrong passphrase — VerifyPassphrase should fail.
	v2, err := NewVaultBackend("wrong-pass", WithVaultPath(dbPath))
	if err != nil {
		t.Fatalf("NewVaultBackend v2: %v", err)
	}
	defer func() { _ = v2.Close() }()

	err = v2.VerifyPassphrase()
	if !errors.Is(err, ErrWrongPassphrase) {
		t.Fatalf("VerifyPassphrase (wrong): got %v, want ErrWrongPassphrase", err)
	}
}

func TestVaultBackend_VerifyPassphrase_NotInitialized(t *testing.T) {
	v := testVault(t)

	// VerifyPassphrase on uninitialized vault returns ErrVaultNotInitialized.
	err := v.VerifyPassphrase()
	if !errors.Is(err, ErrVaultNotInitialized) {
		t.Fatalf("VerifyPassphrase (uninitialized): got %v, want ErrVaultNotInitialized", err)
	}
}

func TestVaultBackend_InitializePersistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "vault.db")

	// Initialize, close, reopen — should still be initialized.
	v1, err := NewVaultBackend("pass123", WithVaultPath(dbPath))
	if err != nil {
		t.Fatalf("NewVaultBackend v1: %v", err)
	}
	if err := v1.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := v1.Close(); err != nil {
		t.Fatalf("Close v1: %v", err)
	}

	v2, err := NewVaultBackend("pass123", WithVaultPath(dbPath))
	if err != nil {
		t.Fatalf("NewVaultBackend v2: %v", err)
	}
	defer func() { _ = v2.Close() }()

	initialized, err := v2.IsInitialized()
	if err != nil {
		t.Fatalf("IsInitialized v2: %v", err)
	}
	if !initialized {
		t.Fatal("vault should remain initialized after close/reopen")
	}

	if err := v2.VerifyPassphrase(); err != nil {
		t.Fatalf("VerifyPassphrase v2: %v", err)
	}
}

func TestVaultBackend_SecretsWorkWithInitialization(t *testing.T) {
	v := testVault(t)

	// Initialize the vault.
	if err := v.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// Secrets should still work after initialization.
	if err := v.Set("api_key", "secret123"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := v.Get("api_key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if val != "secret123" {
		t.Fatalf("Get: got %q, want %q", val, "secret123")
	}

	keys, err := v.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("List: got %d keys, want 1", len(keys))
	}
}

func TestVaultBackend_ReopenAfterClose(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "vault.db")

	v, err := NewVaultBackend("pass", WithVaultPath(dbPath))
	if err != nil {
		t.Fatalf("NewVaultBackend: %v", err)
	}

	// Set, close, then set again (lazy reopen).
	if err := v.Set("k1", "v1"); err != nil {
		t.Fatalf("Set before close: %v", err)
	}
	if err := v.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Operations after close should lazily reopen the database.
	if err := v.Set("k2", "v2"); err != nil {
		t.Fatalf("Set after close: %v", err)
	}

	val, err := v.Get("k2")
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if val != "v2" {
		t.Fatalf("Get after reopen: got %q, want %q", val, "v2")
	}
}
