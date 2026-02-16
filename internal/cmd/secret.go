package cmd

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/audit"
	"github.com/xcke/envref/internal/backend"
	"github.com/xcke/envref/internal/config"
	"github.com/xcke/envref/internal/output"
)

// newSecretCmd creates the secret command group for managing secrets in backends.
func newSecretCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret",
		Short: "Manage secrets stored in backends",
		Long: `Manage secrets stored in secret backends (OS keychain, vault, etc.).

Use subcommands to set, get, delete, and list secrets for the current project.
Secrets are namespaced by project name from .envref.yaml.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			setVaultCmdContext(cmd)
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			clearVaultCmdContext()
		},
	}

	cmd.AddCommand(newSecretSetCmd())
	cmd.AddCommand(newSecretGetCmd())
	cmd.AddCommand(newSecretDeleteCmd())
	cmd.AddCommand(newSecretListCmd())
	cmd.AddCommand(newSecretGenerateCmd())
	cmd.AddCommand(newSecretCopyCmd())
	cmd.AddCommand(newSecretRotateCmd())
	cmd.AddCommand(newSecretShareCmd())

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

Use --profile to retrieve a profile-scoped secret (stored under <project>/<profile>/<key>).
If no profile-scoped secret exists, falls back to the project-scoped secret.

Examples:
  envref secret get API_KEY                              # get from default backend
  envref secret get DB_PASS --backend keychain           # get from specific backend
  envref secret get API_KEY --profile staging            # get profile-scoped secret`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			backendName, _ := cmd.Flags().GetString("backend")
			profile, _ := cmd.Flags().GetString("profile")
			return runSecretGet(cmd, args[0], backendName, profile)
		},
	}

	cmd.Flags().StringP("backend", "b", "", "backend to retrieve the secret from (default: first configured)")
	cmd.Flags().StringP("profile", "P", "", "profile scope for the secret (e.g., staging, production)")

	return cmd
}

// runSecretGet retrieves a secret from the configured backend.
func runSecretGet(cmd *cobra.Command, key, backendName, profile string) error {
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
	defer registry.CloseAll()

	// Wrap the target backend with project namespace.
	targetBackend := registry.Backend(backendName)
	if targetBackend == nil {
		return fmt.Errorf("backend %q is not registered", backendName)
	}

	// Resolve effective profile from flag or config.
	effectiveProfile := cfg.EffectiveProfile(profile)

	// If profile is active, try profile-scoped first, then fall back.
	if effectiveProfile != "" {
		profileBackend, pErr := backend.NewProfileNamespacedBackend(targetBackend, cfg.Project, effectiveProfile)
		if pErr != nil {
			return fmt.Errorf("creating profile backend: %w", pErr)
		}
		value, pGetErr := profileBackend.Get(key)
		if pGetErr == nil {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), value)
			return nil
		}
		// Only fall back on not-found; other errors are real failures.
		if !errors.Is(pGetErr, backend.ErrNotFound) {
			return fmt.Errorf("retrieving secret: %w", pGetErr)
		}
	}

	nsBackend, err := backend.NewNamespacedBackend(targetBackend, cfg.Project)
	if err != nil {
		return fmt.Errorf("creating namespaced backend: %w", err)
	}

	// Retrieve the secret from project scope.
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

Use --profile to list only profile-scoped secrets for the given profile.

Examples:
  envref secret list                              # list from default backend
  envref secret list --backend keychain           # list from specific backend
  envref secret list --profile staging            # list profile-scoped secrets`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			backendName, _ := cmd.Flags().GetString("backend")
			profile, _ := cmd.Flags().GetString("profile")
			return runSecretList(cmd, backendName, profile)
		},
	}

	cmd.Flags().StringP("backend", "b", "", "backend to list secrets from (default: first configured)")
	cmd.Flags().StringP("profile", "P", "", "profile scope to list secrets for (e.g., staging, production)")

	return cmd
}

