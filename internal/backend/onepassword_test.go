package backend

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// buildOpMock compiles the mock op CLI helper into a temporary directory
// and returns the path to the built executable.
func buildOpMock(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available, skipping 1password tests")
	}

	dir := t.TempDir()
	binName := "op"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(dir, binName)

	src := filepath.Join("testdata", "op_mock.go")
	cmd := exec.Command("go", "build", "-o", binPath, src)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build op mock: %v", err)
	}
	return binPath
}

func TestOnePasswordBackend_Interface(t *testing.T) {
	var _ Backend = &OnePasswordBackend{}
}

func TestOnePasswordBackend_Name(t *testing.T) {
	b := NewOnePasswordBackend("Personal")
	if b.Name() != "1password" {
		t.Fatalf("Name(): got %q, want %q", b.Name(), "1password")
	}
}

func TestOnePasswordBackend_SetGetDeleteList(t *testing.T) {
	opPath := buildOpMock(t)
	b := NewOnePasswordBackend("TestVault", WithOnePasswordCommand(opPath))

	// List should be empty initially.
	keys, err := b.List()
	if err != nil {
		t.Fatalf("List() initial: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("List() initial: got %v, want empty", keys)
	}

	// Set a key (creates new item).
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

	// Update existing key.
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

func TestOnePasswordBackend_GetNotFound(t *testing.T) {
	opPath := buildOpMock(t)
	b := NewOnePasswordBackend("TestVault", WithOnePasswordCommand(opPath))

	_, err := b.Get("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(nonexistent): got %v, want ErrNotFound", err)
	}
}

func TestOnePasswordBackend_DeleteNotFound(t *testing.T) {
	opPath := buildOpMock(t)
	b := NewOnePasswordBackend("TestVault", WithOnePasswordCommand(opPath))

	err := b.Delete("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete(nonexistent): got %v, want ErrNotFound", err)
	}
}

func TestOnePasswordBackend_InvalidCommand(t *testing.T) {
	b := NewOnePasswordBackend("TestVault", WithOnePasswordCommand("/nonexistent/op"))

	_, err := b.Get("key")
	if err == nil {
		t.Fatal("Get with invalid command: expected error, got nil")
	}
}

func TestOnePasswordBackend_WithAccount(t *testing.T) {
	b := NewOnePasswordBackend("Personal", WithOnePasswordAccount("my.1password.com"))
	if b.account != "my.1password.com" {
		t.Fatalf("account: got %q, want %q", b.account, "my.1password.com")
	}
}

func TestOnePasswordBackend_Options(t *testing.T) {
	b := NewOnePasswordBackend("MyVault",
		WithOnePasswordAccount("team.1password.com"),
		WithOnePasswordCommand("/usr/local/bin/op"),
	)

	if b.vault != "MyVault" {
		t.Fatalf("vault: got %q, want %q", b.vault, "MyVault")
	}
	if b.account != "team.1password.com" {
		t.Fatalf("account: got %q, want %q", b.account, "team.1password.com")
	}
	if b.command != "/usr/local/bin/op" {
		t.Fatalf("command: got %q, want %q", b.command, "/usr/local/bin/op")
	}
}

func TestIsOpNotFoundErr(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{`"mykey" isn't an item in the "Personal" vault`, true},
		{`could not find item "mykey"`, true},
		{`no item found with title "mykey"`, true},
		{`item not found`, true},
		{"permission denied", false},
		{"authentication required", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isOpNotFoundErr(errors.New(tt.msg))
		if got != tt.want {
			t.Errorf("isOpNotFoundErr(%q): got %v, want %v", tt.msg, got, tt.want)
		}
	}
}
