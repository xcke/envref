package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/backend"
	"github.com/xcke/envref/internal/config"
)

// newSecretCmd creates the secret command group for managing secrets in backends.
func newSecretCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret",
		Short: "Manage secrets stored in backends",
		Long: `Manage secrets stored in secret backends (OS keychain, vault, etc.).

Use subcommands to set, get, delete, and list secrets for the current project.
Secrets are namespaced by project name from .envref.yaml.`,
	}

	cmd.AddCommand(newSecretSetCmd())
	cmd.AddCommand(newSecretGetCmd())
	cmd.AddCommand(newSecretDeleteCmd())
	cmd.AddCommand(newSecretListCmd())

	return cmd
}

// newSecretGetCmd creates the secret get subcommand.
func newSecretGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <KEY>",
		Short: "Retrieve a secret from a backend",
		Long: `Retrieve and print a secret value from the configured backend for the current project.

By default, the first configured backend from .envref.yaml is used.
Use --backend to specify a different backend.

Examples:
  envref secret get API_KEY                    # get from default backend
  envref secret get DB_PASS --backend keychain # get from specific backend`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			backendName, _ := cmd.Flags().GetString("backend")
			return runSecretGet(cmd, args[0], backendName)
		},
	}

	cmd.Flags().StringP("backend", "b", "", "backend to retrieve the secret from (default: first configured)")

	return cmd
}

// runSecretGet retrieves a secret from the configured backend.
func runSecretGet(cmd *cobra.Command, key, backendName string) error {
	// Validate key.
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("key must not be empty")
	}

	// Load project config.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, _, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if len(cfg.Backends) == 0 {
		return fmt.Errorf("no backends configured in %s", config.FullFileName)
	}

	// Determine target backend.
	if backendName == "" {
		backendName = cfg.Backends[0].Name
	}

	// Build registry with backends.
	registry, err := buildRegistry(cfg)
	if err != nil {
		return fmt.Errorf("initializing backends: %w", err)
	}

	// Wrap the target backend with project namespace.
	targetBackend := registry.Backend(backendName)
	if targetBackend == nil {
		return fmt.Errorf("backend %q is not registered", backendName)
	}

	nsBackend, err := backend.NewNamespacedBackend(targetBackend, cfg.Project)
	if err != nil {
		return fmt.Errorf("creating namespaced backend: %w", err)
	}

	// Retrieve the secret.
	value, err := nsBackend.Get(key)
	if err != nil {
		return fmt.Errorf("retrieving secret: %w", err)
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout(), value)
	return nil
}

// newSecretListCmd creates the secret list subcommand.
func newSecretListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List secret keys stored in a backend",
		Long: `List all secret keys stored in the configured backend for the current project.

Only key names are shown — secret values are never printed.

By default, the first configured backend from .envref.yaml is used.
Use --backend to specify a different backend.

Examples:
  envref secret list                    # list from default backend
  envref secret list --backend keychain # list from specific backend`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			backendName, _ := cmd.Flags().GetString("backend")
			return runSecretList(cmd, backendName)
		},
	}

	cmd.Flags().StringP("backend", "b", "", "backend to list secrets from (default: first configured)")

	return cmd
}

// runSecretList lists all secret keys for the current project from the configured backend.
func runSecretList(cmd *cobra.Command, backendName string) error {
	// Load project config.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, _, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if len(cfg.Backends) == 0 {
		return fmt.Errorf("no backends configured in %s", config.FullFileName)
	}

	// Determine target backend.
	if backendName == "" {
		backendName = cfg.Backends[0].Name
	}

	// Build registry with backends.
	registry, err := buildRegistry(cfg)
	if err != nil {
		return fmt.Errorf("initializing backends: %w", err)
	}

	// Wrap the target backend with project namespace.
	targetBackend := registry.Backend(backendName)
	if targetBackend == nil {
		return fmt.Errorf("backend %q is not registered", backendName)
	}

	nsBackend, err := backend.NewNamespacedBackend(targetBackend, cfg.Project)
	if err != nil {
		return fmt.Errorf("creating namespaced backend: %w", err)
	}

	// List keys.
	keys, err := nsBackend.List()
	if err != nil {
		return fmt.Errorf("listing secrets: %w", err)
	}

	if len(keys) == 0 {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "no secrets found")
		return nil
	}

	for _, key := range keys {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), key)
	}
	return nil
}

// newSecretDeleteCmd creates the secret delete subcommand.
func newSecretDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <KEY>",
		Short: "Delete a secret from a backend",
		Long: `Delete a secret from the configured backend for the current project.

By default, you will be prompted to confirm the deletion. Use --force to skip
the confirmation prompt.

The first backend from .envref.yaml is used by default.
Use --backend to specify a different backend.

Examples:
  envref secret delete API_KEY                    # delete with confirmation
  envref secret delete API_KEY --force            # delete without confirmation
  envref secret delete DB_PASS --backend keychain # delete from specific backend`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			backendName, _ := cmd.Flags().GetString("backend")
			force, _ := cmd.Flags().GetBool("force")
			return runSecretDelete(cmd, args[0], backendName, force)
		},
	}

	cmd.Flags().StringP("backend", "b", "", "backend to delete the secret from (default: first configured)")
	cmd.Flags().BoolP("force", "f", false, "skip confirmation prompt")

	return cmd
}

