package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xcke/envref/internal/config"
)

// =============================================================================
// Integration tests for envref CLI commands.
//
// These tests exercise end-to-end workflows that span multiple commands,
// verifying that commands work together correctly in realistic project setups.
// =============================================================================

// --- Helpers -----------------------------------------------------------------

// execCmd creates a new root command, sets up output capture, executes the
// command with the given args, and returns stdout, stderr, and any error.
func execCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := NewRootCmd()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

// execCmdWithStdin creates a root command with stdin provided, executes it,
// and returns stdout, stderr, and any error.
func execCmdWithStdin(t *testing.T, stdin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := NewRootCmd()
	outBuf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	root.SetIn(bytes.NewBufferString(stdin))
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

// setupProject creates a temp directory with an .envref.yaml and optional env
// files. Returns the directory path. Caller must chdir if needed.
func setupProject(t *testing.T, project string, envContent, localContent string) string {
	t.Helper()
	dir := t.TempDir()

	// Write .envref.yaml.
	cfgContent := "project: " + project + "\nenv_file: .env\nlocal_file: .env.local\n"
	writeTestFile(t, dir, config.FullFileName, cfgContent)

	if envContent != "" {
		writeTestFile(t, dir, ".env", envContent)
	}
	if localContent != "" {
		writeTestFile(t, dir, ".env.local", localContent)
	}

	return dir
}

// chdir changes to the specified directory and returns a cleanup function
// that restores the original directory.
func chdir(t *testing.T, dir string) {
	t.Helper()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})
}

// --- Root / Version / Help ---------------------------------------------------

func TestIntegration_RootCmd_HasAllSubcommands(t *testing.T) {
	stdout, _, err := execCmd(t, "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedCmds := []string{"version", "get", "set", "list", "init", "secret", "resolve", "profile"}
	for _, cmd := range expectedCmds {
		if !strings.Contains(stdout, cmd) {
			t.Errorf("root help should list %q command, got:\n%s", cmd, stdout)
		}
	}
}

func TestIntegration_VersionCmd_OutputFormat(t *testing.T) {
	stdout, _, err := execCmd(t, "version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(stdout, "envref ") {
		t.Errorf("version output should start with 'envref ', got %q", stdout)
	}
	if !strings.HasSuffix(stdout, "\n") {
		t.Errorf("version output should end with newline, got %q", stdout)
	}
}

func TestIntegration_UnknownCommand_ReturnsError(t *testing.T) {
	_, _, err := execCmd(t, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown command, got nil")
	}
}

func TestIntegration_HelpFlags_AllCommands(t *testing.T) {
	commands := [][]string{
		{"--help"},
		{"version", "--help"},
		{"get", "--help"},
		{"set", "--help"},
		{"list", "--help"},
		{"init", "--help"},
		{"secret", "--help"},
		{"secret", "set", "--help"},
		{"secret", "get", "--help"},
		{"secret", "delete", "--help"},
		{"secret", "list", "--help"},
		{"resolve", "--help"},
		{"profile", "--help"},
		{"profile", "list", "--help"},
	}

	for _, args := range commands {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			stdout, _, err := execCmd(t, args...)
			if err != nil {
				t.Fatalf("unexpected error for %v: %v", args, err)
			}
			if stdout == "" {
				t.Errorf("expected help output for %v, got empty string", args)
			}
		})
	}
}

// --- Init Workflow -----------------------------------------------------------

