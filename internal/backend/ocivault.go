// Package backend provides the Oracle OCI Vault backend, which delegates
// secret operations to the OCI CLI (`oci vault secret` subcommands).
//
// # Prerequisites
//
// The OCI CLI must be installed and configured:
//
//	pip install oci-cli         # or see https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/cliinstall.htm
//	oci setup config            # configure credentials
//
// # Configuration
//
// In .envref.yaml:
//
//	backends:
//	  - name: oci
//	    type: oci-vault
//	    config:
//	      vault_id: ocid1.vault.oc1...     # OCI vault OCID (required)
//	      compartment_id: ocid1.compartment.oc1... # compartment OCID (required)
//	      key_id: ocid1.key.oc1...         # master encryption key OCID (required for set)
//	      profile: DEFAULT                  # OCI CLI config profile (optional)
//
// # How secrets are stored
//
// Secrets are stored in OCI Vault as secret bundles. The secret name is the
// key (namespaced by the project, e.g., "my-app/api_key"). The secret value
// is base64-encoded when stored and decoded on retrieval.
//
// OCI Vault does not support true deletion — secrets are scheduled for deletion
// with a pending period. Delete operations schedule deletion with the minimum
// pending period.
package backend

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Default timeout for OCI CLI operations.
const ociTimeout = 30 * time.Second

// OCIVaultBackend stores secrets in Oracle Cloud Infrastructure Vault
// via the `oci` CLI. Each secret is stored as a vault secret bundle
// with its content base64-encoded.
type OCIVaultBackend struct {
	vaultID       string        // OCI vault OCID
	compartmentID string        // OCI compartment OCID
	keyID         string        // master encryption key OCID
	profile       string        // optional OCI CLI config profile
	command       string        // path to the oci CLI executable
	timeout       time.Duration // max time per CLI invocation
}

// OCIVaultOption configures optional settings for OCIVaultBackend.
type OCIVaultOption func(*OCIVaultBackend)

// WithOCIVaultProfile sets the OCI CLI config profile.
func WithOCIVaultProfile(profile string) OCIVaultOption {
	return func(b *OCIVaultBackend) {
		b.profile = profile
	}
}

// WithOCIVaultCommand overrides the path to the oci CLI executable.
func WithOCIVaultCommand(command string) OCIVaultOption {
	return func(b *OCIVaultBackend) {
		b.command = command
	}
}