// runSecretList lists all secret keys for the current project from the configured backend.
func runSecretList(cmd *cobra.Command, backendName, profile string) error {
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
	defer registry.CloseAll()

	// Wrap the target backend with project namespace.
	targetBackend := registry.Backend(backendName)
	if targetBackend == nil {
		return fmt.Errorf("backend %q is not registered", backendName)
	}

	// Build the appropriate namespaced backend.
	effectiveProfile := cfg.EffectiveProfile(profile)
	var nsBackend *backend.NamespacedBackend
	if effectiveProfile != "" {
		nsBackend, err = backend.NewProfileNamespacedBackend(targetBackend, cfg.Project, effectiveProfile)
	} else {
		nsBackend, err = backend.NewNamespacedBackend(targetBackend, cfg.Project)
	}
	if err != nil {
		return fmt.Errorf("creating namespaced backend: %w", err)
	}

	// List keys.
	keys, err := nsBackend.List()
	if err != nil {
		return fmt.Errorf("listing secrets: %w", err)
	}

	if len(keys) == 0 {
		if effectiveProfile != "" {
			output.NewWriter(cmd).Info("no secrets found for profile %q\n", effectiveProfile)
		} else {
			output.NewWriter(cmd).Info("no secrets found\n")
		}
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

Use --profile to delete a profile-scoped secret.

Examples:
  envref secret delete API_KEY                              # delete with confirmation
  envref secret delete API_KEY --force                      # delete without confirmation
  envref secret delete DB_PASS --backend keychain           # delete from specific backend
  envref secret delete API_KEY --profile staging --force    # delete profile-scoped secret`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			backendName, _ := cmd.Flags().GetString("backend")
			force, _ := cmd.Flags().GetBool("force")
			profile, _ := cmd.Flags().GetString("profile")
			return runSecretDelete(cmd, args[0], backendName, force, profile)
		},
	}

	cmd.Flags().StringP("backend", "b", "", "backend to delete the secret from (default: first configured)")
	cmd.Flags().BoolP("force", "f", false, "skip confirmation prompt")
	cmd.Flags().StringP("profile", "P", "", "profile scope for the secret (e.g., staging, production)")

	return cmd
}

// runSecretDelete removes a secret from the configured backend.
func runSecretDelete(cmd *cobra.Command, key, backendName string, force bool, profile string) error {
	// Validate key.
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("key must not be empty")
	}

	// Load project config.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, configDir, err := config.Load(cwd)
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
	defer registry.CloseAll()

	// Wrap the target backend with project namespace.
	targetBackend := registry.Backend(backendName)
	if targetBackend == nil {
		return fmt.Errorf("backend %q is not registered", backendName)
	}

	// Build the appropriate namespaced backend.
	effectiveProfile := cfg.EffectiveProfile(profile)
	var nsBackend *backend.NamespacedBackend
	if effectiveProfile != "" {
		nsBackend, err = backend.NewProfileNamespacedBackend(targetBackend, cfg.Project, effectiveProfile)
	} else {
		nsBackend, err = backend.NewNamespacedBackend(targetBackend, cfg.Project)
	}
	if err != nil {
		return fmt.Errorf("creating namespaced backend: %w", err)
	}

	// Confirm deletion unless --force is set.
	scopeLabel := fmt.Sprintf("backend %q", backendName)
	if effectiveProfile != "" {
		scopeLabel = fmt.Sprintf("backend %q (profile %q)", backendName, effectiveProfile)
	}
	if !force {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Delete secret %q from %s? [y/N] ", key, scopeLabel)
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

	// Log the operation to the audit log (best-effort).
	_ = newAuditLogger(configDir).Log(audit.Entry{
		Operation: audit.OpDelete,
		Key:       key,
		Backend:   backendName,
		Project:   cfg.Project,
		Profile:   effectiveProfile,
	})

	output.NewWriter(cmd).Info("secret %q deleted from %s\n", key, scopeLabel)
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

Use --profile to store the secret in a profile-scoped namespace
(<project>/<profile>/<key>), allowing different values per environment.

Examples:
  envref secret set API_KEY                              # prompt for value
  envref secret set API_KEY --value sk-123               # non-interactive
  envref secret set DB_PASS --backend keychain           # specific backend
  envref secret set API_KEY --value sk-stg --profile staging  # profile-scoped`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			value, _ := cmd.Flags().GetString("value")
			backendName, _ := cmd.Flags().GetString("backend")
			profile, _ := cmd.Flags().GetString("profile")
			return runSecretSet(cmd, args[0], value, backendName, profile)
		},
	}

	cmd.Flags().StringP("value", "v", "", "secret value (if omitted, prompts for input)")
	cmd.Flags().StringP("backend", "b", "", "backend to store the secret in (default: first configured)")
	cmd.Flags().StringP("profile", "P", "", "profile scope for the secret (e.g., staging, production)")

	return cmd
}