func TestIntegration_InitThenGetSetList(t *testing.T) {
	// Full workflow: init a project, then use get/set/list with the generated files.
	dir := t.TempDir()

	// Step 1: init the project.
	stdout, _, err := execCmd(t, "init", "--dir", dir, "--project", "myapp")
	if err != nil {
		t.Fatalf("init: unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "Initialized envref project") {
		t.Errorf("init: expected success message, got:\n%s", stdout)
	}

	envPath := filepath.Join(dir, ".env")
	localPath := filepath.Join(dir, ".env.local")

	// Step 2: verify we can list the default env vars created by init.
	stdout, _, err = execCmd(t, "list", "--file", envPath, "--local-file", localPath)
	if err != nil {
		t.Fatalf("list after init: unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "APP_NAME=myapp") {
		t.Errorf("list after init: expected APP_NAME=myapp, got:\n%s", stdout)
	}

	// Step 3: get a specific key from the init-generated .env.
	stdout, _, err = execCmd(t, "get", "APP_PORT", "--file", envPath, "--local-file", localPath)
	if err != nil {
		t.Fatalf("get after init: unexpected error: %v", err)
	}
	if strings.TrimSpace(stdout) != "3000" {
		t.Errorf("get APP_PORT: expected 3000, got %q", strings.TrimSpace(stdout))
	}

	// Step 4: set a new value.
	stdout, _, err = execCmd(t, "set", "NEW_VAR=hello", "--file", envPath)
	if err != nil {
		t.Fatalf("set after init: unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "NEW_VAR=hello") {
		t.Errorf("set output: expected NEW_VAR=hello, got %q", stdout)
	}

	// Step 5: verify the new value appears in list.
	stdout, _, err = execCmd(t, "list", "--file", envPath, "--local-file", localPath)
	if err != nil {
		t.Fatalf("list after set: unexpected error: %v", err)
	}
	if !strings.Contains(stdout, "NEW_VAR=hello") {
		t.Errorf("list after set: expected NEW_VAR=hello, got:\n%s", stdout)
	}

	// Step 6: verify get returns the new value.
	stdout, _, err = execCmd(t, "get", "NEW_VAR", "--file", envPath, "--local-file", localPath)
	if err != nil {
		t.Fatalf("get after set: unexpected error: %v", err)
	}
	if strings.TrimSpace(stdout) != "hello" {
		t.Errorf("get NEW_VAR: expected hello, got %q", strings.TrimSpace(stdout))
	}
}

func TestIntegration_InitThenReinit_SkipsExisting(t *testing.T) {
	dir := t.TempDir()

	// First init.
	_, _, err := execCmd(t, "init", "--dir", dir, "--project", "myapp")
	if err != nil {
		t.Fatalf("first init: %v", err)
	}

	// Second init — should skip existing files.
	stdout, _, err := execCmd(t, "init", "--dir", dir, "--project", "myapp")
	if err != nil {
		t.Fatalf("second init: %v", err)
	}

	skipCount := strings.Count(stdout, "skip")
	if skipCount < 3 {
		t.Errorf("second init should skip at least 3 files, got %d skips:\n%s", skipCount, stdout)
	}
}

func TestIntegration_InitForce_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()

	// First init with project "alpha".
	_, _, err := execCmd(t, "init", "--dir", dir, "--project", "alpha")
	if err != nil {
		t.Fatalf("first init: %v", err)
	}

	// Re-init with --force and project "beta".
	stdout, _, err := execCmd(t, "init", "--dir", dir, "--project", "beta", "--force")
	if err != nil {
		t.Fatalf("force init: %v", err)
	}

	// The main project files should be recreated (not skipped).
	if !strings.Contains(stdout, "create .envref.yaml") {
		t.Errorf("force init should recreate .envref.yaml:\n%s", stdout)
	}
	if !strings.Contains(stdout, "create .env") {
		t.Errorf("force init should recreate .env:\n%s", stdout)
	}

	// Note: .gitignore may still show "skip" since --force only affects
	// the project files, not the gitignore idempotency check.

	// Verify config was overwritten.
	cfgData, err := os.ReadFile(filepath.Join(dir, config.FullFileName))
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	if !strings.Contains(string(cfgData), "project: beta") {
		t.Errorf("config should have project 'beta' after force init, got:\n%s", cfgData)
	}
}

func TestIntegration_InitDirenv_GeneratesEnvrc(t *testing.T) {
	dir := t.TempDir()

	stdout, _, err := execCmd(t, "init", "--dir", dir, "--project", "myapp", "--direnv")
	if err != nil {
		t.Fatalf("init --direnv: %v", err)
	}

	// Verify .envrc exists with the eval line.
	envrcData, err := os.ReadFile(filepath.Join(dir, ".envrc"))
	if err != nil {
		t.Fatalf("reading .envrc: %v", err)
	}
	if !strings.Contains(string(envrcData), "envref resolve --direnv") {
		t.Errorf(".envrc should contain envref resolve command, got:\n%s", envrcData)
	}

	// Verify output mentions direnv allow.
	if !strings.Contains(stdout, "direnv allow") {
		t.Errorf("output should mention 'direnv allow':\n%s", stdout)
	}
}

func TestIntegration_InitConfigIsLoadable(t *testing.T) {
	dir := t.TempDir()

	_, _, err := execCmd(t, "init", "--dir", dir, "--project", "loadable-project")
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	// Load and validate the generated config.
	cfg, err := config.LoadFile(filepath.Join(dir, config.FullFileName))
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if cfg.Project != "loadable-project" {
		t.Errorf("project: got %q, want %q", cfg.Project, "loadable-project")
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("config validation failed: %v", err)
	}
}

// --- Get / Set / List Workflows ----------------------------------------------

func TestIntegration_SetThenGet_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	localPath := filepath.Join(dir, ".env.local")

	// Set a value (creates the file).
	_, _, err := execCmd(t, "set", "DATABASE_URL=postgres://localhost:5432/mydb", "--file", envPath)
	if err != nil {
		t.Fatalf("set: %v", err)
	}

	// Get the value back.
	stdout, _, err := execCmd(t, "get", "DATABASE_URL", "--file", envPath, "--local-file", localPath)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if strings.TrimSpace(stdout) != "postgres://localhost:5432/mydb" {
		t.Errorf("get: expected postgres://localhost:5432/mydb, got %q", strings.TrimSpace(stdout))
	}
}

func TestIntegration_SetMultipleThenList(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	localPath := filepath.Join(dir, ".env.local")

	// Set multiple values.
	pairs := []string{"HOST=localhost", "PORT=5432", "DB=myapp"}
	for _, pair := range pairs {
		_, _, err := execCmd(t, "set", pair, "--file", envPath)
		if err != nil {
			t.Fatalf("set %s: %v", pair, err)
		}
	}

	// List all values.
	stdout, _, err := execCmd(t, "list", "--file", envPath, "--local-file", localPath)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	for _, pair := range pairs {
		if !strings.Contains(stdout, pair) {
			t.Errorf("list: expected %q in output, got:\n%s", pair, stdout)
		}
	}
}

func TestIntegration_SetUpdatePreservesOrder(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "A=1\nB=2\nC=3\n")
	localPath := filepath.Join(dir, ".env.local")

	// Update B.
	_, _, err := execCmd(t, "set", "B=updated", "--file", envPath)
	if err != nil {
		t.Fatalf("set: %v", err)
	}

	// List should show updated B in its original position.
	stdout, _, err := execCmd(t, "list", "--file", envPath, "--local-file", localPath)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	expected := "A=1\nB=updated\nC=3\n"
	if stdout != expected {
		t.Errorf("list after update: got %q, want %q", stdout, expected)
	}
}

