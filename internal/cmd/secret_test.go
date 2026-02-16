package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// writeTestConfig writes a .envref.yaml with a keychain backend to the given
// directory and returns the path.
func writeTestConfig(t *testing.T, dir, project string) string {
	t.Helper()
	content := "project: " + project + "\nbackends:\n  - name: keychain\n    type: keychain\n"
	path := filepath.Join(dir, ".envref.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return path
}

func TestSecretSetCmd_WithValueFlag(t *testing.T) {
	// This test validates the command structure and argument parsing.
	// It will fail at the backend level since we don't have a real keychain,
	// but we can verify the command wiring is correct.
	dir := t.TempDir()
	writeTestConfig(t, dir, "testproject")

	// Change to the temp dir so config discovery works.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(errBuf)
	root.SetArgs([]string{"secret", "set", "API_KEY", "--value", "sk-test-123"})

	// The command will likely fail due to keychain not being available in CI,
	// but it should get past argument parsing and config loading.
	err = root.Execute()
	if err != nil {
		// Expected in CI where keychain is not available.
		// Verify it's a backend error, not a command structure error.
		errMsg := err.Error()
		if contains(errMsg, "accepts 1 arg") || contains(errMsg, "unknown command") {
			t.Fatalf("command structure error: %v", err)
		}
	} else {
		// If it succeeds (keychain available), verify output.
		got := buf.String()
		if got != "secret \"API_KEY\" stored in backend \"keychain\"\n" {
			t.Errorf("output: got %q, want %q", got, "secret \"API_KEY\" stored in backend \"keychain\"\n")
		}
	}
}

func TestSecretSetCmd_PromptFromStdin(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "testproject")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(errBuf)
	root.SetIn(bytes.NewBufferString("my-secret-value\n"))
	root.SetArgs([]string{"secret", "set", "DB_PASS"})

	err = root.Execute()
	if err != nil {
		errMsg := err.Error()
		if contains(errMsg, "accepts 1 arg") || contains(errMsg, "unknown command") {
			t.Fatalf("command structure error: %v", err)
		}
		// Verify the prompt was written to stderr.
		if !contains(errBuf.String(), "Enter value for DB_PASS") {
			t.Errorf("expected prompt in stderr, got %q", errBuf.String())
		}
	} else {
		got := buf.String()
		if got != "secret \"DB_PASS\" stored in backend \"keychain\"\n" {
			t.Errorf("output: got %q, want %q", got, "secret \"DB_PASS\" stored in backend \"keychain\"\n")
		}
		// Verify prompt was written to stderr.
		if !contains(errBuf.String(), "Enter value for DB_PASS") {
			t.Errorf("expected prompt in stderr, got %q", errBuf.String())
		}
	}
}

func TestSecretSetCmd_NoArgs(t *testing.T) {
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "set"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing argument, got nil")
	}
}

func TestSecretSetCmd_NoConfig(t *testing.T) {
	dir := t.TempDir()
	// No .envref.yaml in this directory.

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "set", "API_KEY", "--value", "test"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error when no config found, got nil")
	}
	if !contains(err.Error(), "loading config") {
		t.Errorf("expected config loading error, got: %v", err)
	}
}

func TestSecretSetCmd_NoBackends(t *testing.T) {
	dir := t.TempDir()
	// Config with no backends.
	content := "project: testproject\n"
	if err := os.WriteFile(filepath.Join(dir, ".envref.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "set", "API_KEY", "--value", "test"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error when no backends configured, got nil")
	}
	if !contains(err.Error(), "no backends configured") {
		t.Errorf("expected 'no backends configured' error, got: %v", err)
	}
}

func TestSecretSetCmd_InvalidBackend(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "testproject")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "set", "API_KEY", "--value", "test", "--backend", "nonexistent"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent backend, got nil")
	}
	if !contains(err.Error(), "nonexistent") {
		t.Errorf("expected error mentioning backend name, got: %v", err)
	}
}

func TestSecretSetCmd_EmptyStdinPrompt(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "testproject")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetIn(bytes.NewBufferString("\n"))
	root.SetArgs([]string{"secret", "set", "API_KEY"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for empty input, got nil")
	}
	if !contains(err.Error(), "secret value must not be empty") {
		t.Errorf("expected empty value error, got: %v", err)
	}
}

func TestSecretSetCmd_EmptyStdin(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "testproject")

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetIn(bytes.NewBufferString(""))
	root.SetArgs([]string{"secret", "set", "API_KEY"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for empty stdin, got nil")
	}
	if !contains(err.Error(), "no input provided") {
		t.Errorf("expected 'no input provided' error, got: %v", err)
	}
}

// contains is a simple helper to check for substring presence.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
