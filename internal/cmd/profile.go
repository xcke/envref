package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/config"
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
