// Package config defines the .envref.yaml schema and provides functions for
// loading project configuration using Viper.
//
// The configuration file is searched for in the current directory and parent
// directories (project root discovery). A global config at
// ~/.config/envref/config.yaml provides defaults that project-level config
// can override.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/viper"
)

// KnownBackendTypes lists the backend types that envref recognizes. This is
// used for validation warnings — unknown types are flagged but not rejected,
// since future backends may be added via plugins.
var KnownBackendTypes = []string{
	"keychain",
}

// ValidationError is returned when the config is syntactically valid YAML but
// contains semantic errors (missing required fields, invalid values, etc.).
// Callers can use errors.As to distinguish validation failures from I/O errors
// or missing config files.
type ValidationError struct {
	// Problems lists the individual validation issues found.
	Problems []string
}

// Error returns a human-readable summary of all validation problems.
func (e *ValidationError) Error() string {
	return "invalid config: " + strings.Join(e.Problems, "; ")
}

// FileName is the name of the project-level configuration file (without extension).
const FileName = ".envref"

// FileExt is the expected file extension for the configuration file.
const FileExt = "yaml"

// FullFileName is the complete config file name including extension.
const FullFileName = FileName + "." + FileExt

// GlobalFileName is the name of the global config file.
const GlobalFileName = "config.yaml"

// GlobalConfigDir returns the directory for global envref configuration.
// On Unix-like systems this is $XDG_CONFIG_HOME/envref (defaulting to
// ~/.config/envref). On Windows this is %APPDATA%/envref.
func GlobalConfigDir() string {
	if dir := os.Getenv("ENVREF_CONFIG_DIR"); dir != "" {
		return dir
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "envref")
	}
	if runtime.GOOS == "windows" {
		if dir := os.Getenv("APPDATA"); dir != "" {
			return filepath.Join(dir, "envref")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "envref")
}

// GlobalConfigPath returns the full path to the global config file.
// Returns an empty string if the home directory cannot be determined.
func GlobalConfigPath() string {
	dir := GlobalConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, GlobalFileName)
}

// loadGlobalConfig attempts to load the global config file. Returns nil
// (not an error) if the file does not exist. Only returns an error if the
// file exists but cannot be parsed.
func loadGlobalConfig() (*Config, error) {
	path := GlobalConfigPath()
	if path == "" {
		return nil, nil
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	cfg, err := loadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading global config: %w", err)
	}
	return cfg, nil
}

// mergeConfigs merges a global config with a project config. Project values
// take precedence over global values. For scalar fields, a non-zero project
// value overrides the global value. For slices and maps (Backends, Profiles),
// the project config replaces the global config entirely when present.
func mergeConfigs(global, project *Config) *Config {
	if global == nil {
		return project
	}
	if project == nil {
		return global
	}

	merged := *project

	// Scalar fields: use project value if non-empty, otherwise fall back to global.
	if merged.Project == "" {
		merged.Project = global.Project
	}
	if merged.EnvFile == "" || merged.EnvFile == ".env" {
		// Only inherit if project didn't explicitly set env_file.
		// Since ".env" is the Viper default, we need a way to distinguish
		// "explicitly set to .env" from "not set at all". We treat the Viper
		// default as "not set" and let global override in that case.
		// However, if global also has the default, no change is needed.
		if global.EnvFile != "" && global.EnvFile != ".env" {
			merged.EnvFile = global.EnvFile
		}
	}
	if merged.LocalFile == "" || merged.LocalFile == ".env.local" {
		if global.LocalFile != "" && global.LocalFile != ".env.local" {
			merged.LocalFile = global.LocalFile
		}
	}
	if merged.ActiveProfile == "" {
		merged.ActiveProfile = global.ActiveProfile
	}

	// Backends: project replaces entirely if present, otherwise inherit global.
	if len(merged.Backends) == 0 && len(global.Backends) > 0 {
		merged.Backends = make([]BackendConfig, len(global.Backends))
		copy(merged.Backends, global.Backends)
	}

	// Profiles: project replaces entirely if present, otherwise inherit global.
	if len(merged.Profiles) == 0 && len(global.Profiles) > 0 {
		merged.Profiles = make(map[string]ProfileConfig, len(global.Profiles))
		for k, v := range global.Profiles {
			merged.Profiles[k] = v
		}
	}

	return &merged
}

