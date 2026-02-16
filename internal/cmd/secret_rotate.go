package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/backend"
	"github.com/xcke/envref/internal/config"
	"github.com/xcke/envref/internal/output"
)

// historyKeySuffix is the separator between a key name and its history index.
const historyKeySuffix = ".__history."

// newSecretRotateCmd creates the secret rotate subcommand.
func newSecretRotateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rotate <KEY>",
		Short: "Generate a new secret value and store the old one in history",
		Long: `Rotate a secret by generating a new random value and archiving the
previous value in history. The old value is stored under a history key
(<KEY>.__history.1) so it can be recovered if needed.

If the key does not yet exist, a new secret is generated and stored
(equivalent to 'envref secret generate').

Use --keep to control how many historical values are retained (default: 1).
Older entries beyond the keep limit are deleted.

Available charsets:
  alphanumeric  a-z, A-Z, 0-9 (default)
  ascii         alphanumeric + common symbols
  hex           0-9, a-f (lowercase hex)
  base64        standard base64 encoding

Examples:
  envref secret rotate API_KEY                               # rotate with defaults
  envref secret rotate API_KEY --length 64 --charset hex     # custom generation
  envref secret rotate API_KEY --print                       # print the new value
  envref secret rotate API_KEY --keep 3                      # keep 3 history entries
  envref secret rotate API_KEY --profile staging             # profile-scoped
  envref secret rotate API_KEY --backend keychain            # specific backend`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			length, _ := cmd.Flags().GetInt("length")
			charset, _ := cmd.Flags().GetString("charset")
			backendName, _ := cmd.Flags().GetString("backend")
			printVal, _ := cmd.Flags().GetBool("print")
			profile, _ := cmd.Flags().GetString("profile")
			keep, _ := cmd.Flags().GetInt("keep")
			return runSecretRotate(cmd, args[0], length, charset, backendName, printVal, profile, keep)
		},
	}

	cmd.Flags().IntP("length", "l", 32, "length of the generated secret")
	cmd.Flags().StringP("charset", "c", "alphanumeric", "character set: alphanumeric, ascii, hex, base64")
	cmd.Flags().StringP("backend", "b", "", "backend to store the secret in (default: first configured)")
	cmd.Flags().BoolP("print", "p", false, "print the new secret value to stdout")
	cmd.Flags().StringP("profile", "P", "", "profile scope for the secret (e.g., staging, production)")
	cmd.Flags().IntP("keep", "k", 1, "number of historical values to retain")

	return cmd
}

// runSecretRotate generates a new secret, archives the old value in history,
// and stores the new value.
func runSecretRotate(cmd *cobra.Command, key string, length int, charset, backendName string, printVal bool, profile string, keep int) error {
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

	// Validate keep.
	if keep < 0 {
		return fmt.Errorf("keep must be at least 0")
	}
	if keep > 100 {
		return fmt.Errorf("keep must not exceed 100")
	}

	// Generate the new secret.
	newValue, err := generateSecret(length, charset)
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

	w := output.NewWriter(cmd)

	// Try to read the current value for history archival.
	oldValue, err := nsBackend.Get(key)
	hadPrevious := err == nil

	if err != nil && !errors.Is(err, backend.ErrNotFound) {
		return fmt.Errorf("reading current secret: %w", err)
	}

	// Archive old value in history if it exists and keep > 0.
	if hadPrevious && keep > 0 {
		if err := rotateHistory(nsBackend, key, oldValue, keep); err != nil {
			return fmt.Errorf("archiving history: %w", err)
		}
	}

	// If keep is 0 and there was a previous value, clean up any existing history.
	if hadPrevious && keep == 0 {
		cleanupHistory(nsBackend, key)
	}

	// Store the new secret.
	if err := nsBackend.Set(key, newValue); err != nil {
		return fmt.Errorf("storing secret: %w", err)
	}

	if printVal {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), newValue)
	}

	scopeLabel := fmt.Sprintf("backend %q", backendName)
	if effectiveProfile != "" {
		scopeLabel = fmt.Sprintf("backend %q (profile %q)", backendName, effectiveProfile)
	}
	if hadPrevious {
		w.Info("secret %q rotated in %s (%d chars, %s); previous value archived\n", key, scopeLabel, length, charset)
	} else {
		w.Info("secret %q generated and stored in %s (%d chars, %s)\n", key, scopeLabel, length, charset)
	}
	return nil
}

// rotateHistory shifts existing history entries and stores the old value
// as history entry 1. Entries beyond the keep limit are deleted.
//
// History keys follow the pattern: <key>.__history.<N>
// where N=1 is the most recent previous value.
func rotateHistory(nsBackend *backend.NamespacedBackend, key, oldValue string, keep int) error {
	// Shift existing history entries up by one.
	// Start from the highest slot and move down to avoid overwriting.
	for i := keep; i >= 2; i-- {
		srcKey := historyKey(key, i-1)
		dstKey := historyKey(key, i)

		val, err := nsBackend.Get(srcKey)
		if errors.Is(err, backend.ErrNotFound) {
			// No entry at this position — try to clean the destination
			// if it's beyond what we're shifting into.
			continue
		}
		if err != nil {
			return fmt.Errorf("reading history entry %d: %w", i-1, err)
		}

		if err := nsBackend.Set(dstKey, val); err != nil {
			return fmt.Errorf("writing history entry %d: %w", i, err)
		}
	}

	// Store the old value as history entry 1.
	if err := nsBackend.Set(historyKey(key, 1), oldValue); err != nil {
		return fmt.Errorf("writing history entry 1: %w", err)
	}

	// Delete entries beyond the keep limit.
	// We only need to check the slot at keep+1 since we shifted everything up.
	beyondKey := historyKey(key, keep+1)
	_ = nsBackend.Delete(beyondKey) // Ignore ErrNotFound.

	return nil
}

// cleanupHistory removes all history entries for a key.
func cleanupHistory(nsBackend *backend.NamespacedBackend, key string) {
	for i := 1; i <= 100; i++ {
		err := nsBackend.Delete(historyKey(key, i))
		if errors.Is(err, backend.ErrNotFound) {
			break
		}
	}
}

// historyKey returns the history key for the given base key and index.
func historyKey(key string, index int) string {
	return fmt.Sprintf("%s%s%d", key, historyKeySuffix, index)
}
