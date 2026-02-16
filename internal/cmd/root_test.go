package cmd

import (
	"bytes"
	"strings"
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

func TestVerbosityFlagsExist(t *testing.T) {
	root := NewRootCmd()

	// Persistent flags should be accessible on the root.
	flags := []string{"quiet", "verbose", "debug"}
	for _, name := range flags {
		f := root.PersistentFlags().Lookup(name)
		if f == nil {
			t.Errorf("expected persistent flag %q to exist", name)
		}
	}

	// Short flag should exist for quiet.
	qf := root.PersistentFlags().ShorthandLookup("q")
	if qf == nil || qf.Name != "quiet" {
		t.Error("expected -q shorthand for --quiet")
	}
}

func TestVerbosityFlagsMutuallyExclusive(t *testing.T) {
	// Using --quiet and --verbose together should error.
	root := NewRootCmd()
	errBuf := new(bytes.Buffer)
	root.SetErr(errBuf)
	root.SetOut(new(bytes.Buffer))
	root.SetArgs([]string{"--quiet", "--verbose", "version"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when using --quiet and --verbose together")
	}
	// Cobra says "if any flags in the group [...] are set none of the others can be".
	if !strings.Contains(err.Error(), "none of the others can be") {
		t.Errorf("expected mutually exclusive error, got: %v", err)
	}
}

func TestHelpShowsVerbosityFlags(t *testing.T) {
	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"--help"})

	_ = root.Execute()

	help := buf.String()
	for _, flag := range []string{"--quiet", "--verbose", "--debug"} {
		if !strings.Contains(help, flag) {
			t.Errorf("expected help to contain %q", flag)
		}
	}
}
