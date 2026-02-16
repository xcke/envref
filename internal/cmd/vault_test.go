package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// writeVaultTestConfig writes a .envref.yaml with a vault backend to the
// given directory, using a temp path for the vault database.
func writeVaultTestConfig(t *testing.T, dir, project, vaultPath string) string {
	t.Helper()
	content := "project: " + project + "\nbackends:\n  - name: vault\n    type: vault\n    config:\n      path: " + vaultPath + "\n"
	path := filepath.Join(dir, ".envref.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return path
}

func TestVaultInitCmd_WithEnvVar(t *testing.T) {
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

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(errBuf)
	root.SetArgs([]string{"vault", "init"})

	err = root.Execute()
	if err != nil {
		t.Fatalf("vault init failed: %v", err)
	}

	got := buf.String()
	if got == "" {
		t.Error("expected output from vault init")
	}
	if !bytes.Contains([]byte(got), []byte("vault initialized")) {
		t.Errorf("expected 'vault initialized' in output, got: %q", got)
	}

	// Verify the vault database was created.
	if _, err := os.Stat(vaultPath); os.IsNotExist(err) {
		t.Fatal("vault database was not created")
	}
}

func TestVaultInitCmd_AlreadyInitialized(t *testing.T) {
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

	// First init.
	root1 := NewRootCmd()
	root1.SetOut(new(bytes.Buffer))
	root1.SetErr(new(bytes.Buffer))
	root1.SetArgs([]string{"vault", "init"})
	if err := root1.Execute(); err != nil {
		t.Fatalf("first vault init failed: %v", err)
	}

	// Second init should not error but should say already initialized.
	root2 := NewRootCmd()
	buf := new(bytes.Buffer)
	root2.SetOut(buf)
	root2.SetErr(new(bytes.Buffer))
	root2.SetArgs([]string{"vault", "init"})
	if err := root2.Execute(); err != nil {
		t.Fatalf("second vault init should not error: %v", err)
	}

	got := buf.String()
	if !bytes.Contains([]byte(got), []byte("already initialized")) {
		t.Errorf("expected 'already initialized' message, got: %q", got)
	}
}

func TestVaultInitCmd_NoPassphrase_NonInteractive(t *testing.T) {
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

	// Ensure no env var is set.
	t.Setenv("ENVREF_VAULT_PASSPHRASE", "")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(errBuf)
	// Pipe an empty reader (non-terminal) to stdin.
	root.SetIn(bytes.NewReader(nil))
	root.SetArgs([]string{"vault", "init"})

	err = root.Execute()
	if err == nil {
		t.Fatal("expected error when no passphrase and non-interactive")
	}
}

func TestVaultSecretSet_WithInit(t *testing.T) {
	// Test that secret set works after vault init.
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

	// Initialize the vault.
	root1 := NewRootCmd()
	root1.SetOut(new(bytes.Buffer))
	root1.SetErr(new(bytes.Buffer))
	root1.SetArgs([]string{"vault", "init"})
	if err := root1.Execute(); err != nil {
		t.Fatalf("vault init failed: %v", err)
	}

	// Store a secret.
	root2 := NewRootCmd()
	root2.SetOut(new(bytes.Buffer))
	root2.SetErr(new(bytes.Buffer))
	root2.SetArgs([]string{"secret", "set", "API_KEY", "--value", "sk-test-123"})
	if err := root2.Execute(); err != nil {
		t.Fatalf("secret set failed: %v", err)
	}

	// Retrieve the secret.
	root3 := NewRootCmd()
	getBuf := new(bytes.Buffer)
	root3.SetOut(getBuf)
	root3.SetErr(new(bytes.Buffer))
	root3.SetArgs([]string{"secret", "get", "API_KEY"})
	if err := root3.Execute(); err != nil {
		t.Fatalf("secret get failed: %v", err)
	}

	got := getBuf.String()
	if got != "sk-test-123\n" {
		t.Errorf("secret get: got %q, want %q", got, "sk-test-123\n")
	}
}

func TestVaultSecretSet_WrongPassphrase(t *testing.T) {
	// Test that secret operations fail with wrong passphrase after init.
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

	// Initialize with correct passphrase.
	t.Setenv("ENVREF_VAULT_PASSPHRASE", "correct-pass")

	root1 := NewRootCmd()
	root1.SetOut(new(bytes.Buffer))
	root1.SetErr(new(bytes.Buffer))
	root1.SetArgs([]string{"vault", "init"})
	if err := root1.Execute(); err != nil {
		t.Fatalf("vault init failed: %v", err)
	}

	// Try to access with wrong passphrase.
	t.Setenv("ENVREF_VAULT_PASSPHRASE", "wrong-pass")

	root2 := NewRootCmd()
	root2.SetOut(new(bytes.Buffer))
	root2.SetErr(new(bytes.Buffer))
	root2.SetArgs([]string{"secret", "list"})
	err = root2.Execute()
	if err == nil {
		t.Fatal("expected error with wrong passphrase after init")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("wrong vault passphrase")) {
		t.Errorf("expected 'wrong vault passphrase' error, got: %v", err)
	}
}

func TestVaultCmd_Help(t *testing.T) {
	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"vault", "--help"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("vault --help failed: %v", err)
	}

	got := buf.String()
	if !bytes.Contains([]byte(got), []byte("vault init")) {
		t.Errorf("expected 'vault init' in help output, got: %q", got)
	}
}