// Config represents the complete .envref.yaml configuration.
type Config struct {
	// Project is the project name, used as a namespace for secrets.
	Project string `mapstructure:"project" yaml:"project"`

	// EnvFile is the path to the primary .env file (default ".env").
	EnvFile string `mapstructure:"env_file" yaml:"env_file"`

	// LocalFile is the path to the local override file (default ".env.local").
	LocalFile string `mapstructure:"local_file" yaml:"local_file"`

	// ActiveProfile is the name of the currently active profile (e.g., "staging").
	// When set, the resolve pipeline loads .env ← .env.<profile> ← .env.local.
	// Can be overridden at runtime with the --profile flag.
	ActiveProfile string `mapstructure:"active_profile" yaml:"active_profile"`

	// Backends defines the ordered list of secret backends to try when
	// resolving ref:// references. Backends are tried in order; the first
	// one that returns a value wins.
	Backends []BackendConfig `mapstructure:"backends" yaml:"backends"`

	// Profiles defines named environment profiles (e.g., development, staging).
	Profiles map[string]ProfileConfig `mapstructure:"profiles" yaml:"profiles"`
}

// BackendConfig describes a single secret backend.
type BackendConfig struct {
	// Name is the identifier for this backend (e.g., "keychain", "vault", "op").
	Name string `mapstructure:"name" yaml:"name"`

	// Type is the backend type (e.g., "keychain", "encrypted-vault",
	// "1password", "aws-ssm"). If empty, defaults to the value of Name.
	Type string `mapstructure:"type" yaml:"type"`

	// Config holds backend-specific configuration key-value pairs.
	Config map[string]string `mapstructure:"config" yaml:"config"`
}

// ProfileConfig describes a named environment profile.
type ProfileConfig struct {
	// EnvFile is the path to the profile-specific .env file
	// (e.g., ".env.staging"). If empty, defaults to ".env.<profile-name>".
	EnvFile string `mapstructure:"env_file" yaml:"env_file"`
}

// Defaults returns a Config populated with default values.
func Defaults() Config {
	return Config{
		EnvFile:   ".env",
		LocalFile: ".env.local",
	}
}

// EffectiveType returns the backend type, falling back to Name if Type is empty.
func (b BackendConfig) EffectiveType() string {
	if b.Type != "" {
		return b.Type
	}
	return b.Name
}

// ProfileEnvFile returns the env file path for the given profile name.
// If the profile is defined in the Profiles map and has a custom EnvFile,
// that value is returned. Otherwise, the default convention ".env.<name>"
// is used (e.g., ".env.staging", ".env.production").
func (c *Config) ProfileEnvFile(profile string) string {
	if p, ok := c.Profiles[profile]; ok && p.EnvFile != "" {
		return p.EnvFile
	}
	return ".env." + profile
}

// HasProfile reports whether the given profile name is defined in the
// Profiles map. An empty profile name always returns false.
func (c *Config) HasProfile(profile string) bool {
	if profile == "" {
		return false
	}
	_, ok := c.Profiles[profile]
	return ok
}

// EffectiveProfile returns the profile to use, preferring the override
// (e.g., from --profile flag) over the config's ActiveProfile.
// Returns empty string if no profile is active.
func (c *Config) EffectiveProfile(override string) string {
	if override != "" {
		return override
	}
	return c.ActiveProfile
}

