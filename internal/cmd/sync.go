package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"
	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/backend"
	"github.com/xcke/envref/internal/config"
	"github.com/xcke/envref/internal/output"
)

// defaultSyncFile is the default name for the encrypted sync file.
const defaultSyncFile = ".envref.secrets.age"

// newSyncCmd creates the sync command group for syncing secrets via encrypted files.
func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync secrets via shared encrypted git file",
		Long: `Sync project secrets through an age-encrypted file that can be committed to git.

Push exports secrets from a backend into an encrypted file. Pull imports
secrets from an encrypted file into a backend. The file is encrypted with
age so only holders of the corresponding private keys can decrypt it.

This enables secure secret sharing through version control without exposing
plaintext values.

Typical workflow:
  1. envref sync push --to age1... --to age1...   # encrypt secrets to file
  2. git add .envref.secrets.age && git commit     # commit the encrypted file
  3. git pull                                       # teammate pulls the file
  4. envref sync pull --identity key.txt           # teammate decrypts into backend`,
	}

	cmd.AddCommand(newSyncPushCmd())
	cmd.AddCommand(newSyncPullCmd())

	return cmd
}

// newSyncPushCmd creates the sync push subcommand.
func newSyncPushCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Export secrets to an encrypted sync file",
		Long: `Export all secrets from a backend into an age-encrypted file.

The file contains a JSON map of key-value pairs, encrypted for the specified
recipients. Each recipient's age public key is provided via --to flags
(repeatable), --to-file flags pointing to files containing public keys,
or --to-team to use all team members defined in .envref.yaml.

At least one recipient must be specified. The encrypted output is written
to .envref.secrets.age by default (configurable via --file).

Examples:
  envref sync push --to age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p
  envref sync push --to age1... --to age1...     # multiple recipients
  envref sync push --to-file team-keys.txt       # keys from file (one per line)
  envref sync push --to-team                     # encrypt for all team members
  envref sync push --to-team --to age1...        # team members + extra recipients
  envref sync push --file secrets.age            # custom output file
  envref sync push --backend keychain            # specific backend
  envref sync push --profile staging             # profile-scoped secrets`,
		RunE: func(cmd *cobra.Command, args []string) error {
			toKeys, _ := cmd.Flags().GetStringArray("to")
			toFiles, _ := cmd.Flags().GetStringArray("to-file")
			toTeam, _ := cmd.Flags().GetBool("to-team")
			file, _ := cmd.Flags().GetString("file")
			backendName, _ := cmd.Flags().GetString("backend")
			profile, _ := cmd.Flags().GetString("profile")
			return runSyncPush(cmd, toKeys, toFiles, toTeam, file, backendName, profile)
		},
	}

	cmd.Flags().StringArray("to", nil, "recipient's age public key (age1...) — repeatable")
	cmd.Flags().StringArray("to-file", nil, "file containing age public keys (one per line) — repeatable")
	cmd.Flags().Bool("to-team", false, "encrypt for all team members defined in .envref.yaml")
	cmd.Flags().StringP("file", "f", defaultSyncFile, "path to the encrypted sync file")
	cmd.Flags().StringP("backend", "b", "", "backend to export secrets from (default: first configured)")
	cmd.Flags().StringP("profile", "P", "", "profile scope for secrets (e.g., staging, production)")

	return cmd
}

// newSyncPullCmd creates the sync pull subcommand.
func newSyncPullCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Import secrets from an encrypted sync file",
		Long: `Import secrets from an age-encrypted sync file into a backend.

Reads the encrypted file (default .envref.secrets.age), decrypts it using the
specified age identity (private key), and stores each key-value pair in the
configured backend.

The identity can be provided via --identity (path to key file) or the
AGE_IDENTITY environment variable.

By default, existing secrets in the backend are not overwritten. Use --force
to overwrite existing values.

Examples:
  envref sync pull --identity key.txt
  envref sync pull --identity key.txt --force     # overwrite existing secrets
  envref sync pull --file secrets.age             # custom input file
  envref sync pull --backend keychain             # specific backend
  envref sync pull --profile staging              # profile-scoped secrets
  AGE_IDENTITY=key.txt envref sync pull           # identity via env var`,
		RunE: func(cmd *cobra.Command, args []string) error {
			identityFile, _ := cmd.Flags().GetString("identity")
			file, _ := cmd.Flags().GetString("file")
			backendName, _ := cmd.Flags().GetString("backend")
			profile, _ := cmd.Flags().GetString("profile")
			force, _ := cmd.Flags().GetBool("force")
			return runSyncPull(cmd, identityFile, file, backendName, profile, force)
		},
	}

	cmd.Flags().StringP("identity", "i", "", "path to age identity (private key) file")
	cmd.Flags().StringP("file", "f", defaultSyncFile, "path to the encrypted sync file")
	cmd.Flags().StringP("backend", "b", "", "backend to import secrets into (default: first configured)")
	cmd.Flags().StringP("profile", "P", "", "profile scope for secrets (e.g., staging, production)")
	cmd.Flags().Bool("force", false, "overwrite existing secrets in the backend")

	return cmd
}

