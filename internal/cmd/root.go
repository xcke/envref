// Package cmd defines the CLI commands for envref.
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags.
var version = "dev"

// NewRootCmd creates the root command for envref.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "envref",
		Short: "Separate config from secrets in .env files",
		Long: `envref is a CLI tool that separates config from secrets in .env files,
so teams never store plaintext secrets on disk or in git again.

Replace secret values with ref:// references, and envref resolves them
from your OS keychain or other secret backends at runtime.`,
		SilenceUsage: true,
	}

	// Global verbosity flags (mutually exclusive by convention).
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "suppress informational output (errors only)")
	rootCmd.PersistentFlags().Bool("verbose", false, "show additional detail")
	rootCmd.PersistentFlags().Bool("debug", false, "show debug information")
	rootCmd.MarkFlagsMutuallyExclusive("quiet", "verbose", "debug")

	// Color control flag. Also respects NO_COLOR env var (https://no-color.org/).
	rootCmd.PersistentFlags().Bool("no-color", false, "disable colorized output")

	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newGetCmd())
	rootCmd.AddCommand(newSetCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newSecretCmd())
	rootCmd.AddCommand(newResolveCmd())
	rootCmd.AddCommand(newProfileCmd())
	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newDoctorCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newCompletionCmd())
	rootCmd.AddCommand(newEditCmd())
	rootCmd.AddCommand(newVaultCmd())
	rootCmd.AddCommand(newAuditCmd())
	rootCmd.AddCommand(newAuditLogCmd())
	rootCmd.AddCommand(newSyncCmd())
	rootCmd.AddCommand(newTeamCmd())
	rootCmd.AddCommand(newBackendCmd())
	rootCmd.AddCommand(newOnboardCmd())

	return rootCmd
}

// newVersionCmd creates the version subcommand.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of envref",
		Run: func(cmd *cobra.Command, args []string) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "envref %s\n", version)
		},
	}
}

// Execute runs the root command.
func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		var exitErr *exitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.code)
		}
		os.Exit(1)
	}
}