// runSecretSet stores a secret in the configured backend.
func runSecretSet(cmd *cobra.Command, key, value, backendName, profile string) error {
	// Validate key.
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("key must not be empty")
	}

	// Load project config.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, configDir, err := config.Load(cwd)
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
	defer registry.CloseAll()

	// Wrap the target backend with project namespace.
	targetBackend := registry.Backend(backendName)
	if targetBackend == nil {
		return fmt.Errorf("backend %q is not registered", backendName)
	}

	// Build the appropriate namespaced backend.
	effectiveProfile := cfg.EffectiveProfile(profile)
	var nsBackend *backend.NamespacedBackend
	if effectiveProfile != "" {
		nsBackend, err = backend.NewProfileNamespacedBackend(targetBackend, cfg.Project, effectiveProfile)
	} else {
		nsBackend, err = backend.NewNamespacedBackend(targetBackend, cfg.Project)
	}
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

	// Log the operation to the audit log (best-effort).
	_ = newAuditLogger(configDir).Log(audit.Entry{
		Operation: audit.OpSet,
		Key:       key,
		Backend:   backendName,
		Project:   cfg.Project,
		Profile:   effectiveProfile,
	})

	scopeLabel := fmt.Sprintf("backend %q", backendName)
	if effectiveProfile != "" {
		scopeLabel = fmt.Sprintf("backend %q (profile %q)", backendName, effectiveProfile)
	}
	output.NewWriter(cmd).Info("secret %q stored in %s\n", key, scopeLabel)
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
Use --profile to store in a profile-scoped namespace.

Examples:
  envref secret generate API_KEY                                    # 32 char alphanumeric
  envref secret generate API_KEY --length 64                        # 64 char alphanumeric
  envref secret generate API_KEY --charset hex                      # hex string
  envref secret generate API_KEY --print                            # print the generated value
  envref secret generate API_KEY --profile staging                  # profile-scoped
  envref secret generate API_KEY --backend keychain                 # specific backend`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			length, _ := cmd.Flags().GetInt("length")
			charset, _ := cmd.Flags().GetString("charset")
			backendName, _ := cmd.Flags().GetString("backend")
			printVal, _ := cmd.Flags().GetBool("print")
			profile, _ := cmd.Flags().GetString("profile")
			return runSecretGenerate(cmd, args[0], length, charset, backendName, printVal, profile)
		},
	}

	cmd.Flags().IntP("length", "l", 32, "length of the generated secret")
	cmd.Flags().StringP("charset", "c", "alphanumeric", "character set: alphanumeric, ascii, hex, base64")
	cmd.Flags().StringP("backend", "b", "", "backend to store the secret in (default: first configured)")
	cmd.Flags().BoolP("print", "p", false, "print the generated secret value to stdout")
	cmd.Flags().StringP("profile", "P", "", "profile scope for the secret (e.g., staging, production)")

	return cmd
}

// runSecretGenerate generates a random secret and stores it in the configured backend.
func runSecretGenerate(cmd *cobra.Command, key string, length int, charset, backendName string, printVal bool, profile string) error {
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

	cfg, configDir, err := config.Load(cwd)
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
	defer registry.CloseAll()

	// Wrap the target backend with project namespace.
	targetBackend := registry.Backend(backendName)
	if targetBackend == nil {
		return fmt.Errorf("backend %q is not registered", backendName)
	}

	// Build the appropriate namespaced backend.
	effectiveProfile := cfg.EffectiveProfile(profile)
	var nsBackend *backend.NamespacedBackend
	if effectiveProfile != "" {
		nsBackend, err = backend.NewProfileNamespacedBackend(targetBackend, cfg.Project, effectiveProfile)
	} else {
		nsBackend, err = backend.NewNamespacedBackend(targetBackend, cfg.Project)
	}
	if err != nil {
		return fmt.Errorf("creating namespaced backend: %w", err)
	}

	// Store the generated secret.
	if err := nsBackend.Set(key, value); err != nil {
		return fmt.Errorf("storing secret: %w", err)
	}

	// Log the operation to the audit log (best-effort).
	_ = newAuditLogger(configDir).Log(audit.Entry{
		Operation: audit.OpGenerate,
		Key:       key,
		Backend:   backendName,
		Project:   cfg.Project,
		Profile:   effectiveProfile,
	})

	if printVal {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), value)
	}

	scopeLabel := fmt.Sprintf("backend %q", backendName)
	if effectiveProfile != "" {
		scopeLabel = fmt.Sprintf("backend %q (profile %q)", backendName, effectiveProfile)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "secret %q generated and stored in %s (%d chars, %s)\n", key, scopeLabel, length, charset)
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