func TestIntegration_LocalOverridesPrecedence(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "HOST=prod.example.com\nPORT=443\n")
	localPath := writeTestFile(t, dir, ".env.local", "HOST=localhost\n")

	// Get HOST should return the local override.
	stdout, _, err := execCmd(t, "get", "HOST", "--file", envPath, "--local-file", localPath)
	if err != nil {
		t.Fatalf("get HOST: %v", err)
	}
	if strings.TrimSpace(stdout) != "localhost" {
		t.Errorf("expected localhost, got %q", strings.TrimSpace(stdout))
	}

	// Get PORT should return the base value (not overridden).
	stdout, _, err = execCmd(t, "get", "PORT", "--file", envPath, "--local-file", localPath)
	if err != nil {
		t.Fatalf("get PORT: %v", err)
	}
	if strings.TrimSpace(stdout) != "443" {
		t.Errorf("expected 443, got %q", strings.TrimSpace(stdout))
	}

	// List should show the merged result.
	stdout, _, err = execCmd(t, "list", "--file", envPath, "--local-file", localPath)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(stdout, "HOST=localhost") {
		t.Errorf("list should show local override, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "PORT=443") {
		t.Errorf("list should show base PORT, got:\n%s", stdout)
	}
}

func TestIntegration_SetToLocal_IsolatesFiles(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "HOST=prod\n")
	localPath := filepath.Join(dir, ".env.local")

	// Set a secret to local file.
	_, _, err := execCmd(t, "set", "SECRET=mysecret", "--file", envPath, "--local-file", localPath, "--local")
	if err != nil {
		t.Fatalf("set --local: %v", err)
	}

	// Verify .env was not modified.
	envData, _ := os.ReadFile(envPath)
	if strings.Contains(string(envData), "SECRET") {
		t.Error(".env should not contain SECRET")
	}

	// Verify .env.local has the secret.
	localData, _ := os.ReadFile(localPath)
	if !strings.Contains(string(localData), "SECRET=mysecret") {
		t.Errorf(".env.local should contain SECRET=mysecret, got:\n%s", localData)
	}

	// Get should return the merged value.
	stdout, _, err := execCmd(t, "get", "SECRET", "--file", envPath, "--local-file", localPath)
	if err != nil {
		t.Fatalf("get SECRET: %v", err)
	}
	if strings.TrimSpace(stdout) != "mysecret" {
		t.Errorf("expected mysecret, got %q", strings.TrimSpace(stdout))
	}
}

func TestIntegration_VariableInterpolation(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\nDB_PORT=5432\nDB_NAME=myapp\nDB_URL=postgres://${DB_HOST}:${DB_PORT}/${DB_NAME}\n")
	localPath := filepath.Join(dir, ".env.local")

	// Get should return the interpolated value.
	stdout, _, err := execCmd(t, "get", "DB_URL", "--file", envPath, "--local-file", localPath)
	if err != nil {
		t.Fatalf("get DB_URL: %v", err)
	}
	expected := "postgres://localhost:5432/myapp"
	if strings.TrimSpace(stdout) != expected {
		t.Errorf("expected %q, got %q", expected, strings.TrimSpace(stdout))
	}
}

func TestIntegration_InterpolationWithLocalOverride(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "HOST=prod\nPORT=443\nURL=https://${HOST}:${PORT}\n")
	localPath := writeTestFile(t, dir, ".env.local", "HOST=localhost\nPORT=8080\n")

	// Get should use local overrides for interpolation.
	stdout, _, err := execCmd(t, "get", "URL", "--file", envPath, "--local-file", localPath)
	if err != nil {
		t.Fatalf("get URL: %v", err)
	}
	expected := "https://localhost:8080"
	if strings.TrimSpace(stdout) != expected {
		t.Errorf("expected %q, got %q", expected, strings.TrimSpace(stdout))
	}
}

func TestIntegration_RefValuesInList_MaskedByDefault(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "PORT=8080\nAPI_KEY=ref://secrets/api_key\nDB_PASS=ref://keychain/db_pass\n")
	localPath := filepath.Join(dir, ".env.local")

	// List without --show-secrets should mask ref values.
	stdout, _, err := execCmd(t, "list", "--file", envPath, "--local-file", localPath)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(stdout, "PORT=8080") {
		t.Error("non-ref value should be shown")
	}
	if !strings.Contains(stdout, "API_KEY=ref://***") {
		t.Errorf("ref value should be masked, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "DB_PASS=ref://***") {
		t.Errorf("ref value should be masked, got:\n%s", stdout)
	}

	// List with --show-secrets should reveal ref values.
	stdout, _, err = execCmd(t, "list", "--show-secrets", "--file", envPath, "--local-file", localPath)
	if err != nil {
		t.Fatalf("list --show-secrets: %v", err)
	}
	if !strings.Contains(stdout, "API_KEY=ref://secrets/api_key") {
		t.Errorf("--show-secrets should reveal ref values, got:\n%s", stdout)
	}
}

func TestIntegration_GetSetGet_ValuePersistence(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "KEY=original\n")
	localPath := filepath.Join(dir, ".env.local")

	// Verify original value.
	stdout, _, err := execCmd(t, "get", "KEY", "--file", envPath, "--local-file", localPath)
	if err != nil {
		t.Fatalf("get original: %v", err)
	}
	if strings.TrimSpace(stdout) != "original" {
		t.Errorf("expected original, got %q", strings.TrimSpace(stdout))
	}

	// Update value.
	_, _, err = execCmd(t, "set", "KEY=updated", "--file", envPath)
	if err != nil {
		t.Fatalf("set: %v", err)
	}

	// Verify updated value.
	stdout, _, err = execCmd(t, "get", "KEY", "--file", envPath, "--local-file", localPath)
	if err != nil {
		t.Fatalf("get updated: %v", err)
	}
	if strings.TrimSpace(stdout) != "updated" {
		t.Errorf("expected updated, got %q", strings.TrimSpace(stdout))
	}
}

// --- Error Handling ----------------------------------------------------------

