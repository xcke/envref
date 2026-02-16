package cmd

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/output"
	"github.com/xcke/envref/internal/parser"
)

// auditFinding represents a potential plaintext secret found by the audit command.
type auditFinding struct {
	File    string
	Line    int
	Key     string
	Reason  string
	Pattern string // which detection method flagged it
}

// newAuditCmd creates the audit subcommand.
func newAuditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Warn about secrets that might be in plaintext .env files",
		Long: `Scan .env files for values that look like they might be plaintext secrets
rather than safe ref:// references.

Detection methods:
  - Pattern matching: known API key/token prefixes (sk-, ghp_, aws_, etc.)
  - Key name heuristics: keys named SECRET, TOKEN, PASSWORD, API_KEY, etc.
    with non-trivial plaintext values
  - Entropy analysis: high-entropy strings that look like base64/hex-encoded
    secrets (tokens, hashes, private keys)

Values that are ref:// references, variable interpolations (${}), empty,
or short/low-entropy are not flagged.

The command exits with code 1 if any potential secrets are found, making it
suitable for CI pipelines and pre-commit hooks.

Examples:
  envref audit                        # check .env and .env.local
  envref audit --file .env.staging    # check a specific file
  envref audit --min-entropy 4.0      # raise entropy threshold (stricter)`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			envFile, _ := cmd.Flags().GetString("file")
			localFile, _ := cmd.Flags().GetString("local-file")
			minEntropy, _ := cmd.Flags().GetFloat64("min-entropy")
			return runAudit(cmd, envFile, localFile, minEntropy)
		},
	}

	cmd.Flags().StringP("file", "f", ".env", "path to the .env file")
	cmd.Flags().String("local-file", ".env.local", "path to the .env.local override file")
	cmd.Flags().Float64("min-entropy", 3.5, "minimum Shannon entropy to flag a value (bits per character)")

	return cmd
}

// runAudit implements the audit command logic.
func runAudit(cmd *cobra.Command, envPath, localPath string, minEntropy float64) error {
	w := output.NewWriter(cmd)

	var allFindings []auditFinding

	for _, path := range []string{envPath, localPath} {
		if !fileExists(path) {
			continue
		}
		findings, err := auditFile(path, minEntropy)
		if err != nil {
			return fmt.Errorf("auditing %s: %w", path, err)
		}
		allFindings = append(allFindings, findings...)
	}

	if len(allFindings) == 0 {
		if !w.IsQuiet() {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: no plaintext secrets detected\n", w.Green("OK"))
		}
		return nil
	}

	printAuditFindings(w, allFindings)

	return fmt.Errorf("%d potential secret(s) found", len(allFindings))
}

// auditFile parses a .env file and checks each entry for potential plaintext secrets.
func auditFile(path string, minEntropy float64) ([]auditFinding, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}

	entries, _, parseErr := parser.Parse(f)
	closeErr := f.Close()
	if parseErr != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, parseErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("closing %s: %w", path, closeErr)
	}

	var findings []auditFinding
	for _, entry := range entries {
		findings = append(findings, auditEntry(path, entry, minEntropy)...)
	}

	return findings, nil
}

// auditEntry checks a single entry for potential plaintext secrets.
func auditEntry(path string, entry parser.Entry, minEntropy float64) []auditFinding {
	// Skip ref:// references — these are already safe.
	if entry.IsRef {
		return nil
	}

	value := entry.Value

	// Skip empty values.
	if value == "" {
		return nil
	}

	// Skip variable interpolation references (e.g. ${OTHER_VAR}).
	if strings.Contains(value, "${") {
		return nil
	}

	// Skip obviously non-secret values: booleans, small numbers, localhost, etc.
	if isSafeValue(value) {
		return nil
	}

	var findings []auditFinding

	// Check 1: Known API key / token prefixes.
	if pattern, ok := matchesKnownSecretPattern(value); ok {
		findings = append(findings, auditFinding{
			File:    path,
			Line:    entry.Line,
			Key:     entry.Key,
			Reason:  fmt.Sprintf("value matches known secret pattern (%s)", pattern),
			Pattern: "pattern",
		})
		return findings
	}

	// Check 2: Key name suggests a secret + value looks non-trivial.
	if isSecretKeyName(entry.Key) && len(value) >= 8 {
		findings = append(findings, auditFinding{
			File:    path,
			Line:    entry.Line,
			Key:     entry.Key,
			Reason:  fmt.Sprintf("key name suggests a secret and value is non-trivial (%d chars)", len(value)),
			Pattern: "key-name",
		})
		return findings
	}

	// Check 3: High entropy value (potential base64/hex token).
	if len(value) >= 16 {
		entropy := shannonEntropy(value)
		if entropy >= minEntropy {
			findings = append(findings, auditFinding{
				File:    path,
				Line:    entry.Line,
				Key:     entry.Key,
				Reason:  fmt.Sprintf("high-entropy value (%.2f bits/char) may be a secret", entropy),
				Pattern: "entropy",
			})
		}
	}

	return findings
}

