package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestStatusCmd_NoConfig(t *testing.T) {
	// Run from a temp dir with no .envref.yaml.
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
	root.SetArgs([]string{"status"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "No project configured") {
		t.Errorf("expected 'No project configured', got %q", out)
	}
	if !strings.Contains(out, "envref init") {
		t.Errorf("expected init hint, got %q", out)
	}
}

func TestStatusCmd_ConfigOnly(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", "project: myapp\nbackends:\n  - name: keychain\n")
	writeTestFile(t, dir, ".env", "DB_HOST=localhost\nDB_PORT=5432\nAPP_NAME=myapp\n")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"status"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Project: myapp") {
		t.Errorf("expected 'Project: myapp', got %q", out)
	}
	if !strings.Contains(out, "3 keys") {
		t.Errorf("expected '3 keys', got %q", out)
	}
	if !strings.Contains(out, "3 config") {
		t.Errorf("expected '3 config', got %q", out)
	}
	if !strings.Contains(out, "Status: OK") {
		t.Errorf("expected 'Status: OK', got %q", out)
	}
}

func TestStatusCmd_WithRefs(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", "project: myapp\nbackends:\n  - name: keychain\n")
	writeTestFile(t, dir, ".env", "DB_HOST=localhost\nAPI_KEY=ref://secrets/api_key\nDB_PASS=ref://keychain/db_pass\n")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"status"})

	// This will fail to resolve refs (no actual keychain in test),
	// but should still show the status report structure.
	_ = root.Execute()

	out := buf.String()
	if !strings.Contains(out, "3 keys") {
		t.Errorf("expected '3 keys', got %q", out)
	}
	if !strings.Contains(out, "1 config") {
		t.Errorf("expected '1 config', got %q", out)
	}
	if !strings.Contains(out, "2 secrets") {
		t.Errorf("expected '2 secrets', got %q", out)
	}
	if !strings.Contains(out, "Backends: keychain") {
		t.Errorf("expected 'Backends: keychain', got %q", out)
	}
}

func TestStatusCmd_NoEnvFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", "project: myapp\n")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"status"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[--] .env") {
		t.Errorf("expected missing .env indicator, got %q", out)
	}
	if !strings.Contains(out, "envref init") {
		t.Errorf("expected init hint, got %q", out)
	}
}

func TestStatusCmd_WithExample(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", "project: myapp\n")
	writeTestFile(t, dir, ".env", "DB_HOST=localhost\n")
	writeTestFile(t, dir, ".env.example", "DB_HOST=localhost\nDB_PORT=5432\nAPI_KEY=ref://secrets/key\n")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"status"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Missing from .env.example") {
		t.Errorf("expected missing keys section, got %q", out)
	}
	if !strings.Contains(out, "DB_PORT") {
		t.Errorf("expected DB_PORT in missing, got %q", out)
	}
	if !strings.Contains(out, "API_KEY") {
		t.Errorf("expected API_KEY in missing, got %q", out)
	}
	if !strings.Contains(out, "issue(s) found") {
		t.Errorf("expected issues found, got %q", out)
	}
}

func TestStatusCmd_ValidationOK(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", "project: myapp\n")
	writeTestFile(t, dir, ".env", "DB_HOST=localhost\nDB_PORT=5432\n")
	writeTestFile(t, dir, ".env.example", "DB_HOST=localhost\nDB_PORT=5432\n")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"status"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "OK: all keys match") {
		t.Errorf("expected validation OK, got %q", out)
	}
	if !strings.Contains(out, "Status: OK") {
		t.Errorf("expected 'Status: OK', got %q", out)
	}
}

func TestStatusCmd_WithProfile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", "project: myapp\nactive_profile: staging\nprofiles:\n  staging:\n    env_file: .env.staging\n")
	writeTestFile(t, dir, ".env", "DB_HOST=localhost\n")
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
	root.SetArgs([]string{"status"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Profile: staging") {
		t.Errorf("expected 'Profile: staging', got %q", out)
	}
	if !strings.Contains(out, ".env.staging") {
		t.Errorf("expected .env.staging in files, got %q", out)
	}
}

func TestStatusCmd_ProfileOverrideFlag(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", "project: myapp\nprofiles:\n  production:\n    env_file: .env.production\n")
	writeTestFile(t, dir, ".env", "DB_HOST=localhost\n")
	writeTestFile(t, dir, ".env.production", "DB_HOST=prod-host\n")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"status", "--profile", "production"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Profile: production") {
		t.Errorf("expected 'Profile: production', got %q", out)
	}
}

func TestStatusCmd_MissingProfileFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", "project: myapp\n")
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
	root.SetArgs([]string{"status", "--profile", "staging"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "does not exist") {
		t.Errorf("expected hint about missing profile file, got %q", out)
	}
}

func TestStatusCmd_NoBackendsWithRefs(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", "project: myapp\n")
	writeTestFile(t, dir, ".env", "DB_HOST=localhost\nAPI_KEY=ref://secrets/api_key\n")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"status"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "no backends configured") {
		t.Errorf("expected backends warning, got %q", out)
	}
	if !strings.Contains(out, "issue(s) found") {
		t.Errorf("expected issues found, got %q", out)
	}
}

func TestStatusCmd_FileExistenceIndicators(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".envref.yaml", "project: myapp\n")
	writeTestFile(t, dir, ".env", "FOO=bar\n")
	writeTestFile(t, dir, ".env.local", "BAZ=qux\n")

	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"status"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "[ok] .env") {
		t.Errorf("expected [ok] for .env, got %q", out)
	}
	if !strings.Contains(out, "[ok] .env.local") {
		t.Errorf("expected [ok] for .env.local, got %q", out)
	}
	if !strings.Contains(out, "[--] .env.example") {
		t.Errorf("expected [--] for .env.example, got %q", out)
	}
}

func TestStatusCmd_RejectsArguments(t *testing.T) {
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"status", "extra-arg"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for unexpected argument, got nil")
	}
}
