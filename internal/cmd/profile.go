package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/config"
	"github.com/xcke/envref/internal/envfile"
	"github.com/xcke/envref/internal/output"
)

// newProfileCmd creates the profile command group for managing environment profiles.
func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage environment profiles",
		Long: `Manage environment profiles for switching between different configurations
(e.g., development, staging, production).

Profiles define which .env file overlays are applied during resolution.
The merge order is: .env ← .env.<profile> ← .env.local

Use subcommands to list available profiles.`,
	}

	cmd.AddCommand(newProfileListCmd())
	cmd.AddCommand(newProfileUseCmd())
	cmd.AddCommand(newProfileCreateCmd())
	cmd.AddCommand(newProfileDiffCmd())
	cmd.AddCommand(newProfileExportCmd())

	return cmd
}

// newProfileListCmd creates the profile list subcommand.
func newProfileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available profiles",
		Long: `List all available environment profiles for the current project.

Shows profiles defined in .envref.yaml and any convention-based profile
files found on disk (e.g., .env.staging, .env.production).

The active profile (from config or --profile flag) is marked with an asterisk (*).

Examples:
  envref profile list   # list all profiles`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileList(cmd)
		},
	}
}

// newProfileUseCmd creates the profile use subcommand.
func newProfileUseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use <name>",
		Short: "Set the active profile",
		Long: `Set the active environment profile for the current project.

Updates the active_profile field in .envref.yaml so that subsequent
commands (resolve, get, list) use the specified profile by default.

The profile must either be defined in .envref.yaml or exist as a
convention-based file (e.g., .env.<name>) on disk.

Use the --clear flag to deactivate the current profile.

Examples:
  envref profile use staging       # activate the staging profile
  envref profile use production    # switch to production
  envref profile use --clear       # deactivate the current profile`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clear, _ := cmd.Flags().GetBool("clear")
			var name string
			if clear {
				name = ""
			} else {
				if len(args) == 0 {
					return fmt.Errorf("profile name is required (or use --clear to deactivate)")
				}
				name = args[0]
			}
			return runProfileUse(cmd, name)
		},
	}

	cmd.Flags().Bool("clear", false, "clear the active profile")

	return cmd
}

// newProfileCreateCmd creates the profile create subcommand.
func newProfileCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new profile",
		Long: `Create a new environment profile by scaffolding a profile-specific .env file.

Creates the file .env.<name> (or a custom path via --env-file) and optionally
registers the profile in .envref.yaml.

By default, the new file is created with a comment header. Use --from to copy
an existing .env file as a starting point.

Examples:
  envref profile create staging              # create .env.staging
  envref profile create staging --register   # also add to .envref.yaml
  envref profile create staging --from .env  # copy .env as starting point
  envref profile create staging --env-file envs/staging.env  # custom path`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProfileCreate(cmd, args[0])
		},
	}

	cmd.Flags().Bool("register", false, "register the profile in .envref.yaml")
	cmd.Flags().Bool("force", false, "overwrite existing profile file")
	cmd.Flags().String("from", "", "copy an existing file as the profile base")
	cmd.Flags().String("env-file", "", "custom env file path (default .env.<name>)")

	return cmd
}

// runProfileCreate implements the profile create command logic.
func runProfileCreate(cmd *cobra.Command, name string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, projectDir, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	w := output.NewWriter(cmd)

	// Validate the profile name.
	if name == "local" {
		return fmt.Errorf("cannot create profile %q: reserved for local overrides (.env.local)", name)
	}
	if strings.Contains(name, ".") {
		return fmt.Errorf("profile name %q must not contain dots", name)
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("profile name %q must not contain path separators", name)
	}
	if name == "" {
		return fmt.Errorf("profile name is required")
	}

	force, _ := cmd.Flags().GetBool("force")
	register, _ := cmd.Flags().GetBool("register")
	fromFile, _ := cmd.Flags().GetString("from")
	envFileFlag, _ := cmd.Flags().GetString("env-file")

	// Determine the target env file path.
	envFile := ".env." + name
	if envFileFlag != "" {
		envFile = envFileFlag
	}

	targetPath := filepath.Join(projectDir, envFile)

	// Check if file already exists.
	if !force {
		if _, statErr := os.Stat(targetPath); statErr == nil {
			return fmt.Errorf("profile file %s already exists (use --force to overwrite)", envFile)
		}
	}

	// Determine file content.
	var content string
	if fromFile != "" {
		fromPath := filepath.Join(projectDir, fromFile)
		data, readErr := os.ReadFile(fromPath)
		if readErr != nil {
			return fmt.Errorf("reading source file %s: %w", fromFile, readErr)
		}
		content = string(data)
	} else {
		content = fmt.Sprintf("# Environment variables for the %q profile\n# Merge order: .env ← .env.%s ← .env.local\n", name, name)
	}

	// Ensure parent directory exists for custom paths.
	parentDir := filepath.Dir(targetPath)
	if parentDir != projectDir {
		if mkErr := os.MkdirAll(parentDir, 0o755); mkErr != nil {
			return fmt.Errorf("creating directory %s: %w", parentDir, mkErr)
		}
	}

	if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", envFile, err)
	}
	w.Info("Created %s\n", envFile)

	// Optionally register in .envref.yaml.
	if register {
		// Only register with custom env_file if it differs from convention.
		registerEnvFile := ""
		if envFileFlag != "" {
			registerEnvFile = envFileFlag
		}

		configPath := filepath.Join(projectDir, config.FullFileName)
		if addErr := config.AddProfile(configPath, name, registerEnvFile); addErr != nil {
			// If the profile already exists in config, warn but don't fail.
			if cfg.HasProfile(name) {
				w.Warn("Profile %q already registered in config\n", name)
			} else {
				return fmt.Errorf("registering profile in config: %w", addErr)
			}
		} else {
			w.Info("Registered profile %q in %s\n", name, config.FullFileName)
		}
	}

	return nil
}

