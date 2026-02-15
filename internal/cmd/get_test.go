package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
	return path
}

func TestGetCmd_BasicLookup(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\nDB_PORT=5432\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"get", "DB_HOST", "--file", envPath, "--local-file", filepath.Join(dir, ".env.local")})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if got != "localhost\n" {
		t.Errorf("expected %q, got %q", "localhost\n", got)
	}
}

func TestGetCmd_WithLocalOverride(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\nDB_PORT=5432\n")
	localPath := writeTestFile(t, dir, ".env.local", "DB_HOST=127.0.0.1\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"get", "DB_HOST", "--file", envPath, "--local-file", localPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if got != "127.0.0.1\n" {
		t.Errorf("expected %q, got %q", "127.0.0.1\n", got)
	}
}

func TestGetCmd_RefValue(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "API_KEY=ref://secrets/api_key\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"get", "API_KEY", "--file", envPath, "--local-file", filepath.Join(dir, ".env.local")})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	expected := "ref://secrets/api_key\n"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestGetCmd_KeyNotFound(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\n")

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"get", "MISSING_KEY", "--file", envPath, "--local-file", filepath.Join(dir, ".env.local")})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
}

func TestGetCmd_MissingEnvFile(t *testing.T) {
	dir := t.TempDir()

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"get", "DB_HOST", "--file", filepath.Join(dir, "nonexistent.env"), "--local-file", filepath.Join(dir, ".env.local")})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing env file, got nil")
	}
}

func TestGetCmd_NoArgs(t *testing.T) {
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"get"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing argument, got nil")
	}
}

func TestGetCmd_QuotedValue(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", `GREETING="hello world"`)

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"get", "GREETING", "--file", envPath, "--local-file", filepath.Join(dir, ".env.local")})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if got != "hello world\n" {
		t.Errorf("expected %q, got %q", "hello world\n", got)
	}
}

func TestGetCmd_MissingLocalFileIsOptional(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "PORT=8080\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"get", "PORT", "--file", envPath, "--local-file", filepath.Join(dir, ".env.local")})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if got != "8080\n" {
		t.Errorf("expected %q, got %q", "8080\n", got)
	}
}
