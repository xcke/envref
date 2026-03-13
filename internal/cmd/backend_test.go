package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestBackendListCmd_NoConfig(t *testing.T) {
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

	stdout := new(bytes.Buffer)
	root := NewRootCmd()
	root.SetOut(stdout)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"backend", "list"})

	err = root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !contains(out, "No .envref.yaml found") {
		t.Errorf("expected 'No .envref.yaml found', got: %q", out)
	}
	if !contains(out, "Supported backend types") {
		t.Errorf("expected 'Supported backend types', got: %q", out)
	}
	if !contains(out, "keychain") {
		t.Errorf("expected 'keychain' in supported types, got: %q", out)
	}
}

func TestBackendListCmd_WithBackends(t *testing.T) {
	dir := t.TempDir()

	cfgContent := "project: testproject\nbackends:\n  - name: keychain\n    type: keychain\n  - name: vault\n    type: hashicorp-vault\n"
	if err := os.WriteFile(filepath.Join(dir, ".envref.yaml"), []byte(cfgContent), 0o644); err != nil {
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

	stdout := new(bytes.Buffer)
	root := NewRootCmd()
	root.SetOut(stdout)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"backend", "list"})

	err = root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !contains(out, "Configured backends") {
		t.Errorf("expected 'Configured backends', got: %q", out)
	}
	if !contains(out, "keychain") {
		t.Errorf("expected 'keychain' in output, got: %q", out)
	}
	if !contains(out, "hashicorp-vault") {
		t.Errorf("expected 'hashicorp-vault' in output, got: %q", out)
	}
	// Without --all, should NOT show supported types table.
	if contains(out, "Supported backend types") {
		t.Errorf("did not expect 'Supported backend types' without --all, got: %q", out)
	}
}

func TestBackendListCmd_WithAllFlag(t *testing.T) {
	dir := t.TempDir()

	cfgContent := "project: testproject\nbackends:\n  - name: keychain\n    type: keychain\n"
	if err := os.WriteFile(filepath.Join(dir, ".envref.yaml"), []byte(cfgContent), 0o644); err != nil {
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

	stdout := new(bytes.Buffer)
	root := NewRootCmd()
	root.SetOut(stdout)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"backend", "list", "--all"})

	err = root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !contains(out, "Configured backends") {
		t.Errorf("expected 'Configured backends', got: %q", out)
	}
	if !contains(out, "Supported backend types") {
		t.Errorf("expected 'Supported backend types' with --all, got: %q", out)
	}
	if !contains(out, "1password") {
		t.Errorf("expected '1password' in supported types, got: %q", out)
	}
	if !contains(out, "aws-ssm") {
		t.Errorf("expected 'aws-ssm' in supported types, got: %q", out)
	}
}

func TestBackendListCmd_NoBackendsConfigured(t *testing.T) {
	dir := t.TempDir()

	// Config exists but has no backends.
	cfgContent := "project: testproject\n"
	if err := os.WriteFile(filepath.Join(dir, ".envref.yaml"), []byte(cfgContent), 0o644); err != nil {
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

	stdout := new(bytes.Buffer)
	root := NewRootCmd()
	root.SetOut(stdout)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"backend", "list"})

	err = root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	// Config exists but no backends — should show supported types.
	if !contains(out, "Supported backend types") {
		t.Errorf("expected 'Supported backend types' when no backends configured, got: %q", out)
	}
}

func TestBackendCmd_Help(t *testing.T) {
	stdout := new(bytes.Buffer)
	root := NewRootCmd()
	root.SetOut(stdout)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"backend", "--help"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := stdout.String()
	if !contains(out, "List and inspect configured secret backends") {
		t.Errorf("expected backend description in help, got: %q", out)
	}
	if !contains(out, "list") {
		t.Errorf("expected 'list' subcommand in help, got: %q", out)
	}
}
