package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateCmd_AllKeysMatch(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".env.example", "DB_HOST=localhost\nDB_PORT=5432\n")
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=myhost\nDB_PORT=3306\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"validate",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
		"--example", filepath.Join(dir, ".env.example"),
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "OK") {
		t.Errorf("expected OK message, got %q", buf.String())
	}
}

func TestValidateCmd_MissingKeys(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".env.example", "DB_HOST=localhost\nDB_PORT=5432\nAPI_KEY=ref://secrets/key\n")
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=myhost\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(errBuf)
	root.SetArgs([]string{"validate",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
		"--example", filepath.Join(dir, ".env.example"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing keys, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "DB_PORT") {
		t.Errorf("expected DB_PORT in missing keys, got %q", stderr)
	}
	if !strings.Contains(stderr, "API_KEY") {
		t.Errorf("expected API_KEY in missing keys, got %q", stderr)
	}
	if !strings.Contains(stderr, "2 missing") {
		t.Errorf("expected '2 missing' in output, got %q", stderr)
	}
}

func TestValidateCmd_ExtraKeys(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".env.example", "DB_HOST=localhost\n")
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=myhost\nEXTRA_VAR=value\nANOTHER=val\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(errBuf)
	root.SetArgs([]string{"validate",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
		"--example", filepath.Join(dir, ".env.example"),
	})

	// Extra keys alone should NOT cause an error (just warnings).
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error for extra-only keys: %v", err)
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "EXTRA_VAR") {
		t.Errorf("expected EXTRA_VAR in extra keys, got %q", stderr)
	}
	if !strings.Contains(stderr, "ANOTHER") {
		t.Errorf("expected ANOTHER in extra keys, got %q", stderr)
	}
	if !strings.Contains(stderr, "2 extra") {
		t.Errorf("expected '2 extra' in output, got %q", stderr)
	}
}

func TestValidateCmd_MissingAndExtra(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".env.example", "DB_HOST=localhost\nDB_PORT=5432\n")
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=myhost\nEXTRA=val\n")

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"validate",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
		"--example", filepath.Join(dir, ".env.example"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing keys, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "DB_PORT") {
		t.Errorf("expected DB_PORT in missing keys, got %q", stderr)
	}
	if !strings.Contains(stderr, "EXTRA") {
		t.Errorf("expected EXTRA in extra keys, got %q", stderr)
	}
	if !strings.Contains(stderr, "1 missing") {
		t.Errorf("expected '1 missing' in output, got %q", stderr)
	}
	if !strings.Contains(stderr, "1 extra") {
		t.Errorf("expected '1 extra' in output, got %q", stderr)
	}
}

func TestValidateCmd_LocalOverrideSatisfiesRequirement(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".env.example", "DB_HOST=localhost\nAPI_KEY=ref://secrets/key\n")
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=myhost\n")
	localPath := writeTestFile(t, dir, ".env.local", "API_KEY=secret123\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"validate",
		"--file", envPath,
		"--local-file", localPath,
		"--example", filepath.Join(dir, ".env.example"),
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "OK") {
		t.Errorf("expected OK message, got %q", buf.String())
	}
}

func TestValidateCmd_MissingExampleFile(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=myhost\n")

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"validate",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
		"--example", filepath.Join(dir, ".env.example"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing example file, got nil")
	}
}

func TestValidateCmd_MissingEnvFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".env.example", "DB_HOST=localhost\n")

	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"validate",
		"--file", filepath.Join(dir, "nonexistent.env"),
		"--local-file", filepath.Join(dir, ".env.local"),
		"--example", filepath.Join(dir, ".env.example"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing env file, got nil")
	}
}

func TestValidateCmd_EmptyFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".env.example", "# only comments\n\n")
	envPath := writeTestFile(t, dir, ".env", "# only comments\n\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"validate",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
		"--example", filepath.Join(dir, ".env.example"),
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "OK") {
		t.Errorf("expected OK message, got %q", buf.String())
	}
}

