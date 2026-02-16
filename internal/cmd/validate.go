package cmd

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/envfile"
	"github.com/xcke/envref/internal/output"
	"github.com/xcke/envref/internal/schema"
)

// newValidateCmd creates the validate subcommand.
func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Check .env against .env.example schema",
		Long: `Validate that your .env file contains all keys defined in .env.example.

The .env.example file serves as a schema, defining which keys are required.
This command compares the effective (merged) environment against .env.example
and reports:

  - Missing keys: defined in .env.example but not in your environment
  - Extra keys:   defined in your environment but not in .env.example

Missing keys cause a non-zero exit code, making this suitable for CI checks.
Extra keys are reported as warnings but do not cause failure.

Use --ci for CI pipelines: extra keys are treated as errors, output is compact,
and success produces no output (exit code 0 = pass, 1 = fail).

Use --schema to validate values against a .env.schema.json file with type
constraints (string, number, boolean, url, enum, email, port), patterns, and
required/optional declarations.

Examples:
  envref validate                                # compare .env against .env.example
  envref validate --example .env.schema          # use custom schema file
  envref validate --file .env.staging            # validate a specific env file
  envref validate --ci                           # strict CI mode (extra keys are errors, silent on success)
  envref validate --schema .env.schema.json      # validate value types against JSON schema`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			envFile, _ := cmd.Flags().GetString("file")
			localFile, _ := cmd.Flags().GetString("local-file")
			profileFile, _ := cmd.Flags().GetString("profile-file")
			exampleFile, _ := cmd.Flags().GetString("example")
			schemaFile, _ := cmd.Flags().GetString("schema")
			ci, _ := cmd.Flags().GetBool("ci")
			return runValidate(cmd, envFile, profileFile, localFile, exampleFile, schemaFile, ci)
		},
	}

	cmd.Flags().StringP("file", "f", ".env", "path to the .env file")
	cmd.Flags().String("local-file", ".env.local", "path to the .env.local override file")
	cmd.Flags().String("profile-file", "", "path to a profile-specific .env file")
	cmd.Flags().StringP("example", "e", ".env.example", "path to the example/schema .env file")
	cmd.Flags().StringP("schema", "s", "", "path to .env.schema.json for type validation")
	cmd.Flags().Bool("ci", false, "CI mode: extra keys are errors, silent on success, exit code 1 on any failure")

	return cmd
}

