package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"
	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/backend"
	"github.com/xcke/envref/internal/config"
	"github.com/xcke/envref/internal/output"
)

// newSecretShareCmd creates the secret share subcommand.
func newSecretShareCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "share <KEY>",
		Short: "Encrypt a secret for a specific recipient",
		Long: `Encrypt a secret from the configured backend using a recipient's age public key.

The encrypted output is printed to stdout as ASCII-armored age ciphertext.
The recipient can decrypt it using their corresponding private key:

  age --decrypt -i key.txt < shared-secret.age

Use --to to specify the recipient's age X25519 public key directly, or
--to-file to read it from a file. Exactly one of --to or --to-file is required.

Examples:
  envref secret share API_KEY --to age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p
  envref secret share DB_PASS --to-file teammate.pub
  envref secret share API_KEY --to age1... --backend keychain
  envref secret share API_KEY --to age1... --profile staging
  envref secret share API_KEY --to age1... > shared-secret.age`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			to, _ := cmd.Flags().GetString("to")
			toFile, _ := cmd.Flags().GetString("to-file")
			backendName, _ := cmd.Flags().GetString("backend")
			profile, _ := cmd.Flags().GetString("profile")
			return runSecretShare(cmd, args[0], to, toFile, backendName, profile)
		},
	}

	cmd.Flags().String("to", "", "recipient's age public key (age1...)")
	cmd.Flags().String("to-file", "", "file containing the recipient's age public key")
	cmd.Flags().StringP("backend", "b", "", "backend to retrieve the secret from (default: first configured)")
	cmd.Flags().StringP("profile", "P", "", "profile scope for the secret (e.g., staging, production)")

	return cmd
}

// runSecretShare retrieves a secret from the backend and encrypts it for the
// given recipient using age X25519 public key encryption.
func runSecretShare(cmd *cobra.Command, key, to, toFile, backendName, profile string) error {
	// Validate key.
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("key must not be empty")
	}

	// Resolve the recipient public key.
	recipientKey, err := resolveRecipientKey(to, toFile)
	if err != nil {
		return err
	}

	// Parse the age recipient.
	recipient, err := age.ParseX25519Recipient(recipientKey)
	if err != nil {
		return fmt.Errorf("invalid age public key: %w", err)
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

	// Retrieve the secret value (profile-scoped first if applicable).
	value, err := getSecretValue(targetBackend, cfg.Project, effectiveProfile, key)
	if err != nil {
		return fmt.Errorf("retrieving secret: %w", err)
	}

	// Encrypt the value for the recipient.
	encrypted, err := encryptForRecipient(value, recipient)
	if err != nil {
		return fmt.Errorf("encrypting secret: %w", err)
	}

	// Output the encrypted ciphertext to stdout.
	_, _ = fmt.Fprint(cmd.OutOrStdout(), encrypted)

	scopeLabel := fmt.Sprintf("backend %q", backendName)
	if effectiveProfile != "" {
		scopeLabel = fmt.Sprintf("backend %q (profile %q)", backendName, effectiveProfile)
	}
	output.NewWriter(cmd).Verbose("secret %q from %s encrypted for recipient %s\n", key, scopeLabel, truncateKey(recipientKey))

	return nil
}

// resolveRecipientKey resolves the recipient's age public key from either the
// --to flag or the --to-file flag. Exactly one must be provided.
func resolveRecipientKey(to, toFile string) (string, error) {
	if to == "" && toFile == "" {
		return "", fmt.Errorf("either --to or --to-file is required")
	}
	if to != "" && toFile != "" {
		return "", fmt.Errorf("--to and --to-file are mutually exclusive")
	}

	if to != "" {
		return strings.TrimSpace(to), nil
	}

	data, err := os.ReadFile(toFile)
	if err != nil {
		return "", fmt.Errorf("reading public key file: %w", err)
	}

	key := strings.TrimSpace(string(data))
	if key == "" {
		return "", fmt.Errorf("public key file %q is empty", toFile)
	}

	// Take the first non-comment line.
	for _, line := range strings.Split(key, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			return line, nil
		}
	}

	return "", fmt.Errorf("no public key found in %q", toFile)
}

// getSecretValue retrieves a secret from the backend, trying profile scope first.
func getSecretValue(targetBackend backend.Backend, project, profile, key string) (string, error) {
	if profile != "" {
		profileBackend, err := backend.NewProfileNamespacedBackend(targetBackend, project, profile)
		if err != nil {
			return "", fmt.Errorf("creating profile backend: %w", err)
		}
		value, err := profileBackend.Get(key)
		if err == nil {
			return value, nil
		}
		if !isNotFound(err) {
			return "", err
		}
	}

	nsBackend, err := backend.NewNamespacedBackend(targetBackend, project)
	if err != nil {
		return "", fmt.Errorf("creating namespaced backend: %w", err)
	}

	return nsBackend.Get(key)
}

// isNotFound returns true if the error is a backend not-found error.
func isNotFound(err error) bool {
	return err != nil && (err == backend.ErrNotFound || err.Error() == "secret not found")
}

// encryptForRecipient encrypts plaintext using the recipient's age public key
// and returns ASCII-armored ciphertext.
func encryptForRecipient(plaintext string, recipient *age.X25519Recipient) (string, error) {
	var buf bytes.Buffer
	armorWriter := armor.NewWriter(&buf)

	writer, err := age.Encrypt(armorWriter, recipient)
	if err != nil {
		return "", fmt.Errorf("creating encryption writer: %w", err)
	}

	if _, err := writer.Write([]byte(plaintext)); err != nil {
		return "", fmt.Errorf("writing plaintext: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("closing encryption writer: %w", err)
	}

	if err := armorWriter.Close(); err != nil {
		return "", fmt.Errorf("closing armor writer: %w", err)
	}

	return buf.String(), nil
}

// truncateKey returns a shortened version of an age public key for display.
func truncateKey(key string) string {
	if len(key) > 20 {
		return key[:16] + "..."
	}
	return key
}