// Validate checks that the config is well-formed and returns a
// *ValidationError describing any problems found. Returns nil if valid.
func (c *Config) Validate() error {
	var errs []string

	// Project name checks.
	if c.Project == "" {
		errs = append(errs, "project name is required")
	} else if strings.TrimSpace(c.Project) != c.Project {
		errs = append(errs, "project name must not have leading or trailing whitespace")
	} else if strings.ContainsAny(c.Project, "/\\") {
		errs = append(errs, "project name must not contain path separators (/ or \\)")
	}

	// File path checks.
	if c.EnvFile == "" {
		errs = append(errs, "env_file must not be empty")
	} else if filepath.IsAbs(c.EnvFile) {
		errs = append(errs, "env_file must be a relative path, got absolute path")
	}

	if c.LocalFile == "" {
		errs = append(errs, "local_file must not be empty")
	} else if filepath.IsAbs(c.LocalFile) {
		errs = append(errs, "local_file must be a relative path, got absolute path")
	}

	// Validate backends.
	seenBackends := make(map[string]bool)
	for i, b := range c.Backends {
		if b.Name == "" {
			errs = append(errs, fmt.Sprintf("backends[%d]: name is required", i))
			continue
		}
		if seenBackends[b.Name] {
			errs = append(errs, fmt.Sprintf("backends[%d]: duplicate backend name %q", i, b.Name))
		}
		seenBackends[b.Name] = true
	}

	// Validate profiles.
	for name := range c.Profiles {
		if name == "" {
			errs = append(errs, "profiles: empty profile name is not allowed")
		} else if strings.TrimSpace(name) != name {
			errs = append(errs, fmt.Sprintf("profiles: profile name %q must not have leading or trailing whitespace", name))
		}
	}

	// Validate active_profile references an existing profile (if set and profiles are defined).
	if c.ActiveProfile != "" && len(c.Profiles) > 0 {
		if _, ok := c.Profiles[c.ActiveProfile]; !ok {
			errs = append(errs, fmt.Sprintf("active_profile %q is not defined in profiles", c.ActiveProfile))
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return &ValidationError{Problems: errs}
}

// Warnings returns non-fatal issues with the config, such as unknown backend
// types. Unlike Validate, these do not prevent the config from being used.
func (c *Config) Warnings() []string {
	var warnings []string
	for i, b := range c.Backends {
		btype := b.EffectiveType()
		if btype != "" && !isKnownBackendType(btype) {
			warnings = append(warnings, fmt.Sprintf("backends[%d]: unknown backend type %q (known types: %s)",
				i, btype, strings.Join(KnownBackendTypes, ", ")))
		}
	}
	return warnings
}

// isKnownBackendType reports whether the given type string is in the
// KnownBackendTypes list.
func isKnownBackendType(t string) bool {
	for _, kt := range KnownBackendTypes {
		if kt == t {
			return true
		}
	}
	return false
}

// Load reads the .envref.yaml file starting from the given directory and
// searching upward toward the filesystem root. Returns the parsed config
// and the directory where the config file was found (the project root).
//
// If a global config file exists at ~/.config/envref/config.yaml (or the
// path determined by GlobalConfigPath), it is loaded first as a base. The
// project-level config then overrides global values.
//
// If no project-level config file is found, Load returns ErrNotFound.
func Load(startDir string) (*Config, string, error) {
	configDir, err := findConfigDir(startDir)
	if err != nil {
		return nil, "", err
	}

	projectCfg, err := loadFile(filepath.Join(configDir, FullFileName))
	if err != nil {
		return nil, "", err
	}

	globalCfg, err := loadGlobalConfig()
	if err != nil {
		return nil, "", err
	}

	cfg := mergeConfigs(globalCfg, projectCfg)

	if err := cfg.Validate(); err != nil {
		return nil, "", err
	}

	return cfg, configDir, nil
}

// LoadFile reads a config from a specific file path.
func LoadFile(path string) (*Config, error) {
	return loadFile(path)
}

// ErrNotFound is returned when no .envref.yaml is found in the directory tree.
var ErrNotFound = errors.New("no .envref.yaml found")

// findConfigDir walks from startDir up to the filesystem root looking for
// a .envref.yaml file. Returns the directory containing the file, or
// ErrNotFound if none is found.
func findConfigDir(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("resolving path %s: %w", startDir, err)
	}

	for {
		candidate := filepath.Join(dir, FullFileName)
		if _, err := os.Stat(candidate); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root.
			return "", ErrNotFound
		}
		dir = parent
	}
}