func TestIntegration_GetMissingKey_Error(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "A=1\n")

	_, _, err := execCmd(t, "get", "MISSING", "--file", envPath, "--local-file", filepath.Join(dir, ".env.local"))
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestIntegration_GetMissingFile_Error(t *testing.T) {
	dir := t.TempDir()

	_, _, err := execCmd(t, "get", "KEY", "--file", filepath.Join(dir, "nonexistent.env"), "--local-file", filepath.Join(dir, ".env.local"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestIntegration_SetInvalidFormat_Error(t *testing.T) {
	_, _, err := execCmd(t, "set", "NOEQUALS")
	if err == nil {
		t.Fatal("expected error for invalid KEY=VALUE format")
	}
	if !strings.Contains(err.Error(), "KEY=VALUE") {
		t.Errorf("error should mention KEY=VALUE format, got: %v", err)
	}
}

func TestIntegration_SetEmptyKey_Error(t *testing.T) {
	_, _, err := execCmd(t, "set", "=value")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
	if !strings.Contains(err.Error(), "empty key") {
		t.Errorf("error should mention empty key, got: %v", err)
	}
}

func TestIntegration_CommandArgValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"get no args", []string{"get"}},
		{"set no args", []string{"set"}},
		{"init extra args", []string{"init", "extra"}},
		{"list extra args", []string{"list", "extra"}},
		{"secret set no args", []string{"secret", "set"}},
		{"secret get no args", []string{"secret", "get"}},
		{"secret delete no args", []string{"secret", "delete"}},
		{"secret list extra args", []string{"secret", "list", "extra"}},
		{"resolve extra args", []string{"resolve", "extra"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := execCmd(t, tt.args...)
			if err == nil {
				t.Fatalf("expected error for %v, got nil", tt.args)
			}
		})
	}
}

// --- Secret Commands (validation paths) --------------------------------------
// These tests verify command structure and validation without requiring an actual
// keychain backend, since OS keychain may not be available in CI.

func TestIntegration_SecretSet_NoConfig(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_, _, err := execCmd(t, "secret", "set", "API_KEY", "--value", "test")
	if err == nil {
		t.Fatal("expected error when no config found")
	}
	if !strings.Contains(err.Error(), "loading config") {
		t.Errorf("expected config loading error, got: %v", err)
	}
}

func TestIntegration_SecretGet_NoConfig(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_, _, err := execCmd(t, "secret", "get", "API_KEY")
	if err == nil {
		t.Fatal("expected error when no config found")
	}
	if !strings.Contains(err.Error(), "loading config") {
		t.Errorf("expected config loading error, got: %v", err)
	}
}

func TestIntegration_SecretDelete_NoConfig(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_, _, err := execCmd(t, "secret", "delete", "API_KEY", "--force")
	if err == nil {
		t.Fatal("expected error when no config found")
	}
	if !strings.Contains(err.Error(), "loading config") {
		t.Errorf("expected config loading error, got: %v", err)
	}
}

func TestIntegration_SecretList_NoConfig(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_, _, err := execCmd(t, "secret", "list")
	if err == nil {
		t.Fatal("expected error when no config found")
	}
	if !strings.Contains(err.Error(), "loading config") {
		t.Errorf("expected config loading error, got: %v", err)
	}
}

func TestIntegration_SecretCommands_NoBackends(t *testing.T) {
	dir := t.TempDir()
	// Config with no backends.
	writeTestFile(t, dir, config.FullFileName, "project: testproject\n")
	chdir(t, dir)

	tests := []struct {
		name string
		args []string
	}{
		{"set", []string{"secret", "set", "KEY", "--value", "val"}},
		{"get", []string{"secret", "get", "KEY"}},
		{"delete", []string{"secret", "delete", "KEY", "--force"}},
		{"list", []string{"secret", "list"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := execCmd(t, tt.args...)
			if err == nil {
				t.Fatalf("expected error for %s with no backends", tt.name)
			}
			if !strings.Contains(err.Error(), "no backends configured") {
				t.Errorf("expected 'no backends configured' error, got: %v", err)
			}
		})
	}
}

func TestIntegration_SecretCommands_InvalidBackend(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "testproject")
	chdir(t, dir)

	tests := []struct {
		name string
		args []string
	}{
		{"set", []string{"secret", "set", "KEY", "--value", "val", "--backend", "nonexistent"}},
		{"get", []string{"secret", "get", "KEY", "--backend", "nonexistent"}},
		{"delete", []string{"secret", "delete", "KEY", "--force", "--backend", "nonexistent"}},
		{"list", []string{"secret", "list", "--backend", "nonexistent"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := execCmd(t, tt.args...)
			if err == nil {
				t.Fatalf("expected error for %s with invalid backend", tt.name)
			}
			if !strings.Contains(err.Error(), "nonexistent") {
				t.Errorf("expected error mentioning backend name, got: %v", err)
			}
		})
	}
}

func TestIntegration_SecretDelete_ConfirmCancel(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "testproject")
	chdir(t, dir)

	// Provide "n" as stdin to cancel the deletion.
	stdout, stderr, err := execCmdWithStdin(t, "n\n", "secret", "delete", "API_KEY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should show no output on stdout (no deletion happened).
	if stdout != "" {
		t.Errorf("expected no stdout on cancellation, got %q", stdout)
	}

	// Should show cancellation message on stderr.
	if !strings.Contains(stderr, "deletion cancelled") {
		t.Errorf("expected cancellation message in stderr, got %q", stderr)
	}
}

func TestIntegration_SecretSet_EmptyStdin(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "testproject")
	chdir(t, dir)

	// Provide empty stdin (no value).
	_, _, err := execCmdWithStdin(t, "", "secret", "set", "API_KEY")
	if err == nil {
		t.Fatal("expected error for empty stdin")
	}
	if !strings.Contains(err.Error(), "no input provided") {
		t.Errorf("expected 'no input provided' error, got: %v", err)
	}
}

func TestIntegration_SecretSet_BlankLineStdin(t *testing.T) {
	dir := t.TempDir()
	writeTestConfig(t, dir, "testproject")
	chdir(t, dir)

	// Provide blank line as stdin.
	_, _, err := execCmdWithStdin(t, "\n", "secret", "set", "API_KEY")
	if err == nil {
		t.Fatal("expected error for blank line stdin")
	}
	if !strings.Contains(err.Error(), "secret value must not be empty") {
		t.Errorf("expected 'empty value' error, got: %v", err)
	}
}

