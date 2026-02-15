package cmd

import (
	"bytes"
	"testing"
)

func TestNewRootCmd(t *testing.T) {
	cmd := NewRootCmd()
	if cmd.Use != "envref" {
		t.Errorf("expected Use to be 'envref', got %q", cmd.Use)
	}
}

func TestVersionCmd(t *testing.T) {
	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	expected := "envref dev\n"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestRootCmdHelp(t *testing.T) {
	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if len(got) == 0 {
		t.Error("expected help output, got empty string")
	}
}
