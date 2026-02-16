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
}