// --- Resolve Command (no-ref paths and error paths) --------------------------

func TestIntegration_Resolve_NoRefs_PlainOutput(t *testing.T) {
	dir := setupProject(t, "testproject", "HOST=localhost\nPORT=5432\n", "")
	chdir(t, dir)

	stdout, _, err := execCmd(t, "resolve")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	expected := "HOST=localhost\nPORT=5432\n"
	if stdout != expected {
		t.Errorf("resolve: got %q, want %q", stdout, expected)
	}
}

func TestIntegration_Resolve_NoRefs_DirenvOutput(t *testing.T) {
	dir := setupProject(t, "testproject", "HOST=localhost\nPORT=5432\n", "")
	chdir(t, dir)

	stdout, _, err := execCmd(t, "resolve", "--direnv")
	if err != nil {
		t.Fatalf("resolve --direnv: %v", err)
	}

	expected := "export HOST=localhost\nexport PORT=5432\n"
	if stdout != expected {
		t.Errorf("resolve --direnv: got %q, want %q", stdout, expected)
	}
}

func TestIntegration_Resolve_NoRefs_WithInterpolation(t *testing.T) {
	dir := setupProject(t, "testproject", "HOST=localhost\nPORT=5432\nURL=http://${HOST}:${PORT}\n", "")
	chdir(t, dir)

	stdout, _, err := execCmd(t, "resolve")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if !strings.Contains(stdout, "URL=http://localhost:5432") {
		t.Errorf("resolve should interpolate variables, got:\n%s", stdout)
	}
}

