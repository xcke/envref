package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestOnboardCmd_NoConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"onboard"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing config")
	}
	if !strings.Contains(err.Error(), "loading config") {
		t.Errorf("expected config error, got: %v", err)
	}
}

func TestOnboardCmd_NoBackends(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", "project: myapp\n")
	writeTestFile(t, dir, ".env", "APP_NAME=myapp\n")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"onboard"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for no backends")
	}
	if !strings.Contains(err.Error(), "no backends configured") {
		t.Errorf("expected no backends error, got: %v", err)
	}
}

func TestOnboardCmd_NoEnvFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", "project: myapp\nbackends:\n  - name: keychain\n")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"onboard"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing env file")
	}
	if !strings.Contains(err.Error(), "no .env file found") {
		t.Errorf("expected no .env error, got: %v", err)
	}
}

func TestOnboardCmd_AllResolved(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", "project: myapp\nbackends:\n  - name: keychain\n")
	writeTestFile(t, dir, ".env", "DB_HOST=localhost\nDB_PORT=5432\n")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"onboard"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "All secrets are resolved") {
		t.Errorf("expected all resolved message, got %q", out)
	}
}

func TestOnboardCmd_DryRun(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", "project: myapp\nbackends:\n  - name: keychain\n")
	writeTestFile(t, dir, ".env", "DB_HOST=localhost\nAPI_KEY=ref://secrets/api_key\nDB_PASS=ref://secrets/db_pass\n")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"onboard", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Missing secrets:") {
		t.Errorf("expected missing secrets header, got %q", out)
	}
	if !strings.Contains(out, "API_KEY") {
		t.Errorf("expected API_KEY in output, got %q", out)
	}
	if !strings.Contains(out, "DB_PASS") {
		t.Errorf("expected DB_PASS in output, got %q", out)
	}
	if !strings.Contains(out, "[dry-run]") {
		t.Errorf("expected dry-run label, got %q", out)
	}
}

func TestOnboardCmd_DryRunWithExampleMissing(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", "project: myapp\nbackends:\n  - name: keychain\n")
	writeTestFile(t, dir, ".env", "DB_HOST=localhost\n")
	writeTestFile(t, dir, ".env.example", "DB_HOST=localhost\nCACHE_URL=redis://localhost\n")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"onboard", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Missing from .env.example:") {
		t.Errorf("expected missing from example header, got %q", out)
	}
	if !strings.Contains(out, "CACHE_URL") {
		t.Errorf("expected CACHE_URL in output, got %q", out)
	}
}

func TestOnboardCmd_InteractiveSkip(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", "project: myapp\nbackends:\n  - name: keychain\n")
	writeTestFile(t, dir, ".env", "DB_HOST=localhost\nAPI_KEY=ref://secrets/api_key\n")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Simulate user pressing Enter (empty input = skip).
	root := NewRootCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(errBuf)
	root.SetIn(strings.NewReader("\n"))
	root.SetArgs([]string{"onboard"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "skipped") {
		t.Errorf("expected skipped message, got %q", out)
	}
	if !strings.Contains(out, "Skipped:") {
		t.Errorf("expected Skipped summary, got %q", out)
	}
}

func TestOnboardCmd_ShowsProjectInfo(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", "project: myapp\nbackends:\n  - name: keychain\n")
	writeTestFile(t, dir, ".env", "DB_HOST=localhost\n")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"onboard"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Project: myapp") {
		t.Errorf("expected project name, got %q", out)
	}
	if !strings.Contains(out, "Backend: keychain") {
		t.Errorf("expected backend name, got %q", out)
	}
}

func TestOnboardCmd_WithProfile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", `project: myapp
backends:
  - name: keychain
profiles:
  staging:
    env_file: .env.staging
`)
	writeTestFile(t, dir, ".env", "DB_HOST=localhost\nAPI_KEY=ref://secrets/api_key\n")
	writeTestFile(t, dir, ".env.staging", "DB_HOST=staging-host\n")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"onboard", "--profile", "staging", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Profile: staging") {
		t.Errorf("expected profile info, got %q", out)
	}
	if !strings.Contains(out, "API_KEY") {
		t.Errorf("expected API_KEY in output, got %q", out)
	}
}

func TestOnboardCmd_RejectsArgs(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", "project: myapp\nbackends:\n  - name: keychain\n")
	writeTestFile(t, dir, ".env", "DB_HOST=localhost\n")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"onboard", "extra-arg"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for extra args")
	}
}

func TestOnboardCmd_NoRefsOnlyExampleMissing(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", "project: myapp\nbackends:\n  - name: keychain\n")
	writeTestFile(t, dir, ".env", "DB_HOST=localhost\n")
	writeTestFile(t, dir, ".env.example", "DB_HOST=localhost\nAPI_KEY=ref://secrets/api_key\n")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"onboard", "--dry-run"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// Should show the missing keys from example.
	if !strings.Contains(out, "API_KEY") {
		t.Errorf("expected API_KEY from example, got %q", out)
	}
}

func TestFindMissingExampleKeys_NoExampleFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", "project: myapp\nbackends:\n  - name: keychain\n")
	writeTestFile(t, dir, ".env", "DB_HOST=localhost\n")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"onboard"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// No .env.example so should show all resolved.
	if !strings.Contains(out, "All secrets are resolved") {
		t.Errorf("expected all resolved, got %q", out)
	}
}
