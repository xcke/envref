package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/config"
	"github.com/xcke/envref/internal/envfile"
	"github.com/xcke/envref/internal/output"
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
to output in direnv-compatible format (export KEY=VALUE), or use --format
to select from plain, json, shell, or table.

Use --strict to suppress output entirely if any reference fails to resolve.
This is useful in CI pipelines where partial output is unsafe.

Use --watch to continuously monitor .env files for changes and re-resolve
automatically. This is useful for development workflows where env files
change frequently. The output is re-printed on each detected file change.

Examples:
  envref resolve                         # output KEY=VALUE pairs
  envref resolve --profile staging       # use staging profile
  envref resolve --direnv                # output export KEY=VALUE for direnv
  envref resolve --format json           # output as JSON array
  envref resolve --strict                # fail with no output if any ref fails
  envref resolve --watch                 # re-resolve on file changes
  eval "$(envref resolve --direnv)"      # inject into current shell`,
		Args: cobra.NoArgs,
		PreRun: func(cmd *cobra.Command, args []string) {
			setVaultCmdContext(cmd)
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			clearVaultCmdContext()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			direnv, _ := cmd.Flags().GetBool("direnv")
			profile, _ := cmd.Flags().GetString("profile")
			formatStr, _ := cmd.Flags().GetString("format")
			strict, _ := cmd.Flags().GetBool("strict")
			watch, _ := cmd.Flags().GetBool("watch")
			if watch {
				return runResolveWatch(cmd, direnv, profile, formatStr, strict)
			}
			return runResolve(cmd, direnv, profile, formatStr, strict)
		},
	}

	cmd.Flags().Bool("direnv", false, "output in direnv-compatible format (export KEY=VALUE)")
	cmd.Flags().StringP("profile", "P", "", "environment profile to use (e.g., staging, production)")
	cmd.Flags().String("format", "plain", "output format: plain, json, shell, table")
	cmd.Flags().Bool("strict", false, "fail with no output if any reference cannot be resolved")
	cmd.Flags().BoolP("watch", "w", false, "watch .env files for changes and re-resolve automatically")

	return cmd
}

// runResolve implements the resolve command logic.
func runResolve(cmd *cobra.Command, direnv bool, profileOverride, formatStr string, strict bool) error {
	w := output.NewWriter(cmd)

	// --direnv is a shorthand for --format shell.
	if direnv {
		formatStr = "shell"
	}
	format, err := parseFormat(formatStr)
	if err != nil {
		return err
	}

	// Load project config to get project name, backend config, and file paths.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, projectDir, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	w.Debug("config loaded from %s/%s\n", projectDir, config.FullFileName)

	// Resolve file paths relative to the project root.
	envPath := resolveFilePath(projectDir, cfg.EnvFile)
	localPath := resolveFilePath(projectDir, cfg.LocalFile)

	// Determine the active profile and resolve its env file path.
	var profilePath string
	profile := cfg.EffectiveProfile(profileOverride)
	if profile != "" {
		profilePath = resolveFilePath(projectDir, cfg.ProfileEnvFile(profile))
		w.Verbose("using profile %q\n", profile)
	}

	// Load and merge env files.
	env, err := loadAndMergeEnv(cmd, envPath, profilePath, localPath)
	if err != nil {
		return err
	}

	w.Debug("merged %d keys (%d refs)\n", env.Len(), len(env.Refs()))

	// If no refs (including embedded nested refs), just output without backend resolution.
	if !env.HasAnyRefs() {
		return outputEntries(cmd, envToEntries(env), format)
	}

	// Build the backend registry.
	if len(cfg.Backends) == 0 {
		return fmt.Errorf("ref:// references found but no backends configured in %s", config.FullFileName)
	}

	registry, err := buildRegistry(cfg)
	if err != nil {
		return fmt.Errorf("initializing backends: %w", err)
	}
	defer registry.CloseAll()

	w.Debug("registered %d backend(s)\n", len(cfg.Backends))

	// Resolve references (with profile-scoped fallback if profile is active).
	result, err := resolve.ResolveWithProfile(env, registry, cfg.Project, profile)
	if err != nil {
		return fmt.Errorf("resolving references: %w", err)
	}

	// Report resolution errors to stderr.
	for _, keyErr := range result.Errors {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: %s\n", keyErr.Error())
	}

	// In strict mode, suppress all output if any reference failed.
	if strict && !result.Resolved() {
		return fmt.Errorf("%d reference(s) could not be resolved (strict mode: no output produced)", len(result.Errors))
	}

	// Output resolved entries.
	if err := outputEntries(cmd, result.Entries, format); err != nil {
		return err
	}

	if !result.Resolved() {
		return fmt.Errorf("%d reference(s) could not be resolved", len(result.Errors))
	}

	return nil
}

