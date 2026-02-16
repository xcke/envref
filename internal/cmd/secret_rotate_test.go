package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xcke/envref/internal/backend"
)

// --- Unit tests for historyKey ---

func TestHistoryKey(t *testing.T) {
	tests := []struct {
		key   string
		index int
		want  string
	}{
		{"API_KEY", 1, "API_KEY.__history.1"},
		{"API_KEY", 2, "API_KEY.__history.2"},
		{"DB_PASS", 10, "DB_PASS.__history.10"},
	}
	for _, tt := range tests {
		got := historyKey(tt.key, tt.index)
		if got != tt.want {
			t.Errorf("historyKey(%q, %d) = %q, want %q", tt.key, tt.index, got, tt.want)
		}
	}
}

// --- Unit tests for rotateHistory using a real vault backend ---

func setupVaultBackend(t *testing.T) *backend.NamespacedBackend {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test-vault.db")
	v, err := backend.NewVaultBackend("test-passphrase", backend.WithVaultPath(dbPath))
	if err != nil {
		t.Fatalf("NewVaultBackend: %v", err)
	}
	t.Cleanup(func() { _ = v.Close() })

	nsBackend, err := backend.NewNamespacedBackend(v, "testproject")
	if err != nil {
		t.Fatalf("NewNamespacedBackend: %v", err)
	}
	return nsBackend
}

func TestRotateHistory_SingleEntry(t *testing.T) {
	ns := setupVaultBackend(t)

	// Rotate with keep=1: should store old value at .__history.1.
	if err := rotateHistory(ns, "API_KEY", "old-value", 1); err != nil {
		t.Fatalf("rotateHistory: %v", err)
	}

	val, err := ns.Get("API_KEY.__history.1")
	if err != nil {
		t.Fatalf("Get history 1: %v", err)
	}
	if val != "old-value" {
		t.Errorf("history 1: got %q, want %q", val, "old-value")
	}
}

func TestRotateHistory_MultipleEntries(t *testing.T) {
	ns := setupVaultBackend(t)

	// First rotation.
	if err := rotateHistory(ns, "KEY", "value-1", 3); err != nil {
		t.Fatalf("rotateHistory 1: %v", err)
	}

	// Second rotation.
	if err := rotateHistory(ns, "KEY", "value-2", 3); err != nil {
		t.Fatalf("rotateHistory 2: %v", err)
	}

	// Third rotation.
	if err := rotateHistory(ns, "KEY", "value-3", 3); err != nil {
		t.Fatalf("rotateHistory 3: %v", err)
	}

	// Check: history.1 = value-3 (most recent), history.2 = value-2, history.3 = value-1.
	for i, want := range []string{"value-3", "value-2", "value-1"} {
		val, err := ns.Get(historyKey("KEY", i+1))
		if err != nil {
			t.Fatalf("Get history %d: %v", i+1, err)
		}
		if val != want {
			t.Errorf("history %d: got %q, want %q", i+1, val, want)
		}
	}
}

func TestRotateHistory_ExceedsKeep(t *testing.T) {
	ns := setupVaultBackend(t)

	// Rotate 4 times with keep=2.
	for i, val := range []string{"v1", "v2", "v3", "v4"} {
		if err := rotateHistory(ns, "KEY", val, 2); err != nil {
			t.Fatalf("rotateHistory %d: %v", i+1, err)
		}
	}

	// history.1 = v4 (most recent old value), history.2 = v3.
	val1, err := ns.Get(historyKey("KEY", 1))
	if err != nil {
		t.Fatalf("Get history 1: %v", err)
	}
	if val1 != "v4" {
		t.Errorf("history 1: got %q, want %q", val1, "v4")
	}

	val2, err := ns.Get(historyKey("KEY", 2))
	if err != nil {
		t.Fatalf("Get history 2: %v", err)
	}
	if val2 != "v3" {
		t.Errorf("history 2: got %q, want %q", val2, "v3")
	}

	// history.3 should have been deleted (beyond keep=2).
	_, err = ns.Get(historyKey("KEY", 3))
	if !errors.Is(err, backend.ErrNotFound) {
		t.Errorf("expected history 3 to be deleted, got err: %v", err)
	}
}

func TestCleanupHistory(t *testing.T) {
	ns := setupVaultBackend(t)

	// Set up some history entries.
	for i := 1; i <= 3; i++ {
		if err := ns.Set(historyKey("KEY", i), "old"); err != nil {
			t.Fatalf("Set history %d: %v", i, err)
		}
	}

	cleanupHistory(ns, "KEY")

	// All history entries should be gone.
	for i := 1; i <= 3; i++ {
		_, err := ns.Get(historyKey("KEY", i))
		if !errors.Is(err, backend.ErrNotFound) {
			t.Errorf("expected history %d to be deleted, got err: %v", i, err)
		}
	}
}

