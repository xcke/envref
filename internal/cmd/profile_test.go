package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xcke/envref/internal/config"
)

func TestProfileListCmd_ConfiguredProfiles(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
profiles:
  staging:
    env_file: .env.staging
  production:
    env_file: .env.production
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")
	writeTestFile(t, dir, ".env.staging", "KEY=staging\n")
	// production file does not exist on disk

	chdir(t, dir)

	stdout, stderr, err := execCmd(t, "profile", "list")
	require.NoError(t, err)
	assert.Empty(t, stderr)
	assert.Contains(t, stdout, "production")
	assert.Contains(t, stdout, "staging")
	// staging has file on disk
	assert.Contains(t, stdout, ".env.staging")
	// production has no file
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, line := range lines {
		if strings.Contains(line, "production") {
			assert.Contains(t, line, "no file")
		}
		if strings.Contains(line, "staging") {
			assert.Contains(t, line, "file")
			assert.NotContains(t, line, "no file")
		}
	}
}

func TestProfileListCmd_ActiveProfile(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
active_profile: staging
profiles:
  staging:
    env_file: .env.staging
  production:
    env_file: .env.production
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "list")
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, line := range lines {
		if len(line) < 2 {
			continue
		}
		// Output format: "* name..." or "  name..."
		// The profile name starts at position 2.
		rest := line[2:]
		if strings.HasPrefix(rest, "staging") {
			assert.Equal(t, "* ", line[:2], "active profile should be marked with *")
		}
		if strings.HasPrefix(rest, "production") {
			assert.Equal(t, "  ", line[:2], "inactive profile should not be marked")
		}
	}
}

