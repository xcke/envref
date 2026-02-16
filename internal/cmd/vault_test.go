package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/xcke/envref/internal/backend"
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

// initVaultForTest initializes a vault with test data via the CLI and returns
// the vault database path.
func initVaultForTest(t *testing.T, dir string) string {
	t.Helper()
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
		t.Fatalf("vault init: %v", err)
	}

	// Store some secrets.
	for _, kv := range []struct{ key, val string }{
		{"API_KEY", "sk-test-123"},
		{"DB_PASS", "s3cret"},
	} {
		root := NewRootCmd()
		root.SetOut(new(bytes.Buffer))
		root.SetErr(new(bytes.Buffer))
		root.SetArgs([]string{"secret", "set", kv.key, "--value", kv.val})
		if err := root.Execute(); err != nil {
			t.Fatalf("secret set %s: %v", kv.key, err)
		}
	}

	return vaultPath
}

func TestVaultExportCmd_ToFile(t *testing.T) {
	dir := t.TempDir()
	initVaultForTest(t, dir)

	// Export to default file.
	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"vault", "export"})
	if err := root.Execute(); err != nil {
		t.Fatalf("vault export: %v", err)
	}

	got := buf.String()
	if !bytes.Contains([]byte(got), []byte("exported")) {
		t.Errorf("expected 'exported' in output, got: %q", got)
	}

	// Verify the export file exists and contains valid JSON.
	exportPath := filepath.Join(dir, defaultVaultExportFile)
	data, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("reading export file: %v", err)
	}

	var export backend.VaultExport
	if err := json.Unmarshal(data, &export); err != nil {
		t.Fatalf("parsing export JSON: %v", err)
	}
	if len(export.Secrets) != 2 {
		t.Fatalf("export secrets count: got %d, want 2", len(export.Secrets))
	}

	// Verify file permissions are restrictive.
	info, err := os.Stat(exportPath)
	if err != nil {
		t.Fatalf("stat export file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("export file permissions: got %o, want 600", info.Mode().Perm())
	}
}

func TestVaultExportCmd_CustomFile(t *testing.T) {
	dir := t.TempDir()
	initVaultForTest(t, dir)

	customPath := filepath.Join(dir, "my-backup.json")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"vault", "export", "--file", customPath})
	if err := root.Execute(); err != nil {
		t.Fatalf("vault export --file: %v", err)
	}

	if _, err := os.Stat(customPath); os.IsNotExist(err) {
		t.Fatal("custom export file was not created")
	}
}

func TestVaultExportCmd_Stdout(t *testing.T) {
	dir := t.TempDir()
	initVaultForTest(t, dir)

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"vault", "export", "--stdout"})
	if err := root.Execute(); err != nil {
		t.Fatalf("vault export --stdout: %v", err)
	}

	// stdout should contain valid JSON.
	var export backend.VaultExport
	if err := json.Unmarshal(buf.Bytes(), &export); err != nil {
		t.Fatalf("parsing stdout JSON: %v", err)
	}
	if len(export.Secrets) != 2 {
		t.Fatalf("stdout export secrets count: got %d, want 2", len(export.Secrets))
	}
}

func TestVaultImportCmd_FromFile(t *testing.T) {
	dir := t.TempDir()
	initVaultForTest(t, dir)

	// Create an import file with namespaced keys (vault stores keys with
	// project prefix added by NamespacedBackend).
	importData := backend.VaultExport{
		Version:    1,
		ExportedAt: "2025-01-01T00:00:00Z",
		Secrets: map[string]string{
			"testproject/NEW_KEY": "new-value",
			"testproject/API_KEY": "overwritten",
		},
	}
	data, err := json.MarshalIndent(importData, "", "  ")
	if err != nil {
		t.Fatalf("marshal import data: %v", err)
	}

	importPath := filepath.Join(dir, "import.json")
	if err := os.WriteFile(importPath, data, 0o600); err != nil {
		t.Fatalf("write import file: %v", err)
	}

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"vault", "import", "--file", importPath})
	if err := root.Execute(); err != nil {
		t.Fatalf("vault import: %v", err)
	}

	got := buf.String()
	if !bytes.Contains([]byte(got), []byte("imported 2 secrets")) {
		t.Errorf("expected 'imported 2 secrets' in output, got: %q", got)
	}

	// Verify the imported secrets are accessible via secret get
	// (which adds the testproject/ namespace automatically).
	root2 := NewRootCmd()
	getBuf := new(bytes.Buffer)
	root2.SetOut(getBuf)
	root2.SetErr(new(bytes.Buffer))
	root2.SetArgs([]string{"secret", "get", "NEW_KEY"})
	if err := root2.Execute(); err != nil {
		t.Fatalf("secret get NEW_KEY: %v", err)
	}
	if getBuf.String() != "new-value\n" {
		t.Errorf("secret get NEW_KEY: got %q, want %q", getBuf.String(), "new-value\n")
	}

	// Verify the overwritten key.
	root3 := NewRootCmd()
	getBuf2 := new(bytes.Buffer)
	root3.SetOut(getBuf2)
	root3.SetErr(new(bytes.Buffer))
	root3.SetArgs([]string{"secret", "get", "API_KEY"})
	if err := root3.Execute(); err != nil {
		t.Fatalf("secret get API_KEY: %v", err)
	}
	if getBuf2.String() != "overwritten\n" {
		t.Errorf("secret get API_KEY: got %q, want %q", getBuf2.String(), "overwritten\n")
	}
}

