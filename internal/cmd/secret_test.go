package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
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

func TestSecretDeleteCmd_WithForceFlag(t *testing.T) {
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
	root.SetArgs([]string{"secret", "delete", "API_KEY", "--force"})

	err = root.Execute()
	if err != nil {
		// Expected in CI where keychain is not available.
		// Verify it's a backend error, not a command structure error.
		errMsg := err.Error()
		if contains(errMsg, "accepts 1 arg") || contains(errMsg, "unknown command") {
			t.Fatalf("command structure error: %v", err)
		}
	} else {
		got := buf.String()
		if got != "secret \"API_KEY\" deleted from backend \"keychain\"\n" {
			t.Errorf("output: got %q, want %q", got, "secret \"API_KEY\" deleted from backend \"keychain\"\n")
		}
	}
}

func TestSecretDeleteCmd_ConfirmYes(t *testing.T) {
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
	root.SetIn(bytes.NewBufferString("y\n"))
	root.SetArgs([]string{"secret", "delete", "API_KEY"})

	err = root.Execute()
	if err != nil {
		errMsg := err.Error()
		if contains(errMsg, "accepts 1 arg") || contains(errMsg, "unknown command") {
			t.Fatalf("command structure error: %v", err)
		}
	} else {
		got := buf.String()
		if got != "secret \"API_KEY\" deleted from backend \"keychain\"\n" {
			t.Errorf("output: got %q, want %q", got, "secret \"API_KEY\" deleted from backend \"keychain\"\n")
		}
	}

	// Verify prompt was written to stderr.
	if !contains(errBuf.String(), "Delete secret") {
		t.Errorf("expected confirmation prompt in stderr, got %q", errBuf.String())
	}
}

func TestSecretDeleteCmd_ConfirmNo(t *testing.T) {
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
	root.SetIn(bytes.NewBufferString("n\n"))
	root.SetArgs([]string{"secret", "delete", "API_KEY"})

	err = root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not have deleted — no output on stdout.
	if buf.String() != "" {
		t.Errorf("expected no stdout output for cancelled deletion, got %q", buf.String())
	}

	// Stderr should contain cancellation message.
	if !contains(errBuf.String(), "deletion cancelled") {
		t.Errorf("expected cancellation message in stderr, got %q", errBuf.String())
	}
}

func TestSecretDeleteCmd_NoArgs(t *testing.T) {
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "delete"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing argument, got nil")
	}
}

