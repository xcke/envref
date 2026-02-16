package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseFormat_Valid(t *testing.T) {
	tests := []struct {
		input    string
		expected OutputFormat
	}{
		{"plain", FormatPlain},
		{"json", FormatJSON},
		{"shell", FormatShell},
		{"table", FormatTable},
		{"PLAIN", FormatPlain},
		{"JSON", FormatJSON},
		{"Shell", FormatShell},
		{"Table", FormatTable},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseFormat(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("parseFormat(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseFormat_Invalid(t *testing.T) {
	tests := []string{"xml", "csv", "yaml", "", "plaintext"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := parseFormat(input)
			if err == nil {
				t.Fatalf("expected error for invalid format %q, got nil", input)
			}
			if !strings.Contains(err.Error(), "invalid format") {
				t.Errorf("expected 'invalid format' in error, got: %v", err)
			}
		})
	}
}

func TestFormatKVPairs_Plain(t *testing.T) {
	pairs := []kvPair{
		{Key: "DB_HOST", Value: "localhost"},
		{Key: "DB_PORT", Value: "5432"},
	}

	buf := new(bytes.Buffer)
	if err := formatKVPairs(buf, pairs, FormatPlain); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "DB_HOST=localhost\nDB_PORT=5432\n"
	if got := buf.String(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatKVPairs_JSON(t *testing.T) {
	pairs := []kvPair{
		{Key: "DB_HOST", Value: "localhost"},
		{Key: "DB_PORT", Value: "5432"},
	}

	buf := new(bytes.Buffer)
	if err := formatKVPairs(buf, pairs, FormatJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it's valid JSON.
	var result []kvPair
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if result[0].Key != "DB_HOST" || result[0].Value != "localhost" {
		t.Errorf("first entry: got %+v", result[0])
	}
	if result[1].Key != "DB_PORT" || result[1].Value != "5432" {
		t.Errorf("second entry: got %+v", result[1])
	}
}

func TestFormatKVPairs_JSON_Empty(t *testing.T) {
	buf := new(bytes.Buffer)
	if err := formatKVPairs(buf, []kvPair{}, FormatJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result []kvPair
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	if len(result) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result))
	}
}

func TestFormatKVPairs_Shell(t *testing.T) {
	pairs := []kvPair{
		{Key: "HOST", Value: "localhost"},
		{Key: "GREETING", Value: "hello world"},
		{Key: "EMPTY", Value: ""},
	}

	buf := new(bytes.Buffer)
	if err := formatKVPairs(buf, pairs, FormatShell); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	expected := []string{
		"export HOST=localhost",
		"export GREETING='hello world'",
		"export EMPTY=''",
	}

	if len(lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d: %q", len(expected), len(lines), buf.String())
	}
	for i, exp := range expected {
		if lines[i] != exp {
			t.Errorf("line %d: expected %q, got %q", i, exp, lines[i])
		}
	}
}

func TestFormatKVPairs_Table(t *testing.T) {
	pairs := []kvPair{
		{Key: "DB_HOST", Value: "localhost"},
		{Key: "PORT", Value: "5432"},
	}

	buf := new(bytes.Buffer)
	if err := formatKVPairs(buf, pairs, FormatTable); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")

	// Should have header, separator, and 2 data rows.
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %q", len(lines), buf.String())
	}

	// Header should contain KEY and VALUE.
	if !strings.Contains(lines[0], "KEY") || !strings.Contains(lines[0], "VALUE") {
		t.Errorf("header missing KEY/VALUE: %q", lines[0])
	}

	// Separator line should contain dashes.
	if !strings.Contains(lines[1], "---") {
		t.Errorf("separator line should contain dashes: %q", lines[1])
	}

	// Data rows should contain the values.
	if !strings.Contains(lines[2], "DB_HOST") || !strings.Contains(lines[2], "localhost") {
		t.Errorf("first data row missing expected values: %q", lines[2])
	}
	if !strings.Contains(lines[3], "PORT") || !strings.Contains(lines[3], "5432") {
		t.Errorf("second data row missing expected values: %q", lines[3])
	}
}

func TestFormatKVPairs_Table_Empty(t *testing.T) {
	buf := new(bytes.Buffer)
	if err := formatKVPairs(buf, []kvPair{}, FormatTable); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := buf.String(); got != "" {
		t.Errorf("expected empty output for empty table, got %q", got)
	}
}

func TestFormatKVPairs_Table_Alignment(t *testing.T) {
	pairs := []kvPair{
		{Key: "A", Value: "short"},
		{Key: "VERY_LONG_KEY_NAME", Value: "value"},
	}

	buf := new(bytes.Buffer)
	if err := formatKVPairs(buf, pairs, FormatTable); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(lines))
	}

	// All lines should have consistent column alignment.
	// The KEY column should be at least as wide as the longest key.
	for i := 2; i < len(lines); i++ {
		// Each data row key should be padded to align with the header.
		parts := strings.SplitN(lines[i], "  ", 2)
		if len(parts) < 2 {
			t.Errorf("line %d doesn't have expected column separation: %q", i, lines[i])
		}
	}
}

func TestFormatSingleValue_Plain(t *testing.T) {
	buf := new(bytes.Buffer)
	if err := formatSingleValue(buf, "DB_HOST", "localhost", FormatPlain); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "localhost\n"
	if got := buf.String(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatSingleValue_JSON(t *testing.T) {
	buf := new(bytes.Buffer)
	if err := formatSingleValue(buf, "DB_HOST", "localhost", FormatJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result kvPair
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	if result.Key != "DB_HOST" || result.Value != "localhost" {
		t.Errorf("expected {DB_HOST, localhost}, got %+v", result)
	}
}

func TestFormatSingleValue_Shell(t *testing.T) {
	buf := new(bytes.Buffer)
	if err := formatSingleValue(buf, "GREETING", "hello world", FormatShell); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "export GREETING='hello world'\n"
	if got := buf.String(); got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatSingleValue_Table(t *testing.T) {
	buf := new(bytes.Buffer)
	if err := formatSingleValue(buf, "PORT", "8080", FormatTable); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (header, separator, row), got %d: %q", len(lines), buf.String())
	}
	if !strings.Contains(lines[2], "PORT") || !strings.Contains(lines[2], "8080") {
		t.Errorf("data row missing expected values: %q", lines[2])
	}
}

func TestFormatKVPairs_JSON_SpecialChars(t *testing.T) {
	pairs := []kvPair{
		{Key: "URL", Value: `postgres://user:p@ss"word@host/db`},
		{Key: "MULTILINE", Value: "line1\nline2"},
	}

	buf := new(bytes.Buffer)
	if err := formatKVPairs(buf, pairs, FormatJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result []kvPair
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	if result[0].Value != `postgres://user:p@ss"word@host/db` {
		t.Errorf("JSON should preserve special characters: got %q", result[0].Value)
	}
	if result[1].Value != "line1\nline2" {
		t.Errorf("JSON should preserve newlines: got %q", result[1].Value)
	}
}
