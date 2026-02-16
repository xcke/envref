package cmd

import (
	"path/filepath"

	"github.com/xcke/envref/internal/audit"
)

// newAuditLogger creates an audit logger that writes to the .envref.audit.log
// file in the given config directory (the directory containing .envref.yaml).
func newAuditLogger(configDir string) *audit.Logger {
	return audit.NewLogger(filepath.Join(configDir, audit.DefaultFileName))
}