func TestValidateCmd_WithProfileFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".env.example", "DB_HOST=localhost\nSTAGING_VAR=value\n")
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=myhost\n")
	profilePath := writeTestFile(t, dir, ".env.staging", "STAGING_VAR=staging_value\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"validate",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
		"--profile-file", profilePath,
		"--example", filepath.Join(dir, ".env.example"),
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "OK") {
		t.Errorf("expected OK message, got %q", buf.String())
	}
}

func TestValidateCmd_CustomExampleFlag(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".env.schema", "DB_HOST=localhost\nDB_PORT=5432\n")
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=myhost\nDB_PORT=3306\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"validate",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
		"--example", filepath.Join(dir, ".env.schema"),
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "OK") {
		t.Errorf("expected OK message, got %q", buf.String())
	}
}

func TestValidateCmd_CI_AllKeysMatch(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".env.example", "DB_HOST=localhost\nDB_PORT=5432\n")
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=myhost\nDB_PORT=3306\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(errBuf)
	root.SetArgs([]string{"validate",
		"--ci",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
		"--example", filepath.Join(dir, ".env.example"),
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// CI mode: no output on success.
	if buf.String() != "" {
		t.Errorf("expected no stdout in CI mode, got %q", buf.String())
	}
	if errBuf.String() != "" {
		t.Errorf("expected no stderr in CI mode on success, got %q", errBuf.String())
	}
}

func TestValidateCmd_CI_MissingKeys(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".env.example", "DB_HOST=localhost\nDB_PORT=5432\nAPI_KEY=ref://secrets/key\n")
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=myhost\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(errBuf)
	root.SetArgs([]string{"validate",
		"--ci",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
		"--example", filepath.Join(dir, ".env.example"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing keys in CI mode, got nil")
	}

	stderr := errBuf.String()
	// Compact "error:" prefix format.
	if !strings.Contains(stderr, "error: missing key API_KEY") {
		t.Errorf("expected compact error for API_KEY, got %q", stderr)
	}
	if !strings.Contains(stderr, "error: missing key DB_PORT") {
		t.Errorf("expected compact error for DB_PORT, got %q", stderr)
	}
	if !strings.Contains(err.Error(), "2 error(s)") {
		t.Errorf("expected '2 error(s)' in error message, got %q", err.Error())
	}
}

func TestValidateCmd_CI_ExtraKeysAreErrors(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".env.example", "DB_HOST=localhost\n")
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=myhost\nEXTRA_VAR=value\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(errBuf)
	root.SetArgs([]string{"validate",
		"--ci",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
		"--example", filepath.Join(dir, ".env.example"),
	})

	// In CI mode, extra keys ARE errors (unlike default mode).
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for extra keys in CI mode, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "error: extra key EXTRA_VAR") {
		t.Errorf("expected compact error for EXTRA_VAR, got %q", stderr)
	}
	if !strings.Contains(err.Error(), "1 error(s)") {
		t.Errorf("expected '1 error(s)' in error message, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "1 extra") {
		t.Errorf("expected '1 extra' in error message, got %q", err.Error())
	}
}

func TestValidateCmd_CI_MissingAndExtra(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".env.example", "DB_HOST=localhost\nDB_PORT=5432\n")
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=myhost\nEXTRA=val\n")

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"validate",
		"--ci",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
		"--example", filepath.Join(dir, ".env.example"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing+extra keys in CI mode, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "error: missing key DB_PORT") {
		t.Errorf("expected compact error for DB_PORT, got %q", stderr)
	}
	if !strings.Contains(stderr, "error: extra key EXTRA") {
		t.Errorf("expected compact error for EXTRA, got %q", stderr)
	}
	if !strings.Contains(err.Error(), "2 error(s)") {
		t.Errorf("expected '2 error(s)' in error message, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "1 missing") {
		t.Errorf("expected '1 missing' in error message, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "1 extra") {
		t.Errorf("expected '1 extra' in error message, got %q", err.Error())
	}
}

func TestValidateCmd_RejectsArguments(t *testing.T) {
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"validate", "extra-arg"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for unexpected argument, got nil")
	}
}
