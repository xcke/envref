package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xcke/envref/internal/config"
)

func TestConfigShowCmd_PlainOutput(t *testing.T) {
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
	chdir(t, dir)

	stdout, _, err := execCmd(t, "config", "show")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Project: myapp")
	assert.Contains(t, stdout, "EnvFile: .env")
	assert.Contains(t, stdout, "LocalFile: .env.local")
	assert.Contains(t, stdout, "keychain")
	assert.Contains(t, stdout, "staging")
	assert.Contains(t, stdout, "production")
	assert.Contains(t, stdout, config.FullFileName)
}

func TestConfigShowCmd_PlainWithActiveProfile(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
env_file: .env
local_file: .env.local
active_profile: staging
profiles:
  staging:
    env_file: .env.staging
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	chdir(t, dir)

	stdout, _, err := execCmd(t, "config", "show")
	require.NoError(t, err)
	assert.Contains(t, stdout, "ActiveProfile: staging")
	assert.Contains(t, stdout, "(active)")
}

func TestConfigShowCmd_JSONOutput(t *testing.T) {
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
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	chdir(t, dir)

	stdout, _, err := execCmd(t, "config", "show", "--format", "json")
	require.NoError(t, err)

	var output configShowOutput
	err = json.Unmarshal([]byte(stdout), &output)
	require.NoError(t, err, "output should be valid JSON")

	assert.Equal(t, "myapp", output.Project)
	assert.Equal(t, ".env", output.EnvFile)
	assert.Equal(t, ".env.local", output.LocalFile)
	assert.Len(t, output.Backends, 1)
	assert.Equal(t, "keychain", output.Backends[0].Name)
	assert.Equal(t, "keychain", output.Backends[0].Type)
	assert.Contains(t, output.Profiles, "staging")
	assert.Contains(t, output.ConfigFile, config.FullFileName)
}

func TestConfigShowCmd_JSONOutput_ActiveProfile(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
active_profile: staging
profiles:
  staging:
    env_file: .env.staging
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	chdir(t, dir)

	stdout, _, err := execCmd(t, "config", "show", "--format", "json")
	require.NoError(t, err)

	var output configShowOutput
	err = json.Unmarshal([]byte(stdout), &output)
	require.NoError(t, err)
	assert.Equal(t, "staging", output.ActiveProfile)
}

func TestConfigShowCmd_TableOutput(t *testing.T) {
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
	chdir(t, dir)

	stdout, _, err := execCmd(t, "config", "show", "--format", "table")
	require.NoError(t, err)
	assert.Contains(t, stdout, "KEY")
	assert.Contains(t, stdout, "VALUE")
	assert.Contains(t, stdout, "project")
	assert.Contains(t, stdout, "myapp")
	assert.Contains(t, stdout, "backends")
	assert.Contains(t, stdout, "keychain")
	assert.Contains(t, stdout, "profiles")
}

func TestConfigShowCmd_MinimalConfig(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: minimal
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	chdir(t, dir)

	stdout, _, err := execCmd(t, "config", "show")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Project: minimal")
	assert.Contains(t, stdout, "EnvFile: .env")
	assert.Contains(t, stdout, "LocalFile: .env.local")
	// No backends or profiles sections.
	assert.NotContains(t, stdout, "Backends:")
	assert.NotContains(t, stdout, "Profiles:")
}

func TestConfigShowCmd_NoConfig_Error(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_, _, err := execCmd(t, "config", "show")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loading config")
}

func TestConfigShowCmd_InvalidFormat(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	chdir(t, dir)

	_, _, err := execCmd(t, "config", "show", "--format", "invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid format")
}

func TestConfigShowCmd_BackendWithConfig(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
backends:
  - name: vault
    type: hashicorp-vault
    config:
      address: https://vault.example.com
      namespace: myteam
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	chdir(t, dir)

	stdout, _, err := execCmd(t, "config", "show")
	require.NoError(t, err)
	assert.Contains(t, stdout, "vault")
	assert.Contains(t, stdout, "hashicorp-vault")
	assert.Contains(t, stdout, "address: https://vault.example.com")
	assert.Contains(t, stdout, "namespace: myteam")
}

func TestConfigShowCmd_JSONBackendWithConfig(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
backends:
  - name: vault
    type: hashicorp-vault
    config:
      address: https://vault.example.com
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	chdir(t, dir)

	stdout, _, err := execCmd(t, "config", "show", "--format", "json")
	require.NoError(t, err)

	var output configShowOutput
	err = json.Unmarshal([]byte(stdout), &output)
	require.NoError(t, err)
	require.Len(t, output.Backends, 1)
	assert.Equal(t, "vault", output.Backends[0].Name)
	assert.Equal(t, "hashicorp-vault", output.Backends[0].Type)
	assert.Equal(t, "https://vault.example.com", output.Backends[0].Config["address"])
}

func TestConfigShowCmd_Help(t *testing.T) {
	stdout, _, err := execCmd(t, "config", "show", "--help")
	require.NoError(t, err)
	assert.Contains(t, stdout, "effective configuration")
	assert.Contains(t, stdout, "--format")
}

func TestConfigCmd_Help(t *testing.T) {
	stdout, _, err := execCmd(t, "config", "--help")
	require.NoError(t, err)
	assert.Contains(t, stdout, "envref configuration")
	assert.Contains(t, stdout, "show")
}

func TestConfigShowCmd_MultipleBackends(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
backends:
  - name: keychain
    type: keychain
  - name: vault
    type: hashicorp-vault
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	chdir(t, dir)

	stdout, _, err := execCmd(t, "config", "show")
	require.NoError(t, err)

	// Both backends should appear.
	assert.Contains(t, stdout, "keychain")
	assert.Contains(t, stdout, "vault")
	assert.Contains(t, stdout, "hashicorp-vault")

	// Table output should show comma-separated backends.
	stdout, _, err = execCmd(t, "config", "show", "--format", "table")
	require.NoError(t, err)
	assert.Contains(t, stdout, "keychain, vault")
}

func TestConfigShowCmd_ProfilesSorted(t *testing.T) {
	dir := t.TempDir()
	cfgContent := `project: myapp
profiles:
  zebra:
    env_file: .env.zebra
  alpha:
    env_file: .env.alpha
  middle:
    env_file: .env.middle
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	chdir(t, dir)

	stdout, _, err := execCmd(t, "config", "show")
	require.NoError(t, err)

	// Find profile lines — profiles section should be sorted.
	lines := strings.Split(stdout, "\n")
	var profileLines []string
	inProfiles := false
	for _, line := range lines {
		if strings.HasPrefix(line, "Profiles:") {
			inProfiles = true
			continue
		}
		if inProfiles {
			if strings.HasPrefix(line, "  - ") {
				profileLines = append(profileLines, line)
			} else if line != "" {
				break
			}
		}
	}

	require.Len(t, profileLines, 3)
	assert.Contains(t, profileLines[0], "alpha")
	assert.Contains(t, profileLines[1], "middle")
	assert.Contains(t, profileLines[2], "zebra")
}

func TestConfigShowCmd_ConfigFileLocation(t *testing.T) {
	dir := t.TempDir()
	// Resolve symlinks so the path matches what filepath.Join produces
	// (on macOS /var is a symlink to /private/var).
	dir, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	cfgContent := `project: myapp
`
	writeTestFile(t, dir, config.FullFileName, cfgContent)
	chdir(t, dir)

	stdout, _, err := execCmd(t, "config", "show")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Config: "+dir+"/"+config.FullFileName)
}
