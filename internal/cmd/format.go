package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// OutputFormat represents the available output formats.
type OutputFormat string

const (
	// FormatPlain outputs KEY=VALUE pairs (default for list/resolve).
	FormatPlain OutputFormat = "plain"
	// FormatJSON outputs a JSON array of objects.
	FormatJSON OutputFormat = "json"
	// FormatShell outputs export KEY=VALUE pairs (shell-safe quoting).
	FormatShell OutputFormat = "shell"
	// FormatTable outputs aligned columns with headers.
	FormatTable OutputFormat = "table"
)

// validFormats lists all accepted --format values.
var validFormats = []OutputFormat{FormatPlain, FormatJSON, FormatShell, FormatTable}

// parseFormat validates and returns the output format from a string.
func parseFormat(s string) (OutputFormat, error) {
	f := OutputFormat(strings.ToLower(s))
	for _, valid := range validFormats {
		if f == valid {
			return f, nil
		}
	}
	names := make([]string, len(validFormats))
	for i, v := range validFormats {
		names[i] = string(v)
	}
	return "", fmt.Errorf("invalid format %q: must be one of %s", s, strings.Join(names, ", "))
}

// kvPair represents a key-value pair for formatted output.
type kvPair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// formatKVPairs writes key-value pairs in the specified format.
func formatKVPairs(w io.Writer, pairs []kvPair, format OutputFormat) error {
	switch format {
	case FormatJSON:
		return formatKVJSON(w, pairs)
	case FormatShell:
		return formatKVShell(w, pairs)
	case FormatTable:
		return formatKVTable(w, pairs)
	default:
		return formatKVPlain(w, pairs)
	}
}

// formatKVPlain outputs KEY=VALUE pairs, one per line.
func formatKVPlain(w io.Writer, pairs []kvPair) error {
	for _, p := range pairs {
		if _, err := fmt.Fprintf(w, "%s=%s\n", p.Key, p.Value); err != nil {
			return err
		}
	}
	return nil
}

// formatKVJSON outputs a JSON array of {"key": ..., "value": ...} objects.
func formatKVJSON(w io.Writer, pairs []kvPair) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(pairs)
}

// formatKVShell outputs export KEY=VALUE pairs with shell-safe quoting.
func formatKVShell(w io.Writer, pairs []kvPair) error {
	for _, p := range pairs {
		if _, err := fmt.Fprintf(w, "export %s=%s\n", p.Key, shellQuote(p.Value)); err != nil {
			return err
		}
	}
	return nil
}

// formatKVTable outputs an aligned table with KEY and VALUE columns.
func formatKVTable(w io.Writer, pairs []kvPair) error {
	if len(pairs) == 0 {
		return nil
	}

	// Find max key width for alignment.
	maxKey := len("KEY")
	for _, p := range pairs {
		if len(p.Key) > maxKey {
			maxKey = len(p.Key)
		}
	}

	// Print header.
	if _, err := fmt.Fprintf(w, "%-*s  %s\n", maxKey, "KEY", "VALUE"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%-*s  %s\n", maxKey, strings.Repeat("-", maxKey), strings.Repeat("-", 5)); err != nil {
		return err
	}

	// Print rows.
	for _, p := range pairs {
		if _, err := fmt.Fprintf(w, "%-*s  %s\n", maxKey, p.Key, p.Value); err != nil {
			return err
		}
	}
	return nil
}

// formatSingleValue writes a single value in the specified format.
// Used by the get command which returns a single key-value.
func formatSingleValue(w io.Writer, key, value string, format OutputFormat) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(kvPair{Key: key, Value: value})
	case FormatShell:
		_, err := fmt.Fprintf(w, "export %s=%s\n", key, shellQuote(value))
		return err
	case FormatTable:
		return formatKVTable(w, []kvPair{{Key: key, Value: value}})
	default:
		_, err := fmt.Fprintln(w, value)
		return err
	}
}