func TestVaultImportCmd_FromStdin(t *testing.T) {
	dir := t.TempDir()
	initVaultForTest(t, dir)

	importData := backend.VaultExport{
		Version:    1,
		ExportedAt: "2025-01-01T00:00:00Z",
		Secrets:    map[string]string{"STDIN_KEY": "stdin-value"},
	}
	data, err := json.Marshal(importData)
	if err != nil {
		t.Fatalf("marshal import data: %v", err)
	}

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetIn(bytes.NewReader(data))
	root.SetArgs([]string{"vault", "import", "--stdin"})
	if err := root.Execute(); err != nil {
		t.Fatalf("vault import --stdin: %v", err)
	}

	got := buf.String()
	if !bytes.Contains([]byte(got), []byte("imported 1 secret")) {
		t.Errorf("expected 'imported 1 secret' in output, got: %q", got)
	}
}

func TestVaultExportImportCmd_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	initVaultForTest(t, dir)

	// Export to stdout.
	rootExport := NewRootCmd()
	exportBuf := new(bytes.Buffer)
	rootExport.SetOut(exportBuf)
	rootExport.SetErr(new(bytes.Buffer))
	rootExport.SetArgs([]string{"vault", "export", "--stdout"})
	if err := rootExport.Execute(); err != nil {
		t.Fatalf("vault export: %v", err)
	}

	// Verify exported JSON contains the namespaced keys.
	var exportData backend.VaultExport
	if err := json.Unmarshal(exportBuf.Bytes(), &exportData); err != nil {
		t.Fatalf("parsing export JSON: %v", err)
	}
	if len(exportData.Secrets) != 2 {
		t.Fatalf("export secrets count: got %d, want 2", len(exportData.Secrets))
	}

	// Create a new vault directory for import — use the SAME project name
	// so that secret get (which adds namespace) will find the imported keys.
	dir2 := t.TempDir()
	vaultPath2 := filepath.Join(dir2, "test-vault2.db")
	writeVaultTestConfig(t, dir2, "testproject", vaultPath2)
	if err := os.Chdir(dir2); err != nil {
		t.Fatalf("chdir to dir2: %v", err)
	}

	// Initialize the new vault.
	rootInit := NewRootCmd()
	rootInit.SetOut(new(bytes.Buffer))
	rootInit.SetErr(new(bytes.Buffer))
	rootInit.SetArgs([]string{"vault", "init"})
	if err := rootInit.Execute(); err != nil {
		t.Fatalf("vault init dir2: %v", err)
	}

	// Import from the exported data.
	rootImport := NewRootCmd()
	importBuf := new(bytes.Buffer)
	rootImport.SetOut(importBuf)
	rootImport.SetErr(new(bytes.Buffer))
	rootImport.SetIn(bytes.NewReader(exportBuf.Bytes()))
	rootImport.SetArgs([]string{"vault", "import", "--stdin"})
	if err := rootImport.Execute(); err != nil {
		t.Fatalf("vault import: %v", err)
	}

	// Verify secrets are accessible via secret get in the new vault.
	for _, kv := range []struct{ key, val string }{
		{"API_KEY", "sk-test-123"},
		{"DB_PASS", "s3cret"},
	} {
		root := NewRootCmd()
		getBuf := new(bytes.Buffer)
		root.SetOut(getBuf)
		root.SetErr(new(bytes.Buffer))
		root.SetArgs([]string{"secret", "get", kv.key})
		if err := root.Execute(); err != nil {
			t.Fatalf("secret get %s: %v", kv.key, err)
		}
		if getBuf.String() != kv.val+"\n" {
			t.Errorf("secret get %s: got %q, want %q", kv.key, getBuf.String(), kv.val+"\n")
		}
	}
}

func TestVaultImportCmd_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	initVaultForTest(t, dir)

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"vault", "import", "--file", filepath.Join(dir, "nonexistent.json")})
	err := root.Execute()
	if err == nil {
		t.Fatal("vault import with missing file should fail")
	}
}

func TestVaultImportCmd_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	initVaultForTest(t, dir)

	invalidPath := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(invalidPath, []byte("not json"), 0o600); err != nil {
		t.Fatalf("write invalid file: %v", err)
	}

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"vault", "import", "--file", invalidPath})
	err := root.Execute()
	if err == nil {
		t.Fatal("vault import with invalid JSON should fail")
	}
}

func TestVaultCmd_HelpShowsExportImport(t *testing.T) {
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
	if !bytes.Contains([]byte(got), []byte("Export vault secrets")) {
		t.Errorf("expected 'Export vault secrets' in help output, got: %q", got)
	}
	if !bytes.Contains([]byte(got), []byte("Import secrets")) {
		t.Errorf("expected 'Import secrets' in help output, got: %q", got)
	}
}
