package backend

import (
	"encoding/json"
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

func TestVaultBackend_LockUnlock(t *testing.T) {
	v := testVault(t)

	// Initialize the vault (required for lock/unlock).
	if err := v.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// Store a secret while unlocked.
	if err := v.Set("api_key", "secret123"); err != nil {
		t.Fatalf("Set before lock: %v", err)
	}

	// Vault starts unlocked.
	locked, err := v.IsLocked()
	if err != nil {
		t.Fatalf("IsLocked: %v", err)
	}
	if locked {
		t.Fatal("vault should not be locked initially")
	}

	// Lock the vault.
	if err := v.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	locked, err = v.IsLocked()
	if err != nil {
		t.Fatalf("IsLocked after lock: %v", err)
	}
	if !locked {
		t.Fatal("vault should be locked after Lock()")
	}

	// All CRUD operations should fail when locked.
	_, err = v.Get("api_key")
	if !errors.Is(err, ErrVaultLocked) {
		t.Fatalf("Get on locked vault: got %v, want ErrVaultLocked", err)
	}

	err = v.Set("new_key", "value")
	if !errors.Is(err, ErrVaultLocked) {
		t.Fatalf("Set on locked vault: got %v, want ErrVaultLocked", err)
	}

	err = v.Delete("api_key")
	if !errors.Is(err, ErrVaultLocked) {
		t.Fatalf("Delete on locked vault: got %v, want ErrVaultLocked", err)
	}

	_, err = v.List()
	if !errors.Is(err, ErrVaultLocked) {
		t.Fatalf("List on locked vault: got %v, want ErrVaultLocked", err)
	}

	// Unlock the vault.
	if err := v.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	locked, err = v.IsLocked()
	if err != nil {
		t.Fatalf("IsLocked after unlock: %v", err)
	}
	if locked {
		t.Fatal("vault should not be locked after Unlock()")
	}

	// Operations should work again after unlock.
	val, err := v.Get("api_key")
	if err != nil {
		t.Fatalf("Get after unlock: %v", err)
	}
	if val != "secret123" {
		t.Fatalf("Get after unlock: got %q, want %q", val, "secret123")
	}
}

func TestVaultBackend_LockAlreadyLocked(t *testing.T) {
	v := testVault(t)
	if err := v.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	if err := v.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Second lock should fail.
	err := v.Lock()
	if err == nil {
		t.Fatal("double Lock should fail")
	}
}

func TestVaultBackend_UnlockNotLocked(t *testing.T) {
	v := testVault(t)
	if err := v.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// Unlocking when not locked should fail.
	err := v.Unlock()
	if err == nil {
		t.Fatal("Unlock when not locked should fail")
	}
}

func TestVaultBackend_LockNotInitialized(t *testing.T) {
	v := testVault(t)

	// Lock on uninitialized vault should fail.
	err := v.Lock()
	if err == nil {
		t.Fatal("Lock on uninitialized vault should fail")
	}
}

func TestVaultBackend_LockPersistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "vault.db")

	// Create, initialize, lock, close.
	v1, err := NewVaultBackend("pass123", WithVaultPath(dbPath))
	if err != nil {
		t.Fatalf("NewVaultBackend v1: %v", err)
	}
	if err := v1.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := v1.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if err := v1.Close(); err != nil {
		t.Fatalf("Close v1: %v", err)
	}

	// Reopen — lock state should persist.
	v2, err := NewVaultBackend("pass123", WithVaultPath(dbPath))
	if err != nil {
		t.Fatalf("NewVaultBackend v2: %v", err)
	}
	defer func() { _ = v2.Close() }()

	locked, err := v2.IsLocked()
	if err != nil {
		t.Fatalf("IsLocked v2: %v", err)
	}
	if !locked {
		t.Fatal("lock state should persist across close/reopen")
	}

	// Unlock should work after reopen.
	if err := v2.Unlock(); err != nil {
		t.Fatalf("Unlock v2: %v", err)
	}

	locked, err = v2.IsLocked()
	if err != nil {
		t.Fatalf("IsLocked after unlock v2: %v", err)
	}
	if locked {
		t.Fatal("vault should be unlocked after Unlock()")
	}
}