func TestProfileListCmd_DiscoverConventionFiles(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")
	writeTestFile(t, dir, ".env.development", "KEY=dev\n")
	writeTestFile(t, dir, ".env.test", "KEY=test\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "list")
	require.NoError(t, err)
	assert.Contains(t, stdout, "development")
	assert.Contains(t, stdout, "test")
	// Convention-based profiles should show "file" but not "config"
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, line := range lines {
		if strings.Contains(line, "development") || strings.Contains(line, "test") {
			assert.NotContains(t, line, "config")
			assert.Contains(t, line, "file")
		}
	}
}

func TestProfileListCmd_SkipsLocalFile(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")
	writeTestFile(t, dir, ".env.local", "KEY=local\n")
	writeTestFile(t, dir, ".env.staging", "KEY=staging\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "list")
	require.NoError(t, err)
	assert.NotContains(t, stdout, "local")
	assert.Contains(t, stdout, "staging")
}

func TestProfileListCmd_NoProfiles(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")

	chdir(t, dir)

	stdout, stderr, err := execCmd(t, "profile", "list")
	require.NoError(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, "no profiles found")
}

func TestProfileListCmd_MixedConfigAndDisk(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
profiles:
  staging:
    env_file: .env.staging
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")
	writeTestFile(t, dir, ".env.staging", "KEY=staging\n")
	writeTestFile(t, dir, ".env.production", "KEY=prod\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "list")
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	assert.Len(t, lines, 2)

	for _, line := range lines {
		if strings.Contains(line, "staging") {
			assert.Contains(t, line, "config")
			assert.Contains(t, line, "file")
		}
		if strings.Contains(line, "production") {
			assert.NotContains(t, line, "config")
			assert.Contains(t, line, "file")
		}
	}
}

func TestProfileListCmd_CustomEnvFilePath(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
profiles:
  staging:
    env_file: envs/staging.env
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")

	// Create the custom directory and file.
	envsDir := filepath.Join(dir, "envs")
	require.NoError(t, os.Mkdir(envsDir, 0o755))
	writeTestFile(t, envsDir, "staging.env", "KEY=staging\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "list")
	require.NoError(t, err)
	assert.Contains(t, stdout, "staging")
	assert.Contains(t, stdout, "envs/staging.env")
	assert.Contains(t, stdout, "file")
}

func TestProfileListCmd_SortedOutput(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")
	writeTestFile(t, dir, ".env.zebra", "Z=1\n")
	writeTestFile(t, dir, ".env.alpha", "A=1\n")
	writeTestFile(t, dir, ".env.middle", "M=1\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "list")
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	require.Len(t, lines, 3)
	assert.Contains(t, lines[0], "alpha")
	assert.Contains(t, lines[1], "middle")
	assert.Contains(t, lines[2], "zebra")
}

func TestProfileListCmd_SkipsDottedNames(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")
	writeTestFile(t, dir, ".env.staging.bak", "KEY=old\n")
	writeTestFile(t, dir, ".env.staging", "KEY=staging\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "list")
	require.NoError(t, err)
	assert.Contains(t, stdout, "staging")
	// .env.staging.bak should not show up as a profile
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	assert.Len(t, lines, 1)
}

func TestProfileListCmd_NoConfig(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_, _, err := execCmd(t, "profile", "list")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loading config")
}

func TestProfileListCmd_Help(t *testing.T) {
	stdout, _, err := execCmd(t, "profile", "list", "--help")
	require.NoError(t, err)
	assert.Contains(t, stdout, "List all available environment profiles")
	assert.Contains(t, stdout, "envref profile list")
}

func TestProfileCmd_Help(t *testing.T) {
	stdout, _, err := execCmd(t, "profile", "--help")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Manage environment profiles")
	assert.Contains(t, stdout, "list")
	assert.Contains(t, stdout, "use")
}

// --- profile use tests -------------------------------------------------------

func TestProfileUseCmd_SetConfiguredProfile(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
profiles:
  staging:
    env_file: .env.staging
  production:
    env_file: .env.production
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "use", "staging")
	require.NoError(t, err)
	assert.Contains(t, stdout, "staging")

	// Verify the config file was updated.
	cfg, loadErr := config.LoadFile(filepath.Join(dir, config.FullFileName))
	require.NoError(t, loadErr)
	assert.Equal(t, "staging", cfg.ActiveProfile)
}

func TestProfileUseCmd_SetConventionProfile(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")
	writeTestFile(t, dir, ".env.development", "KEY=dev\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "use", "development")
	require.NoError(t, err)
	assert.Contains(t, stdout, "development")

	cfg, loadErr := config.LoadFile(filepath.Join(dir, config.FullFileName))
	require.NoError(t, loadErr)
	assert.Equal(t, "development", cfg.ActiveProfile)
}

func TestProfileUseCmd_SwitchProfile(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
active_profile: staging
profiles:
  staging:
    env_file: .env.staging
  production:
    env_file: .env.production
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "use", "production")
	require.NoError(t, err)
	assert.Contains(t, stdout, "production")

	cfg, loadErr := config.LoadFile(filepath.Join(dir, config.FullFileName))
	require.NoError(t, loadErr)
	assert.Equal(t, "production", cfg.ActiveProfile)
}

func TestProfileUseCmd_ClearProfile(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
active_profile: staging
profiles:
  staging:
    env_file: .env.staging
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "use", "--clear")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Cleared active profile")

	cfg, loadErr := config.LoadFile(filepath.Join(dir, config.FullFileName))
	require.NoError(t, loadErr)
	assert.Empty(t, cfg.ActiveProfile)
}

func TestProfileUseCmd_NonexistentProfile(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
profiles:
  staging:
    env_file: .env.staging
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")

	chdir(t, dir)

	_, _, err := execCmd(t, "profile", "use", "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestProfileUseCmd_NoArgs(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)

	chdir(t, dir)

	_, _, err := execCmd(t, "profile", "use")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "profile name is required")
}

func TestProfileUseCmd_NoConfig(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_, _, err := execCmd(t, "profile", "use", "staging")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loading config")
}

func TestProfileUseCmd_PreservesOtherConfig(t *testing.T) {
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
  production:
    env_file: .env.production
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")

	chdir(t, dir)

	_, _, err := execCmd(t, "profile", "use", "staging")
	require.NoError(t, err)

	cfg, loadErr := config.LoadFile(filepath.Join(dir, config.FullFileName))
	require.NoError(t, loadErr)
	assert.Equal(t, "staging", cfg.ActiveProfile)
	assert.Equal(t, "myapp", cfg.Project)
	assert.Equal(t, ".env", cfg.EnvFile)
	assert.Len(t, cfg.Backends, 1)
	assert.Len(t, cfg.Profiles, 2)
}

func TestProfileUseCmd_Help(t *testing.T) {
	stdout, _, err := execCmd(t, "profile", "use", "--help")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Set the active environment profile")
	assert.Contains(t, stdout, "envref profile use staging")
	assert.Contains(t, stdout, "--clear")
}

// --- profile create tests ----------------------------------------------------

func TestProfileCreateCmd_Basic(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "create", "staging")
	require.NoError(t, err)
	assert.Contains(t, stdout, "staging")

	// Verify the file was created.
	data, readErr := os.ReadFile(filepath.Join(dir, ".env.staging"))
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "staging")
	assert.Contains(t, string(data), "# Environment variables")
}

func TestProfileCreateCmd_WithFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\nDB_HOST=localhost\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "create", "staging", "--from", ".env")
	require.NoError(t, err)
	assert.Contains(t, stdout, "staging")

	// Verify the file was copied from .env.
	data, readErr := os.ReadFile(filepath.Join(dir, ".env.staging"))
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "KEY=value")
	assert.Contains(t, string(data), "DB_HOST=localhost")
}

