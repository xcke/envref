package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xcke/envref/internal/config"
	"github.com/xcke/envref/internal/envfile"
	"github.com/xcke/envref/internal/output"
	"github.com/xcke/envref/internal/ref"
	"github.com/xcke/envref/internal/resolve"
)

// newStatusCmd creates the status subcommand.
func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show environment status overview",
		Long: `Show a summary of the current environment: total keys, config values,
secret references, resolution status, and any issues that need attention.

The status command loads the merged environment (base .env, optional profile,
.env.local), checks which keys are ref:// references, attempts to resolve them
via configured backends, and reports the results.

If a .env.example file exists, missing keys are also reported.

Examples:
  envref status                          # show environment overview
  envref status --profile staging        # show status for staging profile`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			profile, _ := cmd.Flags().GetString("profile")
			return runStatus(cmd, profile)
		},
	}

	cmd.Flags().StringP("profile", "P", "", "environment profile to use (e.g., staging, production)")

	return cmd
}

// statusReport collects all information for the status output.
type statusReport struct {
	// Project and profile info.
	project       string
	activeProfile string
	projectDir    string

	// File existence.
	envFileExists     bool
	localFileExists   bool
	profileFileExists bool
	exampleFileExists bool
	configExists      bool

	// File paths (relative).
	envFilePath     string
	localFilePath   string
	profileFilePath string
	exampleFilePath string

	// Key counts.
	totalKeys  int
	configKeys int
	refKeys    int

	// Resolution results.
	resolvedKeys   int
	unresolvedKeys []string
	resolveErrors  []resolve.KeyErr
	backendsOK     bool
	backendNames   []string

	// Validation results (vs .env.example).
	missingKeys []string
	extraKeys   []string

	// Hints for the user.
	hints []string
}

// runStatus implements the status command logic.
func runStatus(cmd *cobra.Command, profileOverride string) error {
	w := output.NewWriter(cmd)

	report, err := buildStatusReport(cmd, profileOverride)
	if err != nil {
		return err
	}

	printStatusReport(w, report)
	return nil
}

