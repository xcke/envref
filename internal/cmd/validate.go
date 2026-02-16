package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/envfile"
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

Examples:
  envref validate                                # compare .env against .env.example
  envref validate --example .env.schema          # use custom schema file
  envref validate --file .env.staging            # validate a specific env file`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			envFile, _ := cmd.Flags().GetString("file")
			localFile, _ := cmd.Flags().GetString("local-file")
			profileFile, _ := cmd.Flags().GetString("profile-file")
			exampleFile, _ := cmd.Flags().GetString("example")
			return runValidate(cmd, envFile, profileFile, localFile, exampleFile)
		},
	}

	cmd.Flags().StringP("file", "f", ".env", "path to the .env file")
	cmd.Flags().String("local-file", ".env.local", "path to the .env.local override file")
	cmd.Flags().String("profile-file", "", "path to a profile-specific .env file")
	cmd.Flags().StringP("example", "e", ".env.example", "path to the example/schema .env file")

	return cmd
}

// runValidate compares the merged environment against the example schema file.
func runValidate(cmd *cobra.Command, envPath, profilePath, localPath, examplePath string) error {
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

	// Load the example/schema file (required).
	example, exampleWarnings, err := envfile.Load(examplePath)
	if err != nil {
		return fmt.Errorf("loading example file: %w", err)
	}
	printWarnings(cmd, examplePath, exampleWarnings)

	// Load and merge the effective environment.
	merged, err := loadAndMergeEnv(cmd, envPath, profilePath, localPath)
	if err != nil {
		return err
	}

	// Build key sets.
	exampleKeys := keySet(example.Keys())
	envKeys := keySet(merged.Keys())

	// Find missing keys (in example but not in env).
	var missing []string
	for key := range exampleKeys {
		if _, ok := envKeys[key]; !ok {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)

	// Find extra keys (in env but not in example).
	var extra []string
	for key := range envKeys {
		if _, ok := exampleKeys[key]; !ok {
			extra = append(extra, key)
		}
	}
	sort.Strings(extra)

	// Report results.
	if len(missing) == 0 && len(extra) == 0 {
		_, _ = fmt.Fprintf(out, "OK: all %d keys match %s\n", len(exampleKeys), examplePath)
		return nil
	}

	if len(missing) > 0 {
		_, _ = fmt.Fprintf(errOut, "Missing keys (defined in %s but not in environment):\n", examplePath)
		for _, key := range missing {
			_, _ = fmt.Fprintf(errOut, "  - %s\n", key)
		}
	}

	if len(extra) > 0 {
		_, _ = fmt.Fprintf(errOut, "Extra keys (defined in environment but not in %s):\n", examplePath)
		for _, key := range extra {
			_, _ = fmt.Fprintf(errOut, "  - %s\n", key)
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
	_, _ = fmt.Fprintf(errOut, "\nValidation failed: %s\n", strings.Join(parts, ", "))

	// Missing keys are an error; extra keys alone are a warning.
	if len(missing) > 0 {
		return fmt.Errorf("%d required key(s) missing from environment", len(missing))
	}

	return nil
}

// keySet converts a slice of strings to a set (map).
func keySet(keys []string) map[string]struct{} {
	m := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		m[k] = struct{}{}
	}
	return m
}
