package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xcke/envref/internal/config"
)

func TestSyncEnvRef_AddsNewEntry(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("EXISTING=value\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{EnvFile: ".env"}
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "list"})
	cmd, _, _ := root.Find([]string{"secret", "list"})

	err := syncEnvRef(cmd, cfg, dir, "API_KEY", "keychain", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(envPath)
	if !strings.Contains(string(content), "API_KEY=ref://keychain/API_KEY") {
		t.Errorf("expected ref entry in .env, got:\n%s", content)
	}
	if !strings.Contains(string(content), "EXISTING=value") {
		t.Errorf("expected existing entry preserved, got:\n%s", content)
	}
}

func TestSyncEnvRef_UpdatesExistingRef(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("API_KEY=ref://vault/API_KEY\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{EnvFile: ".env"}
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "list"})
	cmd, _, _ := root.Find([]string{"secret", "list"})

	err := syncEnvRef(cmd, cfg, dir, "API_KEY", "keychain", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(envPath)
	if !strings.Contains(string(content), "API_KEY=ref://keychain/API_KEY") {
		t.Errorf("expected updated ref, got:\n%s", content)
	}
	if strings.Contains(string(content), "ref://vault/API_KEY") {
		t.Error("old ref value should have been replaced")
	}
}

func TestSyncEnvRef_SkipsNonRefValue(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	original := "API_KEY=sk-hardcoded-123\n"
	if err := os.WriteFile(envPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{EnvFile: ".env"}
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "list"})
	cmd, _, _ := root.Find([]string{"secret", "list"})

	err := syncEnvRef(cmd, cfg, dir, "API_KEY", "keychain", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(envPath)
	if string(content) != original {
		t.Errorf("non-ref value should not be overwritten, got:\n%s", content)
	}
}

func TestSyncEnvRef_CreatesFileIfMissing(t *testing.T) {
	dir := t.TempDir()

	cfg := &config.Config{EnvFile: ".env"}
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "list"})
	cmd, _, _ := root.Find([]string{"secret", "list"})

	err := syncEnvRef(cmd, cfg, dir, "NEW_KEY", "keychain", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("expected .env to be created: %v", err)
	}
	if !strings.Contains(string(content), "NEW_KEY=ref://keychain/NEW_KEY") {
		t.Errorf("expected ref entry, got:\n%s", content)
	}
}

func TestSyncEnvRef_ProfileUsesProfileEnvFile(t *testing.T) {
	dir := t.TempDir()
	// Create base .env to verify it's NOT modified.
	basePath := filepath.Join(dir, ".env")
	if err := os.WriteFile(basePath, []byte("BASE=value\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		EnvFile: ".env",
		Profiles: map[string]config.ProfileConfig{
			"staging": {EnvFile: ".env.staging"},
		},
	}
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "list"})
	cmd, _, _ := root.Find([]string{"secret", "list"})

	err := syncEnvRef(cmd, cfg, dir, "API_KEY", "keychain", "staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Profile file should have the ref.
	profileContent, err := os.ReadFile(filepath.Join(dir, ".env.staging"))
	if err != nil {
		t.Fatalf("expected .env.staging to be created: %v", err)
	}
	if !strings.Contains(string(profileContent), "API_KEY=ref://keychain/API_KEY") {
		t.Errorf("expected ref in .env.staging, got:\n%s", profileContent)
	}

	// Base .env should be unchanged.
	baseContent, _ := os.ReadFile(basePath)
	if string(baseContent) != "BASE=value\n" {
		t.Errorf("base .env should be unchanged, got:\n%s", baseContent)
	}
}