// runSyncPush exports secrets from a backend and encrypts them into a sync file.
func runSyncPush(cmd *cobra.Command, toKeys, toFiles []string, toTeam bool, file, backendName, profile string) error {
	w := output.NewWriter(cmd)

	// Load project config (needed for backends and potentially team keys).
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, configDir, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// If --to-team is set, append team member public keys to the recipient list.
	if toTeam {
		if len(cfg.Team) == 0 {
			return fmt.Errorf("no team members configured (add with: envref team add <name> <key>)")
		}
		toKeys = append(toKeys, cfg.TeamPublicKeys()...)
	}

	// Collect all recipient public keys.
	recipients, err := collectRecipients(toKeys, toFiles)
	if err != nil {
		return err
	}
	if len(recipients) == 0 {
		return fmt.Errorf("at least one recipient is required (use --to, --to-file, or --to-team)")
	}

	if len(cfg.Backends) == 0 {
		return fmt.Errorf("no backends configured in %s", config.FullFileName)
	}

	// Determine target backend.
	if backendName == "" {
		backendName = cfg.Backends[0].Name
	}

	registry, err := buildRegistry(cfg)
	if err != nil {
		return fmt.Errorf("initializing backends: %w", err)
	}

	targetBackend := registry.Backend(backendName)
	if targetBackend == nil {
		return fmt.Errorf("backend %q is not registered", backendName)
	}

	// Create namespaced backend.
	effectiveProfile := cfg.EffectiveProfile(profile)
	var nsBackend backend.Backend
	if effectiveProfile != "" {
		nsBackend, err = backend.NewProfileNamespacedBackend(targetBackend, cfg.Project, effectiveProfile)
	} else {
		nsBackend, err = backend.NewNamespacedBackend(targetBackend, cfg.Project)
	}
	if err != nil {
		return fmt.Errorf("creating namespaced backend: %w", err)
	}

	// List all secret keys.
	keys, err := nsBackend.List()
	if err != nil {
		return fmt.Errorf("listing secrets: %w", err)
	}

	if len(keys) == 0 {
		return fmt.Errorf("no secrets found in backend %q for project %q", backendName, cfg.Project)
	}

	// Retrieve all secret values.
	secrets := make(map[string]string, len(keys))
	for _, key := range keys {
		value, err := nsBackend.Get(key)
		if err != nil {
			return fmt.Errorf("reading secret %q: %w", key, err)
		}
		secrets[key] = value
	}

	// Serialize secrets to JSON.
	jsonData, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling secrets: %w", err)
	}

	// Encrypt for all recipients.
	encrypted, err := encryptForRecipients(string(jsonData), recipients)
	if err != nil {
		return fmt.Errorf("encrypting secrets: %w", err)
	}

	// Write the encrypted file relative to the project root.
	outPath := file
	if !filepath.IsAbs(outPath) {
		outPath = filepath.Join(configDir, outPath)
	}

	if err := os.WriteFile(outPath, []byte(encrypted), 0o644); err != nil {
		return fmt.Errorf("writing sync file: %w", err)
	}

	w.Info("pushed %d secrets to %s (encrypted for %d recipients)\n", len(secrets), file, len(recipients))

	// Log keys at verbose level.
	sortedKeys := make([]string, 0, len(secrets))
	for k := range secrets {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)
	for _, k := range sortedKeys {
		w.Verbose("  %s\n", k)
	}

	return nil
}

