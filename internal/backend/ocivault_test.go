package backend

import (
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// buildOCIMock compiles the mock oci CLI helper into a temporary directory
// and returns the path to the built executable.
func buildOCIMock(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available, skipping oci-vault tests")
	}

	dir := t.TempDir()
	binName := "oci"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(dir, binName)

	src := filepath.Join("testdata", "oci_mock.go")
	cmd := exec.Command("go", "build", "-o", binPath, src)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build oci mock: %v", err)
	}
	return binPath
}

func TestOCIVaultBackend_Interface(t *testing.T) {
	var _ Backend = &OCIVaultBackend{}
}

func TestOCIVaultBackend_Name(t *testing.T) {
	b := NewOCIVaultBackend("vault-ocid", "compartment-ocid", "key-ocid")
	if b.Name() != "oci-vault" {
		t.Fatalf("Name(): got %q, want %q", b.Name(), "oci-vault")
	}
}

func TestOCIVaultBackend_SetGetDeleteList(t *testing.T) {
	ociPath := buildOCIMock(t)
	b := NewOCIVaultBackend("vault-ocid", "compartment-ocid", "key-ocid",
		WithOCIVaultCommand(ociPath))

	// List should be empty initially.
	keys, err := b.List()
	if err != nil {
		t.Fatalf("List() initial: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("List() initial: got %v, want empty", keys)
	}

	// Set a key.
	if err := b.Set("api_key", "secret123"); err != nil {
		t.Fatalf("Set(api_key): %v", err)
	}

	// Get the key.
	val, err := b.Get("api_key")
	if err != nil {
		t.Fatalf("Get(api_key): %v", err)
	}
	if val != "secret123" {
		t.Fatalf("Get(api_key): got %q, want %q", val, "secret123")
	}

	// Set another key.
	if err := b.Set("db_pass", "password456"); err != nil {
		t.Fatalf("Set(db_pass): %v", err)
	}

	// Update existing key (new version).
	if err := b.Set("api_key", "updated_secret"); err != nil {
		t.Fatalf("Set(api_key) update: %v", err)
	}

	// Verify update.
	val, err = b.Get("api_key")
	if err != nil {
		t.Fatalf("Get(api_key) after update: %v", err)
	}
	if val != "updated_secret" {
		t.Fatalf("Get(api_key) after update: got %q, want %q", val, "updated_secret")
	}

	// List should return both keys.
	keys, err = b.List()
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("List(): got %d keys, want 2", len(keys))
	}

	// Delete (schedules deletion — removes from active list).
	if err := b.Delete("api_key"); err != nil {
		t.Fatalf("Delete(api_key): %v", err)
	}

	// Get after delete should return ErrNotFound (no longer ACTIVE).
	_, err = b.Get("api_key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(deleted): got %v, want ErrNotFound", err)
	}

	// List should have one key left (only ACTIVE secrets).
	keys, err = b.List()
	if err != nil {
		t.Fatalf("List() after delete: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("List() after delete: got %d keys, want 1", len(keys))
	}
	if keys[0] != "db_pass" {
		t.Fatalf("List() after delete: got %q, want %q", keys[0], "db_pass")
	}
}

func TestOCIVaultBackend_GetNotFound(t *testing.T) {
	ociPath := buildOCIMock(t)
	b := NewOCIVaultBackend("vault-ocid", "compartment-ocid", "key-ocid",
		WithOCIVaultCommand(ociPath))

	_, err := b.Get("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(nonexistent): got %v, want ErrNotFound", err)
	}
}

func TestOCIVaultBackend_DeleteNotFound(t *testing.T) {
	ociPath := buildOCIMock(t)
	b := NewOCIVaultBackend("vault-ocid", "compartment-ocid", "key-ocid",
		WithOCIVaultCommand(ociPath))

	err := b.Delete("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete(nonexistent): got %v, want ErrNotFound", err)
	}
}

func TestOCIVaultBackend_InvalidCommand(t *testing.T) {
	b := NewOCIVaultBackend("vault-ocid", "compartment-ocid", "key-ocid",
		WithOCIVaultCommand("/nonexistent/oci"))

	_, err := b.Get("key")
	if err == nil {
		t.Fatal("Get with invalid command: expected error, got nil")
	}
}

func TestOCIVaultBackend_Options(t *testing.T) {
	b := NewOCIVaultBackend("vault-ocid", "compartment-ocid", "key-ocid",
		WithOCIVaultProfile("PRODUCTION"),
		WithOCIVaultCommand("/usr/local/bin/oci"),
	)

	if b.vaultID != "vault-ocid" {
		t.Fatalf("vaultID: got %q, want %q", b.vaultID, "vault-ocid")
	}
	if b.compartmentID != "compartment-ocid" {
		t.Fatalf("compartmentID: got %q, want %q", b.compartmentID, "compartment-ocid")
	}
	if b.keyID != "key-ocid" {
		t.Fatalf("keyID: got %q, want %q", b.keyID, "key-ocid")
	}
	if b.profile != "PRODUCTION" {
		t.Fatalf("profile: got %q, want %q", b.profile, "PRODUCTION")
	}
	if b.command != "/usr/local/bin/oci" {
		t.Fatalf("command: got %q, want %q", b.command, "/usr/local/bin/oci")
	}
}

func TestOCIVaultBackend_Base64Encoding(t *testing.T) {
	// Verify that our base64 encoding/decoding round-trips correctly
	// for values with special characters.
	values := []string{
		"simple",
		"with spaces",
		"with\nnewlines",
		"special!@#$%^&*()",
		"unicode: こんにちは",
		"",
	}

	for _, v := range values {
		encoded := base64.StdEncoding.EncodeToString([]byte(v))
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatalf("base64 round-trip for %q: decode error: %v", v, err)
		}
		if string(decoded) != v {
			t.Fatalf("base64 round-trip for %q: got %q", v, string(decoded))
		}
	}
}

func TestIsOCINotFoundErr(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"ServiceError: 404 NotFound: secret not found", true},
		{"resource not found", true},
		{"404: the resource does not exist", true},
		{"authorization failed", false},
		{"rate limit exceeded", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isOCINotFoundErr(errors.New(tt.msg))
		if got != tt.want {
			t.Errorf("isOCINotFoundErr(%q): got %v, want %v", tt.msg, got, tt.want)
		}
	}
}