func TestVaultBackend_LockWrongPassphrase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "vault.db")

	// Initialize with correct passphrase.
	v1, err := NewVaultBackend("correct-pass", WithVaultPath(dbPath))
	if err != nil {
		t.Fatalf("NewVaultBackend v1: %v", err)
	}
	if err := v1.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := v1.Close(); err != nil {
		t.Fatalf("Close v1: %v", err)
	}

	// Try to lock with wrong passphrase.
	v2, err := NewVaultBackend("wrong-pass", WithVaultPath(dbPath))
	if err != nil {
		t.Fatalf("NewVaultBackend v2: %v", err)
	}
	defer func() { _ = v2.Close() }()

	err = v2.Lock()
	if err == nil {
		t.Fatal("Lock with wrong passphrase should fail")
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

func TestVaultBackend_Export(t *testing.T) {
	v := testVault(t)
	if err := v.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// Export empty vault.
	export, err := v.Export()
	if err != nil {
		t.Fatalf("Export empty: %v", err)
	}
	if len(export.Secrets) != 0 {
		t.Fatalf("Export empty: got %d secrets, want 0", len(export.Secrets))
	}
	if export.Version != exportVersion {
		t.Fatalf("Export version: got %d, want %d", export.Version, exportVersion)
	}
	if export.ExportedAt == "" {
		t.Fatal("Export ExportedAt should not be empty")
	}

	// Add secrets and export.
	secrets := map[string]string{
		"api_key":    "sk-secret-123",
		"db_pass":    "p@ssw0rd!",
		"unicode_val": "こんにちは",
	}
	for k, val := range secrets {
		if err := v.Set(k, val); err != nil {
			t.Fatalf("Set(%s): %v", k, err)
		}
	}

	export, err = v.Export()
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(export.Secrets) != len(secrets) {
		t.Fatalf("Export: got %d secrets, want %d", len(export.Secrets), len(secrets))
	}
	for k, want := range secrets {
		got, ok := export.Secrets[k]
		if !ok {
			t.Fatalf("Export missing key %q", k)
		}
		if got != want {
			t.Fatalf("Export[%q]: got %q, want %q", k, got, want)
		}
	}
}

func TestVaultBackend_ExportJSON(t *testing.T) {
	v := testVault(t)
	if err := v.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	if err := v.Set("key1", "value1"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	data, err := v.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}

	// Verify it's valid JSON.
	var export VaultExport
	if err := json.Unmarshal(data, &export); err != nil {
		t.Fatalf("unmarshal exported JSON: %v", err)
	}
	if export.Version != exportVersion {
		t.Fatalf("version: got %d, want %d", export.Version, exportVersion)
	}
	if val, ok := export.Secrets["key1"]; !ok || val != "value1" {
		t.Fatalf("secrets[key1]: got %q, want %q", val, "value1")
	}
}

func TestVaultBackend_Import(t *testing.T) {
	v := testVault(t)
	if err := v.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	export := &VaultExport{
		Version:    exportVersion,
		ExportedAt: "2025-01-01T00:00:00Z",
		Secrets: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}

	count, err := v.Import(export)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if count != 3 {
		t.Fatalf("Import count: got %d, want 3", count)
	}

	// Verify all keys were imported.
	for k, want := range export.Secrets {
		got, err := v.Get(k)
		if err != nil {
			t.Fatalf("Get(%s): %v", k, err)
		}
		if got != want {
			t.Fatalf("Get(%s): got %q, want %q", k, got, want)
		}
	}
}

func TestVaultBackend_ImportOverwrites(t *testing.T) {
	v := testVault(t)
	if err := v.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// Pre-set a key.
	if err := v.Set("key1", "original"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Import with the same key but different value.
	export := &VaultExport{
		Version:    exportVersion,
		ExportedAt: "2025-01-01T00:00:00Z",
		Secrets:    map[string]string{"key1": "imported"},
	}

	count, err := v.Import(export)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if count != 1 {
		t.Fatalf("Import count: got %d, want 1", count)
	}

	got, err := v.Get("key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "imported" {
		t.Fatalf("Get after import: got %q, want %q", got, "imported")
	}
}

func TestVaultBackend_ImportJSON(t *testing.T) {
	v := testVault(t)
	if err := v.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	jsonData := []byte(`{
		"version": 1,
		"exported_at": "2025-01-01T00:00:00Z",
		"secrets": {
			"api_key": "sk-123",
			"db_pass": "secret"
		}
	}`)

	count, err := v.ImportJSON(jsonData)
	if err != nil {
		t.Fatalf("ImportJSON: %v", err)
	}
	if count != 2 {
		t.Fatalf("ImportJSON count: got %d, want 2", count)
	}

	got, err := v.Get("api_key")
	if err != nil {
		t.Fatalf("Get(api_key): %v", err)
	}
	if got != "sk-123" {
		t.Fatalf("Get(api_key): got %q, want %q", got, "sk-123")
	}
}

func TestVaultBackend_ExportImportRoundtrip(t *testing.T) {
	dir := t.TempDir()

	// Create vault 1 with secrets.
	dbPath1 := filepath.Join(dir, "vault1.db")
	v1, err := NewVaultBackend("pass1", WithVaultPath(dbPath1))
	if err != nil {
		t.Fatalf("NewVaultBackend v1: %v", err)
	}
	if err := v1.Initialize(); err != nil {
		t.Fatalf("Initialize v1: %v", err)
	}

	secrets := map[string]string{
		"key1":    "value1",
		"key2":    "multi\nline\nvalue",
		"key3":    `{"json": "value"}`,
		"unicode": "日本語テスト",
	}
	for k, val := range secrets {
		if err := v1.Set(k, val); err != nil {
			t.Fatalf("Set(%s): %v", k, err)
		}
	}

	// Export from v1 as JSON.
	data, err := v1.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}
	_ = v1.Close()

	// Create vault 2 with different passphrase and import.
	dbPath2 := filepath.Join(dir, "vault2.db")
	v2, err := NewVaultBackend("different-pass", WithVaultPath(dbPath2))
	if err != nil {
		t.Fatalf("NewVaultBackend v2: %v", err)
	}
	defer func() { _ = v2.Close() }()
	if err := v2.Initialize(); err != nil {
		t.Fatalf("Initialize v2: %v", err)
	}

	count, err := v2.ImportJSON(data)
	if err != nil {
		t.Fatalf("ImportJSON: %v", err)
	}
	if count != len(secrets) {
		t.Fatalf("ImportJSON count: got %d, want %d", count, len(secrets))
	}

	// Verify all secrets match.
	for k, want := range secrets {
		got, err := v2.Get(k)
		if err != nil {
			t.Fatalf("Get(%s) from v2: %v", k, err)
		}
		if got != want {
			t.Fatalf("Get(%s) from v2: got %q, want %q", k, got, want)
		}
	}
}

func TestVaultBackend_ExportLocked(t *testing.T) {
	v := testVault(t)
	if err := v.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := v.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	_, err := v.Export()
	if !errors.Is(err, ErrVaultLocked) {
		t.Fatalf("Export on locked vault: got %v, want ErrVaultLocked", err)
	}
}

func TestVaultBackend_ImportLocked(t *testing.T) {
	v := testVault(t)
	if err := v.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if err := v.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	export := &VaultExport{
		Version:    exportVersion,
		ExportedAt: "2025-01-01T00:00:00Z",
		Secrets:    map[string]string{"k": "v"},
	}
	_, err := v.Import(export)
	if !errors.Is(err, ErrVaultLocked) {
		t.Fatalf("Import on locked vault: got %v, want ErrVaultLocked", err)
	}
}

func TestVaultBackend_ImportNil(t *testing.T) {
	v := testVault(t)
	if err := v.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	_, err := v.Import(nil)
	if err == nil {
		t.Fatal("Import nil should fail")
	}
}

func TestVaultBackend_ImportBadVersion(t *testing.T) {
	v := testVault(t)
	if err := v.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	export := &VaultExport{
		Version:    999,
		ExportedAt: "2025-01-01T00:00:00Z",
		Secrets:    map[string]string{"k": "v"},
	}
	_, err := v.Import(export)
	if err == nil {
		t.Fatal("Import with bad version should fail")
	}
}

func TestVaultBackend_ImportJSON_InvalidJSON(t *testing.T) {
	v := testVault(t)
	if err := v.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	_, err := v.ImportJSON([]byte("not json"))
	if err == nil {
		t.Fatal("ImportJSON with invalid JSON should fail")
	}
}

func TestVaultBackend_ExportNotInitialized(t *testing.T) {
	v := testVault(t)

	// Export on uninitialized vault should fail (passphrase verification fails).
	_, err := v.Export()
	if err == nil {
		t.Fatal("Export on uninitialized vault should fail")
	}
}

func TestVaultBackend_ImportEmptySecrets(t *testing.T) {
	v := testVault(t)
	if err := v.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	export := &VaultExport{
		Version:    exportVersion,
		ExportedAt: "2025-01-01T00:00:00Z",
		Secrets:    map[string]string{},
	}

	count, err := v.Import(export)
	if err != nil {
		t.Fatalf("Import empty: %v", err)
	}
	if count != 0 {
		t.Fatalf("Import empty count: got %d, want 0", count)
	}
}
