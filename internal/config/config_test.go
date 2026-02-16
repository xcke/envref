package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeFile is a test helper that creates a file with the given content.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.EnvFile != ".env" {
		t.Errorf("EnvFile = %q, want %q", cfg.EnvFile, ".env")
	}
	if cfg.LocalFile != ".env.local" {
		t.Errorf("LocalFile = %q, want %q", cfg.LocalFile, ".env.local")
	}
	if cfg.Project != "" {
		t.Errorf("Project = %q, want empty", cfg.Project)
	}
}

func TestBackendConfig_EffectiveType(t *testing.T) {
	tests := []struct {
		name     string
		backend  BackendConfig
		wantType string
	}{
		{
			name:     "type set explicitly",
			backend:  BackendConfig{Name: "main", Type: "keychain"},
			wantType: "keychain",
		},
		{
			name:     "type falls back to name",
			backend:  BackendConfig{Name: "keychain"},
			wantType: "keychain",
		},
		{
			name:     "both empty",
			backend:  BackendConfig{},
			wantType: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.backend.EffectiveType()
			if got != tt.wantType {
				t.Errorf("EffectiveType() = %q, want %q", got, tt.wantType)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid minimal config",
			config: Config{
				Project:   "myapp",
				EnvFile:   ".env",
				LocalFile: ".env.local",
			},
			wantErr: false,
		},
		{
			name: "valid config with backends",
			config: Config{
				Project:   "myapp",
				EnvFile:   ".env",
				LocalFile: ".env.local",
				Backends: []BackendConfig{
					{Name: "keychain", Type: "keychain"},
					{Name: "vault", Type: "encrypted-vault"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with profiles",
			config: Config{
				Project:   "myapp",
				EnvFile:   ".env",
				LocalFile: ".env.local",
				Profiles: map[string]ProfileConfig{
					"staging":    {EnvFile: ".env.staging"},
					"production": {EnvFile: ".env.production"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing project name",
			config: Config{
				EnvFile:   ".env",
				LocalFile: ".env.local",
			},
			wantErr: true,
			errMsg:  "project name is required",
		},
		{
			name: "empty env_file",
			config: Config{
				Project:   "myapp",
				EnvFile:   "",
				LocalFile: ".env.local",
			},
			wantErr: true,
			errMsg:  "env_file must not be empty",
		},
		{
			name: "empty local_file",
			config: Config{
				Project:   "myapp",
				EnvFile:   ".env",
				LocalFile: "",
			},
			wantErr: true,
			errMsg:  "local_file must not be empty",
		},
		{
			name: "backend missing name",
			config: Config{
				Project:   "myapp",
				EnvFile:   ".env",
				LocalFile: ".env.local",
				Backends:  []BackendConfig{{Type: "keychain"}},
			},
			wantErr: true,
			errMsg:  "backends[0]: name is required",
		},
		{
			name: "duplicate backend names",
			config: Config{
				Project:   "myapp",
				EnvFile:   ".env",
				LocalFile: ".env.local",
				Backends: []BackendConfig{
					{Name: "keychain"},
					{Name: "keychain"},
				},
			},
			wantErr: true,
			errMsg:  "duplicate backend name",
		},
		{
			name: "multiple errors",
			config: Config{
				EnvFile:   "",
				LocalFile: "",
			},
			wantErr: true,
			errMsg:  "project name is required",
		},
		{
			name: "project name with leading whitespace",
			config: Config{
				Project:   " myapp",
				EnvFile:   ".env",
				LocalFile: ".env.local",
			},
			wantErr: true,
			errMsg:  "project name must not have leading or trailing whitespace",
		},
		{
			name: "project name with trailing whitespace",
			config: Config{
				Project:   "myapp ",
				EnvFile:   ".env",
				LocalFile: ".env.local",
			},
			wantErr: true,
			errMsg:  "project name must not have leading or trailing whitespace",
		},
		{
			name: "project name with forward slash",
			config: Config{
				Project:   "my/app",
				EnvFile:   ".env",
				LocalFile: ".env.local",
			},
			wantErr: true,
			errMsg:  "project name must not contain path separators",
		},
		{
			name: "project name with backslash",
			config: Config{
				Project:   "my\\app",
				EnvFile:   ".env",
				LocalFile: ".env.local",
			},
			wantErr: true,
			errMsg:  "project name must not contain path separators",
		},
		{
			name: "absolute env_file path",
			config: Config{
				Project:   "myapp",
				EnvFile:   "/etc/.env",
				LocalFile: ".env.local",
			},
			wantErr: true,
			errMsg:  "env_file must be a relative path",
		},
		{
			name: "absolute local_file path",
			config: Config{
				Project:   "myapp",
				EnvFile:   ".env",
				LocalFile: "/etc/.env.local",
			},
			wantErr: true,
			errMsg:  "local_file must be a relative path",
		},
		{
			name: "profile name with whitespace",
			config: Config{
				Project:   "myapp",
				EnvFile:   ".env",
				LocalFile: ".env.local",
				Profiles: map[string]ProfileConfig{
					" staging ": {EnvFile: ".env.staging"},
				},
			},
			wantErr: true,
			errMsg:  "must not have leading or trailing whitespace",
		},
		{
			name: "relative subdirectory env_file is valid",
			config: Config{
				Project:   "myapp",
				EnvFile:   "config/.env",
				LocalFile: "config/.env.local",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr && tt.errMsg != "" {
				if got := err.Error(); !contains(got, tt.errMsg) {
					t.Errorf("error = %q, want to contain %q", got, tt.errMsg)
				}
			}
		})
	}
}

func TestLoadFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		check   func(t *testing.T, cfg *Config)
		wantErr bool
	}{
		{
			name: "full config",
			content: `project: myapp
env_file: .env
local_file: .env.local
backends:
  - name: keychain
    type: keychain
  - name: vault
    type: encrypted-vault
    config:
      path: ~/.config/envref/vault.db
profiles:
  staging:
    env_file: .env.staging
  production:
    env_file: .env.production
`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if cfg.Project != "myapp" {
					t.Errorf("Project = %q, want %q", cfg.Project, "myapp")
				}
				if cfg.EnvFile != ".env" {
					t.Errorf("EnvFile = %q, want %q", cfg.EnvFile, ".env")
				}
				if cfg.LocalFile != ".env.local" {
					t.Errorf("LocalFile = %q, want %q", cfg.LocalFile, ".env.local")
				}
				if len(cfg.Backends) != 2 {
					t.Fatalf("len(Backends) = %d, want 2", len(cfg.Backends))
				}
				if cfg.Backends[0].Name != "keychain" {
					t.Errorf("Backends[0].Name = %q, want %q", cfg.Backends[0].Name, "keychain")
				}
				if cfg.Backends[1].Name != "vault" {
					t.Errorf("Backends[1].Name = %q, want %q", cfg.Backends[1].Name, "vault")
				}
				if cfg.Backends[1].Config["path"] != "~/.config/envref/vault.db" {
					t.Errorf("Backends[1].Config[path] = %q, want %q", cfg.Backends[1].Config["path"], "~/.config/envref/vault.db")
				}
				if len(cfg.Profiles) != 2 {
					t.Fatalf("len(Profiles) = %d, want 2", len(cfg.Profiles))
				}
				if cfg.Profiles["staging"].EnvFile != ".env.staging" {
					t.Errorf("Profiles[staging].EnvFile = %q, want %q", cfg.Profiles["staging"].EnvFile, ".env.staging")
				}
			},
		},
		{
			name: "minimal config with defaults",
			content: `project: simple
`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if cfg.Project != "simple" {
					t.Errorf("Project = %q, want %q", cfg.Project, "simple")
				}
				if cfg.EnvFile != ".env" {
					t.Errorf("EnvFile = %q, want %q (default)", cfg.EnvFile, ".env")
				}
				if cfg.LocalFile != ".env.local" {
					t.Errorf("LocalFile = %q, want %q (default)", cfg.LocalFile, ".env.local")
				}
			},
		},
		{
			name: "custom env paths",
			content: `project: custom
env_file: config/.env
local_file: config/.env.local
`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if cfg.EnvFile != "config/.env" {
					t.Errorf("EnvFile = %q, want %q", cfg.EnvFile, "config/.env")
				}
				if cfg.LocalFile != "config/.env.local" {
					t.Errorf("LocalFile = %q, want %q", cfg.LocalFile, "config/.env.local")
				}
			},
		},
		{
			name: "backends with config maps",
			content: `project: with-config
backends:
  - name: op
    type: 1password
    config:
      vault: Development
      account: my-team
`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if len(cfg.Backends) != 1 {
					t.Fatalf("len(Backends) = %d, want 1", len(cfg.Backends))
				}
				b := cfg.Backends[0]
				if b.Name != "op" {
					t.Errorf("Name = %q, want %q", b.Name, "op")
				}
				if b.Type != "1password" {
					t.Errorf("Type = %q, want %q", b.Type, "1password")
				}
				if b.Config["vault"] != "Development" {
					t.Errorf("Config[vault] = %q, want %q", b.Config["vault"], "Development")
				}
				if b.Config["account"] != "my-team" {
					t.Errorf("Config[account] = %q, want %q", b.Config["account"], "my-team")
				}
			},
		},
		{
			name:    "empty file",
			content: "",
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				// Defaults should be applied for env_file and local_file.
				if cfg.EnvFile != ".env" {
					t.Errorf("EnvFile = %q, want %q", cfg.EnvFile, ".env")
				}
				if cfg.LocalFile != ".env.local" {
					t.Errorf("LocalFile = %q, want %q", cfg.LocalFile, ".env.local")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeFile(t, dir, FullFileName, tt.content)

			cfg, err := LoadFile(path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestLoadFile_NotFound(t *testing.T) {
	_, err := LoadFile("/nonexistent/path/.envref.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, FullFileName, "{{invalid yaml")

	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoad_FindsConfigInCurrentDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, FullFileName, "project: found-here\n")

	cfg, root, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project != "found-here" {
		t.Errorf("Project = %q, want %q", cfg.Project, "found-here")
	}
	if root != dir {
		t.Errorf("root = %q, want %q", root, dir)
	}
}

func TestLoad_FindsConfigInParentDir(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "subdir")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}
	writeFile(t, parent, FullFileName, "project: parent-project\n")

	cfg, root, err := Load(child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project != "parent-project" {
		t.Errorf("Project = %q, want %q", cfg.Project, "parent-project")
	}
	if root != parent {
		t.Errorf("root = %q, want %q", root, parent)
	}
}

func TestLoad_FindsConfigInGrandparentDir(t *testing.T) {
	grandparent := t.TempDir()
	parent := filepath.Join(grandparent, "a")
	child := filepath.Join(parent, "b")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatalf("creating dirs: %v", err)
	}
	writeFile(t, grandparent, FullFileName, "project: gp\n")

	cfg, root, err := Load(child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project != "gp" {
		t.Errorf("Project = %q, want %q", cfg.Project, "gp")
	}
	if root != grandparent {
		t.Errorf("root = %q, want %q", root, grandparent)
	}
}

func TestLoad_NotFound(t *testing.T) {
	dir := t.TempDir()
	// No config file anywhere in the tree (temp dirs are under /tmp or similar).
	_, _, err := Load(dir)
	if err == nil {
		t.Fatal("expected ErrNotFound")
	}
	if err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestLoad_ClosestConfigWins(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "sub")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}
	writeFile(t, parent, FullFileName, "project: parent\n")
	writeFile(t, child, FullFileName, "project: child\n")

	cfg, root, err := Load(child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project != "child" {
		t.Errorf("Project = %q, want %q (closest config should win)", cfg.Project, "child")
	}
	if root != child {
		t.Errorf("root = %q, want %q", root, child)
	}
}

func TestConfig_ProfileEnvFile(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		profile  string
		expected string
	}{
		{
			name: "profile with custom env_file",
			config: Config{
				Profiles: map[string]ProfileConfig{
					"staging": {EnvFile: "config/.env.stg"},
				},
			},
			profile:  "staging",
			expected: "config/.env.stg",
		},
		{
			name: "profile with empty env_file uses convention",
			config: Config{
				Profiles: map[string]ProfileConfig{
					"staging": {},
				},
			},
			profile:  "staging",
			expected: ".env.staging",
		},
		{
			name:     "undefined profile uses convention",
			config:   Config{},
			profile:  "production",
			expected: ".env.production",
		},
		{
			name: "profile defined but no custom path",
			config: Config{
				Profiles: map[string]ProfileConfig{
					"development": {EnvFile: ""},
				},
			},
			profile:  "development",
			expected: ".env.development",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.ProfileEnvFile(tt.profile)
			if got != tt.expected {
				t.Errorf("ProfileEnvFile(%q) = %q, want %q", tt.profile, got, tt.expected)
			}
		})
	}
}

func TestConfig_HasProfile(t *testing.T) {
	cfg := Config{
		Profiles: map[string]ProfileConfig{
			"staging":    {EnvFile: ".env.staging"},
			"production": {},
		},
	}

	tests := []struct {
		name     string
		profile  string
		expected bool
	}{
		{"defined profile", "staging", true},
		{"another defined profile", "production", true},
		{"undefined profile", "development", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.HasProfile(tt.profile)
			if got != tt.expected {
				t.Errorf("HasProfile(%q) = %v, want %v", tt.profile, got, tt.expected)
			}
		})
	}
}

func TestConfig_HasProfile_NilMap(t *testing.T) {
	cfg := Config{}
	if cfg.HasProfile("staging") {
		t.Error("HasProfile should return false for nil profiles map")
	}
}

func TestConfig_EffectiveProfile(t *testing.T) {
	tests := []struct {
		name          string
		activeProfile string
		override      string
		expected      string
	}{
		{"override wins over config", "staging", "production", "production"},
		{"config used when no override", "staging", "", "staging"},
		{"empty when both empty", "", "", ""},
		{"override with empty config", "", "production", "production"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{ActiveProfile: tt.activeProfile}
			got := cfg.EffectiveProfile(tt.override)
			if got != tt.expected {
				t.Errorf("EffectiveProfile(%q) = %q, want %q", tt.override, got, tt.expected)
			}
		})
	}
}

func TestConfig_Validate_ActiveProfile(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid active_profile references defined profile",
			config: Config{
				Project:       "myapp",
				EnvFile:       ".env",
				LocalFile:     ".env.local",
				ActiveProfile: "staging",
				Profiles: map[string]ProfileConfig{
					"staging": {EnvFile: ".env.staging"},
				},
			},
			wantErr: false,
		},
		{
			name: "active_profile references undefined profile",
			config: Config{
				Project:       "myapp",
				EnvFile:       ".env",
				LocalFile:     ".env.local",
				ActiveProfile: "staging",
				Profiles: map[string]ProfileConfig{
					"production": {EnvFile: ".env.production"},
				},
			},
			wantErr: true,
			errMsg:  "active_profile \"staging\" is not defined in profiles",
		},
		{
			name: "active_profile without any profiles defined is allowed",
			config: Config{
				Project:       "myapp",
				EnvFile:       ".env",
				LocalFile:     ".env.local",
				ActiveProfile: "staging",
			},
			wantErr: false,
		},
		{
			name: "no active_profile is always valid",
			config: Config{
				Project:   "myapp",
				EnvFile:   ".env",
				LocalFile: ".env.local",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr && tt.errMsg != "" {
				if got := err.Error(); !contains(got, tt.errMsg) {
					t.Errorf("error = %q, want to contain %q", got, tt.errMsg)
				}
			}
		})
	}
}

func TestLoadFile_WithActiveProfile(t *testing.T) {
	dir := t.TempDir()
	content := `project: myapp
active_profile: staging
profiles:
  staging:
    env_file: .env.staging
  production:
    env_file: .env.production
`
	path := writeFile(t, dir, FullFileName, content)

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ActiveProfile != "staging" {
		t.Errorf("ActiveProfile = %q, want %q", cfg.ActiveProfile, "staging")
	}
	if cfg.ProfileEnvFile("staging") != ".env.staging" {
		t.Errorf("ProfileEnvFile(staging) = %q, want %q", cfg.ProfileEnvFile("staging"), ".env.staging")
	}
	if cfg.ProfileEnvFile("production") != ".env.production" {
		t.Errorf("ProfileEnvFile(production) = %q, want %q", cfg.ProfileEnvFile("production"), ".env.production")
	}
}

func TestSetActiveProfile_UpdateExisting(t *testing.T) {
	dir := t.TempDir()
	content := `project: myapp
active_profile: staging
profiles:
  staging:
    env_file: .env.staging
  production:
    env_file: .env.production
`
	path := writeFile(t, dir, FullFileName, content)

	if err := SetActiveProfile(path, "production"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the file was updated correctly.
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("loading updated config: %v", err)
	}
	if cfg.ActiveProfile != "production" {
		t.Errorf("ActiveProfile = %q, want %q", cfg.ActiveProfile, "production")
	}
	// Verify other fields are preserved.
	if cfg.Project != "myapp" {
		t.Errorf("Project = %q, want %q", cfg.Project, "myapp")
	}
	if len(cfg.Profiles) != 2 {
		t.Errorf("len(Profiles) = %d, want 2", len(cfg.Profiles))
	}
}

func TestSetActiveProfile_InsertNew(t *testing.T) {
	dir := t.TempDir()
	content := `project: myapp
env_file: .env
local_file: .env.local
profiles:
  staging:
    env_file: .env.staging
`
	path := writeFile(t, dir, FullFileName, content)

	if err := SetActiveProfile(path, "staging"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("loading updated config: %v", err)
	}
	if cfg.ActiveProfile != "staging" {
		t.Errorf("ActiveProfile = %q, want %q", cfg.ActiveProfile, "staging")
	}
	if cfg.Project != "myapp" {
		t.Errorf("Project = %q, want %q", cfg.Project, "myapp")
	}
}

func TestSetActiveProfile_ClearProfile(t *testing.T) {
	dir := t.TempDir()
	content := `project: myapp
active_profile: staging
profiles:
  staging:
    env_file: .env.staging
`
	path := writeFile(t, dir, FullFileName, content)

	if err := SetActiveProfile(path, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("loading updated config: %v", err)
	}
	if cfg.ActiveProfile != "" {
		t.Errorf("ActiveProfile = %q, want empty", cfg.ActiveProfile)
	}
	if cfg.Project != "myapp" {
		t.Errorf("Project = %q, want %q", cfg.Project, "myapp")
	}
}

func TestSetActiveProfile_PreservesComments(t *testing.T) {
	dir := t.TempDir()
	content := `# envref project configuration
project: myapp
env_file: .env
# This is a comment
local_file: .env.local
`
	path := writeFile(t, dir, FullFileName, content)

	if err := SetActiveProfile(path, "staging"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	got := string(data)
	if !contains(got, "# envref project configuration") {
		t.Error("comment was lost from file")
	}
	if !contains(got, "# This is a comment") {
		t.Error("inline comment was lost from file")
	}
	if !contains(got, "active_profile: staging") {
		t.Error("active_profile was not inserted")
	}
}

func TestSetActiveProfile_NonexistentFile(t *testing.T) {
	err := SetActiveProfile("/nonexistent/path/.envref.yaml", "staging")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestAddProfile_NewProfilesSection(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeFile(t, dir, FullFileName, cfgContent)

	path := filepath.Join(dir, FullFileName)
	err := AddProfile(path, "staging", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, loadErr := LoadFile(path)
	if loadErr != nil {
		t.Fatalf("LoadFile error: %v", loadErr)
	}
	if !cfg.HasProfile("staging") {
		t.Error("profile 'staging' should exist")
	}
}

func TestAddProfile_ExistingProfilesSection(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
profiles:
  staging:
    env_file: .env.staging
`
	writeFile(t, dir, FullFileName, cfgContent)

	path := filepath.Join(dir, FullFileName)
	err := AddProfile(path, "production", ".env.production")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, loadErr := LoadFile(path)
	if loadErr != nil {
		t.Fatalf("LoadFile error: %v", loadErr)
	}
	if !cfg.HasProfile("production") {
		t.Error("profile 'production' should exist")
	}
	if cfg.Profiles["production"].EnvFile != ".env.production" {
		t.Errorf("EnvFile = %q, want %q", cfg.Profiles["production"].EnvFile, ".env.production")
	}
	// Staging should still be there.
	if !cfg.HasProfile("staging") {
		t.Error("profile 'staging' should still exist")
	}
}

func TestAddProfile_DuplicateProfile(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
profiles:
  staging:
    env_file: .env.staging
`
	writeFile(t, dir, FullFileName, cfgContent)

	path := filepath.Join(dir, FullFileName)
	err := AddProfile(path, "staging", "")
	if err == nil {
		t.Fatal("expected error for duplicate profile")
	}
	if !contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want to contain 'already exists'", err.Error())
	}
}

func TestAddProfile_WithCustomEnvFile(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeFile(t, dir, FullFileName, cfgContent)

	path := filepath.Join(dir, FullFileName)
	err := AddProfile(path, "staging", "envs/staging.env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, loadErr := LoadFile(path)
	if loadErr != nil {
		t.Fatalf("LoadFile error: %v", loadErr)
	}
	if !cfg.HasProfile("staging") {
		t.Error("profile 'staging' should exist")
	}
	if cfg.Profiles["staging"].EnvFile != "envs/staging.env" {
		t.Errorf("EnvFile = %q, want %q", cfg.Profiles["staging"].EnvFile, "envs/staging.env")
	}
}

func TestAddProfile_NonexistentFile(t *testing.T) {
	err := AddProfile("/nonexistent/path/.envref.yaml", "staging", "")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestAddProfile_PreservesExistingContent(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
backends:
  - name: keychain
    type: keychain
profiles:
  staging:
    env_file: .env.staging
`
	writeFile(t, dir, FullFileName, cfgContent)

	path := filepath.Join(dir, FullFileName)
	err := AddProfile(path, "production", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, loadErr := LoadFile(path)
	if loadErr != nil {
		t.Fatalf("LoadFile error: %v", loadErr)
	}
	// Original config should be preserved.
	if cfg.Project != "myapp" {
		t.Errorf("Project = %q, want %q", cfg.Project, "myapp")
	}
	if len(cfg.Backends) != 1 {
		t.Fatalf("len(Backends) = %d, want 1", len(cfg.Backends))
	}
	if cfg.Backends[0].Name != "keychain" {
		t.Errorf("Backends[0].Name = %q, want %q", cfg.Backends[0].Name, "keychain")
	}
	if !cfg.HasProfile("staging") {
		t.Error("profile 'staging' should still exist")
	}
	if !cfg.HasProfile("production") {
		t.Error("profile 'production' should exist")
	}
}

func TestGlobalConfigDir(t *testing.T) {
	t.Run("uses ENVREF_CONFIG_DIR if set", func(t *testing.T) {
		t.Setenv("ENVREF_CONFIG_DIR", "/custom/config/dir")
		got := GlobalConfigDir()
		if got != "/custom/config/dir" {
			t.Errorf("GlobalConfigDir() = %q, want %q", got, "/custom/config/dir")
		}
	})

	t.Run("uses XDG_CONFIG_HOME if set", func(t *testing.T) {
		t.Setenv("ENVREF_CONFIG_DIR", "")
		t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
		got := GlobalConfigDir()
		if got != filepath.Join("/xdg/config", "envref") {
			t.Errorf("GlobalConfigDir() = %q, want %q", got, filepath.Join("/xdg/config", "envref"))
		}
	})

	t.Run("falls back to ~/.config/envref", func(t *testing.T) {
		t.Setenv("ENVREF_CONFIG_DIR", "")
		t.Setenv("XDG_CONFIG_HOME", "")
		got := GlobalConfigDir()
		if got == "" {
			t.Fatal("GlobalConfigDir() returned empty string")
		}
		if !filepath.IsAbs(got) {
			t.Errorf("GlobalConfigDir() = %q, want absolute path", got)
		}
	})
}

func TestGlobalConfigPath(t *testing.T) {
	t.Setenv("ENVREF_CONFIG_DIR", "/test/config")
	got := GlobalConfigPath()
	want := filepath.Join("/test/config", GlobalFileName)
	if got != want {
		t.Errorf("GlobalConfigPath() = %q, want %q", got, want)
	}
}

func TestMergeConfigs(t *testing.T) {
	tests := []struct {
		name    string
		global  *Config
		project *Config
		check   func(t *testing.T, cfg *Config)
	}{
		{
			name:   "nil global returns project as-is",
			global: nil,
			project: &Config{
				Project:   "myapp",
				EnvFile:   ".env",
				LocalFile: ".env.local",
			},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if cfg.Project != "myapp" {
					t.Errorf("Project = %q, want %q", cfg.Project, "myapp")
				}
			},
		},
		{
			name: "nil project returns global as-is",
			global: &Config{
				Project:   "global-app",
				EnvFile:   ".env",
				LocalFile: ".env.local",
			},
			project: nil,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if cfg.Project != "global-app" {
					t.Errorf("Project = %q, want %q", cfg.Project, "global-app")
				}
			},
		},
		{
			name: "project scalars override global",
			global: &Config{
				Project:       "global-app",
				EnvFile:       "global/.env",
				LocalFile:     "global/.env.local",
				ActiveProfile: "staging",
			},
			project: &Config{
				Project:       "my-project",
				EnvFile:       "project/.env",
				LocalFile:     "project/.env.local",
				ActiveProfile: "production",
			},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if cfg.Project != "my-project" {
					t.Errorf("Project = %q, want %q", cfg.Project, "my-project")
				}
				if cfg.EnvFile != "project/.env" {
					t.Errorf("EnvFile = %q, want %q", cfg.EnvFile, "project/.env")
				}
				if cfg.LocalFile != "project/.env.local" {
					t.Errorf("LocalFile = %q, want %q", cfg.LocalFile, "project/.env.local")
				}
				if cfg.ActiveProfile != "production" {
					t.Errorf("ActiveProfile = %q, want %q", cfg.ActiveProfile, "production")
				}
			},
		},
		{
			name: "project inherits global project name when empty",
			global: &Config{
				Project: "global-default",
			},
			project: &Config{
				Project:   "",
				EnvFile:   ".env",
				LocalFile: ".env.local",
			},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if cfg.Project != "global-default" {
					t.Errorf("Project = %q, want %q", cfg.Project, "global-default")
				}
			},
		},
		{
			name: "project inherits global active_profile when empty",
			global: &Config{
				ActiveProfile: "staging",
			},
			project: &Config{
				Project:   "myapp",
				EnvFile:   ".env",
				LocalFile: ".env.local",
			},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if cfg.ActiveProfile != "staging" {
					t.Errorf("ActiveProfile = %q, want %q", cfg.ActiveProfile, "staging")
				}
			},
		},
		{
			name: "project inherits global backends when empty",
			global: &Config{
				Backends: []BackendConfig{
					{Name: "keychain", Type: "keychain"},
					{Name: "vault", Type: "encrypted-vault"},
				},
			},
			project: &Config{
				Project:   "myapp",
				EnvFile:   ".env",
				LocalFile: ".env.local",
			},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if len(cfg.Backends) != 2 {
					t.Fatalf("len(Backends) = %d, want 2", len(cfg.Backends))
				}
				if cfg.Backends[0].Name != "keychain" {
					t.Errorf("Backends[0].Name = %q, want %q", cfg.Backends[0].Name, "keychain")
				}
				if cfg.Backends[1].Name != "vault" {
					t.Errorf("Backends[1].Name = %q, want %q", cfg.Backends[1].Name, "vault")
				}
			},
		},
		{
			name: "project backends replace global entirely",
			global: &Config{
				Backends: []BackendConfig{
					{Name: "keychain", Type: "keychain"},
					{Name: "vault", Type: "encrypted-vault"},
				},
			},
			project: &Config{
				Project:   "myapp",
				EnvFile:   ".env",
				LocalFile: ".env.local",
				Backends: []BackendConfig{
					{Name: "op", Type: "1password"},
				},
			},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if len(cfg.Backends) != 1 {
					t.Fatalf("len(Backends) = %d, want 1", len(cfg.Backends))
				}
				if cfg.Backends[0].Name != "op" {
					t.Errorf("Backends[0].Name = %q, want %q", cfg.Backends[0].Name, "op")
				}
			},
		},
		{
			name: "project inherits global profiles when empty",
			global: &Config{
				Profiles: map[string]ProfileConfig{
					"staging":    {EnvFile: ".env.staging"},
					"production": {EnvFile: ".env.production"},
				},
			},
			project: &Config{
				Project:   "myapp",
				EnvFile:   ".env",
				LocalFile: ".env.local",
			},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if len(cfg.Profiles) != 2 {
					t.Fatalf("len(Profiles) = %d, want 2", len(cfg.Profiles))
				}
				if cfg.Profiles["staging"].EnvFile != ".env.staging" {
					t.Errorf("Profiles[staging].EnvFile = %q, want %q", cfg.Profiles["staging"].EnvFile, ".env.staging")
				}
			},
		},
		{
			name: "project profiles replace global entirely",
			global: &Config{
				Profiles: map[string]ProfileConfig{
					"staging": {EnvFile: ".env.staging"},
				},
			},
			project: &Config{
				Project:   "myapp",
				EnvFile:   ".env",
				LocalFile: ".env.local",
				Profiles: map[string]ProfileConfig{
					"dev": {EnvFile: ".env.dev"},
				},
			},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if len(cfg.Profiles) != 1 {
					t.Fatalf("len(Profiles) = %d, want 1", len(cfg.Profiles))
				}
				if _, ok := cfg.Profiles["dev"]; !ok {
					t.Error("expected 'dev' profile to be present")
				}
				if _, ok := cfg.Profiles["staging"]; ok {
					t.Error("global 'staging' profile should not be present")
				}
			},
		},
		{
			name: "global non-default env_file inherited when project uses default",
			global: &Config{
				EnvFile: "config/.env",
			},
			project: &Config{
				Project:   "myapp",
				EnvFile:   ".env", // Viper default
				LocalFile: ".env.local",
			},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if cfg.EnvFile != "config/.env" {
					t.Errorf("EnvFile = %q, want %q (inherited from global)", cfg.EnvFile, "config/.env")
				}
			},
		},
		{
			name: "global non-default local_file inherited when project uses default",
			global: &Config{
				LocalFile: "config/.env.local",
			},
			project: &Config{
				Project:   "myapp",
				EnvFile:   ".env",
				LocalFile: ".env.local", // Viper default
			},
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if cfg.LocalFile != "config/.env.local" {
					t.Errorf("LocalFile = %q, want %q (inherited from global)", cfg.LocalFile, "config/.env.local")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeConfigs(tt.global, tt.project)
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestLoad_WithGlobalConfig(t *testing.T) {
	// Create a temp directory for the global config.
	globalDir := t.TempDir()
	t.Setenv("ENVREF_CONFIG_DIR", globalDir)

	// Write global config with default backends.
	writeFile(t, globalDir, GlobalFileName, `backends:
  - name: keychain
    type: keychain
profiles:
  staging:
    env_file: .env.staging
`)

	// Create project directory with minimal project config.
	projectDir := t.TempDir()
	writeFile(t, projectDir, FullFileName, `project: myapp
`)

	cfg, root, err := Load(projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root != projectDir {
		t.Errorf("root = %q, want %q", root, projectDir)
	}
	if cfg.Project != "myapp" {
		t.Errorf("Project = %q, want %q", cfg.Project, "myapp")
	}
	// Should inherit backends from global.
	if len(cfg.Backends) != 1 {
		t.Fatalf("len(Backends) = %d, want 1 (inherited from global)", len(cfg.Backends))
	}
	if cfg.Backends[0].Name != "keychain" {
		t.Errorf("Backends[0].Name = %q, want %q", cfg.Backends[0].Name, "keychain")
	}
	// Should inherit profiles from global.
	if len(cfg.Profiles) != 1 {
		t.Fatalf("len(Profiles) = %d, want 1 (inherited from global)", len(cfg.Profiles))
	}
	if cfg.Profiles["staging"].EnvFile != ".env.staging" {
		t.Errorf("Profiles[staging].EnvFile = %q, want %q", cfg.Profiles["staging"].EnvFile, ".env.staging")
	}
}

func TestLoad_ProjectOverridesGlobal(t *testing.T) {
	globalDir := t.TempDir()
	t.Setenv("ENVREF_CONFIG_DIR", globalDir)

	writeFile(t, globalDir, GlobalFileName, `backends:
  - name: keychain
    type: keychain
active_profile: staging
`)

	projectDir := t.TempDir()
	writeFile(t, projectDir, FullFileName, `project: myapp
backends:
  - name: op
    type: 1password
active_profile: production
`)

	cfg, _, err := Load(projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Project backends should completely replace global.
	if len(cfg.Backends) != 1 {
		t.Fatalf("len(Backends) = %d, want 1", len(cfg.Backends))
	}
	if cfg.Backends[0].Name != "op" {
		t.Errorf("Backends[0].Name = %q, want %q (project should override global)", cfg.Backends[0].Name, "op")
	}
	if cfg.ActiveProfile != "production" {
		t.Errorf("ActiveProfile = %q, want %q (project should override global)", cfg.ActiveProfile, "production")
	}
}

func TestLoad_NoGlobalConfig(t *testing.T) {
	// Point to a non-existent global config directory.
	t.Setenv("ENVREF_CONFIG_DIR", t.TempDir())

	projectDir := t.TempDir()
	writeFile(t, projectDir, FullFileName, `project: standalone
backends:
  - name: keychain
    type: keychain
`)

	cfg, _, err := Load(projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project != "standalone" {
		t.Errorf("Project = %q, want %q", cfg.Project, "standalone")
	}
	if len(cfg.Backends) != 1 {
		t.Errorf("len(Backends) = %d, want 1", len(cfg.Backends))
	}
}

func TestLoad_InvalidGlobalConfig(t *testing.T) {
	globalDir := t.TempDir()
	t.Setenv("ENVREF_CONFIG_DIR", globalDir)
	writeFile(t, globalDir, GlobalFileName, "{{invalid yaml")

	projectDir := t.TempDir()
	writeFile(t, projectDir, FullFileName, `project: myapp
`)

	_, _, err := Load(projectDir)
	if err == nil {
		t.Fatal("expected error for invalid global config")
	}
	if !contains(err.Error(), "global config") {
		t.Errorf("error = %q, want to contain 'global config'", err.Error())
	}
}

func TestValidationError_Type(t *testing.T) {
	cfg := Config{
		EnvFile:   ".env",
		LocalFile: ".env.local",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Should be extractable as *ValidationError.
	var valErr *ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("error should be *ValidationError, got %T", err)
	}
	if len(valErr.Problems) == 0 {
		t.Error("ValidationError.Problems should not be empty")
	}
	if !contains(valErr.Error(), "project name is required") {
		t.Errorf("error message = %q, want to contain 'project name is required'", valErr.Error())
	}
}

func TestValidationError_MultipleProblems(t *testing.T) {
	cfg := Config{
		Project:   " bad/name ",
		EnvFile:   "",
		LocalFile: "",
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var valErr *ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("error should be *ValidationError, got %T", err)
	}
	// Should report multiple problems.
	if len(valErr.Problems) < 2 {
		t.Errorf("expected at least 2 problems, got %d: %v", len(valErr.Problems), valErr.Problems)
	}
}

func TestConfig_Warnings(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		wantCount int
		wantMsg   string
	}{
		{
			name: "no warnings for known backend type",
			config: Config{
				Backends: []BackendConfig{
					{Name: "keychain", Type: "keychain"},
				},
			},
			wantCount: 0,
		},
		{
			name: "warning for unknown backend type",
			config: Config{
				Backends: []BackendConfig{
					{Name: "vault", Type: "encrypted-vault"},
				},
			},
			wantCount: 1,
			wantMsg:   "unknown backend type",
		},
		{
			name: "multiple warnings for multiple unknown types",
			config: Config{
				Backends: []BackendConfig{
					{Name: "vault", Type: "encrypted-vault"},
					{Name: "op", Type: "1password"},
				},
			},
			wantCount: 2,
		},
		{
			name: "no warnings for empty backends",
			config: Config{
				Backends: nil,
			},
			wantCount: 0,
		},
		{
			name: "backend with name fallback to known type",
			config: Config{
				Backends: []BackendConfig{
					{Name: "keychain"},
				},
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := tt.config.Warnings()
			if len(warnings) != tt.wantCount {
				t.Errorf("len(Warnings()) = %d, want %d: %v", len(warnings), tt.wantCount, warnings)
			}
			if tt.wantMsg != "" && tt.wantCount > 0 {
				if !contains(warnings[0], tt.wantMsg) {
					t.Errorf("warning = %q, want to contain %q", warnings[0], tt.wantMsg)
				}
			}
		})
	}
}

func TestLoad_ValidatesOnLoad(t *testing.T) {
	// Point to a non-existent global config directory.
	t.Setenv("ENVREF_CONFIG_DIR", t.TempDir())

	// Create a project config with no project name — should fail validation.
	projectDir := t.TempDir()
	writeFile(t, projectDir, FullFileName, `env_file: .env
local_file: .env.local
`)

	_, _, err := Load(projectDir)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	var valErr *ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("error should be *ValidationError, got %T: %v", err, err)
	}
	if !contains(valErr.Error(), "project name is required") {
		t.Errorf("error = %q, want to contain 'project name is required'", valErr.Error())
	}
}

func TestLoad_ValidatesAbsolutePaths(t *testing.T) {
	t.Setenv("ENVREF_CONFIG_DIR", t.TempDir())

	projectDir := t.TempDir()
	writeFile(t, projectDir, FullFileName, `project: myapp
env_file: /etc/.env
`)

	_, _, err := Load(projectDir)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	var valErr *ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("error should be *ValidationError, got %T: %v", err, err)
	}
	if !contains(valErr.Error(), "env_file must be a relative path") {
		t.Errorf("error = %q, want to contain 'env_file must be a relative path'", valErr.Error())
	}
}

func TestLoad_ValidatesProjectNameFormat(t *testing.T) {
	t.Setenv("ENVREF_CONFIG_DIR", t.TempDir())

	projectDir := t.TempDir()
	writeFile(t, projectDir, FullFileName, `project: "my/bad/project"
`)

	_, _, err := Load(projectDir)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	var valErr *ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("error should be *ValidationError, got %T: %v", err, err)
	}
	if !contains(valErr.Error(), "path separators") {
		t.Errorf("error = %q, want to contain 'path separators'", valErr.Error())
	}
}

func TestLoad_ValidConfigPassesValidation(t *testing.T) {
	t.Setenv("ENVREF_CONFIG_DIR", t.TempDir())

	projectDir := t.TempDir()
	writeFile(t, projectDir, FullFileName, `project: myapp
env_file: .env
local_file: .env.local
backends:
  - name: keychain
    type: keychain
profiles:
  staging:
    env_file: .env.staging
`)

	cfg, root, err := Load(projectDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project != "myapp" {
		t.Errorf("Project = %q, want %q", cfg.Project, "myapp")
	}
	if root != projectDir {
		t.Errorf("root = %q, want %q", root, projectDir)
	}
}

// contains reports whether s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsAt(s, substr)
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