// buildStatusReport gathers all status information.
func buildStatusReport(cmd *cobra.Command, profileOverride string) (*statusReport, error) {
	report := &statusReport{}

	// Try to load config.
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}

	cfg, projectDir, cfgErr := config.Load(cwd)
	if cfgErr != nil {
		var valErr *config.ValidationError
		if errors.As(cfgErr, &valErr) {
			// Config exists but has validation problems — report them as hints.
			report.configExists = true
			for _, p := range valErr.Problems {
				report.hints = append(report.hints, fmt.Sprintf("Config issue: %s", p))
			}
			return report, nil
		}
		// No config found — report minimal status.
		report.configExists = false
		report.hints = append(report.hints, "No .envref.yaml found. Run \"envref init\" to set up your project.")
		return report, nil
	}

	report.configExists = true
	report.project = cfg.Project
	report.projectDir = projectDir

	// Determine active profile.
	profile := cfg.EffectiveProfile(profileOverride)
	report.activeProfile = profile

	// Resolve file paths.
	envPath := resolveFilePath(projectDir, cfg.EnvFile)
	localPath := resolveFilePath(projectDir, cfg.LocalFile)
	report.envFilePath = cfg.EnvFile
	report.localFilePath = cfg.LocalFile

	var profilePath string
	if profile != "" {
		profileEnvFile := cfg.ProfileEnvFile(profile)
		profilePath = resolveFilePath(projectDir, profileEnvFile)
		report.profileFilePath = profileEnvFile
	}

	// Check file existence.
	report.envFileExists = fileExists(envPath)
	report.localFileExists = fileExists(localPath)
	if profilePath != "" {
		report.profileFileExists = fileExists(profilePath)
	}

	examplePath := resolveFilePath(projectDir, ".env.example")
	report.exampleFilePath = ".env.example"
	report.exampleFileExists = fileExists(examplePath)

	// Load env files — if the base .env doesn't exist, we can't continue.
	if !report.envFileExists {
		report.hints = append(report.hints, fmt.Sprintf("No %s file found. Run \"envref init\" to create one.", cfg.EnvFile))
		return report, nil
	}

	env, err := loadAndMergeEnv(cmd, envPath, profilePath, localPath)
	if err != nil {
		return nil, err
	}

	// Count keys by type.
	report.totalKeys = env.Len()
	for _, entry := range env.All() {
		if entry.IsRef {
			report.refKeys++
		} else {
			report.configKeys++
		}
	}

	// Backend info.
	report.backendNames = make([]string, 0, len(cfg.Backends))
	for _, bc := range cfg.Backends {
		report.backendNames = append(report.backendNames, bc.Name)
	}

	// Attempt ref resolution if there are refs and backends are configured.
	if report.refKeys > 0 {
		if len(cfg.Backends) == 0 {
			report.backendsOK = false
			report.unresolvedKeys = collectRefKeys(env)
			report.hints = append(report.hints, "ref:// references found but no backends configured. Add backends to .envref.yaml.")
		} else {
			registry, regErr := buildRegistry(cfg)
			if regErr != nil {
				report.backendsOK = false
				report.hints = append(report.hints, fmt.Sprintf("Failed to initialize backends: %v", regErr))
				report.unresolvedKeys = collectRefKeys(env)
			} else {
				report.backendsOK = true
				result, resolveErr := resolve.Resolve(env, registry, cfg.Project)
				if resolveErr != nil {
					report.hints = append(report.hints, fmt.Sprintf("Resolution failed: %v", resolveErr))
					report.unresolvedKeys = collectRefKeys(env)
				} else {
					report.resolveErrors = result.Errors
					report.resolvedKeys = report.refKeys - len(result.Errors)
					for _, keyErr := range result.Errors {
						report.unresolvedKeys = append(report.unresolvedKeys, keyErr.Key)
					}
				}
			}
		}
	}

	// Check against .env.example if it exists.
	if report.exampleFileExists {
		example, _, exErr := envfile.Load(examplePath)
		if exErr == nil {
			exampleKeys := keySet(example.Keys())
			envKeys := keySet(env.Keys())

			for key := range exampleKeys {
				if _, ok := envKeys[key]; !ok {
					report.missingKeys = append(report.missingKeys, key)
				}
			}
			sort.Strings(report.missingKeys)

			for key := range envKeys {
				if _, ok := exampleKeys[key]; !ok {
					report.extraKeys = append(report.extraKeys, key)
				}
			}
			sort.Strings(report.extraKeys)
		}
	}

	// Generate hints.
	if len(report.missingKeys) > 0 {
		report.hints = append(report.hints, fmt.Sprintf("%d key(s) in .env.example are missing from your environment. Run \"envref validate\" for details.", len(report.missingKeys)))
	}

	if len(report.unresolvedKeys) > 0 && report.backendsOK {
		for _, keyErr := range report.resolveErrors {
			parsed, parseErr := ref.Parse(keyErr.Ref)
			if parseErr != nil {
				report.hints = append(report.hints, fmt.Sprintf("Set missing secret: envref secret set %s", keyErr.Key))
			} else {
				report.hints = append(report.hints, fmt.Sprintf("Set missing secret: envref secret set %s  (backend: %s)", parsed.Path, parsed.Backend))
			}
		}
	}

	if profile != "" && !report.profileFileExists && report.profileFilePath != "" {
		report.hints = append(report.hints, fmt.Sprintf("Profile %q is active but %s does not exist.", profile, report.profileFilePath))
	}

	if !report.exampleFileExists {
		report.hints = append(report.hints, "No .env.example found. Consider creating one as a schema for required keys.")
	}

	return report, nil
}