// newSecretCopyCmd creates the secret copy subcommand.
func newSecretCopyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "copy <KEY>",
		Short: "Copy a secret from another project or profile",
		Long: `Copy a secret from another project's namespace into the current project.

The --from flag specifies the source project name. The secret is read from the
source project's namespace and stored in the current project's namespace using
the same key name.

Use --profile to store the copied secret in a profile-scoped namespace.
Use --from-profile to read from a profile-scoped namespace in the source project.

By default, the first configured backend from .envref.yaml is used for both
reading and writing. Use --backend to specify a different backend.

Examples:
  envref secret copy API_KEY --from other-project                          # copy from project
  envref secret copy API_KEY --from other-project --profile staging        # copy into profile scope
  envref secret copy API_KEY --from other-project --from-profile prod      # copy from profile scope
  envref secret copy DB_PASS --from staging --backend keychain             # specific backend`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fromProject, _ := cmd.Flags().GetString("from")
			backendName, _ := cmd.Flags().GetString("backend")
			profile, _ := cmd.Flags().GetString("profile")
			fromProfile, _ := cmd.Flags().GetString("from-profile")
			return runSecretCopy(cmd, args[0], fromProject, backendName, profile, fromProfile)
		},
	}

	cmd.Flags().String("from", "", "source project to copy the secret from (required)")
	_ = cmd.MarkFlagRequired("from")
	cmd.Flags().StringP("backend", "b", "", "backend to use for reading and writing (default: first configured)")
	cmd.Flags().StringP("profile", "P", "", "destination profile scope (e.g., staging, production)")
	cmd.Flags().String("from-profile", "", "source profile scope to copy from")

	return cmd
}

// runSecretCopy copies a secret from another project's namespace into the current project.
func runSecretCopy(cmd *cobra.Command, key, fromProject, backendName, profile, fromProfile string) error {
	// Validate key.
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("key must not be empty")
	}

	// Validate source project.
	if strings.TrimSpace(fromProject) == "" {
		return fmt.Errorf("--from project must not be empty")
	}

	// Load project config.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, configDir, err := config.Load(cwd)
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
	defer registry.CloseAll()

	// Get the raw backend.
	targetBackend := registry.Backend(backendName)
	if targetBackend == nil {
		return fmt.Errorf("backend %q is not registered", backendName)
	}

	// Create namespaced backend for the source project (optionally profile-scoped).
	var srcBackend *backend.NamespacedBackend
	if fromProfile != "" {
		srcBackend, err = backend.NewProfileNamespacedBackend(targetBackend, fromProject, fromProfile)
	} else {
		srcBackend, err = backend.NewNamespacedBackend(targetBackend, fromProject)
	}
	if err != nil {
		return fmt.Errorf("creating source namespace: %w", err)
	}

	// Create namespaced backend for the current (destination) project.
	effectiveProfile := cfg.EffectiveProfile(profile)
	var dstBackend *backend.NamespacedBackend
	if effectiveProfile != "" {
		dstBackend, err = backend.NewProfileNamespacedBackend(targetBackend, cfg.Project, effectiveProfile)
	} else {
		dstBackend, err = backend.NewNamespacedBackend(targetBackend, cfg.Project)
	}
	if err != nil {
		return fmt.Errorf("creating destination namespace: %w", err)
	}

	// Read from source.
	srcLabel := fmt.Sprintf("project %q", fromProject)
	if fromProfile != "" {
		srcLabel = fmt.Sprintf("project %q (profile %q)", fromProject, fromProfile)
	}
	value, err := srcBackend.Get(key)
	if err != nil {
		return fmt.Errorf("reading secret from %s: %w", srcLabel, err)
	}

	// Write to destination.
	if err := dstBackend.Set(key, value); err != nil {
		return fmt.Errorf("storing secret: %w", err)
	}

	// Log the operation to the audit log (best-effort).
	detail := fmt.Sprintf("from %s", srcLabel)
	_ = newAuditLogger(configDir).Log(audit.Entry{
		Operation: audit.OpCopy,
		Key:       key,
		Backend:   backendName,
		Project:   cfg.Project,
		Profile:   effectiveProfile,
		Detail:    detail,
	})

	dstLabel := fmt.Sprintf("%q", cfg.Project)
	if effectiveProfile != "" {
		dstLabel = fmt.Sprintf("%q (profile %q)", cfg.Project, effectiveProfile)
	}
	output.NewWriter(cmd).Info("secret %q copied from %s to %s (backend %q)\n", key, srcLabel, dstLabel, backendName)
	return nil
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
	case "vault":
		return createVaultBackend(bc)
	case "1password":
		return createOnePasswordBackend(bc), nil
	case "aws-ssm":
		return createAWSSSMBackend(bc), nil
	case "oci-vault":
		return createOCIVaultBackend(bc), nil
	case "hashicorp-vault":
		return createHashiVaultBackend(bc), nil
	case "plugin":
		return createPluginBackend(bc)
	default:
		return nil, fmt.Errorf("unknown backend type %q", bc.EffectiveType())
	}
}

