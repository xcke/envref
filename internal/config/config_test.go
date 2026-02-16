package config

import (
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
