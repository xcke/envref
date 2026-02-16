package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xcke/envref/internal/config"
)

func TestInitCmd_CreatesAllFiles(t *testing.T) {
	dir := t.TempDir()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"init", "--dir", dir, "--project", "myapp"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify .envref.yaml was created.
	cfgPath := filepath.Join(dir, config.FullFileName)
	cfgData, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading %s: %v", cfgPath, err)
	}
	if !strings.Contains(string(cfgData), "project: myapp") {
		t.Errorf(".envref.yaml should contain 'project: myapp', got:\n%s", cfgData)
	}

	// Verify .env was created.
	envData, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("reading .env: %v", err)
	}
	if !strings.Contains(string(envData), "APP_NAME=myapp") {
		t.Errorf(".env should contain 'APP_NAME=myapp', got:\n%s", envData)
	}

	// Verify .env.local was created.
	localData, err := os.ReadFile(filepath.Join(dir, ".env.local"))
	if err != nil {
		t.Fatalf("reading .env.local: %v", err)
	}
	if !strings.Contains(string(localData), "Local overrides") {
		t.Errorf(".env.local should contain comment, got:\n%s", localData)
	}

	// Verify .gitignore was created with .env.local entry.
	giData, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}
	if !strings.Contains(string(giData), ".env.local") {
		t.Errorf(".gitignore should contain '.env.local', got:\n%s", giData)
	}

	// Verify no .envrc created (no --direnv).
	if _, err := os.Stat(filepath.Join(dir, ".envrc")); err == nil {
		t.Error(".envrc should not be created without --direnv flag")
	}

	// Verify output messages.
	output := buf.String()
	if !strings.Contains(output, "create .envref.yaml") {
		t.Errorf("output should mention creating .envref.yaml: %s", output)
	}
	if !strings.Contains(output, "Initialized envref project") {
		t.Errorf("output should contain success message: %s", output)
	}
}