func TestIntegration_Resolve_NoRefs_WithLocalOverride(t *testing.T) {
	dir := setupProject(t, "testproject", "HOST=prod\nPORT=443\n", "HOST=localhost\nPORT=8080\n")
	chdir(t, dir)

	stdout, _, err := execCmd(t, "resolve")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if !strings.Contains(stdout, "HOST=localhost") {
		t.Errorf("resolve should use local override, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "PORT=8080") {
		t.Errorf("resolve should use local override, got:\n%s", stdout)
	}
}

func TestIntegration_Resolve_NoConfig_Error(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_, _, err := execCmd(t, "resolve")
	if err == nil {
		t.Fatal("expected error when no config found")
	}
	if !strings.Contains(err.Error(), "loading config") {
		t.Errorf("expected config loading error, got: %v", err)
	}
}

func TestIntegration_Resolve_WithRefs_NoBackends_Error(t *testing.T) {
	dir := t.TempDir()
	// Config with no backends.
	writeTestFile(t, dir, config.FullFileName, "project: testproject\n")
	writeTestFile(t, dir, ".env", "API_KEY=ref://secrets/api_key\n")
	chdir(t, dir)

	_, _, err := execCmd(t, "resolve")
	if err == nil {
		t.Fatal("expected error for refs with no backends")
	}
	if !strings.Contains(err.Error(), "no backends configured") {
		t.Errorf("expected 'no backends configured' error, got: %v", err)
	}
}

func TestIntegration_Resolve_DirenvQuoting(t *testing.T) {
	dir := setupProject(t, "testproject", "SIMPLE=hello\nSPACES=hello world\nEMPTY=\n", "")
	chdir(t, dir)

	stdout, _, err := execCmd(t, "resolve", "--direnv")
	if err != nil {
		t.Fatalf("resolve --direnv: %v", err)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	expectedLines := []string{
		"export SIMPLE=hello",
		"export SPACES='hello world'",
		"export EMPTY=''",
	}

	if len(lines) != len(expectedLines) {
		t.Fatalf("expected %d lines, got %d: %q", len(expectedLines), len(lines), stdout)
	}
	for i, want := range expectedLines {
		if lines[i] != want {
			t.Errorf("line %d: got %q, want %q", i, lines[i], want)
		}
	}
}

// --- Full Init-to-Resolve Workflow -------------------------------------------

func TestIntegration_FullWorkflow_InitSetResolve(t *testing.T) {
	dir := t.TempDir()

	// Step 1: init the project.
	_, _, err := execCmd(t, "init", "--dir", dir, "--project", "fulltest")
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	envPath := filepath.Join(dir, ".env")
	localPath := filepath.Join(dir, ".env.local")

	// Step 2: set additional values.
	_, _, err = execCmd(t, "set", "DB_HOST=localhost", "--file", envPath)
	if err != nil {
		t.Fatalf("set DB_HOST: %v", err)
	}
	_, _, err = execCmd(t, "set", "DB_PORT=5432", "--file", envPath)
	if err != nil {
		t.Fatalf("set DB_PORT: %v", err)
	}

	// Step 3: list all values — should include both init defaults and new values.
	stdout, _, err := execCmd(t, "list", "--file", envPath, "--local-file", localPath)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, expected := range []string{"APP_NAME=myapp", "DB_HOST=localhost", "DB_PORT=5432"} {
		if !strings.Contains(stdout, expected) {
			t.Errorf("list: expected %q, got:\n%s", expected, stdout)
		}
	}

	// Step 4: resolve (no refs, so should pass without backends).
	chdir(t, dir)
	stdout, _, err = execCmd(t, "resolve")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !strings.Contains(stdout, "DB_HOST=localhost") {
		t.Errorf("resolve should include DB_HOST, got:\n%s", stdout)
	}
}

func TestIntegration_FullWorkflow_SetLocalOverrideResolve(t *testing.T) {
	dir := setupProject(t, "testproject",
		"HOST=prod.example.com\nPORT=443\nURL=https://${HOST}:${PORT}/api\n",
		"")
	chdir(t, dir)

	// Set local overrides.
	envPath := filepath.Join(dir, ".env")
	localPath := filepath.Join(dir, ".env.local")
	_, _, err := execCmd(t, "set", "HOST=localhost", "--file", envPath, "--local-file", localPath, "--local")
	if err != nil {
		t.Fatalf("set --local HOST: %v", err)
	}
	_, _, err = execCmd(t, "set", "PORT=8080", "--file", envPath, "--local-file", localPath, "--local")
	if err != nil {
		t.Fatalf("set --local PORT: %v", err)
	}

	// Resolve should use local overrides and interpolate.
	stdout, _, err := execCmd(t, "resolve")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if !strings.Contains(stdout, "HOST=localhost") {
		t.Errorf("resolve should use local HOST override, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "PORT=8080") {
		t.Errorf("resolve should use local PORT override, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "URL=https://localhost:8080/api") {
		t.Errorf("resolve should interpolate with local overrides, got:\n%s", stdout)
	}
}

// --- Edge Cases --------------------------------------------------------------

func TestIntegration_SetGet_SpecialCharacters(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	localPath := filepath.Join(dir, ".env.local")

	tests := []struct {
		name  string
		key   string
		value string
	}{
		{"url with query", "URL", "https://example.com/api?key=val&other=2"},
		{"empty value", "EMPTY", ""},
		{"numeric", "COUNT", "42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := execCmd(t, "set", tt.key+"="+tt.value, "--file", envPath)
			if err != nil {
				t.Fatalf("set: %v", err)
			}

			stdout, _, err := execCmd(t, "get", tt.key, "--file", envPath, "--local-file", localPath)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			got := strings.TrimSpace(stdout)
			if got != tt.value {
				t.Errorf("roundtrip: got %q, want %q", got, tt.value)
			}
		})
	}
}

func TestIntegration_ListEmpty_NoOutput(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "# only comments\n\n")
	localPath := filepath.Join(dir, ".env.local")

	stdout, _, err := execCmd(t, "list", "--file", envPath, "--local-file", localPath)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty output for comments-only .env, got %q", stdout)
	}
}

func TestIntegration_SetGet_QuotedValues(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	localPath := filepath.Join(dir, ".env.local")

	// Set a value with spaces (gets auto-quoted in the file).
	_, _, err := execCmd(t, "set", "GREETING=hello world", "--file", envPath)
	if err != nil {
		t.Fatalf("set: %v", err)
	}

	// Get should return the unquoted value.
	stdout, _, err := execCmd(t, "get", "GREETING", "--file", envPath, "--local-file", localPath)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if strings.TrimSpace(stdout) != "hello world" {
		t.Errorf("expected 'hello world', got %q", strings.TrimSpace(stdout))
	}
}

func TestIntegration_ConfigDiscovery_FromSubdirectory(t *testing.T) {
	dir := setupProject(t, "testproject", "HOST=localhost\nPORT=5432\n", "")

	// Create a subdirectory and chdir to it.
	subdir := filepath.Join(dir, "src", "app")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	chdir(t, subdir)

	// Resolve should find the config in the parent directory.
	stdout, _, err := execCmd(t, "resolve")
	if err != nil {
		t.Fatalf("resolve from subdirectory: %v", err)
	}
	if !strings.Contains(stdout, "HOST=localhost") {
		t.Errorf("resolve should find config from subdirectory, got:\n%s", stdout)
	}
}

func TestIntegration_Resolve_PreservesKeyOrder(t *testing.T) {
	dir := setupProject(t, "testproject", "Z_LAST=1\nA_FIRST=2\nM_MIDDLE=3\n", "")
	chdir(t, dir)

	stdout, _, err := execCmd(t, "resolve")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), stdout)
	}
	// Keys should preserve file order, not be sorted alphabetically.
	if !strings.HasPrefix(lines[0], "Z_LAST=") {
		t.Errorf("first line should be Z_LAST, got %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "A_FIRST=") {
		t.Errorf("second line should be A_FIRST, got %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "M_MIDDLE=") {
		t.Errorf("third line should be M_MIDDLE, got %q", lines[2])
	}
}

func TestIntegration_Resolve_EmptyEnv(t *testing.T) {
	dir := setupProject(t, "testproject", "# comments only\n\n", "")
	chdir(t, dir)

	stdout, _, err := execCmd(t, "resolve")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if stdout != "" {
		t.Errorf("expected empty output, got %q", stdout)
	}
}

func TestIntegration_Resolve_DirenvEmptyValues(t *testing.T) {
	dir := setupProject(t, "testproject", "KEY=\n", "")
	chdir(t, dir)

	stdout, _, err := execCmd(t, "resolve", "--direnv")
	if err != nil {
		t.Fatalf("resolve --direnv: %v", err)
	}
	if !strings.Contains(stdout, "export KEY=''") {
		t.Errorf("empty value should be quoted in direnv output, got:\n%s", stdout)
	}
}

func TestIntegration_InitThenResolve_ConfigLoads(t *testing.T) {
	dir := t.TempDir()

	// Init creates the config and env files.
	_, _, err := execCmd(t, "init", "--dir", dir, "--project", "configtest")
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	// Change to the project dir and resolve.
	chdir(t, dir)
	stdout, _, err := execCmd(t, "resolve")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// Should output the default env vars from init.
	if !strings.Contains(stdout, "APP_NAME=myapp") {
		t.Errorf("resolve should output init defaults, got:\n%s", stdout)
	}
}

// --- Profile Support ---------------------------------------------------------

func TestIntegration_GetWithProfileFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".env", "HOST=prod.example.com\nPORT=443\n")
	writeTestFile(t, dir, ".env.staging", "HOST=staging.example.com\nPORT=8443\n")

	envPath := filepath.Join(dir, ".env")
	profilePath := filepath.Join(dir, ".env.staging")
	localPath := filepath.Join(dir, ".env.local")

	// Get HOST should return the profile value (profile overrides base).
	stdout, _, err := execCmd(t, "get", "HOST",
		"--file", envPath,
		"--profile-file", profilePath,
		"--local-file", localPath)
	if err != nil {
		t.Fatalf("get HOST with profile: %v", err)
	}
	if strings.TrimSpace(stdout) != "staging.example.com" {
		t.Errorf("expected staging.example.com, got %q", strings.TrimSpace(stdout))
	}
}