// runSecretDelete removes a secret from the configured backend.
func runSecretDelete(cmd *cobra.Command, key, backendName string, force bool) error {
	// Validate key.
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("key must not be empty")
	}

	// Load project config.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, _, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if len(cfg.Backends) == 0 {
		return fmt.Errorf("no backends configured in %s", config.FullFileName)
	}

	// Determine target backend.
	if backendName == "" {
		backendName = cfg.Backends[0].Name
	}

	// Build registry with backends.
	registry, err := buildRegistry(cfg)
	if err != nil {
		return fmt.Errorf("initializing backends: %w", err)
	}

	// Wrap the target backend with project namespace.
	targetBackend := registry.Backend(backendName)
	if targetBackend == nil {
		return fmt.Errorf("backend %q is not registered", backendName)
	}

	nsBackend, err := backend.NewNamespacedBackend(targetBackend, cfg.Project)
	if err != nil {
		return fmt.Errorf("creating namespaced backend: %w", err)
	}

	// Confirm deletion unless --force is set.
	if !force {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Delete secret %q from backend %q? [y/N] ", key, backendName)
		answer, err := readLine(cmd.InOrStdin())
		if err != nil {
			return fmt.Errorf("reading confirmation: %w", err)
		}
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "deletion cancelled")
			return nil
		}
	}

	// Delete the secret.
	if err := nsBackend.Delete(key); err != nil {
		return fmt.Errorf("deleting secret: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "secret %q deleted from backend %q\n", key, backendName)
	return nil
}

// newSecretSetCmd creates the secret set subcommand.
func newSecretSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <KEY>",
		Short: "Store a secret in a backend",
		Long: `Store a secret value in the configured backend for the current project.

If --value is not provided, you will be prompted to enter the value from stdin
(input is hidden when connected to a terminal).

The secret is stored in the first backend from .envref.yaml by default.
Use --backend to specify a different backend.

Examples:
  envref secret set API_KEY                    # prompt for value
  envref secret set API_KEY --value sk-123     # non-interactive
  envref secret set DB_PASS --backend keychain # specific backend`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			value, _ := cmd.Flags().GetString("value")
			backendName, _ := cmd.Flags().GetString("backend")
			return runSecretSet(cmd, args[0], value, backendName)
		},
	}

	cmd.Flags().StringP("value", "v", "", "secret value (if omitted, prompts for input)")
	cmd.Flags().StringP("backend", "b", "", "backend to store the secret in (default: first configured)")

	return cmd
}

// runSecretSet stores a secret in the configured backend.
func runSecretSet(cmd *cobra.Command, key, value, backendName string) error {
	// Validate key.
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("key must not be empty")
	}

	// Load project config.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, _, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if len(cfg.Backends) == 0 {
		return fmt.Errorf("no backends configured in %s", config.FullFileName)
	}

	// Determine target backend.
	if backendName == "" {
		backendName = cfg.Backends[0].Name
	}

	// Build registry with backends.
	registry, err := buildRegistry(cfg)
	if err != nil {
		return fmt.Errorf("initializing backends: %w", err)
	}

	// Wrap the target backend with project namespace.
	targetBackend := registry.Backend(backendName)
	if targetBackend == nil {
		return fmt.Errorf("backend %q is not registered", backendName)
	}

	nsBackend, err := backend.NewNamespacedBackend(targetBackend, cfg.Project)
	if err != nil {
		return fmt.Errorf("creating namespaced backend: %w", err)
	}

	// If no value provided, prompt for it.
	if value == "" {
		prompted, err := promptSecret(cmd, key)
		if err != nil {
			return err
		}
		value = prompted
	}

	// Store the secret.
	if err := nsBackend.Set(key, value); err != nil {
		return fmt.Errorf("storing secret: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "secret %q stored in backend %q\n", key, backendName)
	return nil
}

// promptSecret prompts the user to enter a secret value from stdin.
// The prompt is written to stderr so it doesn't interfere with piped output.
func promptSecret(cmd *cobra.Command, key string) (string, error) {
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Enter value for %s: ", key)

	value, err := readLine(cmd.InOrStdin())
	if err != nil {
		return "", fmt.Errorf("reading secret value: %w", err)
	}

	if value == "" {
		return "", fmt.Errorf("secret value must not be empty")
	}

	return value, nil
}

// readLine reads a single line from the reader, trimming the trailing newline.
func readLine(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		return scanner.Text(), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("no input provided")
}

// buildRegistry creates a backend registry from the config, instantiating
// backends based on their type.
func buildRegistry(cfg *config.Config) (*backend.Registry, error) {
	registry := backend.NewRegistry()

	for _, bc := range cfg.Backends {
		b, err := createBackend(bc)
		if err != nil {
			return nil, fmt.Errorf("backend %q: %w", bc.Name, err)
		}
		if err := registry.Register(b); err != nil {
			return nil, err
		}
	}

	return registry, nil
}

// createBackend instantiates a backend based on its config type.
func createBackend(bc config.BackendConfig) (backend.Backend, error) {
	switch bc.EffectiveType() {
	case "keychain":
		return backend.NewKeychainBackend(), nil
	default:
		return nil, fmt.Errorf("unknown backend type %q", bc.EffectiveType())
	}
}
