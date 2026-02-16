// Package audit provides an append-only, JSON-lines audit log for tracking
// secret operations (set, delete, rotate, copy, generate) in an envref project.
//
// The log file is stored at .envref.audit.log in the project root (alongside
// .envref.yaml). Each line is a JSON object representing a single operation.
// The file is designed to be committed to git so the full history of who
// changed what is preserved in version control.
package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"time"
)

// Operation represents the type of secret operation that was performed.
type Operation string

const (
	// OpSet is logged when a secret is created or updated.
	OpSet Operation = "set"
	// OpDelete is logged when a secret is removed.
	OpDelete Operation = "delete"
	// OpGenerate is logged when a secret is randomly generated and stored.
	OpGenerate Operation = "generate"
	// OpRotate is logged when a secret is rotated (new value generated,
	// old value archived to history).
	OpRotate Operation = "rotate"
	// OpCopy is logged when a secret is copied from another project.
	OpCopy Operation = "copy"
	// OpImport is logged when secrets are imported via sync pull.
	OpImport Operation = "import"
)

// Entry is a single audit log record. Each record captures who performed
// what operation on which key, when, and with what backend.
type Entry struct {
	// Timestamp is the UTC time the operation occurred (RFC 3339).
	Timestamp string `json:"timestamp"`
	// User is the OS username of the person who ran the command.
	User string `json:"user"`
	// Operation is the type of secret operation (set, delete, etc.).
	Operation Operation `json:"operation"`
	// Key is the secret key name (without project/profile prefix).
	Key string `json:"key"`
	// Backend is the name of the backend used (e.g., "keychain", "vault").
	Backend string `json:"backend"`
	// Project is the project namespace.
	Project string `json:"project"`
	// Profile is the profile scope, if any. Empty for project-scoped ops.
	Profile string `json:"profile,omitempty"`
	// Detail contains optional extra context (e.g., source project for copy).
	Detail string `json:"detail,omitempty"`
}

// Logger writes audit entries to a JSON-lines file.
type Logger struct {
	path string
}

// NewLogger creates a Logger that appends entries to the given file path.
// The file is created on the first write if it does not exist.
func NewLogger(path string) *Logger {
	return &Logger{path: path}
}

// Log appends a single audit entry to the log file. The entry's Timestamp
// and User fields are set automatically if empty.
func (l *Logger) Log(e Entry) error {
	if e.Timestamp == "" {
		e.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	if e.User == "" {
		e.User = currentUser()
	}

	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshaling audit entry: %w", err)
	}
	data = append(data, '\n')

	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening audit log: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("writing audit entry: %w", err)
	}

	return f.Close()
}

// Read returns all entries from the audit log, in order from oldest to newest.
// Returns an empty slice (not an error) if the file does not exist.
func (l *Logger) Read() ([]Entry, error) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading audit log: %w", err)
	}

	return ParseEntries(data)
}

// ParseEntries parses JSON-lines audit log data into a slice of Entry values.
// Blank lines are silently skipped. Malformed lines produce an error.
func ParseEntries(data []byte) ([]Entry, error) {
	var entries []Entry
	start := 0
	for i := 0; i <= len(data); i++ {
		if i == len(data) || data[i] == '\n' {
			line := data[start:i]
			start = i + 1
			if len(line) == 0 {
				continue
			}
			var e Entry
			if err := json.Unmarshal(line, &e); err != nil {
				return nil, fmt.Errorf("parsing audit entry: %w", err)
			}
			entries = append(entries, e)
		}
	}
	return entries, nil
}

// Path returns the file path of the audit log.
func (l *Logger) Path() string {
	return l.path
}

// DefaultFileName is the name of the audit log file placed next to .envref.yaml.
const DefaultFileName = ".envref.audit.log"

// currentUser returns the current OS username, or "unknown" if it cannot be determined.
func currentUser() string {
	u, err := user.Current()
	if err != nil {
		return "unknown"
	}
	return u.Username
}
