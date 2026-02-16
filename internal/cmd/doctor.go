package cmd

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/parser"
)

// issue represents a single problem found by the doctor command.
type issue struct {
	File    string
	Line    int
	Key     string
	Message string
}

// newDoctorCmd creates the doctor subcommand.
func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check for common issues in .env files",
		Long: `Scan .env files for common problems that may cause subtle bugs or security issues.

Checks performed:
  - Duplicate keys (last value wins, but earlier definitions are shadowed)
  - Trailing whitespace in unquoted values
  - Unquoted values containing spaces (may lose data with some tools)
  - Empty values without explicit intent (KEY= with no value or quotes)
  - .env file not listed in .gitignore (risk of committing secrets)
  - .envrc exists but is not trusted by direnv

The command exits with code 1 if any issues are found, making it suitable
for CI pipelines and pre-commit hooks.

Examples:
  envref doctor                        # check .env and .env.local
  envref doctor --file .env.staging    # check a specific file`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			envFile, _ := cmd.Flags().GetString("file")
			localFile, _ := cmd.Flags().GetString("local-file")
			return runDoctor(cmd, envFile, localFile)
		},
	}

	cmd.Flags().StringP("file", "f", ".env", "path to the .env file")
	cmd.Flags().String("local-file", ".env.local", "path to the .env.local override file")

	return cmd
}

// runDoctor implements the doctor command logic.
func runDoctor(cmd *cobra.Command, envPath, localPath string) error {
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

	var allIssues []issue

	// Check each .env file that exists.
	for _, path := range []string{envPath, localPath} {
		if !fileExists(path) {
			continue
		}
		fileIssues, err := checkEnvFile(path)
		if err != nil {
			return fmt.Errorf("checking %s: %w", path, err)
		}
		allIssues = append(allIssues, fileIssues...)
	}

	// Check project-level concerns.
	allIssues = append(allIssues, checkGitignore(envPath)...)
	allIssues = append(allIssues, checkDirenvTrust()...)

	if len(allIssues) == 0 {
		_, _ = fmt.Fprintf(out, "OK: no issues found\n")
		return nil
	}

	// Report issues grouped by file.
	printIssues(errOut, allIssues)

	return fmt.Errorf("%d issue(s) found", len(allIssues))
}

// checkEnvFile parses a single .env file and checks for common issues.
func checkEnvFile(path string) ([]issue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}

	entries, warnings, parseErr := parser.Parse(f)
	closeErr := f.Close()
	if parseErr != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, parseErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("closing %s: %w", path, closeErr)
	}

	var issues []issue

	// Convert parser duplicate-key warnings to issues.
	for _, w := range warnings {
		issues = append(issues, issue{
			File:    path,
			Line:    w.Line,
			Message: w.Message,
		})
	}

	// Check each entry for problems.
	for _, entry := range entries {
		issues = append(issues, checkEntry(path, entry)...)
	}

	// Check for trailing whitespace at the raw line level. The parser trims
	// lines before extracting values, so we scan the file separately.
	trailingIssues, err := checkTrailingWhitespace(path)
	if err == nil {
		issues = append(issues, trailingIssues...)
	}

	return issues, nil
}

// checkEntry checks a single entry for common problems.
func checkEntry(path string, entry parser.Entry) []issue {
	var issues []issue

	// Unquoted value containing spaces — may be parsed differently by other tools.
	if entry.Quote == parser.QuoteNone && strings.Contains(entry.Value, " ") {
		issues = append(issues, issue{
			File:    path,
			Line:    entry.Line,
			Key:     entry.Key,
			Message: fmt.Sprintf("unquoted value with spaces for %q (consider quoting to avoid portability issues)", entry.Key),
		})
	}

	// Empty value without explicit quoting — might be unintentional.
	// We only flag KEY= (no value, no quotes). KEY="" or KEY='' is intentional.
	if entry.Value == "" && entry.Quote == parser.QuoteNone {
		issues = append(issues, issue{
			File:    path,
			Line:    entry.Line,
			Key:     entry.Key,
			Message: fmt.Sprintf("empty value for %q without explicit quotes (use KEY=\"\" if intentional)", entry.Key),
		})
	}

	return issues
}