// runProfileUse implements the profile use command logic.
func runProfileUse(cmd *cobra.Command, name string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, projectDir, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	w := output.NewWriter(cmd)

	// If clearing the active profile, just update and return.
	if name == "" {
		configPath := filepath.Join(projectDir, config.FullFileName)
		if err := config.SetActiveProfile(configPath, ""); err != nil {
			return fmt.Errorf("updating config: %w", err)
		}
		w.Info("Cleared active profile\n")
		return nil
	}

	// Validate that the profile exists (in config or on disk).
	if !cfg.HasProfile(name) {
		envFile := ".env." + name
		diskPath := filepath.Join(projectDir, envFile)
		if _, statErr := os.Stat(diskPath); statErr != nil {
			return fmt.Errorf("profile %q not found (not in config and %s does not exist)", name, envFile)
		}
	}

	configPath := filepath.Join(projectDir, config.FullFileName)
	if err := config.SetActiveProfile(configPath, name); err != nil {
		return fmt.Errorf("updating config: %w", err)
	}

	w.Info("Active profile set to %q\n", name)
	return nil
}

// profileInfo holds information about a discovered profile.
type profileInfo struct {
	Name     string
	EnvFile  string
	InConfig bool
	OnDisk   bool
	Active   bool
}

// runProfileList implements the profile list command logic.
func runProfileList(cmd *cobra.Command) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	cfg, projectDir, err := config.Load(cwd)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	activeProfile := cfg.EffectiveProfile("")

	// Collect profiles from config.
	profiles := make(map[string]*profileInfo)
	for name := range cfg.Profiles {
		envFile := cfg.ProfileEnvFile(name)
		_, diskErr := os.Stat(filepath.Join(projectDir, envFile))
		profiles[name] = &profileInfo{
			Name:     name,
			EnvFile:  envFile,
			InConfig: true,
			OnDisk:   diskErr == nil,
			Active:   name == activeProfile,
		}
	}

	// Discover convention-based .env.* files on disk.
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return fmt.Errorf("reading project directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, ".env.") {
			continue
		}
		profileName := strings.TrimPrefix(name, ".env.")
		// Skip .env.local — it's the local override, not a profile.
		if profileName == "local" {
			continue
		}
		// Skip names with additional dots (e.g., .env.local.bak).
		if strings.Contains(profileName, ".") {
			continue
		}
		if profileName == "" {
			continue
		}
		if _, exists := profiles[profileName]; !exists {
			profiles[profileName] = &profileInfo{
				Name:     profileName,
				EnvFile:  name,
				InConfig: false,
				OnDisk:   true,
				Active:   profileName == activeProfile,
			}
		}
	}

	if len(profiles) == 0 {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "no profiles found")
		return nil
	}

	// Sort profiles by name for stable output.
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)

	out := cmd.OutOrStdout()
	for _, name := range names {
		p := profiles[name]
		marker := "  "
		if p.Active {
			marker = "* "
		}

		var status []string
		if p.InConfig {
			status = append(status, "config")
		}
		if p.OnDisk {
			status = append(status, "file")
		} else {
			status = append(status, "no file")
		}

		_, _ = fmt.Fprintf(out, "%s%-20s %s (%s)\n", marker, p.Name, p.EnvFile, strings.Join(status, ", "))
	}

	return nil
}

