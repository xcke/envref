package output

import (
	"bytes"
	"os"
	"testing"
)

func TestIsTerminal_Buffer(t *testing.T) {
	// A bytes.Buffer is not a terminal.
	var buf bytes.Buffer
	if isTerminal(&buf) {
		t.Error("expected bytes.Buffer to not be a terminal")
	}
}

func TestIsTerminal_DevNull(t *testing.T) {
	// /dev/null is a device but not a char device on all platforms.
	// This test just verifies the function doesn't panic.
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Skipf("cannot open %s: %v", os.DevNull, err)
	}
	defer func() { _ = f.Close() }()
	_ = isTerminal(f)
}

func TestColorEnabled_NoColorFlag(t *testing.T) {
	// --no-color flag should disable color regardless of terminal.
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Skipf("cannot open %s: %v", os.DevNull, err)
	}
	defer func() { _ = f.Close() }()

	if colorEnabled(f, true) {
		t.Error("expected color to be disabled when noColorFlag is true")
	}
}

func TestColorEnabled_NoColorEnv(t *testing.T) {
	// NO_COLOR env var should disable color.
	t.Setenv("NO_COLOR", "1")

	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Skipf("cannot open %s: %v", os.DevNull, err)
	}
	defer func() { _ = f.Close() }()

	if colorEnabled(f, false) {
		t.Error("expected color to be disabled when NO_COLOR is set")
	}
}

func TestColorEnabled_Buffer(t *testing.T) {
	// A non-terminal writer should not have color enabled.
	var buf bytes.Buffer
	if colorEnabled(&buf, false) {
		t.Error("expected color to be disabled for non-terminal writer")
	}
}

func TestColorize(t *testing.T) {
	result := colorize(ansiRed, "error")
	expected := ansiRed + "error" + ansiReset
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestColorize_Empty(t *testing.T) {
	result := colorize("", "text")
	if result != "text" {
		t.Errorf("expected %q, got %q", "text", result)
	}
}

func TestWriter_ColorHelpers_Disabled(t *testing.T) {
	// When color is disabled, helpers should return text unchanged.
	w := &Writer{color: false}

	tests := []struct {
		name string
		fn   func(string) string
	}{
		{"Red", w.Red},
		{"Green", w.Green},
		{"Yellow", w.Yellow},
		{"Cyan", w.Cyan},
		{"Bold", w.Bold},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn("text")
			if result != "text" {
				t.Errorf("expected %q, got %q", "text", result)
			}
		})
	}
}

func TestWriter_ColorHelpers_Enabled(t *testing.T) {
	// When color is enabled, helpers should wrap text with ANSI codes.
	w := &Writer{color: true}

	tests := []struct {
		name     string
		fn       func(string) string
		expected string
	}{
		{"Red", w.Red, ansiRed + "text" + ansiReset},
		{"Green", w.Green, ansiGreen + "text" + ansiReset},
		{"Yellow", w.Yellow, ansiYellow + "text" + ansiReset},
		{"Cyan", w.Cyan, ansiCyan + "text" + ansiReset},
		{"Bold", w.Bold, ansiBold + "text" + ansiReset},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn("text")
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestWriter_ColorEnabled(t *testing.T) {
	w := &Writer{color: true}
	if !w.ColorEnabled() {
		t.Error("expected ColorEnabled to return true")
	}

	w2 := &Writer{color: false}
	if w2.ColorEnabled() {
		t.Error("expected ColorEnabled to return false")
	}
}

func TestWriter_Prefixes_NoColor(t *testing.T) {
	w := &Writer{color: false}

	if w.warnPrefix() != "warning:" {
		t.Errorf("expected 'warning:', got %q", w.warnPrefix())
	}
	if w.errorPrefix() != "error:" {
		t.Errorf("expected 'error:', got %q", w.errorPrefix())
	}
	if w.debugPrefix() != "debug:" {
		t.Errorf("expected 'debug:', got %q", w.debugPrefix())
	}
}

func TestWriter_Prefixes_Color(t *testing.T) {
	w := &Writer{color: true}

	if w.warnPrefix() != ansiYellow+"warning:"+ansiReset {
		t.Errorf("unexpected warn prefix: %q", w.warnPrefix())
	}
	if w.errorPrefix() != ansiRed+"error:"+ansiReset {
		t.Errorf("unexpected error prefix: %q", w.errorPrefix())
	}
	if w.debugPrefix() != ansiCyan+"debug:"+ansiReset {
		t.Errorf("unexpected debug prefix: %q", w.debugPrefix())
	}
}

func TestWriter_Success_Normal(t *testing.T) {
	stdout := new(bytes.Buffer)
	w := &Writer{
		out:       stdout,
		errOut:    new(bytes.Buffer),
		verbosity: VerbosityNormal,
		color:     false,
	}
	w.Success("all good\n")
	expected := "OK: all good\n"
	if stdout.String() != expected {
		t.Errorf("expected %q, got %q", expected, stdout.String())
	}
}

func TestWriter_Success_Quiet(t *testing.T) {
	stdout := new(bytes.Buffer)
	w := &Writer{
		out:       stdout,
		errOut:    new(bytes.Buffer),
		verbosity: VerbosityQuiet,
		color:     false,
	}
	w.Success("should not appear\n")
	if stdout.String() != "" {
		t.Errorf("expected no output in quiet mode, got %q", stdout.String())
	}
}

func TestWriter_WarnOutput_Color(t *testing.T) {
	stderr := new(bytes.Buffer)
	w := &Writer{
		out:       new(bytes.Buffer),
		errOut:    stderr,
		verbosity: VerbosityNormal,
		color:     true,
	}
	w.Warn("watch out\n")
	expected := ansiYellow + "warning:" + ansiReset + " watch out\n"
	if stderr.String() != expected {
		t.Errorf("expected %q, got %q", expected, stderr.String())
	}
}

func TestWriter_ErrorOutput_Color(t *testing.T) {
	stderr := new(bytes.Buffer)
	w := &Writer{
		out:       new(bytes.Buffer),
		errOut:    stderr,
		verbosity: VerbosityNormal,
		color:     true,
	}
	w.Error("fail\n")
	expected := ansiRed + "error:" + ansiReset + " fail\n"
	if stderr.String() != expected {
		t.Errorf("expected %q, got %q", expected, stderr.String())
	}
}

func TestWriter_DebugOutput_Color(t *testing.T) {
	stderr := new(bytes.Buffer)
	w := &Writer{
		out:       new(bytes.Buffer),
		errOut:    stderr,
		verbosity: VerbosityDebug,
		color:     true,
	}
	w.Debug("trace\n")
	expected := ansiCyan + "debug:" + ansiReset + " trace\n"
	if stderr.String() != expected {
		t.Errorf("expected %q, got %q", expected, stderr.String())
	}
}
