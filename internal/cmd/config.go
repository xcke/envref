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
)

// newConfigCmd creates the config command group.
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage envref configuration",
		Long: `View and manage envref configuration.

The effective configuration is the result of merging the global config
(~/.config/envref/config.yaml) with the project-level .envref.yaml.`,
	}

	cmd.AddCommand(newConfigShowCmd())

	return cmd
}

// newConfigShowCmd creates the config show subcommand.
func newConfigShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Print the resolved effective configuration",
		Long: `Print the effective configuration after merging global defaults with
the project-level .envref.yaml.

Shows all configuration fields including project name, file paths,
active profile, backends, and profiles. Indicates which config files
were loaded and any warnings about the configuration.

Output format can be specified with --format (plain, json, table).

Examples:
  envref config show                 # print effective config
  envref config show --format json   # output as JSON
  envref config show --format table  # output as aligned table`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			formatStr, _ := cmd.Flags().GetString("format")
			return runConfigShow(cmd, formatStr)
		},
	}

	cmd.Flags().String("format", "plain", "output format: plain, json, table")

	return cmd
}

// configShowOutput is the structure used for JSON output of config show.
type configShowOutput struct {
	Project       string                `json:"project"`
	EnvFile       string                `json:"env_file"`
	LocalFile     string                `json:"local_file"`
	ActiveProfile string                `json:"active_profile,omitempty"`
	Backends      []configBackendOutput `json:"backends,omitempty"`
	Profiles      map[string]string     `json:"profiles,omitempty"`
	ConfigFile    string                `json:"config_file"`
	GlobalConfig  string                `json:"global_config,omitempty"`
}

// configBackendOutput represents a backend in JSON output.
type configBackendOutput struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Config map[string]string `json:"config,omitempty"`
}

// runConfigShow implements the config show command logic.
func runConfigShow(cmd *cobra.Command, formatStr string) error {
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

	out := cmd.OutOrStdout()

	switch format {
	case FormatJSON:
		return printConfigJSON(out, cfg, projectDir)
	case FormatTable:
		return printConfigTable(out, cfg, projectDir)
	default:
		return printConfigPlain(out, cfg, projectDir)
	}
}

// printConfigJSON outputs the effective config as JSON.
func printConfigJSON(w io.Writer, cfg *config.Config, projectDir string) error {
	output := configShowOutput{
		Project:       cfg.Project,
		EnvFile:       cfg.EnvFile,
		LocalFile:     cfg.LocalFile,
		ActiveProfile: cfg.ActiveProfile,
		ConfigFile:    filepath.Join(projectDir, config.FullFileName),
	}

	if globalPath := config.GlobalConfigPath(); globalPath != "" {
		if _, err := os.Stat(globalPath); err == nil {
			output.GlobalConfig = globalPath
		}
	}

	for _, b := range cfg.Backends {
		output.Backends = append(output.Backends, configBackendOutput{
			Name:   b.Name,
			Type:   b.EffectiveType(),
			Config: b.Config,
		})
	}

	if len(cfg.Profiles) > 0 {
		output.Profiles = make(map[string]string, len(cfg.Profiles))
		for name := range cfg.Profiles {
			output.Profiles[name] = cfg.ProfileEnvFile(name)
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

// printConfigPlain outputs the effective config in a human-readable format.
func printConfigPlain(w io.Writer, cfg *config.Config, projectDir string) error {
	write := func(format string, args ...interface{}) {
		_, _ = fmt.Fprintf(w, format, args...)
	}

	write("Project: %s\n", cfg.Project)
	write("EnvFile: %s\n", cfg.EnvFile)
	write("LocalFile: %s\n", cfg.LocalFile)

	if cfg.ActiveProfile != "" {
		write("ActiveProfile: %s\n", cfg.ActiveProfile)
	}

	// Backends.
	if len(cfg.Backends) > 0 {
		write("\nBackends:\n")
		for _, b := range cfg.Backends {
			write("  - %s (type: %s)\n", b.Name, b.EffectiveType())
			if len(b.Config) > 0 {
				keys := sortedKeys(b.Config)
				for _, k := range keys {
					write("    %s: %s\n", k, b.Config[k])
				}
			}
		}
	}

	// Profiles.
	if len(cfg.Profiles) > 0 {
		write("\nProfiles:\n")
		names := sortedProfileNames(cfg.Profiles)
		for _, name := range names {
			envFile := cfg.ProfileEnvFile(name)
			active := ""
			if name == cfg.ActiveProfile {
				active = " (active)"
			}
			write("  - %s -> %s%s\n", name, envFile, active)
		}
	}

	// Config file locations.
	write("\nConfig: %s\n", filepath.Join(projectDir, config.FullFileName))
	if globalPath := config.GlobalConfigPath(); globalPath != "" {
		if _, err := os.Stat(globalPath); err == nil {
			write("Global: %s\n", globalPath)
		}
	}

	// Warnings.
	if warnings := cfg.Warnings(); len(warnings) > 0 {
		write("\nWarnings:\n")
		for _, w := range warnings {
			write("  - %s\n", w)
		}
	}

	return nil
}

// printConfigTable outputs the effective config as an aligned table.
func printConfigTable(w io.Writer, cfg *config.Config, projectDir string) error {
	pairs := []kvPair{
		{Key: "project", Value: cfg.Project},
		{Key: "env_file", Value: cfg.EnvFile},
		{Key: "local_file", Value: cfg.LocalFile},
	}

	if cfg.ActiveProfile != "" {
		pairs = append(pairs, kvPair{Key: "active_profile", Value: cfg.ActiveProfile})
	}

	if len(cfg.Backends) > 0 {
		names := make([]string, len(cfg.Backends))
		for i, b := range cfg.Backends {
			names[i] = b.Name
		}
		pairs = append(pairs, kvPair{Key: "backends", Value: strings.Join(names, ", ")})
	}

	if len(cfg.Profiles) > 0 {
		names := sortedProfileNames(cfg.Profiles)
		pairs = append(pairs, kvPair{Key: "profiles", Value: strings.Join(names, ", ")})
	}

	pairs = append(pairs, kvPair{Key: "config_file", Value: filepath.Join(projectDir, config.FullFileName)})

	if globalPath := config.GlobalConfigPath(); globalPath != "" {
		if _, err := os.Stat(globalPath); err == nil {
			pairs = append(pairs, kvPair{Key: "global_config", Value: globalPath})
		}
	}

	return formatKVTable(w, pairs)
}

// sortedKeys returns the keys of a map sorted alphabetically.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sortedProfileNames returns profile names sorted alphabetically.
func sortedProfileNames(profiles map[string]config.ProfileConfig) []string {
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