func TestProfileCreateCmd_WithRegister(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "create", "staging", "--register")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Registered")

	// Verify the profile was added to config.
	cfg, loadErr := config.LoadFile(filepath.Join(dir, config.FullFileName))
	require.NoError(t, loadErr)
	assert.True(t, cfg.HasProfile("staging"))
}

func TestProfileCreateCmd_WithRegisterAndCustomEnvFile(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "create", "staging", "--register", "--env-file", "envs/staging.env")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Registered")

	// Verify the profile was added with custom env_file.
	cfg, loadErr := config.LoadFile(filepath.Join(dir, config.FullFileName))
	require.NoError(t, loadErr)
	assert.True(t, cfg.HasProfile("staging"))
	assert.Equal(t, "envs/staging.env", cfg.Profiles["staging"].EnvFile)

	// Verify the file was created at the custom path.
	_, statErr := os.Stat(filepath.Join(dir, "envs", "staging.env"))
	assert.NoError(t, statErr)
}

func TestProfileCreateCmd_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")
	writeTestFile(t, dir, ".env.staging", "EXISTING=true\n")

	chdir(t, dir)

	_, _, err := execCmd(t, "profile", "create", "staging")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestProfileCreateCmd_ForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")
	writeTestFile(t, dir, ".env.staging", "EXISTING=true\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "create", "staging", "--force")
	require.NoError(t, err)
	assert.Contains(t, stdout, "staging")

	// Verify the file was overwritten.
	data, readErr := os.ReadFile(filepath.Join(dir, ".env.staging"))
	require.NoError(t, readErr)
	assert.NotContains(t, string(data), "EXISTING")
	assert.Contains(t, string(data), "# Environment variables")
}

func TestProfileCreateCmd_ReservedNameLocal(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")

	chdir(t, dir)

	_, _, err := execCmd(t, "profile", "create", "local")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reserved")
}

func TestProfileCreateCmd_InvalidNameWithDots(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")

	chdir(t, dir)

	_, _, err := execCmd(t, "profile", "create", "staging.bak")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must not contain dots")
}

func TestProfileCreateCmd_NoArgs(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)

	chdir(t, dir)

	_, _, err := execCmd(t, "profile", "create")
	assert.Error(t, err)
}

func TestProfileCreateCmd_NoConfig(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_, _, err := execCmd(t, "profile", "create", "staging")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loading config")
}

func TestProfileCreateCmd_FromNonexistentFile(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "KEY=value\n")

	chdir(t, dir)

	_, _, err := execCmd(t, "profile", "create", "staging", "--from", ".env.nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reading source file")
}

