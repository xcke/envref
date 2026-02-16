package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompletionBash(t *testing.T) {
	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"completion", "bash"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "bash") {
		t.Error("expected bash completion script output")
	}
	if len(got) < 100 {
		t.Errorf("expected substantial completion script, got %d bytes", len(got))
	}
}

func TestCompletionZsh(t *testing.T) {
	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"completion", "zsh"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "zsh") {
		t.Error("expected zsh completion script output")
	}
	if len(got) < 100 {
		t.Errorf("expected substantial completion script, got %d bytes", len(got))
	}
}

func TestCompletionFish(t *testing.T) {
	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"completion", "fish"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "fish") {
		t.Error("expected fish completion script output")
	}
	if len(got) < 100 {
		t.Errorf("expected substantial completion script, got %d bytes", len(got))
	}
}

func TestCompletionPowershell(t *testing.T) {
	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"completion", "powershell"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if len(got) < 100 {
		t.Errorf("expected substantial completion script, got %d bytes", len(got))
	}
}

func TestCompletionUnsupportedShell(t *testing.T) {
	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"completion", "tcsh"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for unsupported shell")
	}
	if !strings.Contains(err.Error(), "unsupported shell") {
		t.Errorf("expected 'unsupported shell' in error, got %q", err.Error())
	}
}

func TestCompletionNoArgs(t *testing.T) {
	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"completion"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when no shell argument provided")
	}
}

func TestCompletionTooManyArgs(t *testing.T) {
	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"completion", "bash", "extra"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when too many arguments provided")
	}
}
