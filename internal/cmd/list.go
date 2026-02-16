package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/parser"
)

// newListCmd creates the list subcommand.
func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all environment variables",
		Long: `Print all key-value pairs from the merged .env and .env.local files.

When a profile file is specified with --profile-file, it is loaded between
.env and .env.local: .env ← profile ← .env.local.

By default, values that are ref:// secret references are masked. Use
--show-secrets to reveal the full ref:// URIs.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			envFile, _ := cmd.Flags().GetString("file")
			localFile, _ := cmd.Flags().GetString("local-file")
			profileFile, _ := cmd.Flags().GetString("profile-file")
			showSecrets, _ := cmd.Flags().GetBool("show-secrets")
			return runList(cmd, envFile, profileFile, localFile, showSecrets)
		},
	}

	cmd.Flags().StringP("file", "f", ".env", "path to the .env file")
	cmd.Flags().String("local-file", ".env.local", "path to the .env.local override file")
	cmd.Flags().String("profile-file", "", "path to a profile-specific .env file (e.g., .env.staging)")
	cmd.Flags().Bool("show-secrets", false, "show ref:// values instead of masking them")

	return cmd
}

// runList loads env files, merges them, and prints all key-value pairs.
func runList(cmd *cobra.Command, envPath, profilePath, localPath string, showSecrets bool) error {
	merged, err := loadAndMergeEnv(cmd, envPath, profilePath, localPath)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	for _, entry := range merged.All() {
		value := displayValue(entry, showSecrets)
		_, _ = fmt.Fprintf(out, "%s=%s\n", entry.Key, value)
	}

	return nil
}

// displayValue returns the value to display for an entry. If the entry is a
// ref:// reference and showSecrets is false, the value is masked.
func displayValue(entry parser.Entry, showSecrets bool) string {
	if entry.IsRef && !showSecrets {
		return "ref://***"
	}
	return entry.Value
}