func TestSyncEnvRef_NoEnvFlagSkips(t *testing.T) {
	dir := t.TempDir()

	cfg := &config.Config{EnvFile: ".env"}
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "list", "--no-env"})
	// Find the secret parent command and set the persistent flag on it.
	secretCmd, _, _ := root.Find([]string{"secret"})
	_ = secretCmd.PersistentFlags().Set("no-env", "true")
	// Find the leaf command which inherits the flag.
	cmd, _, _ := root.Find([]string{"secret", "list"})

	err := syncEnvRef(cmd, cfg, dir, "API_KEY", "keychain", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No .env file should be created.
	if _, err := os.Stat(filepath.Join(dir, ".env")); err == nil {
		t.Error("expected .env to not be created when --no-env is set")
	}
}

func TestSyncEnvRef_PreservesExistingEntries(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	original := "FOO=bar\nBAZ=qux\n"
	if err := os.WriteFile(envPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{EnvFile: ".env"}
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "list"})
	cmd, _, _ := root.Find([]string{"secret", "list"})

	err := syncEnvRef(cmd, cfg, dir, "SECRET", "keychain", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(envPath)
	s := string(content)
	if !strings.Contains(s, "FOO=bar") {
		t.Error("expected FOO=bar preserved")
	}
	if !strings.Contains(s, "BAZ=qux") {
		t.Error("expected BAZ=qux preserved")
	}
	if !strings.Contains(s, "SECRET=ref://keychain/SECRET") {
		t.Error("expected SECRET ref added")
	}
}

func TestRemoveEnvRef_RemovesRefEntry(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("FOO=bar\nAPI_KEY=ref://keychain/API_KEY\nBAZ=qux\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{EnvFile: ".env"}
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "list"})
	cmd, _, _ := root.Find([]string{"secret", "list"})

	err := removeEnvRef(cmd, cfg, dir, "API_KEY", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(envPath)
	s := string(content)
	if strings.Contains(s, "API_KEY") {
		t.Errorf("expected API_KEY removed, got:\n%s", s)
	}
	if !strings.Contains(s, "FOO=bar") {
		t.Error("expected FOO=bar preserved")
	}
	if !strings.Contains(s, "BAZ=qux") {
		t.Error("expected BAZ=qux preserved")
	}
}

func TestRemoveEnvRef_SkipsNonRefValue(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	original := "API_KEY=sk-hardcoded\n"
	if err := os.WriteFile(envPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{EnvFile: ".env"}
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "list"})
	cmd, _, _ := root.Find([]string{"secret", "list"})

	err := removeEnvRef(cmd, cfg, dir, "API_KEY", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(envPath)
	if string(content) != original {
		t.Errorf("non-ref value should not be removed, got:\n%s", content)
	}
}

func TestRemoveEnvRef_NoopIfKeyMissing(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	original := "FOO=bar\n"
	if err := os.WriteFile(envPath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{EnvFile: ".env"}
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "list"})
	cmd, _, _ := root.Find([]string{"secret", "list"})

	err := removeEnvRef(cmd, cfg, dir, "NONEXISTENT", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(envPath)
	if string(content) != original {
		t.Errorf("file should be unchanged, got:\n%s", content)
	}
}

func TestRemoveEnvRef_ProfileUsesProfileEnvFile(t *testing.T) {
	dir := t.TempDir()
	profilePath := filepath.Join(dir, ".env.staging")
	if err := os.WriteFile(profilePath, []byte("API_KEY=ref://keychain/API_KEY\nOTHER=val\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		EnvFile: ".env",
		Profiles: map[string]config.ProfileConfig{
			"staging": {EnvFile: ".env.staging"},
		},
	}
	root := NewRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"secret", "list"})
	cmd, _, _ := root.Find([]string{"secret", "list"})

	err := removeEnvRef(cmd, cfg, dir, "API_KEY", "staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(profilePath)
	s := string(content)
	if strings.Contains(s, "API_KEY") {
		t.Errorf("expected API_KEY removed from profile file, got:\n%s", s)
	}
	if !strings.Contains(s, "OTHER=val") {
		t.Error("expected OTHER=val preserved")
	}
}
