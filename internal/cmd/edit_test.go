package cmd

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

// =============================================================================
// Tests for the "envref edit" command.
// =============================================================================

func TestEditCmd_Help(t *testing.T) {
	stdout, _, err := execCmd(t, "edit", "--help")
	if err != nil {
		t.Fatalf("edit --help: %v", err)
	}

	expected := []string{
		"Open a .env file in your default editor",
		"--local",
		"--config",
		"--profile",
		"$VISUAL",
		"$EDITOR",
	}
	for _, s := range expected {
		if !strings.Contains(stdout, s) {
			t.Errorf("edit --help: missing %q in output:\n%s", s, stdout)
		}
	}
}

func TestEditCmd_RegisteredInRoot(t *testing.T) {
	stdout, _, err := execCmd(t, "--help")
	if err != nil {
		t.Fatalf("root --help: %v", err)
	}

	if !strings.Contains(stdout, "edit") {
		t.Errorf("root help should list 'edit' command, got:\n%s", stdout)
	}
}

func TestEditCmd_NoConfig_Error(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_, _, err := execCmd(t, "edit")
	if err == nil {
		t.Fatal("expected error when no config exists, got nil")
	}
	if !strings.Contains(err.Error(), "loading config") {
		t.Errorf("expected config loading error, got: %v", err)
	}
}

func TestEditCmd_FileNotExist_Error(t *testing.T) {
	dir := setupProject(t, "testproject", "", "")
	chdir(t, dir)

	// The .env file was not created (empty string), so it shouldn't exist.
	_, _, err := execCmd(t, "edit")
	if err == nil {
		t.Fatal("expected error when .env file does not exist, got nil")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' error, got: %v", err)
	}
}

func TestEditCmd_LocalFileNotExist_Error(t *testing.T) {
	dir := setupProject(t, "testproject", "KEY=value\n", "")
	chdir(t, dir)

	// .env.local was not created.
	_, _, err := execCmd(t, "edit", "--local")
	if err == nil {
		t.Fatal("expected error when .env.local does not exist, got nil")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' error, got: %v", err)
	}
}

func TestEditCmd_ProfileFileNotExist_Error(t *testing.T) {
	dir := setupProject(t, "testproject", "KEY=value\n", "")
	chdir(t, dir)

	_, _, err := execCmd(t, "edit", "--profile", "staging")
	if err == nil {
		t.Fatal("expected error when profile file does not exist, got nil")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' error, got: %v", err)
	}
}

func TestEditCmd_ExplicitFileNotExist_Error(t *testing.T) {
	_, _, err := execCmd(t, "edit", "/nonexistent/file.env")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' error, got: %v", err)
	}
}

func TestEditCmd_MutuallyExclusiveFlags(t *testing.T) {
	_, _, err := execCmd(t, "edit", "--local", "--config")
	if err == nil {
		t.Fatal("expected error for mutually exclusive flags, got nil")
	}
}

func TestEditCmd_TooManyArgs(t *testing.T) {
	_, _, err := execCmd(t, "edit", "file1", "file2")
	if err == nil {
		t.Fatal("expected error for too many args, got nil")
	}
}

func TestEditorCommand_Visual(t *testing.T) {
	t.Setenv("VISUAL", "code")
	t.Setenv("EDITOR", "vim")

	got := editorCommand()
	if got != "code" {
		t.Errorf("expected VISUAL to take precedence, got %q", got)
	}
}

func TestEditorCommand_Editor(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "nano")

	got := editorCommand()
	if got != "nano" {
		t.Errorf("expected EDITOR fallback, got %q", got)
	}
}

func TestEditorCommand_DefaultVi(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")

	got := editorCommand()
	if got != "vi" {
		t.Errorf("expected vi default, got %q", got)
	}
}

func TestEditCmd_EditorNotFound_Error(t *testing.T) {
	dir := setupProject(t, "testproject", "KEY=value\n", "")
	chdir(t, dir)

	// Set EDITOR to a nonexistent binary.
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "nonexistent_editor_xyz_12345")

	_, _, err := execCmd(t, "edit")
	if err == nil {
		t.Fatal("expected error when editor not found, got nil")
	}
	if !strings.Contains(err.Error(), "not found in PATH") {
		t.Errorf("expected 'not found in PATH' error, got: %v", err)
	}
}

func TestEditCmd_ConfigFlag_OpensConfigFile(t *testing.T) {
	dir := setupProject(t, "testproject", "KEY=value\n", "")
	chdir(t, dir)

	// Set EDITOR to "true" which is a no-op binary that always succeeds.
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "true")

	_, _, err := execCmd(t, "edit", "--config")
	if err != nil {
		t.Fatalf("edit --config: %v", err)
	}
}

func TestEditCmd_DefaultFile_OpensEnvFile(t *testing.T) {
	dir := setupProject(t, "testproject", "KEY=value\n", "")
	chdir(t, dir)

	// Set EDITOR to "true" which is a no-op binary that always succeeds.
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "true")

	_, _, err := execCmd(t, "edit")
	if err != nil {
		t.Fatalf("edit default: %v", err)
	}
}

func TestEditCmd_LocalFlag_OpensLocalFile(t *testing.T) {
	dir := setupProject(t, "testproject", "KEY=value\n", "LOCAL=override\n")
	chdir(t, dir)

	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "true")

	_, _, err := execCmd(t, "edit", "--local")
	if err != nil {
		t.Fatalf("edit --local: %v", err)
	}
}

func TestEditCmd_ProfileFlag_OpensProfileFile(t *testing.T) {
	dir := setupProject(t, "testproject", "KEY=value\n", "")
	writeTestFile(t, dir, ".env.staging", "STAGING=true\n")
	chdir(t, dir)

	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "true")

	_, _, err := execCmd(t, "edit", "--profile", "staging")
	if err != nil {
		t.Fatalf("edit --profile staging: %v", err)
	}
}

func TestEditCmd_ExplicitFile_Opens(t *testing.T) {
	dir := t.TempDir()
	filePath := writeTestFile(t, dir, "custom.env", "CUSTOM=1\n")

	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "true")

	_, _, err := execCmd(t, "edit", filePath)
	if err != nil {
		t.Fatalf("edit explicit file: %v", err)
	}
}

func TestEditCmd_EditorReceivesFilePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses a shell script as editor")
	}
	dir := setupProject(t, "testproject", "KEY=value\n", "")
	chdir(t, dir)

	// Use a script that writes the received argument to a marker file.
	markerFile := dir + "/editor_called.txt"
	scriptPath := dir + "/test_editor.sh"
	scriptContent := "#!/bin/sh\necho \"$1\" > " + markerFile + "\n"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("writing test editor: %v", err)
	}

	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", scriptPath)

	_, _, err := execCmd(t, "edit")
	if err != nil {
		t.Fatalf("edit: %v", err)
	}

	data, err := os.ReadFile(markerFile)
	if err != nil {
		t.Fatalf("reading marker file: %v", err)
	}

	got := strings.TrimSpace(string(data))
	if !strings.HasSuffix(got, ".env") {
		t.Errorf("editor should receive .env path, got %q", got)
	}
}