func TestInitCmd_DirenvFlag(t *testing.T) {
	dir := t.TempDir()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(errBuf)
	root.SetArgs([]string{"init", "--dir", dir, "--project", "myapp", "--direnv"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify .envrc was created.
	envrcData, err := os.ReadFile(filepath.Join(dir, ".envrc"))
	if err != nil {
		t.Fatalf("reading .envrc: %v", err)
	}
	if !strings.Contains(string(envrcData), "envref resolve --direnv") {
		t.Errorf(".envrc should contain envref resolve command, got:\n%s", envrcData)
	}

	// Verify output mentions direnv allow.
	output := buf.String()
	if !strings.Contains(output, "direnv allow") {
		t.Errorf("output should mention 'direnv allow': %s", output)
	}
}

func TestInitCmd_DirenvNotInstalled(t *testing.T) {
	dir := t.TempDir()

	// Ensure direnv is not in PATH for this test.
	t.Setenv("PATH", dir)

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(errBuf)
	root.SetArgs([]string{"init", "--dir", dir, "--project", "myapp", "--direnv"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// .envrc should still be created.
	if _, err := os.Stat(filepath.Join(dir, ".envrc")); err != nil {
		t.Fatalf(".envrc should exist: %v", err)
	}

	// Stdout should tell user to run direnv allow manually.
	output := buf.String()
	if !strings.Contains(output, "direnv allow") {
		t.Errorf("output should mention 'direnv allow': %s", output)
	}

	// Stderr should warn about direnv not being installed.
	errOutput := errBuf.String()
	if !strings.Contains(errOutput, "direnv is not installed") {
		t.Errorf("stderr should warn about direnv not being installed: %s", errOutput)
	}
	if !strings.Contains(errOutput, "https://direnv.net") {
		t.Errorf("stderr should include direnv install URL: %s", errOutput)
	}
}

func TestInitCmd_SkipsExistingFiles(t *testing.T) {
	dir := t.TempDir()

	// Create existing files.
	existingContent := "existing content\n"
	writeTestFile(t, dir, config.FullFileName, existingContent)
	writeTestFile(t, dir, ".env", existingContent)

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"init", "--dir", dir, "--project", "myapp"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify existing files were not overwritten.
	cfgData, _ := os.ReadFile(filepath.Join(dir, config.FullFileName))
	if string(cfgData) != existingContent {
		t.Errorf(".envref.yaml should not be overwritten, got:\n%s", cfgData)
	}

	envData, _ := os.ReadFile(filepath.Join(dir, ".env"))
	if string(envData) != existingContent {
		t.Errorf(".env should not be overwritten, got:\n%s", envData)
	}

	// Verify output mentions skipping.
	output := buf.String()
	if !strings.Contains(output, "skip .envref.yaml") {
		t.Errorf("output should mention skipping .envref.yaml: %s", output)
	}
	if !strings.Contains(output, "skip .env") {
		t.Errorf("output should mention skipping .env: %s", output)
	}
}

func TestInitCmd_ForceOverwritesFiles(t *testing.T) {
	dir := t.TempDir()

	// Create existing file.
	writeTestFile(t, dir, config.FullFileName, "old content\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"init", "--dir", dir, "--project", "myapp", "--force"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file was overwritten.
	cfgData, _ := os.ReadFile(filepath.Join(dir, config.FullFileName))
	if !strings.Contains(string(cfgData), "project: myapp") {
		t.Errorf(".envref.yaml should be overwritten with new content, got:\n%s", cfgData)
	}

	output := buf.String()
	if !strings.Contains(output, "create .envref.yaml") {
		t.Errorf("output should mention creating .envref.yaml: %s", output)
	}
}

func TestInitCmd_DefaultProjectName(t *testing.T) {
	dir := t.TempDir()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"init", "--dir", dir})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Project name should default to directory basename.
	dirName := filepath.Base(dir)
	cfgData, _ := os.ReadFile(filepath.Join(dir, config.FullFileName))
	if !strings.Contains(string(cfgData), "project: "+dirName) {
		t.Errorf(".envref.yaml should use dir name as project, got:\n%s", cfgData)
	}
}

func TestInitCmd_AppendToExistingGitignore(t *testing.T) {
	dir := t.TempDir()

	// Create existing .gitignore without .env.local.
	writeTestFile(t, dir, ".gitignore", "node_modules/\n*.log\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"init", "--dir", dir, "--project", "myapp"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	giData, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	content := string(giData)

	// Should preserve existing entries.
	if !strings.Contains(content, "node_modules/") {
		t.Error(".gitignore should preserve existing entries")
	}

	// Should add .env.local.
	if !strings.Contains(content, ".env.local") {
		t.Errorf(".gitignore should contain '.env.local', got:\n%s", content)
	}

	output := buf.String()
	if !strings.Contains(output, "update .gitignore") {
		t.Errorf("output should mention updating .gitignore: %s", output)
	}
}

func TestInitCmd_SkipGitignoreEntryIfPresent(t *testing.T) {
	dir := t.TempDir()

	// Create .gitignore that already has .env.local.
	writeTestFile(t, dir, ".gitignore", ".env.local\nnode_modules/\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"init", "--dir", dir, "--project", "myapp"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify .gitignore was not modified (no duplicate).
	giData, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	count := strings.Count(string(giData), ".env.local")
	if count != 1 {
		t.Errorf(".env.local should appear once in .gitignore, found %d times", count)
	}

	output := buf.String()
	if !strings.Contains(output, "skip .gitignore") {
		t.Errorf("output should mention skipping .gitignore: %s", output)
	}
}

func TestInitCmd_RejectsArgs(t *testing.T) {
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"init", "extra-arg"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for extra arguments, got nil")
	}
}

func TestInitCmd_QuietSuppressesOutput(t *testing.T) {
	dir := t.TempDir()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"--quiet", "init", "--dir", dir, "--project", "myapp"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// In quiet mode, no informational output should be shown.
	output := buf.String()
	if strings.Contains(output, "create") {
		t.Errorf("quiet mode should suppress file creation messages, got: %s", output)
	}
	if strings.Contains(output, "Initialized") {
		t.Errorf("quiet mode should suppress initialization message, got: %s", output)
	}

	// But the files should still be created.
	if _, err := os.Stat(filepath.Join(dir, config.FullFileName)); err != nil {
		t.Errorf(".envref.yaml should still be created in quiet mode: %v", err)
	}
}

func TestInitCmd_ConfigIsLoadable(t *testing.T) {
	dir := t.TempDir()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"init", "--dir", dir, "--project", "test-project"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the generated config is valid and loadable.
	cfg, err := config.LoadFile(filepath.Join(dir, config.FullFileName))
	if err != nil {
		t.Fatalf("loading generated config: %v", err)
	}

	if cfg.Project != "test-project" {
		t.Errorf("project: got %q, want %q", cfg.Project, "test-project")
	}
	if cfg.EnvFile != ".env" {
		t.Errorf("env_file: got %q, want %q", cfg.EnvFile, ".env")
	}
	if cfg.LocalFile != ".env.local" {
		t.Errorf("local_file: got %q, want %q", cfg.LocalFile, ".env.local")
	}

	// Config should pass validation.
	if err := cfg.Validate(); err != nil {
		t.Errorf("generated config should be valid: %v", err)
	}
}
