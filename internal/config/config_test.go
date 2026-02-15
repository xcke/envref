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