func TestSecretDeleteCmd_NoConfig(t *testing.T) {
	dir := t.TempDir()

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
	root.SetArgs([]string{"secret", "delete", "API_KEY", "--force"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error when no config found, got nil")
	}
	if !contains(err.Error(), "loading config") {
		t.Errorf("expected config loading error, got: %v", err)
	}
}

func TestSecretDeleteCmd_NoBackends(t *testing.T) {
	dir := t.TempDir()
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
	root.SetArgs([]string{"secret", "delete", "API_KEY", "--force"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error when no backends configured, got nil")
	}
	if !contains(err.Error(), "no backends configured") {
		t.Errorf("expected 'no backends configured' error, got: %v", err)
	}
}

func TestSecretDeleteCmd_InvalidBackend(t *testing.T) {
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
	root.SetArgs([]string{"secret", "delete", "API_KEY", "--force", "--backend", "nonexistent"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent backend, got nil")
	}
	if !contains(err.Error(), "nonexistent") {
		t.Errorf("expected error mentioning backend name, got: %v", err)
	}
}

func TestSecretListCmd_Success(t *testing.T) {
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
	root.SetArgs([]string{"secret", "list"})

	err = root.Execute()
	if err != nil {
		// Expected in CI where keychain is not available.
		errMsg := err.Error()
		if contains(errMsg, "unknown command") {
			t.Fatalf("command structure error: %v", err)
		}
	}
	// If the backend is available and empty, stderr should show "no secrets found".
	// If the backend is unavailable, the error was already checked above.
}

func TestSecretListCmd_NoConfig(t *testing.T) {
	dir := t.TempDir()

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
	root.SetArgs([]string{"secret", "list"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error when no config found, got nil")
	}
	if !contains(err.Error(), "loading config") {
		t.Errorf("expected config loading error, got: %v", err)
	}
}

func TestSecretListCmd_NoBackends(t *testing.T) {
	dir := t.TempDir()
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
	root.SetArgs([]string{"secret", "list"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error when no backends configured, got nil")
	}
	if !contains(err.Error(), "no backends configured") {
		t.Errorf("expected 'no backends configured' error, got: %v", err)
	}
}

func TestSecretListCmd_InvalidBackend(t *testing.T) {
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
	root.SetArgs([]string{"secret", "list", "--backend", "nonexistent"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent backend, got nil")
	}
	if !contains(err.Error(), "nonexistent") {
		t.Errorf("expected error mentioning backend name, got: %v", err)
	}
}

func TestSecretListCmd_RejectsArguments(t *testing.T) {
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "list", "extra-arg"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for extra arguments, got nil")
	}
}

// --- Tests for secret generate ---

func TestGenerateSecret_Alphanumeric(t *testing.T) {
	t.Run("default length", func(t *testing.T) {
		val, err := generateSecret(32, "alphanumeric")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(val) != 32 {
			t.Errorf("length: got %d, want 32", len(val))
		}
		for _, c := range val {
			if !strings.ContainsRune(charsetAlphanumeric, c) {
				t.Errorf("unexpected character %q in alphanumeric output", c)
			}
		}
	})

	t.Run("length 1", func(t *testing.T) {
		val, err := generateSecret(1, "alphanumeric")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(val) != 1 {
			t.Errorf("length: got %d, want 1", len(val))
		}
	})

	t.Run("length 128", func(t *testing.T) {
		val, err := generateSecret(128, "alphanumeric")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(val) != 128 {
			t.Errorf("length: got %d, want 128", len(val))
		}
	})
}

func TestGenerateSecret_Hex(t *testing.T) {
	tests := []int{1, 16, 32, 64}
	for _, length := range tests {
		val, err := generateSecret(length, "hex")
		if err != nil {
			t.Fatalf("length %d: unexpected error: %v", length, err)
		}
		if len(val) != length {
			t.Errorf("length %d: got %d chars", length, len(val))
		}
		// Verify it's valid hex.
		_, err = hex.DecodeString(val)
		if length%2 != 0 {
			// Odd length needs padding for decode verification.
			_, err = hex.DecodeString(val + "0")
		}
		if err != nil {
			t.Errorf("length %d: not valid hex: %v", length, err)
		}
	}
}

func TestGenerateSecret_Base64(t *testing.T) {
	tests := []int{4, 16, 32, 64}
	for _, length := range tests {
		val, err := generateSecret(length, "base64")
		if err != nil {
			t.Fatalf("length %d: unexpected error: %v", length, err)
		}
		if len(val) != length {
			t.Errorf("length %d: got %d chars", length, len(val))
		}
		// Pad to multiple of 4 for base64 decode check.
		padded := val
		for len(padded)%4 != 0 {
			padded += "="
		}
		_, err = base64.StdEncoding.DecodeString(padded)
		if err != nil {
			t.Errorf("length %d: not valid base64: %v (value: %q)", length, err, val)
		}
	}
}

func TestGenerateSecret_ASCII(t *testing.T) {
	val, err := generateSecret(64, "ascii")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(val) != 64 {
		t.Errorf("length: got %d, want 64", len(val))
	}
	for _, c := range val {
		if !strings.ContainsRune(charsetASCII, c) {
			t.Errorf("unexpected character %q in ascii output", c)
		}
	}
}

func TestGenerateSecret_UnknownCharset(t *testing.T) {
	_, err := generateSecret(32, "unknown")
	if err == nil {
		t.Fatal("expected error for unknown charset, got nil")
	}
	if !contains(err.Error(), "unknown charset") {
		t.Errorf("expected 'unknown charset' error, got: %v", err)
	}
}

func TestGenerateSecret_Uniqueness(t *testing.T) {
	// Generate multiple secrets and verify they are unique.
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		val, err := generateSecret(32, "alphanumeric")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if seen[val] {
			t.Fatalf("duplicate secret generated on iteration %d: %q", i, val)
		}
		seen[val] = true
	}
}

func TestSecretGenerateCmd_Success(t *testing.T) {
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
	root.SetArgs([]string{"secret", "generate", "API_KEY"})

	err = root.Execute()
	if err != nil {
		// Expected in CI where keychain is not available.
		errMsg := err.Error()
		if contains(errMsg, "accepts 1 arg") || contains(errMsg, "unknown command") {
			t.Fatalf("command structure error: %v", err)
		}
	} else {
		// If it succeeds, stdout should be empty (no --print) and stderr should have confirmation.
		if buf.String() != "" {
			t.Errorf("expected no stdout without --print, got %q", buf.String())
		}
		if !contains(errBuf.String(), "generated and stored") {
			t.Errorf("expected confirmation in stderr, got %q", errBuf.String())
		}
	}
}

func TestSecretGenerateCmd_WithPrint(t *testing.T) {
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
	root.SetArgs([]string{"secret", "generate", "API_KEY", "--print", "--length", "16", "--charset", "hex"})

	err = root.Execute()
	if err != nil {
		errMsg := err.Error()
		if contains(errMsg, "accepts 1 arg") || contains(errMsg, "unknown command") {
			t.Fatalf("command structure error: %v", err)
		}
	} else {
		// With --print, stdout should have the generated value.
		output := strings.TrimSpace(buf.String())
		if len(output) != 16 {
			t.Errorf("expected 16 char output, got %d: %q", len(output), output)
		}
		if !contains(errBuf.String(), "hex") {
			t.Errorf("expected charset info in stderr, got %q", errBuf.String())
		}
	}
}

func TestSecretGenerateCmd_NoArgs(t *testing.T) {
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "generate"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing argument, got nil")
	}
}

func TestSecretGenerateCmd_InvalidLength(t *testing.T) {
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
	root.SetArgs([]string{"secret", "generate", "API_KEY", "--length", "0"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for length 0, got nil")
	}
	if !contains(err.Error(), "length must be at least 1") {
		t.Errorf("expected length validation error, got: %v", err)
	}
}

func TestSecretGenerateCmd_LengthTooLarge(t *testing.T) {
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
	root.SetArgs([]string{"secret", "generate", "API_KEY", "--length", "2000"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for length > 1024, got nil")
	}
	if !contains(err.Error(), "length must not exceed 1024") {
		t.Errorf("expected length validation error, got: %v", err)
	}
}

func TestSecretGenerateCmd_InvalidCharset(t *testing.T) {
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
	root.SetArgs([]string{"secret", "generate", "API_KEY", "--charset", "bogus"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for unknown charset, got nil")
	}
	if !contains(err.Error(), "unknown charset") {
		t.Errorf("expected charset validation error, got: %v", err)
	}
}

func TestSecretGenerateCmd_NoConfig(t *testing.T) {
	dir := t.TempDir()

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
	root.SetArgs([]string{"secret", "generate", "API_KEY"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error when no config found, got nil")
	}
	if !contains(err.Error(), "loading config") {
		t.Errorf("expected config loading error, got: %v", err)
	}
}

func TestSecretGenerateCmd_NoBackends(t *testing.T) {
	dir := t.TempDir()
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
	root.SetArgs([]string{"secret", "generate", "API_KEY"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error when no backends configured, got nil")
	}
	if !contains(err.Error(), "no backends configured") {
		t.Errorf("expected 'no backends configured' error, got: %v", err)
	}
}

func TestSecretGenerateCmd_InvalidBackend(t *testing.T) {
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
	root.SetArgs([]string{"secret", "generate", "API_KEY", "--backend", "nonexistent"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent backend, got nil")
	}
	if !contains(err.Error(), "nonexistent") {
		t.Errorf("expected error mentioning backend name, got: %v", err)
	}
}

// --- Tests for secret copy ---

func TestSecretCopyCmd_Success(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "destproject")

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
	root.SetArgs([]string{"secret", "copy", "API_KEY", "--from", "srcproject"})

	err = root.Execute()
	if err != nil {
		// Expected in CI where keychain is not available.
		errMsg := err.Error()
		if contains(errMsg, "accepts 1 arg") || contains(errMsg, "unknown command") {
			t.Fatalf("command structure error: %v", err)
		}
	} else {
		got := buf.String()
		want := "secret \"API_KEY\" copied from project \"srcproject\" to \"destproject\" (backend \"keychain\")\n"
		if got != want {
			t.Errorf("output: got %q, want %q", got, want)
		}
	}
}

func TestSecretCopyCmd_NoArgs(t *testing.T) {
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "copy"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing argument, got nil")
	}
}

func TestSecretCopyCmd_MissingFromFlag(t *testing.T) {
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "copy", "API_KEY"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing --from flag, got nil")
	}
	if !contains(err.Error(), "from") {
		t.Errorf("expected error about --from flag, got: %v", err)
	}
}

func TestSecretCopyCmd_NoConfig(t *testing.T) {
	dir := t.TempDir()

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
	root.SetArgs([]string{"secret", "copy", "API_KEY", "--from", "other"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error when no config found, got nil")
	}
	if !contains(err.Error(), "loading config") {
		t.Errorf("expected config loading error, got: %v", err)
	}
}

func TestSecretCopyCmd_NoBackends(t *testing.T) {
	dir := t.TempDir()
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
	root.SetArgs([]string{"secret", "copy", "API_KEY", "--from", "other"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error when no backends configured, got nil")
	}
	if !contains(err.Error(), "no backends configured") {
		t.Errorf("expected 'no backends configured' error, got: %v", err)
	}
}

func TestSecretCopyCmd_InvalidBackend(t *testing.T) {
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
	root.SetArgs([]string{"secret", "copy", "API_KEY", "--from", "other", "--backend", "nonexistent"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent backend, got nil")
	}
	if !contains(err.Error(), "nonexistent") {
		t.Errorf("expected error mentioning backend name, got: %v", err)
	}
}

func TestSecretCopyCmd_EmptyKey(t *testing.T) {
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
	root.SetArgs([]string{"secret", "copy", "  ", "--from", "other"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for empty key, got nil")
	}
	if !contains(err.Error(), "key must not be empty") {
		t.Errorf("expected empty key error, got: %v", err)
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
