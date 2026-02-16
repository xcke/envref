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

	return cmd
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
