package cmd

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
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
	cmd.AddCommand(newSecretGenerateCmd())

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

// Predefined character sets for secret generation.
const (
	charsetAlphanumeric = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	charsetASCII        = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+[]{}|;:,.<>?"
)

// newSecretGenerateCmd creates the secret generate subcommand.
func newSecretGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate <KEY>",
		Short: "Generate and store a random secret",
		Long: `Generate a cryptographically random secret and store it in the configured backend.

The generated secret uses a configurable character set and length.
By default, a 32-character alphanumeric secret is generated.

Available charsets:
  alphanumeric  a-z, A-Z, 0-9 (default)
  ascii         alphanumeric + common symbols
  hex           0-9, a-f (lowercase hex)
  base64        standard base64 encoding

Use --print to display the generated secret value on stdout.

Examples:
  envref secret generate API_KEY                          # 32 char alphanumeric
  envref secret generate API_KEY --length 64              # 64 char alphanumeric
  envref secret generate API_KEY --charset hex            # hex string
  envref secret generate API_KEY --charset base64         # base64 encoded
  envref secret generate API_KEY --charset ascii          # include symbols
  envref secret generate API_KEY --print                  # print the generated value
  envref secret generate API_KEY --backend keychain       # specific backend`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			length, _ := cmd.Flags().GetInt("length")
			charset, _ := cmd.Flags().GetString("charset")
			backendName, _ := cmd.Flags().GetString("backend")
			printVal, _ := cmd.Flags().GetBool("print")
			return runSecretGenerate(cmd, args[0], length, charset, backendName, printVal)
		},
	}

	cmd.Flags().IntP("length", "l", 32, "length of the generated secret")
	cmd.Flags().StringP("charset", "c", "alphanumeric", "character set: alphanumeric, ascii, hex, base64")
	cmd.Flags().StringP("backend", "b", "", "backend to store the secret in (default: first configured)")
	cmd.Flags().BoolP("print", "p", false, "print the generated secret value to stdout")

	return cmd
}

// runSecretGenerate generates a random secret and stores it in the configured backend.
func runSecretGenerate(cmd *cobra.Command, key string, length int, charset, backendName string, printVal bool) error {
	// Validate key.
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("key must not be empty")
	}

	// Validate length.
	if length < 1 {
		return fmt.Errorf("length must be at least 1")
	}
	if length > 1024 {
		return fmt.Errorf("length must not exceed 1024")
	}

	// Generate the secret.
	value, err := generateSecret(length, charset)
	if err != nil {
		return fmt.Errorf("generating secret: %w", err)
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

	// Store the generated secret.
	if err := nsBackend.Set(key, value); err != nil {
		return fmt.Errorf("storing secret: %w", err)
	}

	if printVal {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), value)
	}

	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "secret %q generated and stored in backend %q (%d chars, %s)\n", key, backendName, length, charset)
	return nil
}

// generateSecret produces a cryptographically random string of the given length
// using the specified character set.
func generateSecret(length int, charset string) (string, error) {
	switch charset {
	case "hex":
		return generateHex(length)
	case "base64":
		return generateBase64(length)
	case "alphanumeric":
		return generateFromCharset(length, charsetAlphanumeric)
	case "ascii":
		return generateFromCharset(length, charsetASCII)
	default:
		return "", fmt.Errorf("unknown charset %q (valid: alphanumeric, ascii, hex, base64)", charset)
	}
}

// generateFromCharset generates a random string of the given length by sampling
// uniformly from the provided character set using crypto/rand.
func generateFromCharset(length int, chars string) (string, error) {
	max := big.NewInt(int64(len(chars)))
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("reading random bytes: %w", err)
		}
		result[i] = chars[n.Int64()]
	}
	return string(result), nil
}

// generateHex generates a random hex-encoded string of exactly the given length.
func generateHex(length int) (string, error) {
	// Each byte produces 2 hex characters.
	numBytes := (length + 1) / 2
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	return hex.EncodeToString(b)[:length], nil
}

// generateBase64 generates a random base64-encoded string of at least the given length,
// truncated to exactly length characters.
func generateBase64(length int) (string, error) {
	// Base64 encodes 3 bytes into 4 chars. Over-allocate to ensure enough output.
	numBytes := (length*3)/4 + 3
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(b)
	if len(encoded) < length {
		return "", fmt.Errorf("base64 encoding produced fewer characters than expected")
	}
	return encoded[:length], nil
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