// NewOCIVaultBackend creates a new OCIVaultBackend that delegates to the `oci` CLI.
// The vaultID, compartmentID, and keyID are required OCI resource identifiers.
func NewOCIVaultBackend(vaultID, compartmentID, keyID string, opts ...OCIVaultOption) *OCIVaultBackend {
	b := &OCIVaultBackend{
		vaultID:       vaultID,
		compartmentID: compartmentID,
		keyID:         keyID,
		command:       "oci",
		timeout:       ociTimeout,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Name returns "oci-vault", the identifier used in .envref.yaml configuration
// and ref:// URIs.
func (b *OCIVaultBackend) Name() string {
	return "oci-vault"
}

// ociSecretBundle represents the relevant fields from
// `oci secrets secret-bundle get`.
type ociSecretBundle struct {
	Data struct {
		SecretBundleContent struct {
			Content     string `json:"content"`
			ContentType string `json:"content-type"`
		} `json:"secret-bundle-content"`
	} `json:"data"`
}

// ociSecretSummary represents a secret in `oci vault secret list` output.
type ociSecretSummary struct {
	Data []struct {
		ID         string `json:"id"`
		SecretName string `json:"secret-name"`
		State      string `json:"lifecycle-state"`
	} `json:"data"`
}

// Get retrieves the secret value for the given key from OCI Vault.
// Returns ErrNotFound if no secret with that name exists.
func (b *OCIVaultBackend) Get(key string) (string, error) {
	// First, find the secret OCID by name.
	secretID, err := b.findSecretID(key)
	if err != nil {
		return "", err
	}

	// Get the secret bundle content.
	args := []string{
		"secrets", "secret-bundle", "get",
		"--secret-id", secretID,
		"--stage", "CURRENT",
		"--output", "json",
	}
	args = b.appendGlobalFlags(args)

	stdout, err := b.run(args)
	if err != nil {
		return "", NewKeyError(b.Name(), key, fmt.Errorf("oci get secret bundle: %w", err))
	}

	var bundle ociSecretBundle
	if err := json.Unmarshal(stdout, &bundle); err != nil {
		return "", NewKeyError(b.Name(), key, fmt.Errorf("parse response: %w", err))
	}

	// Decode the base64-encoded content.
	decoded, err := base64.StdEncoding.DecodeString(bundle.Data.SecretBundleContent.Content)
	if err != nil {
		return "", NewKeyError(b.Name(), key, fmt.Errorf("decode secret content: %w", err))
	}

	return string(decoded), nil
}

// Set stores a secret value under the given key in OCI Vault.
// If a secret with that name already exists, a new version is created.
// Otherwise, a new secret is created.
func (b *OCIVaultBackend) Set(key, value string) error {
	encoded := base64.StdEncoding.EncodeToString([]byte(value))

	// Check if the secret already exists.
	secretID, err := b.findSecretID(key)
	if err != nil && err != ErrNotFound {
		return err
	}

	if secretID != "" {
		// Update existing secret with a new version.
		args := []string{
			"vault", "secret", "update-base64",
			"--secret-id", secretID,
			"--secret-content-content", encoded,
			"--output", "json",
		}
		args = b.appendGlobalFlags(args)

		if _, err := b.run(args); err != nil {
			return NewKeyError(b.Name(), key, fmt.Errorf("oci update secret: %w", err))
		}
		return nil
	}

	// Create new secret.
	args := []string{
		"vault", "secret", "create-base64",
		"--vault-id", b.vaultID,
		"--compartment-id", b.compartmentID,
		"--key-id", b.keyID,
		"--secret-name", key,
		"--secret-content-content", encoded,
		"--output", "json",
	}
	args = b.appendGlobalFlags(args)

	if _, err := b.run(args); err != nil {
		return NewKeyError(b.Name(), key, fmt.Errorf("oci create secret: %w", err))
	}
	return nil
}

// Delete schedules the secret for deletion in OCI Vault. OCI Vault does not
// support immediate deletion — secrets are scheduled with the minimum
// pending period. Returns ErrNotFound if no secret with that name exists.
func (b *OCIVaultBackend) Delete(key string) error {
	secretID, err := b.findSecretID(key)
	if err != nil {
		return err
	}

	args := []string{
		"vault", "secret", "schedule-secret-deletion",
		"--secret-id", secretID,
		"--output", "json",
	}
	args = b.appendGlobalFlags(args)

	if _, err := b.run(args); err != nil {
		return NewKeyError(b.Name(), key, fmt.Errorf("oci delete secret: %w", err))
	}
	return nil
}

// List returns all active secret keys in the configured vault and compartment.
func (b *OCIVaultBackend) List() ([]string, error) {
	args := []string{
		"vault", "secret", "list",
		"--compartment-id", b.compartmentID,
		"--vault-id", b.vaultID,
		"--lifecycle-state", "ACTIVE",
		"--output", "json",
	}
	args = b.appendGlobalFlags(args)

	stdout, err := b.run(args)
	if err != nil {
		return nil, fmt.Errorf("oci-vault list: %w", err)
	}

	var result ociSecretSummary
	if err := json.Unmarshal(stdout, &result); err != nil {
		return nil, fmt.Errorf("oci-vault list: parse response: %w", err)
	}

	keys := make([]string, 0, len(result.Data))
	for _, s := range result.Data {
		keys = append(keys, s.SecretName)
	}
	return keys, nil
}

// findSecretID looks up the OCID for a secret by name.
// Returns ErrNotFound if no active secret with that name exists.
func (b *OCIVaultBackend) findSecretID(key string) (string, error) {
	args := []string{
		"vault", "secret", "list",
		"--compartment-id", b.compartmentID,
		"--vault-id", b.vaultID,
		"--name", key,
		"--lifecycle-state", "ACTIVE",
		"--output", "json",
	}
	args = b.appendGlobalFlags(args)

	stdout, err := b.run(args)
	if err != nil {
		if isOCINotFoundErr(err) {
			return "", ErrNotFound
		}
		return "", NewKeyError(b.Name(), key, fmt.Errorf("oci list secrets: %w", err))
	}

	var result ociSecretSummary
	if err := json.Unmarshal(stdout, &result); err != nil {
		return "", NewKeyError(b.Name(), key, fmt.Errorf("parse response: %w", err))
	}

	if len(result.Data) == 0 {
		return "", ErrNotFound
	}

	return result.Data[0].ID, nil
}

// appendGlobalFlags adds --profile flag if configured.
func (b *OCIVaultBackend) appendGlobalFlags(args []string) []string {
	if b.profile != "" {
		args = append(args, "--profile", b.profile)
	}
	return args
}

// run executes the oci CLI with the given arguments and returns stdout.
func (b *OCIVaultBackend) run(args []string) ([]byte, error) {
	cmd := exec.Command(b.command, args...) //nolint:gosec // Command path comes from trusted config or default "oci"

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start oci: %w", err)
	}

	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			stderrMsg := strings.TrimSpace(stderr.String())
			if stderrMsg != "" {
				return nil, fmt.Errorf("%s", stderrMsg)
			}
			return nil, err
		}
	case <-time.After(b.timeout):
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("oci cli timed out after %s", b.timeout)
	}

	return stdout.Bytes(), nil
}

// isOCINotFoundErr checks whether an error from the OCI CLI indicates that
// a resource was not found.
func isOCINotFoundErr(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") ||
		strings.Contains(msg, "404") ||
		strings.Contains(msg, "notfound")
}
