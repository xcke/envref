package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/config"
	"github.com/xcke/envref/internal/envfile"
	"github.com/xcke/envref/internal/output"
	"github.com/xcke/envref/internal/parser"
	"github.com/xcke/envref/internal/ref"
)

// syncEnvRef adds or updates a ref:// entry in the appropriate .env file
// after a secret is stored in a backend. If the key already exists with a
// non-ref value, it is left untouched to avoid overwriting manual overrides.
func syncEnvRef(cmd *cobra.Command, cfg *config.Config, configDir, key, backendName, effectiveProfile string) error {
	if noEnvFlag(cmd) {
		return nil
	}

	targetPath := envRefTargetPath(cfg, configDir, effectiveProfile)

	env, _, err := envfile.LoadOptional(targetPath)
	if err != nil {
		return fmt.Errorf("loading %s: %w", targetPath, err)
	}

	// Don't overwrite existing non-ref values.
	if existing, found := env.Get(key); found && !existing.IsRef {
		return nil
	}

	refValue := ref.Prefix + backendName + "/" + key
	env.Set(parser.Entry{
		Key:   key,
		Value: refValue,
		Raw:   refValue,
		IsRef: true,
	})

	if err := env.Write(targetPath); err != nil {
		return fmt.Errorf("writing %s: %w", targetPath, err)
	}

	w := output.NewWriter(cmd)
	relPath, _ := filepath.Rel(configDir, targetPath)
	if relPath == "" {
		relPath = targetPath
	}
	w.Verbose("added %s=%s to %s\n", key, refValue, relPath)
	return nil
}

// removeEnvRef removes a ref:// entry from the appropriate .env file after
// a secret is deleted from a backend. Only ref:// values are removed; non-ref
// values are left untouched.
func removeEnvRef(cmd *cobra.Command, cfg *config.Config, configDir, key, effectiveProfile string) error {
	if noEnvFlag(cmd) {
		return nil
	}

	targetPath := envRefTargetPath(cfg, configDir, effectiveProfile)

	env, _, err := envfile.LoadOptional(targetPath)
	if err != nil {
		return fmt.Errorf("loading %s: %w", targetPath, err)
	}

	existing, found := env.Get(key)
	if !found {
		return nil
	}

	// Don't remove non-ref values.
	if !existing.IsRef {
		return nil
	}

	env.Delete(key)

	if err := env.Write(targetPath); err != nil {
		return fmt.Errorf("writing %s: %w", targetPath, err)
	}

	w := output.NewWriter(cmd)
	relPath, _ := filepath.Rel(configDir, targetPath)
	if relPath == "" {
		relPath = targetPath
	}
	w.Verbose("removed %s from %s\n", key, relPath)
	return nil
}

// noEnvFlag checks if the --no-env flag is set on the command or any parent.
func noEnvFlag(cmd *cobra.Command) bool {
	f := cmd.Flag("no-env")
	return f != nil && f.Value.String() == "true"
}

// envRefTargetPath returns the absolute path to the .env file that should be
// updated for the given profile. If a profile is active, the profile-specific
// env file is used; otherwise the base env file from config.
func envRefTargetPath(cfg *config.Config, configDir, effectiveProfile string) string {
	if effectiveProfile != "" {
		return filepath.Join(configDir, cfg.ProfileEnvFile(effectiveProfile))
	}
	return filepath.Join(configDir, cfg.EnvFile)
}
