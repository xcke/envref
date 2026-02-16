package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorCmd_NoIssues(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\nDB_PORT=5432\n")
	writeTestFile(t, dir, ".gitignore", ".env\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"doctor",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "OK: no issues found") {
		t.Errorf("expected OK message, got %q", buf.String())
	}
}

func TestDoctorCmd_NoIssues_Quiet(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\nDB_PORT=5432\n")
	writeTestFile(t, dir, ".gitignore", ".env\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"--quiet", "doctor",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.String() != "" {
		t.Errorf("expected no output in quiet mode, got %q", buf.String())
	}
}

func TestDoctorCmd_DuplicateKeys(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\nDB_PORT=5432\nDB_HOST=other\n")
	writeTestFile(t, dir, ".gitignore", ".env\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(errBuf)
	root.SetArgs([]string{"doctor",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for duplicate keys, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "duplicate key") {
		t.Errorf("expected duplicate key warning, got %q", stderr)
	}
	if !strings.Contains(stderr, "DB_HOST") {
		t.Errorf("expected DB_HOST in output, got %q", stderr)
	}
}

func TestDoctorCmd_TrailingWhitespace(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost   \nDB_PORT=5432\n")
	writeTestFile(t, dir, ".gitignore", ".env\n")

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"doctor",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for trailing whitespace, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "trailing whitespace") {
		t.Errorf("expected trailing whitespace issue, got %q", stderr)
	}
	if !strings.Contains(stderr, "DB_HOST") {
		t.Errorf("expected DB_HOST in output, got %q", stderr)
	}
}

func TestDoctorCmd_UnquotedSpaces(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "GREETING=hello world\nDB_HOST=localhost\n")
	writeTestFile(t, dir, ".gitignore", ".env\n")

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"doctor",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for unquoted spaces, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "unquoted value with spaces") {
		t.Errorf("expected unquoted spaces issue, got %q", stderr)
	}
	if !strings.Contains(stderr, "GREETING") {
		t.Errorf("expected GREETING in output, got %q", stderr)
	}
}

func TestDoctorCmd_QuotedSpacesOK(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "GREETING=\"hello world\"\nDB_HOST=localhost\n")
	writeTestFile(t, dir, ".gitignore", ".env\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"doctor",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "OK") {
		t.Errorf("expected OK message, got %q", buf.String())
	}
}

func TestDoctorCmd_EmptyValue(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=\nDB_PORT=5432\n")
	writeTestFile(t, dir, ".gitignore", ".env\n")

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"doctor",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for empty value, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "empty value") {
		t.Errorf("expected empty value issue, got %q", stderr)
	}
	if !strings.Contains(stderr, "DB_HOST") {
		t.Errorf("expected DB_HOST in output, got %q", stderr)
	}
}

func TestDoctorCmd_ExplicitEmptyOK(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=\"\"\nDB_PORT=5432\n")
	writeTestFile(t, dir, ".gitignore", ".env\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"doctor",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "OK") {
		t.Errorf("expected OK message, got %q", buf.String())
	}
}

func TestDoctorCmd_MissingGitignore(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\n")
	// No .gitignore created.

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"doctor",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing .gitignore, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, ".gitignore") {
		t.Errorf("expected .gitignore issue, got %q", stderr)
	}
}

func TestDoctorCmd_GitignoreWithoutEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\n")
	writeTestFile(t, dir, ".gitignore", "*.log\nnode_modules/\n")

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"doctor",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for .env not in .gitignore, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, ".gitignore") {
		t.Errorf("expected .gitignore issue, got %q", stderr)
	}
}

func TestDoctorCmd_GitignoreWithEnvStar(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\n")
	writeTestFile(t, dir, ".gitignore", ".env*\n")

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"doctor",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "OK") {
		t.Errorf("expected OK message, got %q", buf.String())
	}
}

func TestDoctorCmd_CustomFileSkipsGitignoreCheck(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env.staging", "DB_HOST=localhost\n")
	// No .gitignore — but custom filename should skip the .gitignore check.

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"doctor",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "OK") {
		t.Errorf("expected OK message, got %q", buf.String())
	}
}

func TestDoctorCmd_MultipleIssues(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=\nDB_HOST=localhost\nGREETING=hello world\n")
	writeTestFile(t, dir, ".gitignore", ".env\n")

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"doctor",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for multiple issues, got nil")
	}

	stderr := errBuf.String()
	// Should contain duplicate key, empty value, and unquoted spaces.
	if !strings.Contains(stderr, "duplicate key") {
		t.Errorf("expected duplicate key issue, got %q", stderr)
	}
	if !strings.Contains(stderr, "empty value") {
		t.Errorf("expected empty value issue, got %q", stderr)
	}
	if !strings.Contains(stderr, "unquoted value with spaces") {
		t.Errorf("expected unquoted spaces issue, got %q", stderr)
	}
	if !strings.Contains(stderr, "Doctor found") {
		t.Errorf("expected summary line, got %q", stderr)
	}
}

func TestDoctorCmd_BothFilesChecked(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "DB_HOST=localhost\n")
	localPath := writeTestFile(t, dir, ".env.local", "SECRET=\n")
	writeTestFile(t, dir, ".gitignore", ".env\n")

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"doctor",
		"--file", envPath,
		"--local-file", localPath,
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for issues in .env.local, got nil")
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "empty value") {
		t.Errorf("expected empty value issue from .env.local, got %q", stderr)
	}
}

func TestDoctorCmd_NoFiles(t *testing.T) {
	dir := t.TempDir()

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"doctor",
		"--file", filepath.Join(dir, ".env"),
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	// No files exist — should report OK (no files to check, no .env to protect).
	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "OK") {
		t.Errorf("expected OK message, got %q", buf.String())
	}
}

func TestDoctorCmd_RejectsArguments(t *testing.T) {
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"doctor", "extra-arg"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for unexpected argument, got nil")
	}
}

func TestDoctorCmd_IssueCount(t *testing.T) {
	dir := t.TempDir()
	envPath := writeTestFile(t, dir, ".env", "A=\nB=\nC=\n")
	writeTestFile(t, dir, ".gitignore", ".env\n")

	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetOut(new(bytes.Buffer))
	root.SetErr(errBuf)
	root.SetArgs([]string{"doctor",
		"--file", envPath,
		"--local-file", filepath.Join(dir, ".env.local"),
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "3 issue(s) found") {
		t.Errorf("expected 3 issues, got error: %v", err)
	}

	stderr := errBuf.String()
	if !strings.Contains(stderr, "Doctor found 3 issue(s)") {
		t.Errorf("expected 'Doctor found 3 issue(s)' in stderr, got %q", stderr)
	}
}

func TestGitignoreCovers(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		filename string
		want     bool
	}{
		{"exact match", ".env\n", ".env", true},
		{"star pattern", ".env*\n", ".env", true},
		{"no match", "node_modules\n", ".env", false},
		{"empty file", "", ".env", false},
		{"comment line", "# .env\n", ".env", false},
		{"with other entries", "node_modules\n.env\n*.log\n", ".env", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.content)
			got := gitignoreCovers(r, tt.filename)
			if got != tt.want {
				t.Errorf("gitignoreCovers(%q, %q) = %v, want %v", tt.content, tt.filename, got, tt.want)
			}
		})
	}
}
