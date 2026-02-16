package backend

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// buildVaultMock compiles the mock vault CLI helper into a temporary directory
// and returns the path to the built executable.
func buildVaultMock(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available, skipping hashicorp-vault tests")
	}

	dir := t.TempDir()
	binName := "vault"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(dir, binName)

	src := filepath.Join("testdata", "vault_mock.go")
	cmd := exec.Command("go", "build", "-o", binPath, src)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build vault mock: %v", err)
	}
	return binPath
}

func TestHashiVaultBackend_Interface(t *testing.T) {
	var _ Backend = &HashiVaultBackend{}
}

func TestHashiVaultBackend_Name(t *testing.T) {
	b := NewHashiVaultBackend("secret", "envref")
	if b.Name() != "hashicorp-vault" {
		t.Fatalf("Name(): got %q, want %q", b.Name(), "hashicorp-vault")
	}
}

func TestHashiVaultBackend_SetGetDeleteList(t *testing.T) {
	vaultPath := buildVaultMock(t)
	b := NewHashiVaultBackend("secret", "test", WithHashiVaultCommand(vaultPath))

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

	// Update existing key (overwrite).
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

	// Delete.
	if err := b.Delete("api_key"); err != nil {
		t.Fatalf("Delete(api_key): %v", err)
	}

	// Get after delete should return ErrNotFound.
	_, err = b.Get("api_key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(deleted): got %v, want ErrNotFound", err)
	}

	// List should have one key left.
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

func TestHashiVaultBackend_GetNotFound(t *testing.T) {
	vaultPath := buildVaultMock(t)
	b := NewHashiVaultBackend("secret", "test", WithHashiVaultCommand(vaultPath))

	_, err := b.Get("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(nonexistent): got %v, want ErrNotFound", err)
	}
}

func TestHashiVaultBackend_DeleteNotFound(t *testing.T) {
	vaultPath := buildVaultMock(t)
	b := NewHashiVaultBackend("secret", "test", WithHashiVaultCommand(vaultPath))

	err := b.Delete("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete(nonexistent): got %v, want ErrNotFound", err)
	}
}

func TestHashiVaultBackend_InvalidCommand(t *testing.T) {
	b := NewHashiVaultBackend("secret", "test", WithHashiVaultCommand("/nonexistent/vault"))

	_, err := b.Get("key")
	if err == nil {
		t.Fatal("Get with invalid command: expected error, got nil")
	}
}

func TestHashiVaultBackend_Options(t *testing.T) {
	b := NewHashiVaultBackend("kv", "myapp/prod",
		WithHashiVaultAddr("https://vault.example.com:8200"),
		WithHashiVaultNamespace("admin"),
		WithHashiVaultToken("s.mytesttoken"),
		WithHashiVaultCommand("/usr/local/bin/vault"),
	)

	if b.mount != "kv" {
		t.Fatalf("mount: got %q, want %q", b.mount, "kv")
	}
	if b.prefix != "myapp/prod" {
		t.Fatalf("prefix: got %q, want %q", b.prefix, "myapp/prod")
	}
	if b.addr != "https://vault.example.com:8200" {
		t.Fatalf("addr: got %q, want %q", b.addr, "https://vault.example.com:8200")
	}
	if b.namespace != "admin" {
		t.Fatalf("namespace: got %q, want %q", b.namespace, "admin")
	}
	if b.token != "s.mytesttoken" {
		t.Fatalf("token: got %q, want %q", b.token, "s.mytesttoken")
	}
	if b.command != "/usr/local/bin/vault" {
		t.Fatalf("command: got %q, want %q", b.command, "/usr/local/bin/vault")
	}
}

func TestHashiVaultBackend_SecretPath(t *testing.T) {
	tests := []struct {
		prefix string
		key    string
		want   string
	}{
		{"envref", "api_key", "envref/api_key"},
		{"myapp/prod", "db_pass", "myapp/prod/db_pass"},
		{"", "api_key", "api_key"},
	}
	for _, tt := range tests {
		b := NewHashiVaultBackend("secret", tt.prefix)
		got := b.secretPath(tt.key)
		if got != tt.want {
			t.Errorf("secretPath(%q) with prefix %q: got %q, want %q", tt.key, tt.prefix, got, tt.want)
		}
	}
}

func TestIsHashiVaultNotFoundErr(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"No value found at secret/data/envref/api_key", true},
		{"no secrets at this path", true},
		{"not found", true},
		{"invalid path", true},
		{"permission denied", false},
		{"connection refused", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isHashiVaultNotFoundErr(errors.New(tt.msg))
		if got != tt.want {
			t.Errorf("isHashiVaultNotFoundErr(%q): got %v, want %v", tt.msg, got, tt.want)
		}
	}
}