func TestProfileCreateCmd_Help(t *testing.T) {
	stdout, _, err := execCmd(t, "profile", "create", "--help")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Create a new environment profile")
	assert.Contains(t, stdout, "--register")
	assert.Contains(t, stdout, "--force")
	assert.Contains(t, stdout, "--from")
	assert.Contains(t, stdout, "--env-file")
}

func TestProfileCreateCmd_VisibleInHelp(t *testing.T) {
	stdout, _, err := execCmd(t, "profile", "--help")
	require.NoError(t, err)
	assert.Contains(t, stdout, "create")
}

// --- profile diff tests ------------------------------------------------------

func TestProfileDiffCmd_ShowsDifferences(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
profiles:
  staging:
    env_file: .env.staging
  production:
    env_file: .env.production
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "APP_NAME=myapp\nLOG_LEVEL=info\n")
	writeTestFile(t, dir, ".env.staging", "DB_HOST=staging-db\nLOG_LEVEL=debug\n")
	writeTestFile(t, dir, ".env.production", "DB_HOST=prod-db\nCACHE_TTL=3600\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "diff", "staging", "production")
	require.NoError(t, err)

	// CACHE_TTL is only in production (only_b → +)
	assert.Contains(t, stdout, "CACHE_TTL")
	// DB_HOST differs between profiles (changed → ~)
	assert.Contains(t, stdout, "DB_HOST")
	// LOG_LEVEL: staging sets debug, production inherits info from base — changed
	assert.Contains(t, stdout, "LOG_LEVEL")
	// Summary line
	assert.Contains(t, stdout, "difference(s)")
}

func TestProfileDiffCmd_IdenticalProfiles(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
profiles:
  staging:
    env_file: .env.staging
  staging2:
    env_file: .env.staging2
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "APP_NAME=myapp\n")
	writeTestFile(t, dir, ".env.staging", "DB_HOST=staging-db\n")
	writeTestFile(t, dir, ".env.staging2", "DB_HOST=staging-db\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "diff", "staging", "staging2")
	require.NoError(t, err)
	assert.Contains(t, stdout, "identical")
}

func TestProfileDiffCmd_OnlyInFirstProfile(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
profiles:
  staging:
    env_file: .env.staging
  production:
    env_file: .env.production
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "APP_NAME=myapp\n")
	writeTestFile(t, dir, ".env.staging", "EXTRA_KEY=staging-only\n")
	writeTestFile(t, dir, ".env.production", "")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "diff", "staging", "production")
	require.NoError(t, err)
	assert.Contains(t, stdout, "EXTRA_KEY")
	assert.Contains(t, stdout, "-")
	assert.Contains(t, stdout, "1 only in staging")
}

func TestProfileDiffCmd_OnlyInSecondProfile(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
profiles:
  staging:
    env_file: .env.staging
  production:
    env_file: .env.production
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "APP_NAME=myapp\n")
	writeTestFile(t, dir, ".env.staging", "")
	writeTestFile(t, dir, ".env.production", "PROD_KEY=prod-only\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "diff", "staging", "production")
	require.NoError(t, err)
	assert.Contains(t, stdout, "PROD_KEY")
	assert.Contains(t, stdout, "+")
	assert.Contains(t, stdout, "1 only in production")
}

func TestProfileDiffCmd_ChangedValues(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
profiles:
  staging:
    env_file: .env.staging
  production:
    env_file: .env.production
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "")
	writeTestFile(t, dir, ".env.staging", "DB_HOST=staging-db\n")
	writeTestFile(t, dir, ".env.production", "DB_HOST=prod-db\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "diff", "staging", "production")
	require.NoError(t, err)
	assert.Contains(t, stdout, "DB_HOST")
	assert.Contains(t, stdout, "staging-db")
	assert.Contains(t, stdout, "prod-db")
	assert.Contains(t, stdout, "1 changed")
}

func TestProfileDiffCmd_JSONFormat(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
profiles:
  staging:
    env_file: .env.staging
  production:
    env_file: .env.production
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "APP_NAME=myapp\n")
	writeTestFile(t, dir, ".env.staging", "DB_HOST=staging-db\n")
	writeTestFile(t, dir, ".env.production", "DB_HOST=prod-db\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "diff", "staging", "production", "--format", "json")
	require.NoError(t, err)
	assert.Contains(t, stdout, `"key"`)
	assert.Contains(t, stdout, `"kind"`)
	assert.Contains(t, stdout, `"changed"`)
	assert.Contains(t, stdout, `"DB_HOST"`)
}