// knownSecretPatterns maps pattern names to regex patterns that match known
// API key / token formats.
var knownSecretPatterns = []struct {
	Name    string
	Pattern *regexp.Regexp
}{
	{"Stripe key", regexp.MustCompile(`^[sr]k_(live|test)_[A-Za-z0-9]{10,}$`)},
	{"GitHub token", regexp.MustCompile(`^gh[ps]_[A-Za-z0-9]{36,}$`)},
	{"GitHub fine-grained token", regexp.MustCompile(`^github_pat_[A-Za-z0-9_]{20,}$`)},
	{"AWS access key", regexp.MustCompile(`^AKIA[0-9A-Z]{16}$`)},
	{"Slack token", regexp.MustCompile(`^xox[bporas]-[A-Za-z0-9-]+$`)},
	{"Slack webhook", regexp.MustCompile(`^https://hooks\.slack\.com/`)},
	{"npm token", regexp.MustCompile(`^npm_[A-Za-z0-9]{36,}$`)},
	{"PyPI token", regexp.MustCompile(`^pypi-[A-Za-z0-9_-]{50,}$`)},
	{"SendGrid key", regexp.MustCompile(`^SG\.[A-Za-z0-9_-]{22,}\.[A-Za-z0-9_-]{22,}$`)},
	{"Twilio key", regexp.MustCompile(`^SK[0-9a-f]{32}$`)},
	{"Mailgun key", regexp.MustCompile(`^key-[A-Za-z0-9]{32,}$`)},
	{"Square token", regexp.MustCompile(`^sq0[a-z]{3}-[A-Za-z0-9_-]{22,}$`)},
	{"Heroku API key", regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)},
	{"JWT", regexp.MustCompile(`^eyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}$`)},
	{"private key header", regexp.MustCompile(`^-----BEGIN (RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`)},
}

// matchesKnownSecretPattern checks if a value matches any known secret format.
func matchesKnownSecretPattern(value string) (string, bool) {
	for _, p := range knownSecretPatterns {
		if p.Pattern.MatchString(value) {
			return p.Name, true
		}
	}
	return "", false
}

// secretKeyWords are substrings in key names that suggest the value is a secret.
var secretKeyWords = []string{
	"SECRET",
	"PASSWORD",
	"PASSWD",
	"TOKEN",
	"API_KEY",
	"APIKEY",
	"PRIVATE_KEY",
	"PRIVATE_KEY_ID",
	"ACCESS_KEY",
	"AUTH_KEY",
	"CREDENTIALS",
	"ENCRYPTION_KEY",
	"SIGNING_KEY",
	"CLIENT_SECRET",
}

// isSecretKeyName checks if a key name contains words suggesting it holds a secret.
func isSecretKeyName(key string) bool {
	upper := strings.ToUpper(key)
	for _, word := range secretKeyWords {
		if strings.Contains(upper, word) {
			return true
		}
	}
	return false
}

// safeValues are exact values that should never be flagged.
var safeValues = map[string]bool{
	"true":      true,
	"false":     true,
	"yes":       true,
	"no":        true,
	"on":        true,
	"off":       true,
	"null":      true,
	"none":      true,
	"localhost": true,
	"127.0.0.1": true,
	"0.0.0.0":   true,
	"::1":       true,
}

// isSafeValue returns true for values that are clearly not secrets.
func isSafeValue(value string) bool {
	lower := strings.ToLower(value)

	// Exact match against known safe values.
	if safeValues[lower] {
		return true
	}

	// Very short values (< 6 chars) are unlikely to be secrets.
	if len(value) < 6 {
		return true
	}

	// URLs to localhost or common internal services (not webhooks).
	if strings.HasPrefix(lower, "http://localhost") ||
		strings.HasPrefix(lower, "http://127.0.0.1") ||
		strings.HasPrefix(lower, "http://0.0.0.0") {
		return true
	}

	return false
}

// shannonEntropy calculates the Shannon entropy of a string in bits per character.
// Higher entropy values indicate more randomness, which is typical of secrets.
func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}

	freq := make(map[rune]int)
	for _, r := range s {
		freq[r]++
	}

	length := float64(len([]rune(s)))
	entropy := 0.0
	for _, count := range freq {
		p := float64(count) / length
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}

// printAuditFindings formats and prints all audit findings to stderr.
func printAuditFindings(w *output.Writer, findings []auditFinding) {
	out := w.Stderr()

	// Group by file, preserving order of first appearance.
	var fileOrder []string
	grouped := make(map[string][]auditFinding)
	for _, f := range findings {
		if _, exists := grouped[f.File]; !exists {
			fileOrder = append(fileOrder, f.File)
		}
		grouped[f.File] = append(grouped[f.File], f)
	}

	for _, file := range fileOrder {
		_, _ = fmt.Fprintf(out, "%s:\n", w.Bold(file))
		for _, f := range grouped[file] {
			_, _ = fmt.Fprintf(out, "  line %d: %s — %s\n",
				f.Line,
				w.Yellow(f.Key),
				f.Reason,
			)
		}
	}

	hint := "Use ref:// references to keep secrets out of .env files"
	_, _ = fmt.Fprintf(out, "\nAudit found %s. %s\n",
		w.Red(fmt.Sprintf("%d potential secret(s)", len(findings))),
		hint,
	)
}
