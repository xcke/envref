package cmd

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestListCmd_BasicList(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\nDB_PORT=5432\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"list", "--file", envPath, "--local-file", filepath.Join(dir, ".env.local")})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "DB_HOST=localhost\nDB_PORT=5432\n"
	if got := buf.String(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestListCmd_MasksRefsByDefault(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "PORT=8080\nAPI_KEY=ref://secrets/api_key\nDB_PASS=ref://keychain/db_pass\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"list", "--file", envPath, "--local-file", filepath.Join(dir, ".env.local")})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "PORT=8080\nAPI_KEY=ref://***\nDB_PASS=ref://***\n"
	if got := buf.String(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestListCmd_ShowSecrets(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "PORT=8080\nAPI_KEY=ref://secrets/api_key\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"list", "--show-secrets", "--file", envPath, "--local-file", filepath.Join(dir, ".env.local")})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "PORT=8080\nAPI_KEY=ref://secrets/api_key\n"
	if got := buf.String(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestListCmd_WithLocalOverride(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\nDB_PORT=5432\n")
	localPath := writeTestFile(t, dir, ".env.local", "DB_HOST=127.0.0.1\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"list", "--file", envPath, "--local-file", localPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "DB_HOST=127.0.0.1\nDB_PORT=5432\n"
	if got := buf.String(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestListCmd_EmptyEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "# only comments\n\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"list", "--file", envPath, "--local-file", filepath.Join(dir, ".env.local")})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := buf.String(); got != "" {
		t.Errorf("expected empty output, got %q", got)
	}
}

func TestListCmd_MissingEnvFile(t *testing.T) {
	dir := t.TempDir()

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"list", "--file", filepath.Join(dir, "nonexistent.env"), "--local-file", filepath.Join(dir, ".env.local")})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing env file, got nil")
	}
}

func TestListCmd_MissingLocalFileIsOptional(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "PORT=8080\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"list", "--file", envPath, "--local-file", filepath.Join(dir, ".env.local")})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "PORT=8080\n"
	if got := buf.String(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestListCmd_RejectsArguments(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "PORT=8080\n")

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"list", "extra", "--file", envPath})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for unexpected argument, got nil")
	}
}
