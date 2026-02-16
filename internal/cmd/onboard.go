package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/audit"
	"github.com/xcke/envref/internal/backend"
	"github.com/xcke/envref/internal/config"
	"github.com/xcke/envref/internal/envfile"
	"github.com/xcke/envref/internal/output"
	"github.com/xcke/envref/internal/ref"
	"github.com/xcke/envref/internal/resolve"
)

// newOnboardCmd creates the onboard subcommand.
func newOnboardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "onboard",
		Short: "Interactive setup for new team members",
		Long: `Walk through all missing secrets and set them up interactively.

The onboard command identifies which secrets are missing or unresolved for the
current project by loading the environment files, attempting to resolve all
ref:// references, and prompting for values for any that fail.

This is the recommended first step after cloning a project that uses envref.

Use --profile to onboard for a specific environment profile.
Use --backend to specify which backend to store secrets in.
Use --dry-run to see what secrets are missing without being prompted.

Examples:
  envref onboard                            # set up all missing secrets
  envref onboard --profile staging          # set up secrets for staging
  envref onboard --backend keychain         # store in specific backend
  envref onboard --dry-run                  # list missing secrets only`,
		Args: cobra.NoArgs,
		PreRun: func(cmd *cobra.Command, args []string) {
			setVaultCmdContext(cmd)
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			clearVaultCmdContext()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			profile, _ := cmd.Flags().GetString("profile")
			backendName, _ := cmd.Flags().GetString("backend")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			return runOnboard(cmd, profile, backendName, dryRun)
		},
	}

	cmd.Flags().StringP("profile", "P", "", "environment profile to use (e.g., staging, production)")
	cmd.Flags().StringP("backend", "b", "", "backend to store secrets in (default: first configured)")
	cmd.Flags().Bool("dry-run", false, "list missing secrets without prompting")

	return cmd
}

// missingSecret describes a secret that needs to be set during onboarding.
type missingSecret struct {
	// Key is the environment variable name.
	Key string
	// Ref is the original ref:// URI from the .env file.
	Ref string
	// Backend is the parsed backend name from the ref URI (e.g., "secrets", "keychain").
	Backend string
	// Path is the parsed secret path from the ref URI.
	Path string
}

