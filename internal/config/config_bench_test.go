package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xcke/envref/internal/config"
)

func BenchmarkLoad(b *testing.B) {
	dir := b.TempDir()
	cfgContent := `project: bench-project
env_file: .env
local_file: .env.local
backends:
  - name: keychain
    type: keychain
profiles:
  development:
    env_file: .env.development
  staging:
    env_file: .env.staging
`
	cfgPath := filepath.Join(dir, ".envref.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for b.Loop() {
		_, _, _ = config.Load(dir)
	}
}

func BenchmarkValidate(b *testing.B) {
	cfg := &config.Config{
		Project:   "bench-project",
		EnvFile:   ".env",
		LocalFile: ".env.local",
		Backends: []config.BackendConfig{
			{Name: "keychain", Type: "keychain"},
			{Name: "vault", Type: "vault"},
		},
		Profiles: map[string]config.ProfileConfig{
			"development": {EnvFile: ".env.development"},
			"staging":     {EnvFile: ".env.staging"},
			"production":  {EnvFile: ".env.production"},
		},
	}
	b.ResetTimer()
	for b.Loop() {
		_ = cfg.Validate()
	}
}
