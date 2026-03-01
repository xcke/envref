package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/config"
	"github.com/xcke/envref/internal/output"
)

// backendTypeInfo describes a supported backend type for display purposes.
type backendTypeInfo struct {
	Name        string
	Description string
}

// supportedBackendTypes lists all known backend types with descriptions.
var supportedBackendTypes = []backendTypeInfo{
	{Name: "keychain", Description: "macOS Keychain / Linux secret-service"},
	{Name: "1password", Description: "1Password CLI (op)"},
	{Name: "aws-ssm", Description: "AWS Systems Manager Parameter Store"},
	{Name: "oci-vault", Description: "Oracle Cloud Infrastructure Vault"},
	{Name: "hashicorp-vault", Description: "HashiCorp Vault"},
}

// newBackendCmd creates the backend command group for managing secret backends.
func newBackendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backend",
		Short: "Manage secret backends",
		Long: `List and inspect configured secret backends.

Secret backends are defined in the .envref.yaml config file under the
"backends" section. They are tried in order when resolving ref:// references;
the first backend that returns a value wins.`,
	}

	cmd.AddCommand(newBackendListCmd())

	return cmd
}

// newBackendListCmd creates the backend list subcommand.
func newBackendListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available secret backends",
		Long: `List secret backends configured in the project's .envref.yaml.

By default, shows only backends configured for the current project.
Use --all to also show all supported backend types.

Examples:
  envref backend list          # show configured backends
  envref backend list --all    # also show all supported types`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			all, _ := cmd.Flags().GetBool("all")
			return runBackendList(cmd, all)
		},
	}

	cmd.Flags().Bool("all", false, "show all supported backend types, not just configured ones")

	return cmd
}

// runBackendList prints configured backends and optionally all supported types.
func runBackendList(cmd *cobra.Command, all bool) error {
	w := output.NewWriter(cmd)

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, _, err := config.Load(cwd)
	if err != nil && !errors.Is(err, config.ErrNotFound) {
		return fmt.Errorf("loading config: %w", err)
	}

	// If we have a config, show configured backends.
	if cfg != nil && len(cfg.Backends) > 0 {
		w.Info("Configured backends:\n")

		for _, b := range cfg.Backends {
			w.Info("  %-20s type=%s\n", b.Name, b.EffectiveType())
			if w.IsVerbose() {
				for k, v := range b.Config {
					w.Verbose("    %s=%s\n", k, v)
				}
			}
		}

		w.Verbose("\n%d backend(s) configured\n", len(cfg.Backends))

		if !all {
			return nil
		}
		w.Info("\n")
	}

	// No config: suggest init.
	if cfg == nil {
		w.Info("No .envref.yaml found. Run \"envref init\" to set up a project.\n\n")
	}

	// Show supported types when: no config, no backends configured, or --all.
	noBackends := cfg == nil || len(cfg.Backends) == 0
	if noBackends || all {
		printSupportedTypes(w)
	}

	return nil
}

// printSupportedTypes prints the table of all known backend types.
func printSupportedTypes(w *output.Writer) {
	w.Info("Supported backend types:\n")

	// Sort by name for consistent output.
	sorted := make([]backendTypeInfo, len(supportedBackendTypes))
	copy(sorted, supportedBackendTypes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	for _, bt := range sorted {
		w.Info("  %-20s %s\n", bt.Name, bt.Description)
	}
}