// printStatusReport formats and prints the status report.
func printStatusReport(w *output.Writer, report *statusReport) {
	out := w.Stdout()
	write := func(format string, args ...interface{}) {
		_, _ = fmt.Fprintf(out, format, args...)
	}

	if !report.configExists {
		write("No project configured.\n")
		if len(report.hints) > 0 {
			write("\n%s\n", w.Bold("Hints:"))
			for _, hint := range report.hints {
				write("  - %s\n", w.Yellow(hint))
			}
		}
		return
	}

	// Project header.
	write("%s %s\n", w.Bold("Project:"), report.project)

	if report.activeProfile != "" {
		write("%s %s\n", w.Bold("Profile:"), report.activeProfile)
	}

	// Files section.
	write("\n%s\n", w.Bold("Files:"))
	write("  %s %s\n", statusIcon(w, report.envFileExists), report.envFilePath)
	if report.profileFilePath != "" {
		write("  %s %s\n", statusIcon(w, report.profileFileExists), report.profileFilePath)
	}
	write("  %s %s\n", statusIcon(w, report.localFileExists), report.localFilePath)
	write("  %s %s\n", statusIcon(w, report.exampleFileExists), report.exampleFilePath)

	if !report.envFileExists {
		if len(report.hints) > 0 {
			write("\n%s\n", w.Bold("Hints:"))
			for _, hint := range report.hints {
				write("  - %s\n", w.Yellow(hint))
			}
		}
		return
	}

	// Environment summary.
	write("\n%s %d keys", w.Bold("Environment:"), report.totalKeys)
	if report.totalKeys > 0 {
		parts := []string{}
		if report.configKeys > 0 {
			parts = append(parts, fmt.Sprintf("%d config", report.configKeys))
		}
		if report.refKeys > 0 {
			parts = append(parts, fmt.Sprintf("%d secrets", report.refKeys))
		}
		write(" (%s)", strings.Join(parts, ", "))
	}
	write("\n")

	// Secrets resolution.
	if report.refKeys > 0 {
		write("\n%s\n", w.Bold("Secrets:"))
		if len(report.backendNames) > 0 {
			write("  Backends: %s\n", strings.Join(report.backendNames, ", "))
		} else {
			write("  Backends: %s\n", w.Yellow("(none configured)"))
		}
		write("  Resolved: %d/%d\n", report.resolvedKeys, report.refKeys)
		if len(report.unresolvedKeys) > 0 {
			write("  Missing:  %s\n", w.Red(strings.Join(report.unresolvedKeys, ", ")))
		}
	}

	// Validation summary.
	if report.exampleFileExists {
		write("\n%s\n", w.Bold("Validation:"))
		if len(report.missingKeys) == 0 && len(report.extraKeys) == 0 {
			write("  %s: all keys match %s\n", w.Green("OK"), report.exampleFilePath)
		} else {
			if len(report.missingKeys) > 0 {
				write("  %s %s: %s\n", w.Red("Missing from"), report.exampleFilePath, strings.Join(report.missingKeys, ", "))
			}
			if len(report.extraKeys) > 0 {
				write("  %s %s: %s\n", w.Yellow("Extra (not in"), report.exampleFilePath+")", strings.Join(report.extraKeys, ", "))
			}
		}
	}

	// Hints.
	if len(report.hints) > 0 {
		write("\n%s\n", w.Bold("Hints:"))
		for _, hint := range report.hints {
			write("  - %s\n", w.Yellow(hint))
		}
	}

	// Overall status.
	if len(report.unresolvedKeys) == 0 && len(report.missingKeys) == 0 {
		write("\nStatus: %s\n", w.Green("OK"))
	} else {
		issues := 0
		issues += len(report.unresolvedKeys)
		issues += len(report.missingKeys)
		write("\nStatus: %s\n", w.Red(fmt.Sprintf("%d issue(s) found", issues)))
	}
}

// statusIcon returns a colored check or cross indicator for file existence.
func statusIcon(w *output.Writer, exists bool) string {
	if exists {
		return w.Green("[ok]")
	}
	return w.Yellow("[--]")
}

// collectRefKeys returns the keys of all ref:// entries in the env.
func collectRefKeys(env *envfile.Env) []string {
	refs := env.Refs()
	keys := make([]string, len(refs))
	for i, r := range refs {
		keys[i] = r.Key
	}
	return keys
}

// fileExists reports whether a file exists at the given path.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