func TestIntegration_GetWithProfileFile_LocalWins(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".env", "HOST=prod.example.com\n")
	writeTestFile(t, dir, ".env.staging", "HOST=staging.example.com\n")
	writeTestFile(t, dir, ".env.local", "HOST=localhost\n")

	envPath := filepath.Join(dir, ".env")
	profilePath := filepath.Join(dir, ".env.staging")
	localPath := filepath.Join(dir, ".env.local")

	// Local should win over profile.
	stdout, _, err := execCmd(t, "get", "HOST",
		"--file", envPath,
		"--profile-file", profilePath,
		"--local-file", localPath)
	if err != nil {
		t.Fatalf("get HOST: %v", err)
	}
	if strings.TrimSpace(stdout) != "localhost" {
		t.Errorf("expected localhost (local wins), got %q", strings.TrimSpace(stdout))
	}
}

func TestIntegration_ListWithProfileFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".env", "HOST=prod\nPORT=443\n")
	writeTestFile(t, dir, ".env.staging", "HOST=staging\nDB_HOST=staging-db\n")

	envPath := filepath.Join(dir, ".env")
	profilePath := filepath.Join(dir, ".env.staging")
	localPath := filepath.Join(dir, ".env.local")

	stdout, _, err := execCmd(t, "list",
		"--file", envPath,
		"--profile-file", profilePath,
		"--local-file", localPath)
	if err != nil {
		t.Fatalf("list with profile: %v", err)
	}

	// HOST should be overridden by profile.
	if !strings.Contains(stdout, "HOST=staging") {
		t.Errorf("expected HOST=staging, got:\n%s", stdout)
	}
	// PORT should remain from base.
	if !strings.Contains(stdout, "PORT=443") {
		t.Errorf("expected PORT=443, got:\n%s", stdout)
	}
	// DB_HOST added by profile.
	if !strings.Contains(stdout, "DB_HOST=staging-db") {
		t.Errorf("expected DB_HOST=staging-db, got:\n%s", stdout)
	}
}

func TestIntegration_ResolveWithProfile(t *testing.T) {
	dir := t.TempDir()

	// Config with active_profile.
	cfgContent := `project: testproject
active_profile: staging
profiles:
  staging:
    env_file: .env.staging
  production:
    env_file: .env.production
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "HOST=prod\nPORT=443\n")
	writeTestFile(t, dir, ".env.staging", "HOST=staging.example.com\nSTAGING_ONLY=true\n")
	chdir(t, dir)

	stdout, _, err := execCmd(t, "resolve")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if !strings.Contains(stdout, "HOST=staging.example.com") {
		t.Errorf("resolve should use staging profile, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "PORT=443") {
		t.Errorf("resolve should keep base PORT, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "STAGING_ONLY=true") {
		t.Errorf("resolve should include staging-only var, got:\n%s", stdout)
	}
}

func TestIntegration_ResolveWithProfileFlag_OverridesConfig(t *testing.T) {
	dir := t.TempDir()

	// Config with active_profile=staging, but we'll override with --profile=production.
	cfgContent := `project: testproject
active_profile: staging
profiles:
  staging:
    env_file: .env.staging
  production:
    env_file: .env.production
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "HOST=default\n")
	writeTestFile(t, dir, ".env.staging", "HOST=staging\n")
	writeTestFile(t, dir, ".env.production", "HOST=production\n")
	chdir(t, dir)

	stdout, _, err := execCmd(t, "resolve", "--profile", "production")
	if err != nil {
		t.Fatalf("resolve --profile production: %v", err)
	}

	if !strings.Contains(stdout, "HOST=production") {
		t.Errorf("--profile flag should override config active_profile, got:\n%s", stdout)
	}
}

func TestIntegration_ResolveWithProfile_LocalWins(t *testing.T) {
	dir := t.TempDir()

	cfgContent := `project: testproject
active_profile: staging
profiles:
  staging:
    env_file: .env.staging
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "HOST=prod\nPORT=443\n")
	writeTestFile(t, dir, ".env.staging", "HOST=staging\nPORT=8443\n")
	writeTestFile(t, dir, ".env.local", "HOST=localhost\n")
	chdir(t, dir)

	stdout, _, err := execCmd(t, "resolve")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// Local should win: .env ← .env.staging ← .env.local.
	if !strings.Contains(stdout, "HOST=localhost") {
		t.Errorf("local should override profile, got:\n%s", stdout)
	}
	// PORT from staging (not overridden by local).
	if !strings.Contains(stdout, "PORT=8443") {
		t.Errorf("PORT should come from staging profile, got:\n%s", stdout)
	}
}

func TestIntegration_ResolveWithProfile_MissingProfileFile(t *testing.T) {
	dir := t.TempDir()

	// Profile file doesn't exist — should work (profile is optional like .env.local).
	cfgContent := `project: testproject
profiles:
  staging:
    env_file: .env.staging
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "HOST=prod\nPORT=443\n")
	chdir(t, dir)

	stdout, _, err := execCmd(t, "resolve", "--profile", "staging")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// Should fall through to base values since profile file is missing.
	if !strings.Contains(stdout, "HOST=prod") {
		t.Errorf("should use base value when profile file missing, got:\n%s", stdout)
	}
}

