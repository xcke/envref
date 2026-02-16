// Package backend provides the HashiCorp Vault backend, which delegates
// secret operations to the Vault CLI (`vault` subcommands).
//
// # Prerequisites
//
// The Vault CLI must be installed and authenticated:
//
//	brew install vault        # or see https://developer.hashicorp.com/vault/install
//	vault login               # or set VAULT_TOKEN / use app role
//
// # Configuration
//
// In .envref.yaml:
//
//	backends:
//	  - name: hcvault
//	    type: hashicorp-vault
//	    config:
//	      mount: secret              # KV v2 secrets engine mount (default: "secret")
//	      prefix: envref             # path prefix within the mount (default: "envref")
//	      addr: https://vault.example.com:8200  # Vault server address (optional, uses VAULT_ADDR)
//	      namespace: admin           # Vault enterprise namespace (optional)
//	      token: s.xxx              # Vault token (optional, uses VAULT_TOKEN)
//
// # How secrets are stored
//
// Secrets are stored in a KV v2 secrets engine. Each secret is a KV entry
// at "<mount>/data/<prefix>/<key>" with the value stored under the "value"
// field. For example, with the default mount "secret" and prefix "envref",
// the key "api_key" is stored at "secret/data/envref/api_key".
package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Default timeout for Vault CLI operations.
const hashiVaultTimeout = 30 * time.Second

// HashiVaultBackend stores secrets in HashiCorp Vault via the `vault` CLI.
// Secrets are stored in a KV v2 secrets engine under a configurable mount
// and path prefix.
type HashiVaultBackend struct {
	mount     string        // KV v2 mount path (e.g., "secret")
	prefix    string        // path prefix within the mount (e.g., "envref")
	addr      string        // optional Vault server address
	namespace string        // optional Vault enterprise namespace
	token     string        // optional Vault token
	command   string        // path to the vault CLI executable
	timeout   time.Duration // max time per CLI invocation
}

// HashiVaultOption configures optional settings for HashiVaultBackend.
type HashiVaultOption func(*HashiVaultBackend)

// WithHashiVaultAddr sets the Vault server address.
func WithHashiVaultAddr(addr string) HashiVaultOption {
	return func(b *HashiVaultBackend) {
		b.addr = addr
	}
}

// WithHashiVaultNamespace sets the Vault enterprise namespace.
func WithHashiVaultNamespace(namespace string) HashiVaultOption {
	return func(b *HashiVaultBackend) {
		b.namespace = namespace
	}
}

// WithHashiVaultToken sets the Vault authentication token.
func WithHashiVaultToken(token string) HashiVaultOption {
	return func(b *HashiVaultBackend) {
		b.token = token
	}
}

// WithHashiVaultCommand overrides the path to the vault CLI executable.
func WithHashiVaultCommand(command string) HashiVaultOption {
	return func(b *HashiVaultBackend) {
		b.command = command
	}
}

