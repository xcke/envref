package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/audit"
	"github.com/xcke/envref/internal/config"
	"github.com/xcke/envref/internal/output"
)

// newAuditLogCmd creates the audit-log command for viewing secret operation history.
func newAuditLogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit-log",
		Short: "Show the secret operations audit log",
		Long: `Display the audit log of secret operations (set, delete, generate,
rotate, copy, import) for the current project.

The audit log is stored as .envref.audit.log in the project root (next to
.envref.yaml) and tracks who performed what secret operation, when, and
on which backend. The log is append-only and designed to be committed to
git for a full team-visible history.

Use --last to limit output to the N most recent entries.
Use --key to filter entries for a specific secret key.
Use --json to output raw JSON lines for scripting.

Examples:
  envref audit-log                        # show all entries
  envref audit-log --last 10              # show 10 most recent entries
  envref audit-log --key API_KEY          # filter by key name
  envref audit-log --json                 # raw JSON output for scripting`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			last, _ := cmd.Flags().GetInt("last")
			key, _ := cmd.Flags().GetString("key")
			jsonOut, _ := cmd.Flags().GetBool("json")
			return runAuditLog(cmd, last, key, jsonOut)
		},
	}

	cmd.Flags().IntP("last", "n", 0, "show only the last N entries (0 = all)")
	cmd.Flags().StringP("key", "k", "", "filter entries by secret key name")
	cmd.Flags().Bool("json", false, "output raw JSON lines")

	return cmd
}

// runAuditLog reads and displays the audit log.
func runAuditLog(cmd *cobra.Command, last int, keyFilter string, jsonOut bool) error {
	w := output.NewWriter(cmd)

	// Load project config to find the audit log file.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	_, configDir, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logPath := filepath.Join(configDir, audit.DefaultFileName)
	logger := audit.NewLogger(logPath)

	entries, err := logger.Read()
	if err != nil {
		return fmt.Errorf("reading audit log: %w", err)
	}

	if len(entries) == 0 {
		if !w.IsQuiet() {
			w.Info("no audit log entries found\n")
		}
		return nil
	}

	// Apply key filter.
	if keyFilter != "" {
		var filtered []audit.Entry
		for _, e := range entries {
			if e.Key == keyFilter {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Apply --last limit.
	if last > 0 && last < len(entries) {
		entries = entries[len(entries)-last:]
	}

	if len(entries) == 0 {
		if !w.IsQuiet() {
			w.Info("no matching audit log entries found\n")
		}
		return nil
	}

	out := cmd.OutOrStdout()

	if jsonOut {
		// Raw JSON lines output.
		for _, e := range entries {
			data, err := jsonMarshal(e)
			if err != nil {
				return fmt.Errorf("marshaling entry: %w", err)
			}
			_, _ = fmt.Fprintln(out, string(data))
		}
		return nil
	}

	// Human-readable table output.
	for _, e := range entries {
		ts := e.Timestamp
		if len(ts) > 19 {
			ts = ts[:19] // Trim to "2006-01-02T15:04:05"
		}

		scope := e.Project
		if e.Profile != "" {
			scope = fmt.Sprintf("%s/%s", e.Project, e.Profile)
		}

		line := fmt.Sprintf("%s  %-8s  %-12s  %-20s  %s",
			ts,
			e.Operation,
			e.User,
			e.Key,
			w.Cyan(fmt.Sprintf("[%s] %s", e.Backend, scope)),
		)
		if e.Detail != "" {
			line += "  " + w.Cyan(e.Detail)
		}
		_, _ = fmt.Fprintln(out, line)
	}

	if !w.IsQuiet() {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "\n%d entries\n", len(entries))
	}

	return nil
}

// jsonMarshal marshals a value to JSON without HTML escaping.
func jsonMarshal(v interface{}) ([]byte, error) {
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// Encode adds a trailing newline; trim it.
	s := buf.String()
	return []byte(strings.TrimSuffix(s, "\n")), nil
}
