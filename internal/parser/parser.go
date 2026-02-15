// Package parser implements a .env file parser that handles quoted values,
// comments, empty lines, multiline values, and variable interpolation markers.
package parser

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"unicode"
)

// Entry represents a single key-value pair parsed from a .env file.
type Entry struct {
	Key   string
	Value string
	// Raw is the original value before any unquoting or processing.
	Raw string
	// Line is the 1-based line number where this entry starts.
	Line int
}

// ParseError represents a parsing error with line context.
type ParseError struct {
	Line    int
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Message)
}

// Parse reads a .env formatted input and returns all entries found.
// It handles:
//   - KEY=VALUE pairs (with optional export prefix)
//   - Single-quoted values (literal, no escape processing)
//   - Double-quoted values (with escape processing: \n, \t, \\, \")
//   - Backtick-quoted values (literal, no escape processing)
//   - Multiline values inside double quotes
//   - Comments (lines starting with #, and inline comments for unquoted values)
//   - Empty lines (skipped)
//   - Whitespace trimming for unquoted values
func Parse(r io.Reader) ([]Entry, error) {
	var entries []Entry
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip empty lines and comments.
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed[0] == '#' {
			continue
		}

		// Strip optional "export " prefix.
		if strings.HasPrefix(trimmed, "export ") {
			trimmed = strings.TrimPrefix(trimmed, "export ")
			trimmed = strings.TrimSpace(trimmed)
		}

		// Find the = separator.
		eqIdx := strings.IndexByte(trimmed, '=')
		if eqIdx < 0 {
			// Lines without = are ignored (not an error, matches dotenv behavior).
			continue
		}

		key := strings.TrimSpace(trimmed[:eqIdx])
		if key == "" {
			continue
		}

		rawValue := trimmed[eqIdx+1:]
		startLine := lineNum

		value, raw, newLineNum, err := parseValue(rawValue, scanner, lineNum)
		if err != nil {
			return entries, &ParseError{Line: startLine, Message: err.Error()}
		}
		lineNum = newLineNum

		entries = append(entries, Entry{
			Key:   key,
			Value: value,
			Raw:   raw,
			Line:  startLine,
		})
	}

	if err := scanner.Err(); err != nil {
		return entries, fmt.Errorf("reading input: %w", err)
	}

	return entries, nil
}

// parseValue handles the value portion of a KEY=VALUE pair.
// It returns the processed value, the raw value, the updated line number, and any error.
func parseValue(rawValue string, scanner *bufio.Scanner, lineNum int) (string, string, int, error) {
	trimmed := strings.TrimLeftFunc(rawValue, unicode.IsSpace)

	if trimmed == "" {
		return "", rawValue, lineNum, nil
	}

	switch trimmed[0] {
	case '\'':
		value, raw, ln, err := parseSingleQuoted(trimmed, lineNum)
		return value, raw, ln, err
	case '"':
		value, raw, ln, err := parseDoubleQuoted(trimmed, scanner, lineNum)
		return value, raw, ln, err
	case '`':
		value, raw, ln, err := parseBacktickQuoted(trimmed, scanner, lineNum)
		return value, raw, ln, err
	default:
		return parseUnquoted(rawValue), rawValue, lineNum, nil
	}
}

// parseSingleQuoted parses a single-quoted value. No escape processing.
// Single-quoted values do not support multiline.
func parseSingleQuoted(raw string, lineNum int) (string, string, int, error) {
	// Find closing quote on the same line.
	closeIdx := strings.IndexByte(raw[1:], '\'')
	if closeIdx < 0 {
		return "", raw, lineNum, fmt.Errorf("unterminated single-quoted value")
	}
	value := raw[1 : closeIdx+1]
	return value, raw, lineNum, nil
}

// parseDoubleQuoted parses a double-quoted value with escape processing.
// Supports multiline values spanning multiple lines.
func parseDoubleQuoted(raw string, scanner *bufio.Scanner, lineNum int) (string, string, int, error) {
	// Collect all content until closing unescaped quote.
	content := raw[1:] // skip opening quote
	var fullRaw strings.Builder
	fullRaw.WriteString(raw)

	for {
		idx, escaped := findClosingDoubleQuote(content)
		if idx >= 0 {
			// Found closing quote.
			segment := content[:idx]
			value := processDoubleQuoteEscapes(escaped[:len(escaped)-1]) // exclude the closing quote marker
			_ = segment
			return value, fullRaw.String(), lineNum, nil
		}

		// No closing quote found — continue to next line if available.
		if !scanner.Scan() {
			return "", fullRaw.String(), lineNum, fmt.Errorf("unterminated double-quoted value")
		}
		lineNum++
		nextLine := scanner.Text()
		fullRaw.WriteByte('\n')
		fullRaw.WriteString(nextLine)
		content = content + "\n" + nextLine
	}
}

// findClosingDoubleQuote scans content for an unescaped double quote.
// Returns the index of the closing quote and the content up to and including it.
// Returns -1 if no closing quote is found.
func findClosingDoubleQuote(s string) (int, string) {
	escaped := false
	for i, ch := range s {
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			return i, s[:i+1]
		}
	}
	return -1, ""
}

// processDoubleQuoteEscapes handles escape sequences in double-quoted values.
func processDoubleQuoteEscapes(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	escaped := false
	for _, ch := range s {
		if escaped {
			switch ch {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			default:
				// Unknown escape — keep both characters.
				b.WriteByte('\\')
				b.WriteRune(ch)
			}
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		b.WriteRune(ch)
	}
	// If trailing backslash, keep it.
	if escaped {
		b.WriteByte('\\')
	}
	return b.String()
}

// parseBacktickQuoted parses a backtick-quoted value. No escape processing.
// Supports multiline values spanning multiple lines.
func parseBacktickQuoted(raw string, scanner *bufio.Scanner, lineNum int) (string, string, int, error) {
	content := raw[1:] // skip opening backtick
	var fullRaw strings.Builder
	fullRaw.WriteString(raw)

	for {
		closeIdx := strings.IndexByte(content, '`')
		if closeIdx >= 0 {
			value := content[:closeIdx]
			return value, fullRaw.String(), lineNum, nil
		}

		if !scanner.Scan() {
			return "", fullRaw.String(), lineNum, fmt.Errorf("unterminated backtick-quoted value")
		}
		lineNum++
		nextLine := scanner.Text()
		fullRaw.WriteByte('\n')
		fullRaw.WriteString(nextLine)
		content = content + "\n" + nextLine
	}
}

// parseUnquoted processes an unquoted value: trims whitespace and strips inline comments.
func parseUnquoted(raw string) string {
	// Inline comments: strip everything after an unquoted #.
	// Only treat # as a comment if preceded by whitespace.
	value := stripInlineComment(raw)
	return strings.TrimSpace(value)
}

// stripInlineComment removes inline comments from unquoted values.
// A # is treated as a comment start only when preceded by whitespace.
func stripInlineComment(s string) string {
	for i := 1; i < len(s); i++ {
		if s[i] == '#' && s[i-1] == ' ' {
			return s[:i]
		}
	}
	return s
}
