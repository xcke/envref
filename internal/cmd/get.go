package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/envfile"
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
	base, err := envfile.Load(envPath)
	if err != nil {
		return fmt.Errorf("loading %s: %w", envPath, err)
	}

	local, err := envfile.LoadOptional(localPath)
	if err != nil {
		return fmt.Errorf("loading %s: %w", localPath, err)
	}

	merged := envfile.Merge(base, local)

	entry, found := merged.Get(key)
	if !found {
		return fmt.Errorf("key %q not found", key)
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), entry.Value)
	return nil
}