// runSyncPull decrypts a sync file and imports secrets into a backend.
func runSyncPull(cmd *cobra.Command, identityFile, file, backendName, profile string, force bool) error {
	w := output.NewWriter(cmd)

	// Resolve identity file.
	if identityFile == "" {
		identityFile = os.Getenv("AGE_IDENTITY")
	}
	if identityFile == "" {
		return fmt.Errorf("identity file is required (use --identity or set AGE_IDENTITY)")
	}

	// Parse age identities from file.
	identities, err := parseIdentityFile(identityFile)
	if err != nil {
		return err
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

	registry, err := buildRegistry(cfg)
	if err != nil {
		return fmt.Errorf("initializing backends: %w", err)
	}

	targetBackend := registry.Backend(backendName)
	if targetBackend == nil {
		return fmt.Errorf("backend %q is not registered", backendName)
	}

	// Create namespaced backend.
	effectiveProfile := cfg.EffectiveProfile(profile)
	var nsBackend backend.Backend
	if effectiveProfile != "" {
		nsBackend, err = backend.NewProfileNamespacedBackend(targetBackend, cfg.Project, effectiveProfile)
	} else {
		nsBackend, err = backend.NewNamespacedBackend(targetBackend, cfg.Project)
	}
	if err != nil {
		return fmt.Errorf("creating namespaced backend: %w", err)
	}

	// Read the encrypted file.
	inPath := file
	if !filepath.IsAbs(inPath) {
		inPath = filepath.Join(configDir, inPath)
	}

	encryptedData, err := os.ReadFile(inPath)
	if err != nil {
		return fmt.Errorf("reading sync file: %w", err)
	}

	// Decrypt.
	plaintext, err := decryptWithIdentities(string(encryptedData), identities)
	if err != nil {
		return fmt.Errorf("decrypting sync file: %w", err)
	}

	// Parse JSON.
	var secrets map[string]string
	if err := json.Unmarshal([]byte(plaintext), &secrets); err != nil {
		return fmt.Errorf("parsing decrypted secrets: %w", err)
	}

	if len(secrets) == 0 {
		w.Info("sync file contains no secrets\n")
		return nil
	}

	// Import secrets into the backend.
	var imported, skipped int
	sortedKeys := make([]string, 0, len(secrets))
	for k := range secrets {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, key := range sortedKeys {
		value := secrets[key]

		// Check if the secret already exists.
		if !force {
			_, err := nsBackend.Get(key)
			if err == nil {
				w.Verbose("  skipped %s (already exists, use --force to overwrite)\n", key)
				skipped++
				continue
			}
		}

		if err := nsBackend.Set(key, value); err != nil {
			return fmt.Errorf("storing secret %q: %w", key, err)
		}
		w.Verbose("  imported %s\n", key)
		imported++
	}

	w.Info("pulled %d secrets from %s (%d imported, %d skipped)\n", len(secrets), file, imported, skipped)

	return nil
}

// collectRecipients gathers age recipients from --to keys and --to-file files.
func collectRecipients(toKeys, toFiles []string) ([]*age.X25519Recipient, error) {
	var recipients []*age.X25519Recipient

	// Parse direct keys.
	for _, key := range toKeys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		r, err := age.ParseX25519Recipient(key)
		if err != nil {
			return nil, fmt.Errorf("invalid age public key %q: %w", truncateKey(key), err)
		}
		recipients = append(recipients, r)
	}

	// Parse keys from files.
	for _, path := range toFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading key file %q: %w", path, err)
		}

		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			r, err := age.ParseX25519Recipient(line)
			if err != nil {
				return nil, fmt.Errorf("invalid age public key in %q: %q: %w", path, truncateKey(line), err)
			}
			recipients = append(recipients, r)
		}
	}

	return recipients, nil
}

// encryptForRecipients encrypts plaintext for multiple age recipients and
// returns ASCII-armored ciphertext.
func encryptForRecipients(plaintext string, recipients []*age.X25519Recipient) (string, error) {
	var buf bytes.Buffer
	armorWriter := armor.NewWriter(&buf)

	// Convert to age.Recipient interface slice.
	ageRecipients := make([]age.Recipient, len(recipients))
	for i, r := range recipients {
		ageRecipients[i] = r
	}

	writer, err := age.Encrypt(armorWriter, ageRecipients...)
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

// parseIdentityFile reads age identities from a key file.
func parseIdentityFile(path string) ([]age.Identity, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening identity file: %w", err)
	}
	defer func() { _ = f.Close() }()

	identities, err := age.ParseIdentities(f)
	if err != nil {
		return nil, fmt.Errorf("parsing identity file %q: %w", path, err)
	}

	if len(identities) == 0 {
		return nil, fmt.Errorf("no identities found in %q", path)
	}

	return identities, nil
}

// decryptWithIdentities decrypts ASCII-armored age ciphertext using the
// provided identities.
func decryptWithIdentities(ciphertext string, identities []age.Identity) (string, error) {
	armorReader := armor.NewReader(strings.NewReader(ciphertext))

	reader, err := age.Decrypt(armorReader, identities...)
	if err != nil {
		return "", fmt.Errorf("decrypting: %w", err)
	}

	plaintext, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("reading decrypted data: %w", err)
	}

	return string(plaintext), nil
}
