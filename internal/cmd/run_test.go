package cmd

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"
)

// =============================================================================
// Tests for the "envref run" command.
// =============================================================================

func TestRunCmd_Help(t *testing.T) {
	stdout, _, err := execCmd(t, "run", "--help")
	if err != nil {
		t.Fatalf("run --help: %v", err)
	}

	expected := []string{
		"Resolve all environment variables",
		"--profile",
		"--strict",
		"envref run -- node server.js",
	}
	for _, s := range expected {
		if !strings.Contains(stdout, s) {
			t.Errorf("run --help: missing %q in output:\n%s", s, stdout)
		}
	}
}

func TestRunCmd_RegisteredInRoot(t *testing.T) {
	stdout, _, err := execCmd(t, "--help")
	if err != nil {
		t.Fatalf("root --help: %v", err)
	}

	if !strings.Contains(stdout, "run") {
		t.Errorf("root help should list 'run' command, got:\n%s", stdout)
	}
}

func TestRunCmd_NoArgs_Error(t *testing.T) {
	dir := setupProject(t, "testproject", "KEY=value\n", "")
	chdir(t, dir)

	_, _, err := execCmd(t, "run")
	if err == nil {
		t.Fatal("expected error when run has no args, got nil")
	}
}

func TestRunCmd_NoConfig_Error(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_, _, err := execCmd(t, "run", "--", "echo", "hello")
	if err == nil {
		t.Fatal("expected error when no config exists, got nil")
	}
}

func TestRunCmd_CommandNotFound_Error(t *testing.T) {
	dir := setupProject(t, "testproject", "KEY=value\n", "")
	chdir(t, dir)

	_, stderr, err := execCmd(t, "run", "--", "nonexistent_binary_xyz_12345")
	if err == nil {
		t.Fatal("expected error for missing command, got nil")
	}

	if !strings.Contains(err.Error(), "command not found") && !strings.Contains(stderr, "command not found") {
		t.Errorf("error should mention 'command not found', got: %v (stderr: %s)", err, stderr)
	}
}

func TestRunCmd_ExecutesCommand_InjectsEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}

	dir := setupProject(t, "testproject", "MY_TEST_VAR=hello_from_envref\nANOTHER=world\n", "")
	chdir(t, dir)

	// Create a small test script that prints the env var.
	scriptPath := dir + "/test_script.sh"
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho \"VAR=$MY_TEST_VAR\"\n"), 0o755); err != nil {
		t.Fatalf("writing test script: %v", err)
	}

	// We can't capture stdout through execCmd because run uses os.Stdout directly.
	// Instead, test that the command runs without error.
	// Use a script that writes to a file instead.
	outFile := dir + "/out.txt"
	scriptContent := "#!/bin/sh\necho \"$MY_TEST_VAR\" > " + outFile + "\n"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("writing test script: %v", err)
	}

	// Run the command using the root command directly.
	root := NewRootCmd()
	root.SetArgs([]string{"run", "--", "/bin/sh", scriptPath})
	err := root.Execute()
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// Read the output file.
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}

	got := strings.TrimSpace(string(data))
	if got != "hello_from_envref" {
		t.Errorf("expected MY_TEST_VAR=hello_from_envref, got %q", got)
	}
}

func TestRunCmd_LocalOverride_InjectsEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}

	dir := setupProject(t, "testproject", "APP=base\n", "APP=override\n")
	chdir(t, dir)

	outFile := dir + "/out.txt"
	scriptPath := dir + "/test_script.sh"
	scriptContent := "#!/bin/sh\necho \"$APP\" > " + outFile + "\n"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("writing test script: %v", err)
	}

	root := NewRootCmd()
	root.SetArgs([]string{"run", "--", "/bin/sh", scriptPath})
	if err := root.Execute(); err != nil {
		t.Fatalf("run: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}

	got := strings.TrimSpace(string(data))
	if got != "override" {
		t.Errorf("expected APP=override (.env.local wins), got %q", got)
	}
}

func TestRunCmd_WithProfile_InjectsEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}

	dir := setupProject(t, "testproject", "APP=base\n", "")
	chdir(t, dir)

	// Create a profile env file.
	writeTestFile(t, dir, ".env.staging", "APP=staging_value\n")

	outFile := dir + "/out.txt"
	scriptPath := dir + "/test_script.sh"
	scriptContent := "#!/bin/sh\necho \"$APP\" > " + outFile + "\n"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("writing test script: %v", err)
	}

	root := NewRootCmd()
	root.SetArgs([]string{"run", "--profile", "staging", "--", "/bin/sh", scriptPath})
	if err := root.Execute(); err != nil {
		t.Fatalf("run: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}

	got := strings.TrimSpace(string(data))
	if got != "staging_value" {
		t.Errorf("expected APP=staging_value (profile override), got %q", got)
	}
}

func TestRunCmd_PropagatesExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}

	dir := setupProject(t, "testproject", "KEY=value\n", "")
	chdir(t, dir)

	root := NewRootCmd()
	root.SetArgs([]string{"run", "--", "/bin/sh", "-c", "exit 42"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for non-zero exit code")
	}

	var exitErr *exitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected exitError, got %T: %v", err, err)
	}
	if exitErr.code != 42 {
		t.Errorf("expected exit code 42, got %d", exitErr.code)
	}
}

func TestRunCmd_PassesArgs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}

	dir := setupProject(t, "testproject", "KEY=value\n", "")
	chdir(t, dir)

	outFile := dir + "/out.txt"
	// The script writes all arguments to the output file.
	scriptPath := dir + "/test_args.sh"
	scriptContent := "#!/bin/sh\necho \"$@\" > " + outFile + "\n"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("writing test script: %v", err)
	}

	root := NewRootCmd()
	root.SetArgs([]string{"run", "--", "/bin/sh", scriptPath, "arg1", "arg2", "arg3"})
	if err := root.Execute(); err != nil {
		t.Fatalf("run: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}

	got := strings.TrimSpace(string(data))
	if got != "arg1 arg2 arg3" {
		t.Errorf("expected args 'arg1 arg2 arg3', got %q", got)
	}
}

func TestRunCmd_Interpolation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows: test uses /bin/sh")
	}

	dir := setupProject(t, "testproject", "HOST=localhost\nPORT=5432\nURL=http://${HOST}:${PORT}/db\n", "")
	chdir(t, dir)

	outFile := dir + "/out.txt"
	scriptPath := dir + "/test_script.sh"
	scriptContent := "#!/bin/sh\necho \"$URL\" > " + outFile + "\n"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0o755); err != nil {
		t.Fatalf("writing test script: %v", err)
	}

	root := NewRootCmd()
	root.SetArgs([]string{"run", "--", "/bin/sh", scriptPath})
	if err := root.Execute(); err != nil {
		t.Fatalf("run: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}

	got := strings.TrimSpace(string(data))
	expected := "http://localhost:5432/db"
	if got != expected {
		t.Errorf("expected URL=%q (interpolated), got %q", expected, got)
	}
}

// --- exitError tests ---------------------------------------------------------

func TestExitError_Error(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{0, "exit status 0"},
		{1, "exit status 1"},
		{42, "exit status 42"},
		{127, "exit status 127"},
	}

	for _, tt := range tests {
		e := &exitError{code: tt.code}
		if got := e.Error(); got != tt.want {
			t.Errorf("exitError{%d}.Error() = %q, want %q", tt.code, got, tt.want)
		}
	}
}