func TestVaultLockCmd(t *testing.T) {
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

	// Initialize the vault first.
	root1 := NewRootCmd()
	root1.SetOut(new(bytes.Buffer))
	root1.SetErr(new(bytes.Buffer))
	root1.SetArgs([]string{"vault", "init"})
	if err := root1.Execute(); err != nil {
		t.Fatalf("vault init failed: %v", err)
	}

	// Lock the vault.
	root2 := NewRootCmd()
	lockBuf := new(bytes.Buffer)
	root2.SetOut(lockBuf)
	root2.SetErr(new(bytes.Buffer))
	root2.SetArgs([]string{"vault", "lock"})
	if err := root2.Execute(); err != nil {
		t.Fatalf("vault lock failed: %v", err)
	}

	got := lockBuf.String()
	if !bytes.Contains([]byte(got), []byte("vault locked")) {
		t.Errorf("expected 'vault locked' in output, got: %q", got)
	}

	// Secret operations should fail while locked.
	root3 := NewRootCmd()
	root3.SetOut(new(bytes.Buffer))
	root3.SetErr(new(bytes.Buffer))
	root3.SetArgs([]string{"secret", "set", "API_KEY", "--value", "test"})
	err = root3.Execute()
	if err == nil {
		t.Fatal("secret set should fail on locked vault")
	}
}

func TestVaultUnlockCmd(t *testing.T) {
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

	// Initialize and lock the vault.
	root1 := NewRootCmd()
	root1.SetOut(new(bytes.Buffer))
	root1.SetErr(new(bytes.Buffer))
	root1.SetArgs([]string{"vault", "init"})
	if err := root1.Execute(); err != nil {
		t.Fatalf("vault init failed: %v", err)
	}

	root2 := NewRootCmd()
	root2.SetOut(new(bytes.Buffer))
	root2.SetErr(new(bytes.Buffer))
	root2.SetArgs([]string{"vault", "lock"})
	if err := root2.Execute(); err != nil {
		t.Fatalf("vault lock failed: %v", err)
	}

	// Unlock the vault.
	root3 := NewRootCmd()
	unlockBuf := new(bytes.Buffer)
	root3.SetOut(unlockBuf)
	root3.SetErr(new(bytes.Buffer))
	root3.SetArgs([]string{"vault", "unlock"})
	if err := root3.Execute(); err != nil {
		t.Fatalf("vault unlock failed: %v", err)
	}

	got := unlockBuf.String()
	if !bytes.Contains([]byte(got), []byte("vault unlocked")) {
		t.Errorf("expected 'vault unlocked' in output, got: %q", got)
	}

	// Secret operations should work again.
	root4 := NewRootCmd()
	root4.SetOut(new(bytes.Buffer))
	root4.SetErr(new(bytes.Buffer))
	root4.SetArgs([]string{"secret", "set", "API_KEY", "--value", "test-123"})
	if err := root4.Execute(); err != nil {
		t.Fatalf("secret set after unlock failed: %v", err)
	}

	root5 := NewRootCmd()
	getBuf := new(bytes.Buffer)
	root5.SetOut(getBuf)
	root5.SetErr(new(bytes.Buffer))
	root5.SetArgs([]string{"secret", "get", "API_KEY"})
	if err := root5.Execute(); err != nil {
		t.Fatalf("secret get after unlock failed: %v", err)
	}

	if getBuf.String() != "test-123\n" {
		t.Errorf("secret get after unlock: got %q, want %q", getBuf.String(), "test-123\n")
	}
}

func TestVaultLockCmd_NotInitialized(t *testing.T) {
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

	// Lock without initializing should fail.
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"vault", "lock"})
	err = root.Execute()
	if err == nil {
		t.Fatal("vault lock on uninitialized vault should fail")
	}
}

func TestVaultCmd_HelpShowsLockUnlock(t *testing.T) {
	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"vault", "--help"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("vault --help failed: %v", err)
	}

	got := buf.String()
	if !bytes.Contains([]byte(got), []byte("Lock the vault")) {
		t.Errorf("expected 'Lock the vault' in help output, got: %q", got)
	}
	if !bytes.Contains([]byte(got), []byte("Unlock the vault")) {
		t.Errorf("expected 'Unlock the vault' in help output, got: %q", got)
	}
}

func TestGetTerminalFd_NonTerminal(t *testing.T) {
	root := NewRootCmd()
	root.SetIn(bytes.NewReader(nil))

	// When stdin is a bytes.Reader (not a file), getTerminalFd should
	// fall back to os.Stdin. In CI that's not a terminal.
	_, isTerm := getTerminalFd(root)
	// In CI, stdin is not a terminal.
	if isTerm {
		t.Skip("skipping: stdin is a terminal in this environment")
	}
}