// runValidate compares the merged environment against the example schema file.
// When ci is true, extra keys are treated as errors, output is compact, and
// success produces no output (exit code 0 = pass, 1 = fail).
// When schemaPath is non-empty, values are also validated against a JSON schema.
func runValidate(cmd *cobra.Command, envPath, profilePath, localPath, examplePath, schemaPath string, ci bool) error {
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()
	w := output.NewWriter(cmd)

	// Load and merge the effective environment.
	merged, err := loadAndMergeEnv(cmd, envPath, profilePath, localPath)
	if err != nil {
		return err
	}

	// Track overall validation state.
	var missing, extra []string
	var schemaErrors []schema.ValidationError
	exampleKeyCount := 0

	// --- Example-based validation (key presence) ---
	// Load the example/schema file (required).
	example, exampleWarnings, err := envfile.Load(examplePath)
	if err != nil {
		return fmt.Errorf("loading example file: %w", err)
	}
	printWarnings(cmd, examplePath, exampleWarnings)

	// Build key sets.
	exampleKeys := keySet(example.Keys())
	envKeys := keySet(merged.Keys())
	exampleKeyCount = len(exampleKeys)

	// Find missing keys (in example but not in env).
	for key := range exampleKeys {
		if _, ok := envKeys[key]; !ok {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)

	// Find extra keys (in env but not in example).
	for key := range envKeys {
		if _, ok := exampleKeys[key]; !ok {
			extra = append(extra, key)
		}
	}
	sort.Strings(extra)

	// --- JSON schema validation (type checking) ---
	if schemaPath != "" {
		s, loadErr := schema.Load(schemaPath)
		if loadErr != nil {
			return fmt.Errorf("loading schema: %w", loadErr)
		}

		// Build a value map from the merged environment.
		valueMap := make(map[string]string, merged.Len())
		for _, entry := range merged.All() {
			valueMap[entry.Key] = entry.Value
		}

		result := s.Validate(valueMap)
		schemaErrors = result.Errors
	}

	// --- Determine if everything is OK ---
	hasKeyErrors := len(missing) > 0 || len(extra) > 0
	hasSchemaErrors := len(schemaErrors) > 0

	if !hasKeyErrors && !hasSchemaErrors {
		if !ci && !w.IsQuiet() {
			msg := fmt.Sprintf("%s: all %d keys match %s", w.Green("OK"), exampleKeyCount, examplePath)
			if schemaPath != "" {
				msg += fmt.Sprintf("; schema %s OK", schemaPath)
			}
			_, _ = fmt.Fprintln(out, msg)
		}
		return nil
	}

	// --- CI mode: compact output ---
	if ci {
		return runValidateCI(errOut, missing, extra, schemaErrors, examplePath)
	}

	// --- Normal mode ---
	if len(missing) > 0 {
		_, _ = fmt.Fprintf(errOut, "%s (defined in %s but not in environment):\n", w.Red("Missing keys"), examplePath)
		for _, key := range missing {
			_, _ = fmt.Fprintf(errOut, "  - %s\n", key)
		}
	}

	if len(extra) > 0 {
		_, _ = fmt.Fprintf(errOut, "%s (defined in environment but not in %s):\n", w.Yellow("Extra keys"), examplePath)
		for _, key := range extra {
			_, _ = fmt.Fprintf(errOut, "  - %s\n", key)
		}
	}

	if len(schemaErrors) > 0 {
		_, _ = fmt.Fprintf(errOut, "%s (from %s):\n", w.Red("Type errors"), schemaPath)
		for _, e := range schemaErrors {
			_, _ = fmt.Fprintf(errOut, "  - %s: %s\n", e.Key, e.Message)
		}
	}

	// Summary line.
	var parts []string
	if len(missing) > 0 {
		parts = append(parts, fmt.Sprintf("%d missing", len(missing)))
	}
	if len(extra) > 0 {
		parts = append(parts, fmt.Sprintf("%d extra", len(extra)))
	}
	if len(schemaErrors) > 0 {
		parts = append(parts, fmt.Sprintf("%d type error(s)", len(schemaErrors)))
	}
	_, _ = fmt.Fprintf(errOut, "\n%s: %s\n", w.Red("Validation failed"), strings.Join(parts, ", "))

	// Missing keys and schema errors are hard failures; extra keys alone are a warning.
	if len(missing) > 0 || len(schemaErrors) > 0 {
		errorCount := len(missing) + len(schemaErrors)
		return fmt.Errorf("%d validation error(s)", errorCount)
	}

	return nil
}

// runValidateCI handles CI mode output. In CI mode, both missing and extra keys
// are errors, and output uses a compact format suitable for CI log parsers.
func runValidateCI(w io.Writer, missing, extra []string, schemaErrors []schema.ValidationError, examplePath string) error {
	for _, key := range missing {
		_, _ = fmt.Fprintf(w, "error: missing key %s (required by %s)\n", key, examplePath)
	}
	for _, key := range extra {
		_, _ = fmt.Fprintf(w, "error: extra key %s (not in %s)\n", key, examplePath)
	}
	for _, e := range schemaErrors {
		_, _ = fmt.Fprintf(w, "error: %s: %s\n", e.Key, e.Message)
	}

	total := len(missing) + len(extra) + len(schemaErrors)
	var parts []string
	if len(missing) > 0 {
		parts = append(parts, fmt.Sprintf("%d missing", len(missing)))
	}
	if len(extra) > 0 {
		parts = append(parts, fmt.Sprintf("%d extra", len(extra)))
	}
	if len(schemaErrors) > 0 {
		parts = append(parts, fmt.Sprintf("%d type error(s)", len(schemaErrors)))
	}
	return fmt.Errorf("validation failed: %d error(s) (%s)", total, strings.Join(parts, ", "))
}

// keySet converts a slice of strings to a set (map).
func keySet(keys []string) map[string]struct{} {
	m := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		m[k] = struct{}{}
	}
	return m
}
