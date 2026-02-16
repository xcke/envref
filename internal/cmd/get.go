package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/parser"
)

// newGetCmd creates the get subcommand.
func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <KEY>",
		Short: "Print the value of an environment variable",
		Long: `Look up a single key from the merged .env and .env.local files and print
its value to stdout.

When a profile file is specified with --profile-file, it is loaded between
.env and .env.local: .env ← profile ← .env.local.

If the value is an unresolved ref:// reference, it is printed as-is.
Use --file to specify a custom .env file path.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			envFile, _ := cmd.Flags().GetString("file")
			localFile, _ := cmd.Flags().GetString("local-file")
			profileFile, _ := cmd.Flags().GetString("profile-file")
			return runGet(cmd, args[0], envFile, profileFile, localFile)
		},
	}

	cmd.Flags().StringP("file", "f", ".env", "path to the .env file")
	cmd.Flags().String("local-file", ".env.local", "path to the .env.local override file")
	cmd.Flags().String("profile-file", "", "path to a profile-specific .env file (e.g., .env.staging)")

	return cmd
}

// runGet loads env files, merges them, and prints the value for the given key.
func runGet(cmd *cobra.Command, key, envPath, profilePath, localPath string) error {
	env, err := loadAndMergeEnv(cmd, envPath, profilePath, localPath)
	if err != nil {
		return err
	}

	entry, found := env.Get(key)
	if !found {
		return fmt.Errorf("key %q not found", key)
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), entry.Value)
	return nil
}

// printWarnings writes parser warnings to stderr for the given file.
func printWarnings(cmd *cobra.Command, path string, warnings []parser.Warning) {
	for _, w := range warnings {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %s\n", path, w)
	}
}
