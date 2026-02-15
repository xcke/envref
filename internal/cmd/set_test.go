package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSetCmd_NewKey(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"set", "DB_PORT=5432", "--file", envPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if got != "DB_PORT=5432\n" {
		t.Errorf("output: got %q, want %q", got, "DB_PORT=5432\n")
	}

	// Verify file was updated.
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(content) != "DB_HOST=localhost\nDB_PORT=5432\n" {
		t.Errorf("file content: got %q, want %q", string(content), "DB_HOST=localhost\nDB_PORT=5432\n")
	}
}

func TestSetCmd_UpdateExistingKey(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\nDB_PORT=5432\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"set", "DB_HOST=127.0.0.1", "--file", envPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file was updated with key in original position.
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(content) != "DB_HOST=127.0.0.1\nDB_PORT=5432\n" {
		t.Errorf("file content: got %q, want %q", string(content), "DB_HOST=127.0.0.1\nDB_PORT=5432\n")
	}
}

func TestSetCmd_WriteToLocal(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\n")
	localPath := filepath.Join(dir, ".env.local")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"set", "SECRET=myvalue", "--file", envPath, "--local-file", localPath, "--local"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify .env.local was created.
	content, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("reading local file: %v", err)
	}
	if string(content) != "SECRET=myvalue\n" {
		t.Errorf("local file content: got %q, want %q", string(content), "SECRET=myvalue\n")
	}

	// Verify .env was not modified.
	envContent, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("reading env file: %v", err)
	}
	if string(envContent) != "DB_HOST=localhost\n" {
		t.Errorf(".env should not be modified: got %q", string(envContent))
	}
}

func TestSetCmd_RefValue(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"set", "API_KEY=ref://secrets/api_key", "--file", envPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if got != "API_KEY=ref://secrets/api_key\n" {
		t.Errorf("output: got %q, want %q", got, "API_KEY=ref://secrets/api_key\n")
	}
}

func TestSetCmd_EmptyValue(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"set", "EMPTY=", "--file", envPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(content) != "DB_HOST=localhost\nEMPTY=\n" {
		t.Errorf("file content: got %q, want %q", string(content), "DB_HOST=localhost\nEMPTY=\n")
	}
}

func TestSetCmd_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"set", "NEW_KEY=new_value", "--file", envPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(content) != "NEW_KEY=new_value\n" {
		t.Errorf("file content: got %q, want %q", string(content), "NEW_KEY=new_value\n")
	}
}

func TestSetCmd_InvalidFormat(t *testing.T) {
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"set", "NOEQUALS"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid format, got nil")
	}
}

func TestSetCmd_EmptyKey(t *testing.T) {
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"set", "=value"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for empty key, got nil")
	}
}

func TestSetCmd_NoArgs(t *testing.T) {
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"set"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing argument, got nil")
	}
}

func TestSetCmd_ValueWithSpaces(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"set", "GREETING=hello world", "--file", envPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify value with spaces is double-quoted in the file.
	content, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if string(content) != "GREETING=\"hello world\"\n" {
		t.Errorf("file content: got %q, want %q", string(content), "GREETING=\"hello world\"\n")
	}

	// Verify we can read it back with get.
	root2 := NewRootCmd()
	buf2 := new(bytes.Buffer)
	root2.SetOut(buf2)
	root2.SetErr(new(bytes.Buffer))
	root2.SetArgs([]string{"get", "GREETING", "--file", envPath, "--local-file", filepath.Join(dir, ".env.local")})

	if err := root2.Execute(); err != nil {
		t.Fatalf("get after set: unexpected error: %v", err)
	}

	got := buf2.String()
	if got != "hello world\n" {
		t.Errorf("get after set: got %q, want %q", got, "hello world\n")
	}
}

func TestSetCmd_ValueWithEqualsSign(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"set", "URL=postgres://user:pass@host/db?sslmode=require", "--file", envPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if got != "URL=postgres://user:pass@host/db?sslmode=require\n" {
		t.Errorf("output: got %q, want %q", got, "URL=postgres://user:pass@host/db?sslmode=require\n")
	}
}
