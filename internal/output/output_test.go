package output

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

// newTestCmd creates a cobra command with the persistent verbosity flags
// and the given args pre-set.
func newTestCmd(args ...string) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	root := &cobra.Command{Use: "test"}
	root.PersistentFlags().BoolP("quiet", "q", false, "suppress informational output")
	root.PersistentFlags().Bool("verbose", false, "show additional detail")
	root.PersistentFlags().Bool("debug", false, "show debug information")

	child := &cobra.Command{
		Use:  "sub",
		RunE: func(cmd *cobra.Command, args []string) error { return nil },
	}
	root.AddCommand(child)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetArgs(append([]string{"sub"}, args...))

	// Execute to parse flags.
	_ = root.Execute()

	return child, stdout, stderr
}

func TestFromCmd_Default(t *testing.T) {
	cmd, _, _ := newTestCmd()
	v := FromCmd(cmd)
	if v != VerbosityNormal {
		t.Errorf("expected VerbosityNormal, got %d", v)
	}
}

func TestFromCmd_Quiet(t *testing.T) {
	cmd, _, _ := newTestCmd("--quiet")
	v := FromCmd(cmd)
	if v != VerbosityQuiet {
		t.Errorf("expected VerbosityQuiet, got %d", v)
	}
}

func TestFromCmd_Verbose(t *testing.T) {
	cmd, _, _ := newTestCmd("--verbose")
	v := FromCmd(cmd)
	if v != VerbosityVerbose {
		t.Errorf("expected VerbosityVerbose, got %d", v)
	}
}

func TestFromCmd_Debug(t *testing.T) {
	cmd, _, _ := newTestCmd("--debug")
	v := FromCmd(cmd)
	if v != VerbosityDebug {
		t.Errorf("expected VerbosityDebug, got %d", v)
	}
}

func TestWriter_Info_Normal(t *testing.T) {
	cmd, stdout, _ := newTestCmd()
	w := NewWriter(cmd)
	w.Info("hello %s\n", "world")
	if stdout.String() != "hello world\n" {
		t.Errorf("expected 'hello world\\n', got %q", stdout.String())
	}
}

func TestWriter_Info_Quiet(t *testing.T) {
	cmd, stdout, _ := newTestCmd("--quiet")
	w := NewWriter(cmd)
	w.Info("should not appear\n")
	if stdout.String() != "" {
		t.Errorf("expected empty output in quiet mode, got %q", stdout.String())
	}
}

func TestWriter_Verbose_Normal(t *testing.T) {
	cmd, _, stderr := newTestCmd()
	w := NewWriter(cmd)
	w.Verbose("detail\n")
	if stderr.String() != "" {
		t.Errorf("expected no verbose output at normal level, got %q", stderr.String())
	}
}

func TestWriter_Verbose_Verbose(t *testing.T) {
	cmd, _, stderr := newTestCmd("--verbose")
	w := NewWriter(cmd)
	w.Verbose("detail\n")
	if stderr.String() != "detail\n" {
		t.Errorf("expected 'detail\\n', got %q", stderr.String())
	}
}

func TestWriter_Verbose_Debug(t *testing.T) {
	cmd, _, stderr := newTestCmd("--debug")
	w := NewWriter(cmd)
	w.Verbose("detail\n")
	if stderr.String() != "detail\n" {
		t.Errorf("expected verbose to show at debug level, got %q", stderr.String())
	}
}

func TestWriter_Debug_Normal(t *testing.T) {
	cmd, _, stderr := newTestCmd()
	w := NewWriter(cmd)
	w.Debug("trace\n")
	if stderr.String() != "" {
		t.Errorf("expected no debug output at normal level, got %q", stderr.String())
	}
}

func TestWriter_Debug_Verbose(t *testing.T) {
	cmd, _, stderr := newTestCmd("--verbose")
	w := NewWriter(cmd)
	w.Debug("trace\n")
	if stderr.String() != "" {
		t.Errorf("expected no debug output at verbose level, got %q", stderr.String())
	}
}

func TestWriter_Debug_Debug(t *testing.T) {
	cmd, _, stderr := newTestCmd("--debug")
	w := NewWriter(cmd)
	w.Debug("trace\n")
	if stderr.String() != "debug: trace\n" {
		t.Errorf("expected 'debug: trace\\n', got %q", stderr.String())
	}
}

func TestWriter_Warn_Normal(t *testing.T) {
	cmd, _, stderr := newTestCmd()
	w := NewWriter(cmd)
	w.Warn("careful\n")
	if stderr.String() != "warning: careful\n" {
		t.Errorf("expected 'warning: careful\\n', got %q", stderr.String())
	}
}

func TestWriter_Warn_Quiet(t *testing.T) {
	cmd, _, stderr := newTestCmd("--quiet")
	w := NewWriter(cmd)
	w.Warn("careful\n")
	if stderr.String() != "" {
		t.Errorf("expected no warnings in quiet mode, got %q", stderr.String())
	}
}

func TestWriter_Error_Always(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"normal", nil},
		{"quiet", []string{"--quiet"}},
		{"verbose", []string{"--verbose"}},
		{"debug", []string{"--debug"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, _, stderr := newTestCmd(tt.args...)
			w := NewWriter(cmd)
			w.Error("fail\n")
			if stderr.String() != "error: fail\n" {
				t.Errorf("expected 'error: fail\\n', got %q", stderr.String())
			}
		})
	}
}

func TestWriter_IsQuiet(t *testing.T) {
	cmd, _, _ := newTestCmd("--quiet")
	w := NewWriter(cmd)
	if !w.IsQuiet() {
		t.Error("expected IsQuiet to be true")
	}
}

func TestWriter_IsVerbose(t *testing.T) {
	cmd, _, _ := newTestCmd("--verbose")
	w := NewWriter(cmd)
	if !w.IsVerbose() {
		t.Error("expected IsVerbose to be true")
	}
}

func TestWriter_IsDebug(t *testing.T) {
	cmd, _, _ := newTestCmd("--debug")
	w := NewWriter(cmd)
	if !w.IsDebug() {
		t.Error("expected IsDebug to be true")
	}
	if !w.IsVerbose() {
		t.Error("expected IsVerbose to be true at debug level")
	}
}