// runResolveWatch implements the resolve --watch mode. It performs an initial
// resolve, then watches the relevant .env files for changes and re-resolves
// on each detected change. File system events are debounced to avoid redundant
// resolves during rapid edits.
func runResolveWatch(cmd *cobra.Command, direnv bool, profileOverride, formatStr string, strict bool) error {
	w := output.NewWriter(cmd)

	if direnv {
		formatStr = "shell"
	}
	format, err := parseFormat(formatStr)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, projectDir, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	envPath := resolveFilePath(projectDir, cfg.EnvFile)
	localPath := resolveFilePath(projectDir, cfg.LocalFile)

	var profilePath string
	profile := cfg.EffectiveProfile(profileOverride)
	if profile != "" {
		profilePath = resolveFilePath(projectDir, cfg.ProfileEnvFile(profile))
	}

	// Perform the initial resolve.
	if err := resolveAndOutput(cmd, cfg, envPath, profilePath, localPath, profile, format, strict); err != nil {
		// In watch mode, print the error but continue watching.
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: %s\n", err)
	}

	// Set up file watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating file watcher: %w", err)
	}
	defer func() { _ = watcher.Close() }()

	// Watch the env files that exist.
	watchPaths := collectWatchPaths(envPath, profilePath, localPath)
	for _, p := range watchPaths {
		if err := watcher.Add(p); err != nil {
			w.Verbose("cannot watch %s: %v\n", p, err)
		} else {
			w.Verbose("watching %s\n", p)
		}
	}

	if len(watchPaths) == 0 {
		return fmt.Errorf("no env files found to watch")
	}

	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "watching %d file(s) for changes... (Ctrl+C to stop)\n", len(watchPaths))

	// Handle signals for clean shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	// Debounce timer: coalesce rapid file changes.
	const debounceDelay = 100 * time.Millisecond
	var debounceTimer *time.Timer
	debounceCh := make(chan struct{}, 1)

	for {
		select {
		case <-sigCh:
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "\nstopping watch\n")
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			// Only react to writes and creates (covers most editors).
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
				continue
			}
			w.Debug("file changed: %s (%s)\n", event.Name, event.Op)

			// Reset debounce timer.
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounceDelay, func() {
				select {
				case debounceCh <- struct{}{}:
				default:
				}
			})

		case <-debounceCh:
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "change detected, re-resolving...\n")

			// Re-add watch paths in case files were recreated (some editors
			// delete + create rather than in-place write).
			for _, p := range watchPaths {
				_ = watcher.Add(p)
			}

			if err := resolveAndOutput(cmd, cfg, envPath, profilePath, localPath, profile, format, strict); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: %s\n", err)
			}

		case watchErr, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			w.Warn("watch error: %v\n", watchErr)
		}
	}
}

// resolveAndOutput runs the full resolve pipeline and outputs the result.
// It is used by the watch loop to re-resolve on each file change.
func resolveAndOutput(cmd *cobra.Command, cfg *config.Config, envPath, profilePath, localPath, profile string, format OutputFormat, strict bool) error {
	env, err := loadAndMergeEnv(cmd, envPath, profilePath, localPath)
	if err != nil {
		return err
	}

	if !env.HasAnyRefs() {
		return outputEntries(cmd, envToEntries(env), format)
	}

	if len(cfg.Backends) == 0 {
		return fmt.Errorf("ref:// references found but no backends configured in %s", config.FullFileName)
	}

	registry, err := buildRegistry(cfg)
	if err != nil {
		return fmt.Errorf("initializing backends: %w", err)
	}
	defer registry.CloseAll()

	result, err := resolve.ResolveWithProfile(env, registry, cfg.Project, profile)
	if err != nil {
		return fmt.Errorf("resolving references: %w", err)
	}

	for _, keyErr := range result.Errors {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: %s\n", keyErr.Error())
	}

	if strict && !result.Resolved() {
		return fmt.Errorf("%d reference(s) could not be resolved (strict mode: no output produced)", len(result.Errors))
	}

	if err := outputEntries(cmd, result.Entries, format); err != nil {
		return err
	}

	if !result.Resolved() {
		return fmt.Errorf("%d reference(s) could not be resolved", len(result.Errors))
	}

	return nil
}

// collectWatchPaths returns the subset of file paths that exist on disk
// and should be watched for changes.
func collectWatchPaths(paths ...string) []string {
	var result []string
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			result = append(result, p)
		}
	}
	return result
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
	w := output.NewWriter(cmd)

	w.Verbose("loading %s\n", envPath)
	base, warnings, err := envfile.Load(envPath)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", envPath, err)
	}
	printWarnings(cmd, envPath, warnings)
	w.Debug("loaded %d entries from %s\n", base.Len(), envPath)

	// Optional profile layer: .env.<profile> (e.g., .env.staging).
	var profile *envfile.Env
	if profilePath != "" {
		w.Verbose("loading profile %s\n", profilePath)
		var profileWarnings []parser.Warning
		profile, profileWarnings, err = envfile.LoadOptional(profilePath)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", profilePath, err)
		}
		printWarnings(cmd, profilePath, profileWarnings)
	}

	w.Verbose("loading %s\n", localPath)
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
func outputEntries(cmd *cobra.Command, entries []resolve.Entry, format OutputFormat) error {
	pairs := make([]kvPair, len(entries))
	for i, entry := range entries {
		pairs[i] = kvPair{Key: entry.Key, Value: entry.Value}
	}
	return formatKVPairs(cmd.OutOrStdout(), pairs, format)
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
