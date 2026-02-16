// Package output provides verbosity-aware output helpers for CLI commands.
//
// It reads --quiet, --verbose, and --debug persistent flags from the cobra
// command tree and provides helper methods to conditionally print messages
// based on the active verbosity level.
package output

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// Verbosity represents the output verbosity level.
type Verbosity int

const (
	// VerbosityQuiet suppresses informational messages. Only data output and
	// errors are shown.
	VerbosityQuiet Verbosity = -1
	// VerbosityNormal is the default verbosity level.
	VerbosityNormal Verbosity = 0
	// VerbosityVerbose shows extra detail such as file paths being loaded.
	VerbosityVerbose Verbosity = 1
	// VerbosityDebug shows internal debug information.
	VerbosityDebug Verbosity = 2
)

// FromCmd reads the --quiet, --verbose, and --debug persistent flags from the
// command and returns the effective verbosity level.
//
// Precedence: --quiet > --debug > --verbose. If none are set, VerbosityNormal
// is returned.
func FromCmd(cmd *cobra.Command) Verbosity {
	quiet, _ := cmd.Flags().GetBool("quiet")
	if quiet {
		return VerbosityQuiet
	}
	debug, _ := cmd.Flags().GetBool("debug")
	if debug {
		return VerbosityDebug
	}
	verbose, _ := cmd.Flags().GetBool("verbose")
	if verbose {
		return VerbosityVerbose
	}
	return VerbosityNormal
}

// Writer is a verbosity-aware writer that wraps a cobra command's output
// streams and provides leveled printing helpers. It optionally colorizes
// prefixes and labels when writing to a terminal.
type Writer struct {
	out       io.Writer
	errOut    io.Writer
	verbosity Verbosity
	color     bool
}

// NewWriter creates a Writer from a cobra command. It reads the verbosity
// flags and captures the command's stdout and stderr writers. Color output
// is automatically enabled when stderr is a terminal and --no-color is not
// set.
func NewWriter(cmd *cobra.Command) *Writer {
	noColor, _ := cmd.Flags().GetBool("no-color")
	errW := cmd.ErrOrStderr()
	return &Writer{
		out:       cmd.OutOrStdout(),
		errOut:    errW,
		verbosity: FromCmd(cmd),
		color:     colorEnabled(errW, noColor),
	}
}

// Verbosity returns the active verbosity level.
func (w *Writer) Verbosity() Verbosity {
	return w.verbosity
}

// IsQuiet returns true if verbosity is set to quiet.
func (w *Writer) IsQuiet() bool {
	return w.verbosity == VerbosityQuiet
}

// IsVerbose returns true if verbosity is at least verbose.
func (w *Writer) IsVerbose() bool {
	return w.verbosity >= VerbosityVerbose
}

// IsDebug returns true if verbosity is at debug level.
func (w *Writer) IsDebug() bool {
	return w.verbosity >= VerbosityDebug
}

// Info prints an informational message to stdout. Suppressed in quiet mode.
func (w *Writer) Info(format string, args ...interface{}) {
	if w.verbosity >= VerbosityNormal {
		_, _ = fmt.Fprintf(w.out, format, args...)
	}
}

// Verbose prints a message to stderr only when --verbose or --debug is active.
func (w *Writer) Verbose(format string, args ...interface{}) {
	if w.verbosity >= VerbosityVerbose {
		_, _ = fmt.Fprintf(w.errOut, format, args...)
	}
}

// Debug prints a message to stderr only when --debug is active.
func (w *Writer) Debug(format string, args ...interface{}) {
	if w.verbosity >= VerbosityDebug {
		msg := fmt.Sprintf(format, args...)
		_, _ = fmt.Fprintf(w.errOut, "%s %s", w.debugPrefix(), msg)
	}
}

// Warn prints a warning to stderr. Shown at all verbosity levels except quiet.
func (w *Writer) Warn(format string, args ...interface{}) {
	if w.verbosity >= VerbosityNormal {
		msg := fmt.Sprintf(format, args...)
		_, _ = fmt.Fprintf(w.errOut, "%s %s", w.warnPrefix(), msg)
	}
}

// Error prints an error to stderr. Always shown regardless of verbosity.
func (w *Writer) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(w.errOut, "%s %s", w.errorPrefix(), msg)
}

// Stdout returns the stdout writer for data output that should always be
// printed regardless of verbosity level.
func (w *Writer) Stdout() io.Writer {
	return w.out
}

// Stderr returns the stderr writer.
func (w *Writer) Stderr() io.Writer {
	return w.errOut
}
