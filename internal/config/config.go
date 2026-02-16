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
	"strings"

	"github.com/spf13/viper"
)

// FileName is the name of the project-level configuration file (without extension).
const FileName = ".envref"

// FileExt is the expected file extension for the configuration file.
const FileExt = "yaml"

// FullFileName is the complete config file name including extension.
const FullFileName = FileName + "." + FileExt

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

// Validate checks that the config is well-formed and returns an error
// describing any problems found.
func (c *Config) Validate() error {
	var errs []string

	if c.Project == "" {
		errs = append(errs, "project name is required")
	}

	if c.EnvFile == "" {
		errs = append(errs, "env_file must not be empty")
	}

	if c.LocalFile == "" {
		errs = append(errs, "local_file must not be empty")
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
	return fmt.Errorf("invalid config: %s", strings.Join(errs, "; "))
}

// Load reads the .envref.yaml file starting from the given directory and
// searching upward toward the filesystem root. Returns the parsed config
// and the directory where the config file was found (the project root).
//
// If no config file is found, Load returns ErrNotFound.
func Load(startDir string) (*Config, string, error) {
	configDir, err := findConfigDir(startDir)
	if err != nil {
		return nil, "", err
	}

	cfg, err := loadFile(filepath.Join(configDir, FullFileName))
	if err != nil {
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