// NewHashiVaultBackend creates a new HashiVaultBackend that delegates to the
// `vault` CLI. The mount parameter specifies the KV v2 secrets engine mount
// path, and prefix specifies the path prefix within the mount.
func NewHashiVaultBackend(mount, prefix string, opts ...HashiVaultOption) *HashiVaultBackend {
	b := &HashiVaultBackend{
		mount:   mount,
		prefix:  prefix,
		command: "vault",
		timeout: hashiVaultTimeout,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Name returns "hashicorp-vault", the identifier used in .envref.yaml
// configuration and ref:// URIs.
func (b *HashiVaultBackend) Name() string {
	return "hashicorp-vault"
}

// vaultKVGetResponse represents the relevant fields from `vault kv get -format=json`.
type vaultKVGetResponse struct {
	Data struct {
		Data map[string]interface{} `json:"data"`
	} `json:"data"`
}

// vaultKVListResponse represents the response from `vault kv list -format=json`.
type vaultKVListResponse struct {
	Data struct {
		Keys []string `json:"keys"`
	} `json:"data"`
}

// secretPath returns the full KV path for a given key (without the mount).
func (b *HashiVaultBackend) secretPath(key string) string {
	if b.prefix == "" {
		return key
	}
	return b.prefix + "/" + key
}

// Get retrieves the secret value for the given key from HashiCorp Vault.
// Returns ErrNotFound if no secret with that path exists.
func (b *HashiVaultBackend) Get(key string) (string, error) {
	args := []string{
		"kv", "get",
		"-mount=" + b.mount,
		"-format=json",
		b.secretPath(key),
	}
	args = b.appendGlobalFlags(args)

	stdout, err := b.run(args)
	if err != nil {
		if isHashiVaultNotFoundErr(err) {
			return "", ErrNotFound
		}
		return "", NewKeyError(b.Name(), key, fmt.Errorf("vault kv get: %w", err))
	}

	var result vaultKVGetResponse
	if err := json.Unmarshal(stdout, &result); err != nil {
		return "", NewKeyError(b.Name(), key, fmt.Errorf("parse response: %w", err))
	}

	val, ok := result.Data.Data["value"]
	if !ok {
		return "", NewKeyError(b.Name(), key, fmt.Errorf("secret has no \"value\" field"))
	}

	strVal, ok := val.(string)
	if !ok {
		return "", NewKeyError(b.Name(), key, fmt.Errorf("secret \"value\" field is not a string"))
	}

	return strVal, nil
}

// Set stores a secret value under the given key in HashiCorp Vault.
// If a secret at that path already exists, it is overwritten (creating a
// new version in KV v2).
func (b *HashiVaultBackend) Set(key, value string) error {
	args := []string{
		"kv", "put",
		"-mount=" + b.mount,
		"-format=json",
		b.secretPath(key),
		"value=" + value,
	}
	args = b.appendGlobalFlags(args)

	if _, err := b.run(args); err != nil {
		return NewKeyError(b.Name(), key, fmt.Errorf("vault kv put: %w", err))
	}
	return nil
}

// Delete removes the secret for the given key from HashiCorp Vault.
// This performs a metadata delete, permanently removing all versions.
// Returns ErrNotFound if no secret with that path exists.
func (b *HashiVaultBackend) Delete(key string) error {
	args := []string{
		"kv", "metadata", "delete",
		"-mount=" + b.mount,
		b.secretPath(key),
	}
	args = b.appendGlobalFlags(args)

	if _, err := b.run(args); err != nil {
		if isHashiVaultNotFoundErr(err) {
			return ErrNotFound
		}
		return NewKeyError(b.Name(), key, fmt.Errorf("vault kv metadata delete: %w", err))
	}
	return nil
}

// List returns all secret keys under the configured prefix.
// The prefix is stripped from the returned keys.
func (b *HashiVaultBackend) List() ([]string, error) {
	path := b.prefix
	if path == "" {
		path = "/"
	}

	args := []string{
		"kv", "list",
		"-mount=" + b.mount,
		"-format=json",
		path,
	}
	args = b.appendGlobalFlags(args)

	stdout, err := b.run(args)
	if err != nil {
		// An empty list path returns a "not found" error in Vault.
		if isHashiVaultNotFoundErr(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("hashicorp-vault list: %w", err)
	}

	var result vaultKVListResponse
	if err := json.Unmarshal(stdout, &result); err != nil {
		return nil, fmt.Errorf("hashicorp-vault list: parse response: %w", err)
	}

	if len(result.Data.Keys) == 0 {
		return []string{}, nil
	}
	return result.Data.Keys, nil
}

// appendGlobalFlags adds -address, -namespace, and environment-override flags
// if configured.
func (b *HashiVaultBackend) appendGlobalFlags(args []string) []string {
	if b.addr != "" {
		args = append(args, "-address="+b.addr)
	}
	if b.namespace != "" {
		args = append(args, "-namespace="+b.namespace)
	}
	return args
}

// run executes the vault CLI with the given arguments and returns stdout.
func (b *HashiVaultBackend) run(args []string) ([]byte, error) {
	cmd := exec.Command(b.command, args...) //nolint:gosec // Command path comes from trusted config or default "vault"

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set VAULT_TOKEN env var if configured, inheriting the rest of the
	// parent environment (which includes VAULT_ADDR, VAULT_TOKEN, etc.
	// from the user's shell).
	if b.token != "" {
		cmd.Env = append(cmd.Environ(), "VAULT_TOKEN="+b.token)
	}

	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start vault: %w", err)
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
		return nil, fmt.Errorf("vault cli timed out after %s", b.timeout)
	}

	return stdout.Bytes(), nil
}

// isHashiVaultNotFoundErr checks whether an error from the Vault CLI indicates
// that a secret or path was not found.
func isHashiVaultNotFoundErr(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no value found") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "no secrets") ||
		strings.Contains(msg, "invalid path")
}
