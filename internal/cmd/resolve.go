package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/config"
	"github.com/xcke/envref/internal/envfile"
	"github.com/xcke/envref/internal/parser"
	"github.com/xcke/envref/internal/resolve"
)

// newResolveCmd creates the resolve subcommand.
func newResolveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve all references and output fully resolved environment",
		Long: `Load .env and .env.local files, merge them, interpolate variables,
resolve all ref:// secret references via configured backends, and output
fully resolved KEY=VALUE pairs to stdout.

When a profile is active (via --profile flag or active_profile in config),
a profile-specific env file is loaded between .env and .env.local:
  .env ← .env.<profile> ← .env.local

By default, output is in KEY=VALUE format (one per line). Use --direnv
to output in direnv-compatible format (export KEY=VALUE).

Examples:
  envref resolve                         # output KEY=VALUE pairs
  envref resolve --profile staging       # use staging profile
  envref resolve --direnv                # output export KEY=VALUE for direnv
  eval "$(envref resolve --direnv)"      # inject into current shell`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			direnv, _ := cmd.Flags().GetBool("direnv")
			profile, _ := cmd.Flags().GetString("profile")
			return runResolve(cmd, direnv, profile)
		},
	}

	cmd.Flags().Bool("direnv", false, "output in direnv-compatible format (export KEY=VALUE)")
	cmd.Flags().StringP("profile", "P", "", "environment profile to use (e.g., staging, production)")

	return cmd
}

// runResolve implements the resolve command logic.
func runResolve(cmd *cobra.Command, direnv bool, profileOverride string) error {
	// Load project config to get project name, backend config, and file paths.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, projectDir, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Resolve file paths relative to the project root.
	envPath := resolveFilePath(projectDir, cfg.EnvFile)
	localPath := resolveFilePath(projectDir, cfg.LocalFile)

	// Determine the active profile and resolve its env file path.
	var profilePath string
	profile := cfg.EffectiveProfile(profileOverride)
	if profile != "" {
		profilePath = resolveFilePath(projectDir, cfg.ProfileEnvFile(profile))
	}

	// Load and merge env files.
	env, err := loadAndMergeEnv(cmd, envPath, profilePath, localPath)
	if err != nil {
		return err
	}

	// If no refs, just output without backend resolution.
	if !env.HasRefs() {
		return outputEntries(cmd, envToEntries(env), direnv)
	}

	// Build the backend registry.
	if len(cfg.Backends) == 0 {
		return fmt.Errorf("ref:// references found but no backends configured in %s", config.FullFileName)
	}

	registry, err := buildRegistry(cfg)
	if err != nil {
		return fmt.Errorf("initializing backends: %w", err)
	}

	// Resolve references.
	result, err := resolve.Resolve(env, registry, cfg.Project)
	if err != nil {
		return fmt.Errorf("resolving references: %w", err)
	}

	// Report resolution errors to stderr.
	for _, keyErr := range result.Errors {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: %s\n", keyErr.Error())
	}

	// Output resolved entries.
	if err := outputEntries(cmd, result.Entries, direnv); err != nil {
		return err
	}

	if !result.Resolved() {
		return fmt.Errorf("%d reference(s) could not be resolved", len(result.Errors))
	}

	return nil
}

// resolveFilePath resolves a potentially relative file path against the project directory.
func resolveFilePath(projectDir, filePath string) string {
	if strings.HasPrefix(filePath, "/") {
		return filePath
	}
	return projectDir + "/" + filePath
}

// loadAndMergeEnv loads the base env file, an optional profile-specific env
// file, and the local override file, merges them in order (base ← profile ←
// local), and interpolates variables.
//
// The profilePath parameter is optional — pass an empty string to skip the
// profile layer (backwards-compatible with the two-layer merge).
func loadAndMergeEnv(cmd *cobra.Command, envPath, profilePath, localPath string) (*envfile.Env, error) {
	base, warnings, err := envfile.Load(envPath)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", envPath, err)
	}
	printWarnings(cmd, envPath, warnings)

	// Optional profile layer: .env.<profile> (e.g., .env.staging).
	var profile *envfile.Env
	if profilePath != "" {
		var profileWarnings []parser.Warning
		profile, profileWarnings, err = envfile.LoadOptional(profilePath)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", profilePath, err)
		}
		printWarnings(cmd, profilePath, profileWarnings)
	}

	local, localWarnings, err := envfile.LoadOptional(localPath)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", localPath, err)
	}
	printWarnings(cmd, localPath, localWarnings)

	// Merge: base ← profile ← local (later layers win on conflicts).
	if profile != nil && profile.Len() > 0 {
		merged := envfile.Merge(base, profile, local)
		envfile.Interpolate(merged)
		return merged, nil
	}

	merged := envfile.Merge(base, local)
	envfile.Interpolate(merged)

	return merged, nil
}

// envToEntries converts an Env to resolve.Entry slice for output.
func envToEntries(env *envfile.Env) []resolve.Entry {
	all := env.All()
	entries := make([]resolve.Entry, len(all))
	for i, e := range all {
		entries[i] = resolve.Entry{
			Key:    e.Key,
			Value:  e.Value,
			WasRef: e.IsRef,
		}
	}
	return entries
}

// outputEntries writes entries to stdout in the appropriate format.
func outputEntries(cmd *cobra.Command, entries []resolve.Entry, direnv bool) error {
	out := cmd.OutOrStdout()
	for _, entry := range entries {
		if direnv {
			_, _ = fmt.Fprintf(out, "export %s=%s\n", entry.Key, shellQuote(entry.Value))
		} else {
			_, _ = fmt.Fprintf(out, "%s=%s\n", entry.Key, entry.Value)
		}
	}
	return nil
}

// shellQuote wraps a value in single quotes for safe shell usage.
// Single quotes inside the value are escaped as '\'' (end quote, escaped quote, start quote).
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	// If value contains no special characters, return as-is for readability.
	if !strings.ContainsAny(s, " \t\n\r'\"\\$`!#&|;(){}[]<>?*~") {
		return s
	}
	// Single-quote the value, escaping embedded single quotes.
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