// SetActiveProfile updates the active_profile field in the config file at path.
// It preserves existing file content by performing a targeted line replacement
// rather than re-marshaling the entire config. If the file does not already
// contain an active_profile line, one is inserted after the project line.
// An empty profile name clears the active profile.
func SetActiveProfile(path, profile string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config %s: %w", path, err)
	}

	lines := strings.Split(string(data), "\n")
	found := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "active_profile:") || strings.HasPrefix(trimmed, "active_profile :") {
			if profile == "" {
				// Remove the line entirely.
				lines = append(lines[:i], lines[i+1:]...)
			} else {
				lines[i] = "active_profile: " + profile
			}
			found = true
			break
		}
	}

	if !found && profile != "" {
		// Insert after the project line, or at the top if no project line.
		insertIdx := 0
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "project:") || strings.HasPrefix(trimmed, "project :") {
				insertIdx = i + 1
				break
			}
		}
		newLine := "active_profile: " + profile
		lines = append(lines[:insertIdx+1], lines[insertIdx:]...)
		lines[insertIdx] = newLine
	}

	content := strings.Join(lines, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing config %s: %w", path, err)
	}
	return nil
}

// AddProfile adds a new profile entry to the config file at path.
// It preserves existing file content by performing a targeted insertion
// into the profiles section. If no profiles section exists, one is created.
// Returns an error if the profile already exists.
func AddProfile(path, profile, envFile string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config %s: %w", path, err)
	}

	lines := strings.Split(string(data), "\n")

	// Check if the profile already exists in the config.
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == profile+":" && isInProfilesSection(lines, line) {
			return fmt.Errorf("profile %q already exists in config", profile)
		}
	}

	// Build the new profile entry. Always include env_file so the profile
	// is parseable by Viper (a key with no sub-keys is treated as null).
	if envFile == "" {
		envFile = ".env." + profile
	}
	entry := "  " + profile + ":\n    env_file: " + envFile

	// Find the profiles section and insert the new entry.
	profilesSectionIdx := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "profiles:" {
			profilesSectionIdx = i
			break
		}
	}

	if profilesSectionIdx >= 0 {
		// Find the end of the profiles section (next top-level key or EOF).
		insertIdx := profilesSectionIdx + 1
		for insertIdx < len(lines) {
			line := lines[insertIdx]
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "#") {
				insertIdx++
				continue
			}
			// Reached a new top-level key.
			break
		}

		// Insert before the next top-level key (or at end of section).
		entryLines := strings.Split(entry, "\n")
		newLines := make([]string, 0, len(lines)+len(entryLines))
		newLines = append(newLines, lines[:insertIdx]...)
		newLines = append(newLines, entryLines...)
		newLines = append(newLines, lines[insertIdx:]...)
		lines = newLines
	} else {
		// No profiles section — add one at the end.
		// Find a good insertion point: before trailing empty lines.
		insertIdx := len(lines)
		for insertIdx > 0 && strings.TrimSpace(lines[insertIdx-1]) == "" {
			insertIdx--
		}

		section := "\nprofiles:\n" + entry
		sectionLines := strings.Split(section, "\n")
		newLines := make([]string, 0, len(lines)+len(sectionLines))
		newLines = append(newLines, lines[:insertIdx]...)
		newLines = append(newLines, sectionLines...)
		newLines = append(newLines, lines[insertIdx:]...)
		lines = newLines
	}

	content := strings.Join(lines, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing config %s: %w", path, err)
	}
	return nil
}

// isInProfilesSection checks if the given line appears within the profiles
// section of the config. This is a heuristic: the line must be indented and
// appear after a "profiles:" top-level key.
func isInProfilesSection(lines []string, target string) bool {
	inProfiles := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "profiles:" {
			inProfiles = true
			continue
		}
		if inProfiles {
			// Check if we've left the profiles section (new top-level key).
			if len(line) > 0 && line[0] != ' ' && line[0] != '#' && trimmed != "" {
				inProfiles = false
				continue
			}
			if line == target {
				return true
			}
		}
	}
	return false
}

// loadFile reads and parses a single config file using Viper.
func loadFile(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)

	// Set defaults.
	v.SetDefault("env_file", ".env")
	v.SetDefault("local_file", ".env.local")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	return cfg, nil
}