// newProfileDiffCmd creates the profile diff subcommand.
func newProfileDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <profile_a> <profile_b>",
		Short: "Show differences between two profiles",
		Long: `Compare the environment variables between two profiles and show differences.

For each profile, the effective environment is computed by merging:
  .env ← .env.<profile> ← .env.local

The diff shows:
  - Keys only in the first profile
  - Keys only in the second profile
  - Keys present in both but with different values

Examples:
  envref profile diff staging production           # compare staging vs production
  envref profile diff development staging --format json  # output as JSON
  envref profile diff staging production --format table  # output as table`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatStr, _ := cmd.Flags().GetString("format")
			return runProfileDiff(cmd, args[0], args[1], formatStr)
		},
	}

	cmd.Flags().String("format", "plain", "output format: plain, json, table")

	return cmd
}

// diffEntry represents a single difference between two profiles.
type diffEntry struct {
	Key    string `json:"key"`
	Kind   string `json:"kind"`   // "only_a", "only_b", "changed"
	ValueA string `json:"value_a,omitempty"`
	ValueB string `json:"value_b,omitempty"`
}

// runProfileDiff implements the profile diff command logic.
func runProfileDiff(cmd *cobra.Command, profileA, profileB, formatStr string) error {
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

	w := output.NewWriter(cmd)

	// Load effective env for each profile.
	envA, err := loadProfileEnv(cmd, cfg, projectDir, profileA)
	if err != nil {
		return fmt.Errorf("loading profile %q: %w", profileA, err)
	}

	envB, err := loadProfileEnv(cmd, cfg, projectDir, profileB)
	if err != nil {
		return fmt.Errorf("loading profile %q: %w", profileB, err)
	}

	// Compute diff.
	diffs := computeProfileDiff(envA, envB)

	if len(diffs) == 0 {
		if !w.IsQuiet() {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Profiles %q and %q are identical (%d keys)\n", profileA, profileB, envA.Len())
		}
		return nil
	}

	return formatDiff(cmd, diffs, profileA, profileB, format, w)
}

// loadProfileEnv loads the effective merged environment for a given profile.
func loadProfileEnv(cmd *cobra.Command, cfg *config.Config, projectDir, profile string) (*envfile.Env, error) {
	envPath := resolveFilePath(projectDir, cfg.EnvFile)
	localPath := resolveFilePath(projectDir, cfg.LocalFile)
	profilePath := resolveFilePath(projectDir, cfg.ProfileEnvFile(profile))

	return loadAndMergeEnv(cmd, envPath, profilePath, localPath)
}

// computeProfileDiff compares two Envs and returns a sorted list of differences.
func computeProfileDiff(envA, envB *envfile.Env) []diffEntry {
	keysA := keySet(envA.Keys())
	keysB := keySet(envB.Keys())

	var diffs []diffEntry

	// Keys only in A.
	for key := range keysA {
		if _, ok := keysB[key]; !ok {
			entryA, _ := envA.Get(key)
			diffs = append(diffs, diffEntry{
				Key:    key,
				Kind:   "only_a",
				ValueA: entryA.Value,
			})
		}
	}

	// Keys only in B.
	for key := range keysB {
		if _, ok := keysA[key]; !ok {
			entryB, _ := envB.Get(key)
			diffs = append(diffs, diffEntry{
				Key:    key,
				Kind:   "only_b",
				ValueB: entryB.Value,
			})
		}
	}

	// Keys in both with different values.
	for key := range keysA {
		if _, ok := keysB[key]; !ok {
			continue
		}
		entryA, _ := envA.Get(key)
		entryB, _ := envB.Get(key)
		if entryA.Value != entryB.Value {
			diffs = append(diffs, diffEntry{
				Key:    key,
				Kind:   "changed",
				ValueA: entryA.Value,
				ValueB: entryB.Value,
			})
		}
	}

	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Key < diffs[j].Key
	})

	return diffs
}

// formatDiff writes the diff entries to stdout in the specified format.
func formatDiff(cmd *cobra.Command, diffs []diffEntry, profileA, profileB string, format OutputFormat, w *output.Writer) error {
	out := cmd.OutOrStdout()

	switch format {
	case FormatJSON:
		return formatDiffJSON(out, diffs)
	case FormatTable:
		return formatDiffTable(out, diffs, profileA, profileB, w)
	default:
		return formatDiffPlain(out, diffs, profileA, profileB, w)
	}
}