// checkTrailingWhitespace scans a .env file line by line to detect unquoted
// values with trailing whitespace. This must be done on raw lines because the
// parser trims lines before extracting values.
func checkTrailingWhitespace(path string) ([]issue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	var issues []issue
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Strip CR for CRLF files.
		line = strings.TrimRight(line, "\r")

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
			continue
		}

		key := strings.TrimSpace(trimmed[:eqIdx])
		if key == "" {
			continue
		}

		// Get the raw value after =.
		rawValue := trimmed[eqIdx+1:]

		// Only check unquoted values (not starting with ', ", or `).
		valueTrimmed := strings.TrimLeft(rawValue, " \t")
		if valueTrimmed != "" && (valueTrimmed[0] == '\'' || valueTrimmed[0] == '"' || valueTrimmed[0] == '`') {
			continue
		}

		// Check if the original line (before TrimSpace) has trailing whitespace
		// after the value portion.
		origEqIdx := strings.IndexByte(line, '=')
		if origEqIdx < 0 {
			continue
		}
		origValue := line[origEqIdx+1:]
		if origValue != strings.TrimRight(origValue, " \t") {
			issues = append(issues, issue{
				File:    path,
				Line:    lineNum,
				Key:     key,
				Message: fmt.Sprintf("trailing whitespace in unquoted value for %q", key),
			})
		}
	}

	scanErr := scanner.Err()
	_ = f.Close()
	return issues, scanErr
}

// checkGitignore verifies that .env is listed in .gitignore.
func checkGitignore(envPath string) []issue {
	// Only check for the default .env file — custom paths are the user's concern.
	base := filepath.Base(envPath)
	if base != ".env" {
		return nil
	}

	dir := filepath.Dir(envPath)
	gitignorePath := filepath.Join(dir, ".gitignore")

	f, err := os.Open(gitignorePath)
	if err != nil {
		// No .gitignore at all — report only if .env exists.
		if os.IsNotExist(err) && fileExists(envPath) {
			return []issue{{
				File:    ".gitignore",
				Message: ".env is not in .gitignore (secrets may be committed to git)",
			}}
		}
		return nil
	}

	covered := gitignoreCovers(f, ".env")
	_ = f.Close()

	if !covered {
		return []issue{{
			File:    gitignorePath,
			Message: ".env is not in .gitignore (secrets may be committed to git)",
		}}
	}

	return nil
}

// gitignoreCovers checks whether a .gitignore file contains a pattern that
// would match the given filename. This is a simplified check — it looks for
// exact matches or common patterns, not full gitignore glob semantics.
func gitignoreCovers(r io.Reader, name string) bool {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Check common patterns that would cover .env:
		// ".env", ".env*", or just the exact name.
		if line == name || line == name+"*" {
			return true
		}
	}
	return false
}

// checkDirenvTrust checks if .envrc exists and whether direnv trusts it.
func checkDirenvTrust() []issue {
	if !fileExists(".envrc") {
		return nil
	}

	// Check if direnv is available.
	if _, err := exec.LookPath("direnv"); err != nil {
		return nil
	}

	if !direnvAllowed(".envrc") {
		return []issue{{
			File:    ".envrc",
			Message: ".envrc exists but may not be trusted by direnv (run \"direnv allow\" to trust it)",
		}}
	}

	return nil
}

// direnvAllowed checks if a .envrc file is in direnv's allow list.
// direnv stores allowed hashes at $XDG_DATA_HOME/direnv/allow/<sha256 of abs path>.
func direnvAllowed(envrcPath string) bool {
	absPath, err := filepath.Abs(envrcPath)
	if err != nil {
		return false
	}

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		dataHome = filepath.Join(home, ".local", "share")
	}

	allowDir := filepath.Join(dataHome, "direnv", "allow")
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(absPath)))

	_, err = os.Stat(filepath.Join(allowDir, hash))
	return err == nil
}

// printIssues formats and prints all issues to the writer.
func printIssues(w io.Writer, issues []issue) {
	// Group by file, preserving order of first appearance.
	var fileOrder []string
	grouped := make(map[string][]issue)
	for _, iss := range issues {
		if _, exists := grouped[iss.File]; !exists {
			fileOrder = append(fileOrder, iss.File)
		}
		grouped[iss.File] = append(grouped[iss.File], iss)
	}

	for _, file := range fileOrder {
		_, _ = fmt.Fprintf(w, "%s:\n", file)
		for _, iss := range grouped[file] {
			if iss.Line > 0 {
				_, _ = fmt.Fprintf(w, "  line %d: %s\n", iss.Line, iss.Message)
			} else {
				_, _ = fmt.Fprintf(w, "  %s\n", iss.Message)
			}
		}
	}

	_, _ = fmt.Fprintf(w, "\nDoctor found %d issue(s)\n", len(issues))
}