// runOnboard implements the onboard command logic.
func runOnboard(cmd *cobra.Command, profileOverride, backendName string, dryRun bool) error {
	w := output.NewWriter(cmd)
	out := w.Stdout()

	// Load project config.
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, projectDir, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if len(cfg.Backends) == 0 {
		return fmt.Errorf("no backends configured in %s — add a backend to .envref.yaml first", config.FullFileName)
	}

	// Determine target backend for storing secrets.
	if backendName == "" {
		backendName = cfg.Backends[0].Name
	}

	// Build registry.
	registry, err := buildRegistry(cfg)
	if err != nil {
		return fmt.Errorf("initializing backends: %w", err)
	}

	targetBackend := registry.Backend(backendName)
	if targetBackend == nil {
		return fmt.Errorf("backend %q is not registered", backendName)
	}

	// Determine active profile.
	profile := cfg.EffectiveProfile(profileOverride)

	// Resolve file paths.
	envPath := resolveFilePath(projectDir, cfg.EnvFile)
	localPath := resolveFilePath(projectDir, cfg.LocalFile)

	var profilePath string
	if profile != "" {
		profileEnvFile := cfg.ProfileEnvFile(profile)
		profilePath = resolveFilePath(projectDir, profileEnvFile)
	}

	// Load and merge environment.
	if !fileExists(envPath) {
		return fmt.Errorf("no %s file found — run \"envref init\" to create one", cfg.EnvFile)
	}

	env, err := loadAndMergeEnv(cmd, envPath, profilePath, localPath)
	if err != nil {
		return fmt.Errorf("loading environment: %w", err)
	}

	// Print header.
	_, _ = fmt.Fprintf(out, "%s %s\n", w.Bold("Project:"), cfg.Project)
	if profile != "" {
		_, _ = fmt.Fprintf(out, "%s %s\n", w.Bold("Profile:"), profile)
	}
	_, _ = fmt.Fprintf(out, "%s %s\n", w.Bold("Backend:"), backendName)

	// Collect missing secrets: refs that fail to resolve.
	missing, err := findMissingSecrets(env, registry, cfg.Project, profile)
	if err != nil {
		return fmt.Errorf("checking secrets: %w", err)
	}

	// Also check .env.example for keys missing from the environment entirely.
	examplePath := resolveFilePath(projectDir, ".env.example")
	missingFromExample := findMissingExampleKeys(examplePath, env)

	if len(missing) == 0 && len(missingFromExample) == 0 {
		_, _ = fmt.Fprintf(out, "\n%s All secrets are resolved and no missing keys found.\n", w.Green("[ok]"))
		_, _ = fmt.Fprintf(out, "You're all set! Run %s to verify.\n", w.Bold("envref status"))
		return nil
	}

	// Print summary of what needs attention.
	_, _ = fmt.Fprintln(out)
	if len(missing) > 0 {
		_, _ = fmt.Fprintf(out, "%s %d secret(s) need to be configured:\n", w.Bold("Missing secrets:"), len(missing))
		for i, m := range missing {
			_, _ = fmt.Fprintf(out, "  %d. %s  %s\n", i+1, m.Key, w.Yellow(m.Ref))
		}
	}

	if len(missingFromExample) > 0 {
		_, _ = fmt.Fprintf(out, "\n%s %d key(s) in .env.example are not in your environment:\n",
			w.Bold("Missing from .env.example:"), len(missingFromExample))
		for _, key := range missingFromExample {
			_, _ = fmt.Fprintf(out, "  - %s\n", key)
		}
	}

	if dryRun {
		_, _ = fmt.Fprintf(out, "\n%s Use %s to set them interactively.\n",
			w.Yellow("[dry-run]"), w.Bold("envref onboard"))
		return nil
	}

	if len(missing) == 0 {
		_, _ = fmt.Fprintf(out, "\nNo ref:// secrets to configure. Add the missing keys to your .env file.\n")
		return nil
	}

	// Interactive loop: prompt for each missing secret.
	_, _ = fmt.Fprintf(out, "\nLet's set up each missing secret. Press Ctrl+C to abort.\n\n")

	// Build namespaced backend for storing.
	var nsBackend *backend.NamespacedBackend
	if profile != "" {
		nsBackend, err = backend.NewProfileNamespacedBackend(targetBackend, cfg.Project, profile)
	} else {
		nsBackend, err = backend.NewNamespacedBackend(targetBackend, cfg.Project)
	}
	if err != nil {
		return fmt.Errorf("creating namespaced backend: %w", err)
	}

	configured := 0
	skipped := 0
	for i, m := range missing {
		_, _ = fmt.Fprintf(out, "[%d/%d] %s\n", i+1, len(missing), w.Bold(m.Key))
		_, _ = fmt.Fprintf(out, "  ref: %s\n", w.Yellow(m.Ref))

		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "  Enter value (or press Enter to skip): ")
		value, err := readLine(cmd.InOrStdin())
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}

		value = strings.TrimSpace(value)
		if value == "" {
			_, _ = fmt.Fprintf(out, "  %s\n\n", w.Yellow("skipped"))
			skipped++
			continue
		}

		// Store the secret using the path from the ref URI.
		if err := nsBackend.Set(m.Path, value); err != nil {
			w.Error("  failed to store: %v\n\n", err)
			skipped++
			continue
		}

		// Log the operation to the audit log (best-effort).
		_ = newAuditLogger(projectDir).Log(audit.Entry{
			Operation: audit.OpSet,
			Key:       m.Path,
			Backend:   backendName,
			Project:   cfg.Project,
			Profile:   profile,
			Detail:    "onboard",
		})

		_, _ = fmt.Fprintf(out, "  %s\n\n", w.Green("stored"))
		configured++
	}

	// Summary.
	_, _ = fmt.Fprintf(out, "%s\n", w.Bold("Summary:"))
	_, _ = fmt.Fprintf(out, "  Configured: %s\n", w.Green(fmt.Sprintf("%d", configured)))
	if skipped > 0 {
		_, _ = fmt.Fprintf(out, "  Skipped:    %s\n", w.Yellow(fmt.Sprintf("%d", skipped)))
	}

	if configured > 0 {
		_, _ = fmt.Fprintf(out, "\nRun %s to verify all secrets resolve.\n", w.Bold("envref status"))
	}
	if skipped > 0 {
		_, _ = fmt.Fprintf(out, "Run %s again to configure skipped secrets.\n", w.Bold("envref onboard"))
	}

	return nil
}

// findMissingSecrets identifies ref:// entries that fail to resolve.
func findMissingSecrets(env *envfile.Env, registry *backend.Registry, project, profile string) ([]missingSecret, error) {
	if !env.HasRefs() {
		return nil, nil
	}

	// Attempt resolution.
	result, err := resolve.ResolveWithProfile(env, registry, project, profile)
	if err != nil {
		return nil, err
	}

	if result.Resolved() {
		return nil, nil
	}

	// Collect failures, deduplicating by ref URI.
	seen := make(map[string]bool)
	var missing []missingSecret
	for _, keyErr := range result.Errors {
		if seen[keyErr.Ref] {
			continue
		}
		seen[keyErr.Ref] = true

		m := missingSecret{
			Key: keyErr.Key,
			Ref: keyErr.Ref,
		}

		parsed, parseErr := ref.Parse(keyErr.Ref)
		if parseErr == nil {
			m.Backend = parsed.Backend
			m.Path = parsed.Path
		} else {
			// Use the key as fallback path if parsing fails.
			m.Path = keyErr.Key
		}

		missing = append(missing, m)
	}

	return missing, nil
}

// findMissingExampleKeys returns keys present in .env.example but not in the merged env.
func findMissingExampleKeys(examplePath string, env *envfile.Env) []string {
	if !fileExists(examplePath) {
		return nil
	}

	example, _, err := envfile.Load(examplePath)
	if err != nil {
		return nil
	}

	envKeys := keySet(env.Keys())
	var missing []string
	for _, key := range example.Keys() {
		if _, ok := envKeys[key]; !ok {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return missing
}
