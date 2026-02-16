package output

import (
	"fmt"
	"io"
	"os"
)

// ANSI escape codes for terminal colors.
const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
)

// isTerminal reports whether w is connected to an interactive terminal.
// It returns true only when w is an *os.File whose Fd() is a terminal.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// colorEnabled determines whether color output should be used based on
// the --no-color flag and whether the writer is a terminal.
//
// Color is enabled when all of the following are true:
//   - The --no-color flag is not set
//   - The NO_COLOR environment variable is not set (https://no-color.org/)
//   - The writer is connected to a terminal
func colorEnabled(w io.Writer, noColorFlag bool) bool {
	if noColorFlag {
		return false
	}
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	return isTerminal(w)
}

// colorize wraps text with ANSI color codes. If color is empty, text is
// returned unchanged.
func colorize(color, text string) string {
	if color == "" {
		return text
	}
	return color + text + ansiReset
}

// Red returns text wrapped in red ANSI codes if color is enabled.
func (w *Writer) Red(text string) string {
	if !w.color {
		return text
	}
	return colorize(ansiRed, text)
}

// Green returns text wrapped in green ANSI codes if color is enabled.
func (w *Writer) Green(text string) string {
	if !w.color {
		return text
	}
	return colorize(ansiGreen, text)
}

// Yellow returns text wrapped in yellow ANSI codes if color is enabled.
func (w *Writer) Yellow(text string) string {
	if !w.color {
		return text
	}
	return colorize(ansiYellow, text)
}

// Cyan returns text wrapped in cyan ANSI codes if color is enabled.
func (w *Writer) Cyan(text string) string {
	if !w.color {
		return text
	}
	return colorize(ansiCyan, text)
}

// Bold returns text wrapped in bold ANSI codes if color is enabled.
func (w *Writer) Bold(text string) string {
	if !w.color {
		return text
	}
	return colorize(ansiBold, text)
}

// ColorEnabled reports whether color output is active for this writer.
func (w *Writer) ColorEnabled() bool {
	return w.color
}

// successPrefix returns a colored "OK" label suitable for success output.
func (w *Writer) successPrefix() string {
	return w.Green("OK")
}

// warnPrefix returns a colored "warning:" label for warning output.
func (w *Writer) warnPrefix() string {
	if w.color {
		return colorize(ansiYellow, "warning:")
	}
	return "warning:"
}

// errorPrefix returns a colored "error:" label for error output.
func (w *Writer) errorPrefix() string {
	if w.color {
		return colorize(ansiRed, "error:")
	}
	return "error:"
}

// debugPrefix returns a colored "debug:" label for debug output.
func (w *Writer) debugPrefix() string {
	if w.color {
		return colorize(ansiCyan, "debug:")
	}
	return "debug:"
}

// Success prints a success message to stdout with a green "OK" prefix.
// Suppressed in quiet mode.
func (w *Writer) Success(format string, args ...interface{}) {
	if w.verbosity >= VerbosityNormal {
		msg := fmt.Sprintf(format, args...)
		_, _ = fmt.Fprintf(w.out, "%s: %s", w.successPrefix(), msg)
	}
}