func TestIntegration_ResolveWithProfile_ConventionNaming(t *testing.T) {
	dir := t.TempDir()

	// Profile not defined in profiles map; should still use .env.<name> convention.
	cfgContent := `project: testproject
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "HOST=default\n")
	writeTestFile(t, dir, ".env.staging", "HOST=staging\n")
	chdir(t, dir)

	stdout, _, err := execCmd(t, "resolve", "--profile", "staging")
	if err != nil {
		t.Fatalf("resolve --profile staging: %v", err)
	}

	if !strings.Contains(stdout, "HOST=staging") {
		t.Errorf("convention-named profile should work, got:\n%s", stdout)
	}
}

func TestIntegration_ResolveWithProfile_DirenvOutput(t *testing.T) {
	dir := t.TempDir()

	cfgContent := `project: testproject
active_profile: staging
profiles:
  staging:
    env_file: .env.staging
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "HOST=prod\n")
	writeTestFile(t, dir, ".env.staging", "HOST=staging host\n")
	chdir(t, dir)

	stdout, _, err := execCmd(t, "resolve", "--direnv")
	if err != nil {
		t.Fatalf("resolve --direnv: %v", err)
	}

	if !strings.Contains(stdout, "export HOST='staging host'") {
		t.Errorf("direnv output should quote profile values, got:\n%s", stdout)
	}
}

// --- Strict Mode Tests -------------------------------------------------------

func TestIntegration_Resolve_Strict_NoRefs_OutputProduced(t *testing.T) {
	dir := setupProject(t, "testproject", "HOST=localhost\nPORT=5432\n", "")
	chdir(t, dir)

	stdout, stderr, err := execCmd(t, "resolve", "--strict")
	if err != nil {
		t.Fatalf("resolve --strict with no refs should succeed, got: %v", err)
	}

	expected := "HOST=localhost\nPORT=5432\n"
	if stdout != expected {
		t.Errorf("resolve --strict: got %q, want %q", stdout, expected)
	}
	if stderr != "" {
		t.Errorf("resolve --strict: unexpected stderr: %q", stderr)
	}
}

func TestIntegration_Resolve_Strict_WithFailedRefs_NoOutput(t *testing.T) {
	dir := t.TempDir()
	cfgContent := "project: testproject\nbackends:\n  - name: keychain\n    type: keychain\n"
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "HOST=localhost\nAPI_KEY=ref://keychain/api_key\n")
	chdir(t, dir)

	// The keychain backend will fail to find the secret (no mock store set up
	// at the cmd level), so the ref will be unresolved.
	stdout, stderr, err := execCmd(t, "resolve", "--strict")
	if err == nil {
		t.Fatal("resolve --strict with failed refs should return error")
	}

	// Strict mode: stdout should be empty (no partial output).
	if stdout != "" {
		t.Errorf("resolve --strict: expected empty stdout, got %q", stdout)
	}

	// Error message should mention strict mode.
	if !strings.Contains(err.Error(), "strict mode") {
		t.Errorf("error should mention strict mode, got: %v", err)
	}

	// Stderr should contain the per-key error.
	if !strings.Contains(stderr, "API_KEY") {
		t.Errorf("stderr should report API_KEY error, got: %q", stderr)
	}
}

func TestIntegration_Resolve_NonStrict_WithFailedRefs_PartialOutput(t *testing.T) {
	dir := t.TempDir()
	cfgContent := "project: testproject\nbackends:\n  - name: keychain\n    type: keychain\n"
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "HOST=localhost\nAPI_KEY=ref://keychain/api_key\n")
	chdir(t, dir)

	// Without --strict, partial output should be produced.
	stdout, stderr, err := execCmd(t, "resolve")
	if err == nil {
		t.Fatal("resolve with failed refs should return error")
	}

	// Non-strict: stdout should contain partial output (the resolved HOST and the unresolved ref).
	if !strings.Contains(stdout, "HOST=localhost") {
		t.Errorf("resolve without strict: should output resolved keys, got %q", stdout)
	}

	// Error message should NOT mention strict mode.
	if strings.Contains(err.Error(), "strict mode") {
		t.Errorf("error should not mention strict mode without --strict, got: %v", err)
	}

	// Stderr should contain the per-key error.
	if !strings.Contains(stderr, "API_KEY") {
		t.Errorf("stderr should report API_KEY error, got: %q", stderr)
	}
}

func TestIntegration_Resolve_Strict_WithDirenv_NoOutput(t *testing.T) {
	dir := t.TempDir()
	cfgContent := "project: testproject\nbackends:\n  - name: keychain\n    type: keychain\n"
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "HOST=localhost\nSECRET=ref://keychain/secret\n")
	chdir(t, dir)

	stdout, _, err := execCmd(t, "resolve", "--strict", "--direnv")
	if err == nil {
		t.Fatal("resolve --strict --direnv with failed refs should return error")
	}

	// Strict mode: no output even when --direnv is requested.
	if stdout != "" {
		t.Errorf("resolve --strict --direnv: expected empty stdout, got %q", stdout)
	}
}

func TestIntegration_Resolve_Strict_HelpText(t *testing.T) {
	stdout, _, err := execCmd(t, "resolve", "--help")
	if err != nil {
		t.Fatalf("resolve --help: %v", err)
	}

	if !strings.Contains(stdout, "--strict") {
		t.Error("resolve help should mention --strict flag")
	}
}

func TestIntegration_ResolveWithProfile_InterpolationAcrossLayers(t *testing.T) {
	dir := t.TempDir()

	cfgContent := `project: testproject
profiles:
  staging:
    env_file: .env.staging
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "DB_HOST=default-db\nDB_PORT=5432\nDB_URL=postgres://${DB_HOST}:${DB_PORT}/app\n")
	writeTestFile(t, dir, ".env.staging", "DB_HOST=staging-db\n")
	chdir(t, dir)

	stdout, _, err := execCmd(t, "resolve", "--profile", "staging")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// DB_URL should interpolate using the staging DB_HOST.
	if !strings.Contains(stdout, "DB_URL=postgres://staging-db:5432/app") {
		t.Errorf("interpolation should use profile overrides, got:\n%s", stdout)
	}
}