// --- Command-level tests ---

func TestSecretRotateCmd_NoArgs(t *testing.T) {
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "rotate"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing argument, got nil")
	}
}

func TestSecretRotateCmd_NoConfig(t *testing.T) {
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
	root.SetArgs([]string{"secret", "rotate", "API_KEY"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error when no config found, got nil")
	}
	if !contains(err.Error(), "loading config") {
		t.Errorf("expected config loading error, got: %v", err)
	}
}

func TestSecretRotateCmd_NoBackends(t *testing.T) {
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
	root.SetArgs([]string{"secret", "rotate", "API_KEY"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error when no backends configured, got nil")
	}
	if !contains(err.Error(), "no backends configured") {
		t.Errorf("expected 'no backends configured' error, got: %v", err)
	}
}

func TestSecretRotateCmd_InvalidBackend(t *testing.T) {
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
	root.SetArgs([]string{"secret", "rotate", "API_KEY", "--backend", "nonexistent"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent backend, got nil")
	}
	if !contains(err.Error(), "nonexistent") {
		t.Errorf("expected error mentioning backend name, got: %v", err)
	}
}

func TestSecretRotateCmd_InvalidLength(t *testing.T) {
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
	root.SetArgs([]string{"secret", "rotate", "API_KEY", "--length", "0"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for length 0, got nil")
	}
	if !contains(err.Error(), "length must be at least 1") {
		t.Errorf("expected length validation error, got: %v", err)
	}
}

func TestSecretRotateCmd_InvalidCharset(t *testing.T) {
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
	root.SetArgs([]string{"secret", "rotate", "API_KEY", "--charset", "bogus"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for unknown charset, got nil")
	}
	if !contains(err.Error(), "unknown charset") {
		t.Errorf("expected charset validation error, got: %v", err)
	}
}

func TestSecretRotateCmd_EmptyKey(t *testing.T) {
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
	root.SetArgs([]string{"secret", "rotate", "  "})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for empty key, got nil")
	}
	if !contains(err.Error(), "key must not be empty") {
		t.Errorf("expected empty key error, got: %v", err)
	}
}

func TestSecretRotateCmd_InvalidKeepNegative(t *testing.T) {
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
	root.SetArgs([]string{"secret", "rotate", "API_KEY", "--keep", "-1"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error for negative keep, got nil")
	}
	if !contains(err.Error(), "keep must be at least 0") {
		t.Errorf("expected keep validation error, got: %v", err)
	}
}

// --- End-to-end tests with vault backend ---

func TestSecretRotateCmd_VaultNewKey(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "test-vault.db")
	writeVaultTestConfig(t, dir, "testproject", vaultPath)

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

	t.Setenv("ENVREF_VAULT_PASSPHRASE", "test-passphrase")

	// Initialize vault.
	root1 := NewRootCmd()
	root1.SetOut(new(bytes.Buffer))
	root1.SetErr(new(bytes.Buffer))
	root1.SetArgs([]string{"vault", "init"})
	if err := root1.Execute(); err != nil {
		t.Fatalf("vault init: %v", err)
	}

	// Rotate a key that doesn't exist yet — should behave like generate.
	root2 := NewRootCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root2.SetOut(buf)
	root2.SetErr(errBuf)
	root2.SetArgs([]string{"secret", "rotate", "NEW_KEY", "--print", "--length", "16", "--charset", "hex"})

	if err := root2.Execute(); err != nil {
		t.Fatalf("secret rotate (new key): %v", err)
	}

	// Parse output: first line is the value, remaining lines are info messages.
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 1 {
		t.Fatal("expected at least one line of output")
	}
	generatedValue := lines[0]
	if len(generatedValue) != 16 {
		t.Errorf("expected 16 char value, got %d: %q", len(generatedValue), generatedValue)
	}

	// Verify the info message says "generated and stored" (not "rotated").
	fullOutput := buf.String()
	if !contains(fullOutput, "generated and stored") {
		t.Errorf("expected 'generated and stored' in output, got: %q", fullOutput)
	}

	// Verify the secret was actually stored.
	root3 := NewRootCmd()
	getBuf := new(bytes.Buffer)
	root3.SetOut(getBuf)
	root3.SetErr(new(bytes.Buffer))
	root3.SetArgs([]string{"secret", "get", "NEW_KEY"})
	if err := root3.Execute(); err != nil {
		t.Fatalf("secret get: %v", err)
	}
	if strings.TrimSpace(getBuf.String()) != generatedValue {
		t.Errorf("stored value %q doesn't match printed value %q", strings.TrimSpace(getBuf.String()), generatedValue)
	}
}

func TestSecretRotateCmd_VaultExistingKey(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "test-vault.db")
	writeVaultTestConfig(t, dir, "testproject", vaultPath)

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

	t.Setenv("ENVREF_VAULT_PASSPHRASE", "test-passphrase")

	// Initialize vault.
	root1 := NewRootCmd()
	root1.SetOut(new(bytes.Buffer))
	root1.SetErr(new(bytes.Buffer))
	root1.SetArgs([]string{"vault", "init"})
	if err := root1.Execute(); err != nil {
		t.Fatalf("vault init: %v", err)
	}

	// Set an initial value.
	root2 := NewRootCmd()
	root2.SetOut(new(bytes.Buffer))
	root2.SetErr(new(bytes.Buffer))
	root2.SetArgs([]string{"secret", "set", "API_KEY", "--value", "original-value"})
	if err := root2.Execute(); err != nil {
		t.Fatalf("secret set: %v", err)
	}

	// Rotate the key.
	root3 := NewRootCmd()
	rotateBuf := new(bytes.Buffer)
	rotateErrBuf := new(bytes.Buffer)
	root3.SetOut(rotateBuf)
	root3.SetErr(rotateErrBuf)
	root3.SetArgs([]string{"secret", "rotate", "API_KEY", "--print"})
	if err := root3.Execute(); err != nil {
		t.Fatalf("secret rotate: %v", err)
	}

	// Parse output: first line is the new value.
	rotateLines := strings.Split(strings.TrimSpace(rotateBuf.String()), "\n")
	if len(rotateLines) < 1 {
		t.Fatal("expected at least one line of rotate output")
	}
	newValue := rotateLines[0]
	if len(newValue) != 32 {
		t.Errorf("expected 32 char default output, got %d: %q", len(newValue), newValue)
	}
	if newValue == "original-value" {
		t.Error("new value should differ from original")
	}

	// Verify info message says "rotated" and "previous value archived".
	fullRotateOutput := rotateBuf.String()
	if !contains(fullRotateOutput, "rotated") {
		t.Errorf("expected 'rotated' in output, got: %q", fullRotateOutput)
	}
	if !contains(fullRotateOutput, "previous value archived") {
		t.Errorf("expected 'previous value archived' in output, got: %q", fullRotateOutput)
	}

	// Verify the current value is the new one.
	root4 := NewRootCmd()
	getBuf := new(bytes.Buffer)
	root4.SetOut(getBuf)
	root4.SetErr(new(bytes.Buffer))
	root4.SetArgs([]string{"secret", "get", "API_KEY"})
	if err := root4.Execute(); err != nil {
		t.Fatalf("secret get: %v", err)
	}
	if strings.TrimSpace(getBuf.String()) != newValue {
		t.Errorf("current value %q doesn't match rotated value %q", strings.TrimSpace(getBuf.String()), newValue)
	}

	// Verify the old value is in history.
	root5 := NewRootCmd()
	histBuf := new(bytes.Buffer)
	root5.SetOut(histBuf)
	root5.SetErr(new(bytes.Buffer))
	root5.SetArgs([]string{"secret", "get", "API_KEY.__history.1"})
	if err := root5.Execute(); err != nil {
		t.Fatalf("secret get history: %v", err)
	}
	if strings.TrimSpace(histBuf.String()) != "original-value" {
		t.Errorf("history value: got %q, want %q", strings.TrimSpace(histBuf.String()), "original-value")
	}
}

func TestSecretRotateCmd_VaultMultipleRotations(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "test-vault.db")
	writeVaultTestConfig(t, dir, "testproject", vaultPath)

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

	t.Setenv("ENVREF_VAULT_PASSPHRASE", "test-passphrase")

	// Initialize vault.
	root1 := NewRootCmd()
	root1.SetOut(new(bytes.Buffer))
	root1.SetErr(new(bytes.Buffer))
	root1.SetArgs([]string{"vault", "init"})
	if err := root1.Execute(); err != nil {
		t.Fatalf("vault init: %v", err)
	}

	// Set an initial value.
	root2 := NewRootCmd()
	root2.SetOut(new(bytes.Buffer))
	root2.SetErr(new(bytes.Buffer))
	root2.SetArgs([]string{"secret", "set", "KEY", "--value", "v0"})
	if err := root2.Execute(); err != nil {
		t.Fatalf("secret set: %v", err)
	}

	// Rotate 3 times with keep=2.
	values := make([]string, 3)
	for i := 0; i < 3; i++ {
		r := NewRootCmd()
		buf := new(bytes.Buffer)
		r.SetOut(buf)
		r.SetErr(new(bytes.Buffer))
		r.SetArgs([]string{"secret", "rotate", "KEY", "--print", "--keep", "2"})
		if err := r.Execute(); err != nil {
			t.Fatalf("rotate %d: %v", i+1, err)
		}
		// First line is the generated value.
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		values[i] = lines[0]
	}

	// After 3 rotations starting from "v0":
	// Current = values[2]
	// history.1 = values[1]  (previous rotation's new value)
	// history.2 = values[0]  (rotation before that)
	// "v0" should have been pushed out (beyond keep=2)

	// Check current value.
	r := NewRootCmd()
	buf := new(bytes.Buffer)
	r.SetOut(buf)
	r.SetErr(new(bytes.Buffer))
	r.SetArgs([]string{"secret", "get", "KEY"})
	if err := r.Execute(); err != nil {
		t.Fatalf("get current: %v", err)
	}
	if strings.TrimSpace(buf.String()) != values[2] {
		t.Errorf("current: got %q, want %q", strings.TrimSpace(buf.String()), values[2])
	}

	// Check history.1 = values[1].
	r2 := NewRootCmd()
	buf2 := new(bytes.Buffer)
	r2.SetOut(buf2)
	r2.SetErr(new(bytes.Buffer))
	r2.SetArgs([]string{"secret", "get", "KEY.__history.1"})
	if err := r2.Execute(); err != nil {
		t.Fatalf("get history 1: %v", err)
	}
	if strings.TrimSpace(buf2.String()) != values[1] {
		t.Errorf("history.1: got %q, want %q", strings.TrimSpace(buf2.String()), values[1])
	}

	// Check history.2 = values[0].
	r3 := NewRootCmd()
	buf3 := new(bytes.Buffer)
	r3.SetOut(buf3)
	r3.SetErr(new(bytes.Buffer))
	r3.SetArgs([]string{"secret", "get", "KEY.__history.2"})
	if err := r3.Execute(); err != nil {
		t.Fatalf("get history 2: %v", err)
	}
	if strings.TrimSpace(buf3.String()) != values[0] {
		t.Errorf("history.2: got %q, want %q", strings.TrimSpace(buf3.String()), values[0])
	}
}

func TestSecretRotateCmd_VaultKeepZero(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "test-vault.db")
	writeVaultTestConfig(t, dir, "testproject", vaultPath)

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

	t.Setenv("ENVREF_VAULT_PASSPHRASE", "test-passphrase")

	// Initialize vault and set an initial value.
	root1 := NewRootCmd()
	root1.SetOut(new(bytes.Buffer))
	root1.SetErr(new(bytes.Buffer))
	root1.SetArgs([]string{"vault", "init"})
	if err := root1.Execute(); err != nil {
		t.Fatalf("vault init: %v", err)
	}

	root2 := NewRootCmd()
	root2.SetOut(new(bytes.Buffer))
	root2.SetErr(new(bytes.Buffer))
	root2.SetArgs([]string{"secret", "set", "KEY", "--value", "original"})
	if err := root2.Execute(); err != nil {
		t.Fatalf("secret set: %v", err)
	}

	// Rotate with keep=0 — no history should be stored.
	root3 := NewRootCmd()
	root3.SetOut(new(bytes.Buffer))
	root3.SetErr(new(bytes.Buffer))
	root3.SetArgs([]string{"secret", "rotate", "KEY", "--keep", "0"})
	if err := root3.Execute(); err != nil {
		t.Fatalf("secret rotate: %v", err)
	}

	// Verify no history entry exists.
	root4 := NewRootCmd()
	root4.SetOut(new(bytes.Buffer))
	root4.SetErr(new(bytes.Buffer))
	root4.SetArgs([]string{"secret", "get", "KEY.__history.1"})
	err = root4.Execute()
	if err == nil {
		t.Error("expected error getting deleted history, got nil")
	}
}

func TestSecretRotateCmd_SubcommandRegistered(t *testing.T) {
	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "rotate", "--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("secret rotate --help: %v", err)
	}

	helpOutput := buf.String()
	if !contains(helpOutput, "rotate") {
		t.Errorf("help output missing 'rotate': %s", helpOutput)
	}
	if !contains(helpOutput, "--keep") {
		t.Errorf("help output missing '--keep' flag: %s", helpOutput)
	}
	if !contains(helpOutput, "--length") {
		t.Errorf("help output missing '--length' flag: %s", helpOutput)
	}
	if !contains(helpOutput, "--charset") {
		t.Errorf("help output missing '--charset' flag: %s", helpOutput)
	}
	if !contains(helpOutput, "--print") {
		t.Errorf("help output missing '--print' flag: %s", helpOutput)
	}
}