// formatDiffPlain outputs a human-readable diff with color indicators.
func formatDiffPlain(out io.Writer, diffs []diffEntry, profileA, profileB string, w *output.Writer) error {
	for _, d := range diffs {
		switch d.Kind {
		case "only_a":
			_, _ = fmt.Fprintf(out, "%s %s=%s\n", w.Red("-"), d.Key, d.ValueA)
		case "only_b":
			_, _ = fmt.Fprintf(out, "%s %s=%s\n", w.Green("+"), d.Key, d.ValueB)
		case "changed":
			_, _ = fmt.Fprintf(out, "%s %s=%s\n", w.Yellow("~"), d.Key, d.ValueA)
			_, _ = fmt.Fprintf(out, "%s %s=%s\n", w.Yellow("~"), d.Key, d.ValueB)
		}
	}

	// Summary.
	var onlyA, onlyB, changed int
	for _, d := range diffs {
		switch d.Kind {
		case "only_a":
			onlyA++
		case "only_b":
			onlyB++
		case "changed":
			changed++
		}
	}

	var parts []string
	if onlyA > 0 {
		parts = append(parts, fmt.Sprintf("%d only in %s", onlyA, profileA))
	}
	if onlyB > 0 {
		parts = append(parts, fmt.Sprintf("%d only in %s", onlyB, profileB))
	}
	if changed > 0 {
		parts = append(parts, fmt.Sprintf("%d changed", changed))
	}
	_, _ = fmt.Fprintf(out, "\n%d difference(s): %s\n", len(diffs), strings.Join(parts, ", "))

	return nil
}

// formatDiffJSON outputs the diff as a JSON array.
func formatDiffJSON(out io.Writer, diffs []diffEntry) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(diffs)
}

// newProfileExportCmd creates the profile export subcommand.
func newProfileExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export <name>",
		Short: "Export a profile as key-value pairs",
		Long: `Export the effective environment of a profile for CI/CD integration.

Computes the merged environment (base .env ← .env.<profile> ← .env.local)
with variable interpolation applied, then outputs the key-value pairs.

The default output format is JSON (an array of {"key": ..., "value": ...}
objects), which is easy to consume in CI/CD pipelines. Use --format to
change the output format (plain, json, shell, table).

Examples:
  envref profile export staging                   # export as JSON
  envref profile export production --format shell # export as shell export lines
  envref profile export staging --format plain    # export as KEY=VALUE pairs
  envref profile export staging > staging.json    # redirect to file`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatStr, _ := cmd.Flags().GetString("format")
			return runProfileExport(cmd, args[0], formatStr)
		},
	}

	cmd.Flags().String("format", "json", "output format: plain, json, shell, table")

	return cmd
}

// runProfileExport implements the profile export command logic.
func runProfileExport(cmd *cobra.Command, name, formatStr string) error {
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

	env, err := loadProfileEnv(cmd, cfg, projectDir, name)
	if err != nil {
		return fmt.Errorf("loading profile %q: %w", name, err)
	}

	entries := envToEntries(env)
	return outputEntries(cmd, entries, format)
}

// formatDiffTable outputs the diff as an aligned table.
func formatDiffTable(out io.Writer, diffs []diffEntry, profileA, profileB string, w *output.Writer) error {
	// Find column widths.
	maxKey := len("KEY")
	maxA := len(profileA)
	maxB := len(profileB)
	for _, d := range diffs {
		if len(d.Key) > maxKey {
			maxKey = len(d.Key)
		}
		if len(d.ValueA) > maxA {
			maxA = len(d.ValueA)
		}
		if len(d.ValueB) > maxB {
			maxB = len(d.ValueB)
		}
	}

	// Header.
	_, _ = fmt.Fprintf(out, "%-*s  %-*s  %-*s  %s\n",
		4, "DIFF", maxKey, "KEY", maxA, profileA, profileB)
	_, _ = fmt.Fprintf(out, "%-*s  %-*s  %-*s  %s\n",
		4, strings.Repeat("-", 4), maxKey, strings.Repeat("-", maxKey),
		maxA, strings.Repeat("-", maxA), strings.Repeat("-", maxB))

	for _, d := range diffs {
		var marker, valA, valB string
		switch d.Kind {
		case "only_a":
			marker = w.Red("-")
			valA = d.ValueA
			valB = ""
		case "only_b":
			marker = w.Green("+")
			valA = ""
			valB = d.ValueB
		case "changed":
			marker = w.Yellow("~")
			valA = d.ValueA
			valB = d.ValueB
		}
		_, _ = fmt.Fprintf(out, "%-*s  %-*s  %-*s  %s\n",
			4, marker, maxKey, d.Key, maxA, valA, valB)
	}

	return nil
}
