package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/envfile"
	"github.com/xcke/envref/internal/parser"
)

// newSetCmd creates the set subcommand.
func newSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <KEY>=<VALUE>",
		Short: "Set an environment variable in a .env file",
		Long: `Set or update a single key-value pair in a .env file.

The argument must be in KEY=VALUE format. If the key already exists in the
target file, its value is updated in place. If the key is new, it is appended.

By default, values are written to .env. Use --local to write to .env.local
instead (for personal overrides that should not be committed).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			localFile, _ := cmd.Flags().GetString("local-file")
			useLocal, _ := cmd.Flags().GetBool("local")

			targetFile := file
			if useLocal {
				targetFile = localFile
			}

			return runSet(cmd, args[0], targetFile)
		},
	}

	cmd.Flags().StringP("file", "f", ".env", "path to the .env file")
	cmd.Flags().String("local-file", ".env.local", "path to the .env.local override file")
	cmd.Flags().Bool("local", false, "write to .env.local instead of .env")

	return cmd
}

// runSet parses the KEY=VALUE argument, loads the target file, updates the
// entry, and writes the file back to disk.
func runSet(cmd *cobra.Command, arg, targetPath string) error {
	key, value, err := parseKeyValue(arg)
	if err != nil {
		return err
	}

	// Load existing file or start fresh if it doesn't exist.
	env, warnings, err := envfile.LoadOptional(targetPath)
	if err != nil {
		return fmt.Errorf("loading %s: %w", targetPath, err)
	}
	printWarnings(cmd, targetPath, warnings)

	// Create the entry.
	entry := parser.Entry{
		Key:   key,
		Value: value,
		Raw:   value,
		IsRef: strings.HasPrefix(value, parser.RefPrefix),
	}

	env.Set(entry)

	if err := env.Write(targetPath); err != nil {
		return fmt.Errorf("writing %s: %w", targetPath, err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", key, value)
	return nil
}

// parseKeyValue splits a KEY=VALUE argument. The key must not be empty.
// The value may be empty (KEY=).
func parseKeyValue(arg string) (string, string, error) {
	eqIdx := strings.IndexByte(arg, '=')
	if eqIdx < 0 {
		return "", "", fmt.Errorf("invalid format %q: expected KEY=VALUE", arg)
	}

	key := strings.TrimSpace(arg[:eqIdx])
	if key == "" {
		return "", "", fmt.Errorf("empty key in %q", arg)
	}

	value := arg[eqIdx+1:]
	return key, value, nil
}