func TestProfileDiffCmd_TableFormat(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
profiles:
  staging:
    env_file: .env.staging
  production:
    env_file: .env.production
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "APP_NAME=myapp\n")
	writeTestFile(t, dir, ".env.staging", "DB_HOST=staging-db\n")
	writeTestFile(t, dir, ".env.production", "DB_HOST=prod-db\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "diff", "staging", "production", "--format", "table")
	require.NoError(t, err)
	// Table should have header with profile names.
	assert.Contains(t, stdout, "staging")
	assert.Contains(t, stdout, "production")
	assert.Contains(t, stdout, "KEY")
	assert.Contains(t, stdout, "DB_HOST")
}

func TestProfileDiffCmd_ConventionProfiles(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "APP_NAME=myapp\n")
	writeTestFile(t, dir, ".env.development", "DEBUG=true\n")
	writeTestFile(t, dir, ".env.test", "DEBUG=false\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "diff", "development", "test")
	require.NoError(t, err)
	assert.Contains(t, stdout, "DEBUG")
	assert.Contains(t, stdout, "true")
	assert.Contains(t, stdout, "false")
}

func TestProfileDiffCmd_MissingProfileFile(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "APP_NAME=myapp\n")
	writeTestFile(t, dir, ".env.staging", "DB_HOST=staging-db\n")
	// .env.production does not exist — loadAndMergeEnv loads profile via LoadOptional

	chdir(t, dir)

	// This should succeed — missing profile file means just the base .env is used.
	stdout, _, err := execCmd(t, "profile", "diff", "staging", "production")
	require.NoError(t, err)
	assert.Contains(t, stdout, "DB_HOST")
}

func TestProfileDiffCmd_NoArgs(t *testing.T) {
	_, _, err := execCmd(t, "profile", "diff")
	assert.Error(t, err)
}

func TestProfileDiffCmd_OneArg(t *testing.T) {
	_, _, err := execCmd(t, "profile", "diff", "staging")
	assert.Error(t, err)
}

func TestProfileDiffCmd_NoConfig(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_, _, err := execCmd(t, "profile", "diff", "staging", "production")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loading config")
}

func TestProfileDiffCmd_Help(t *testing.T) {
	stdout, _, err := execCmd(t, "profile", "diff", "--help")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Compare the environment variables between two profiles")
	assert.Contains(t, stdout, "--format")
	assert.Contains(t, stdout, "envref profile diff staging production")
}

func TestProfileDiffCmd_VisibleInHelp(t *testing.T) {
	stdout, _, err := execCmd(t, "profile", "--help")
	require.NoError(t, err)
	assert.Contains(t, stdout, "diff")
}

func TestProfileDiffCmd_InvalidFormat(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "APP_NAME=myapp\n")
	writeTestFile(t, dir, ".env.staging", "DB_HOST=staging\n")
	writeTestFile(t, dir, ".env.production", "DB_HOST=prod\n")

	chdir(t, dir)

	_, _, err := execCmd(t, "profile", "diff", "staging", "production", "--format", "xml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid format")
}

func TestProfileDiffCmd_LocalOverrideAffectsBoth(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	writeTestFile(t, dir, ".env", "APP_NAME=myapp\n")
	writeTestFile(t, dir, ".env.staging", "DB_HOST=staging-db\n")
	writeTestFile(t, dir, ".env.production", "DB_HOST=prod-db\n")
	// .env.local overrides DB_HOST for both profiles
	writeTestFile(t, dir, ".env.local", "DB_HOST=local-db\n")

	chdir(t, dir)

	stdout, _, err := execCmd(t, "profile", "diff", "staging", "production")
	require.NoError(t, err)
	// Since .env.local overrides DB_HOST in both, they should be identical for DB_HOST
	assert.Contains(t, stdout, "identical")
}
