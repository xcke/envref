package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/envfile"
	"github.com/xcke/envref/internal/parser"
)

// newGetCmd creates the get subcommand.
func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <KEY>",
		Short: "Print the value of an environment variable",
		Long: `Look up a single key from the merged .env and .env.local files and print
its value to stdout.

If the value is an unresolved ref:// reference, it is printed as-is.
Use --file to specify a custom .env file path.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			envFile, _ := cmd.Flags().GetString("file")
			localFile, _ := cmd.Flags().GetString("local-file")
			return runGet(cmd, args[0], envFile, localFile)
		},
	}

	cmd.Flags().StringP("file", "f", ".env", "path to the .env file")
	cmd.Flags().String("local-file", ".env.local", "path to the .env.local override file")

	return cmd
}

// runGet loads env files, merges them, and prints the value for the given key.
func runGet(cmd *cobra.Command, key, envPath, localPath string) error {
	base, warnings, err := envfile.Load(envPath)
	if err != nil {
		return fmt.Errorf("loading %s: %w", envPath, err)
	}
	printWarnings(cmd, envPath, warnings)

	local, localWarnings, err := envfile.LoadOptional(localPath)
	if err != nil {
		return fmt.Errorf("loading %s: %w", localPath, err)
	}
	printWarnings(cmd, localPath, localWarnings)

	merged := envfile.Merge(base, local)
	envfile.Interpolate(merged)

	entry, found := merged.Get(key)
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
