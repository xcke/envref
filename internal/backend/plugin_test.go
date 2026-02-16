package backend

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// buildTestPlugin compiles the test plugin helper into a temporary directory
// and returns the path to the built executable. The test is skipped if the
// go toolchain is not available.
func buildTestPlugin(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available, skipping plugin tests")
	}

	dir := t.TempDir()
	binName := "envref-backend-test"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(dir, binName)

	src := filepath.Join("testdata", "plugin_helper.go")
	cmd := exec.Command("go", "build", "-o", binPath, src)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build test plugin: %v", err)
	}
	return binPath
}

func TestPluginBackend_Interface(t *testing.T) {
	var _ Backend = &PluginBackend{}
}

func TestPluginBackend_Name(t *testing.T) {
	p := NewPluginBackend("test-plugin", "/usr/bin/fake")
	if p.Name() != "test-plugin" {
		t.Fatalf("Name(): got %q, want %q", p.Name(), "test-plugin")
	}
}

func TestPluginBackend_SetGetDeleteList(t *testing.T) {
	binPath := buildTestPlugin(t)
	p := NewPluginBackend("test", binPath)

	// List should be empty initially.
	keys, err := p.List()
	if err != nil {
		t.Fatalf("List() initial: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("List() initial: got %v, want empty", keys)
	}

	// Set a key.
	if err := p.Set("api_key", "secret123"); err != nil {
		t.Fatalf("Set(api_key): %v", err)
	}

	// Get the key.
	val, err := p.Get("api_key")
	if err != nil {
		t.Fatalf("Get(api_key): %v", err)
	}
	if val != "secret123" {
		t.Fatalf("Get(api_key): got %q, want %q", val, "secret123")
	}

	// Set another key.
	if err := p.Set("db_pass", "password456"); err != nil {
		t.Fatalf("Set(db_pass): %v", err)
	}

	// List should return both keys.
	keys, err = p.List()
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("List(): got %d keys, want 2", len(keys))
	}

	// Delete.
	if err := p.Delete("api_key"); err != nil {
		t.Fatalf("Delete(api_key): %v", err)
	}

	// Get after delete should return ErrNotFound.
	_, err = p.Get("api_key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(deleted): got %v, want ErrNotFound", err)
	}

	// List should have one key left.
	keys, err = p.List()
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

func TestPluginBackend_GetNotFound(t *testing.T) {
	binPath := buildTestPlugin(t)
	p := NewPluginBackend("test", binPath)

	_, err := p.Get("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(nonexistent): got %v, want ErrNotFound", err)
	}
}

func TestPluginBackend_DeleteNotFound(t *testing.T) {
	binPath := buildTestPlugin(t)
	p := NewPluginBackend("test", binPath)

	err := p.Delete("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete(nonexistent): got %v, want ErrNotFound", err)
	}
}

func TestPluginBackend_InvalidCommand(t *testing.T) {
	p := NewPluginBackend("bad", "/nonexistent/binary")

	_, err := p.Get("key")
	if err == nil {
		t.Fatal("Get with invalid command: expected error, got nil")
	}
}

func TestPluginBackend_InvalidJSON(t *testing.T) {
	// Create a script that outputs invalid JSON.
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "bad-plugin")

	script := "#!/bin/sh\necho 'not json'"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	p := NewPluginBackend("bad-json", scriptPath)
	_, err := p.Get("key")
	if err == nil {
		t.Fatal("Get with invalid JSON: expected error, got nil")
	}
}

func TestPluginBackend_PluginError(t *testing.T) {
	// Create a script that returns an error response.
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "error-plugin")

	resp := pluginResponse{Error: "permission denied"}
	respBytes, _ := json.Marshal(resp)
	script := "#!/bin/sh\necho '" + string(respBytes) + "'"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	p := NewPluginBackend("error-plugin", scriptPath)
	_, err := p.Get("key")
	if err == nil {
		t.Fatal("Get with plugin error: expected error, got nil")
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatal("expected non-ErrNotFound error")
	}
}

func TestPluginBackend_NonZeroExit(t *testing.T) {
	// Create a script that exits with non-zero and writes to stderr.
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "exit-plugin")

	script := "#!/bin/sh\necho 'backend crashed' >&2\nexit 1"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	p := NewPluginBackend("exit-plugin", scriptPath)
	_, err := p.Get("key")
	if err == nil {
		t.Fatal("Get with exit 1: expected error, got nil")
	}
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"not found", true},
		{"Not Found", true},
		{"secret not found", true},
		{"key not found", true},
		{"key 'foo' not found in store", true},
		{"permission denied", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isNotFoundError(tt.msg)
		if got != tt.want {
			t.Errorf("isNotFoundError(%q): got %v, want %v", tt.msg, got, tt.want)
		}
	}
}

func TestDiscoverPlugin_NotFound(t *testing.T) {
	_, err := DiscoverPlugin("definitely-does-not-exist-xyz")
	if err == nil {
		t.Fatal("DiscoverPlugin: expected error for nonexistent plugin, got nil")
	}
}