// createOnePasswordBackend creates a OnePasswordBackend from the backend config.
// Optional config keys: "vault" (default "Personal"), "account" (optional).
func createOnePasswordBackend(bc config.BackendConfig) *backend.OnePasswordBackend {
	vault := bc.Config["vault"]
	if vault == "" {
		vault = "Personal"
	}

	var opts []backend.OnePasswordOption
	if account := bc.Config["account"]; account != "" {
		opts = append(opts, backend.WithOnePasswordAccount(account))
	}
	if command := bc.Config["command"]; command != "" {
		opts = append(opts, backend.WithOnePasswordCommand(command))
	}
	return backend.NewOnePasswordBackend(vault, opts...)
}

// createVaultBackend creates a VaultBackend from the backend config.
// The passphrase is resolved in order: ENVREF_VAULT_PASSPHRASE env var,
// config.passphrase from .envref.yaml, then interactive terminal prompt
// (if a command context is available). Returns an error if no passphrase
// can be obtained.
func createVaultBackend(bc config.BackendConfig) (*backend.VaultBackend, error) {
	return createVaultBackendWithContext(bc)
}

// createPluginBackend creates a PluginBackend from the backend config.
// If config.command is set, it is used as the plugin executable path.
// Otherwise, the plugin is discovered by searching $PATH for
// "envref-backend-<name>".
func createPluginBackend(bc config.BackendConfig) (*backend.PluginBackend, error) {
	command := bc.Config["command"]
	if command == "" {
		var err error
		command, err = backend.DiscoverPlugin(bc.Name)
		if err != nil {
			return nil, err
		}
	}
	return backend.NewPluginBackend(bc.Name, command), nil
}

// createAWSSSMBackend creates an AWSSSMBackend from the backend config.
// Optional config keys: "prefix" (default "/envref"), "region" (optional),
// "profile" (optional).
func createAWSSSMBackend(bc config.BackendConfig) *backend.AWSSSMBackend {
	prefix := bc.Config["prefix"]
	if prefix == "" {
		prefix = "/envref"
	}

	var opts []backend.AWSSSMOption
	if region := bc.Config["region"]; region != "" {
		opts = append(opts, backend.WithAWSSSMRegion(region))
	}
	if profile := bc.Config["profile"]; profile != "" {
		opts = append(opts, backend.WithAWSSSMProfile(profile))
	}
	if command := bc.Config["command"]; command != "" {
		opts = append(opts, backend.WithAWSSSMCommand(command))
	}
	return backend.NewAWSSSMBackend(prefix, opts...)
}

// createOCIVaultBackend creates an OCIVaultBackend from the backend config.
// Required config keys: "vault_id", "compartment_id", "key_id".
// Optional config keys: "profile" (optional).
func createOCIVaultBackend(bc config.BackendConfig) *backend.OCIVaultBackend {
	vaultID := bc.Config["vault_id"]
	compartmentID := bc.Config["compartment_id"]
	keyID := bc.Config["key_id"]

	var opts []backend.OCIVaultOption
	if profile := bc.Config["profile"]; profile != "" {
		opts = append(opts, backend.WithOCIVaultProfile(profile))
	}
	if command := bc.Config["command"]; command != "" {
		opts = append(opts, backend.WithOCIVaultCommand(command))
	}
	return backend.NewOCIVaultBackend(vaultID, compartmentID, keyID, opts...)
}

// createHashiVaultBackend creates a HashiVaultBackend from the backend config.
// Optional config keys: "mount" (default "secret"), "prefix" (default "envref"),
// "addr" (optional), "namespace" (optional), "token" (optional).
func createHashiVaultBackend(bc config.BackendConfig) *backend.HashiVaultBackend {
	mount := bc.Config["mount"]
	if mount == "" {
		mount = "secret"
	}
	prefix := bc.Config["prefix"]
	if prefix == "" {
		prefix = "envref"
	}

	var opts []backend.HashiVaultOption
	if addr := bc.Config["addr"]; addr != "" {
		opts = append(opts, backend.WithHashiVaultAddr(addr))
	}
	if namespace := bc.Config["namespace"]; namespace != "" {
		opts = append(opts, backend.WithHashiVaultNamespace(namespace))
	}
	if token := bc.Config["token"]; token != "" {
		opts = append(opts, backend.WithHashiVaultToken(token))
	}
	if command := bc.Config["command"]; command != "" {
		opts = append(opts, backend.WithHashiVaultCommand(command))
	}
	return backend.NewHashiVaultBackend(mount, prefix, opts...)
}
