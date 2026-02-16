// Package backend provides the 1Password CLI backend, which delegates secret
// operations to the `op` command-line tool.
//
// # Prerequisites
//
// The 1Password CLI (v2+) must be installed and authenticated:
//
//	brew install 1password-cli   # or see https://1password.com/downloads/command-line/
//	op signin
//
// # Configuration
//
// In .envref.yaml:
//
//	backends:
//	  - name: op
//	    type: 1password
//	    config:
//	      vault: Personal          # 1Password vault name (default: "Personal")
//	      account: my.1password.com # optional: account shorthand or URL
//
// # How secrets are stored
//
// Secrets are stored as 1Password "Secure Note" items. The item title is the
// secret key (namespaced by the project, e.g., "my-app/api_key"). The secret
// value is stored in the "notesPlain" field.
//
// This approach avoids schema assumptions about Login/Password item types and
// provides a simple key-value mapping that aligns with the Backend interface.
package backend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Default timeout for 1Password CLI operations.
const opTimeout = 30 * time.Second

// opCategory is the 1Password item category used for stored secrets.
const opCategory = "Secure Note"

// OnePasswordBackend stores secrets in 1Password via the `op` CLI (v2+).
// Each secret is a "Secure Note" item whose title is the secret key
// and whose "notesPlain" field holds the secret value.
type OnePasswordBackend struct {
	vault   string // 1Password vault name
	account string // optional account shorthand or URL
	command string // path to the op CLI executable
	timeout time.Duration
}

// OnePasswordOption configures optional settings for OnePasswordBackend.
type OnePasswordOption func(*OnePasswordBackend)

// WithOnePasswordAccount sets the 1Password account shorthand or URL.
func WithOnePasswordAccount(account string) OnePasswordOption {
	return func(o *OnePasswordBackend) {
		o.account = account
	}
}

// WithOnePasswordCommand overrides the path to the op CLI executable.
func WithOnePasswordCommand(command string) OnePasswordOption {
	return func(o *OnePasswordBackend) {
		o.command = command
	}
}

// NewOnePasswordBackend creates a new OnePasswordBackend that delegates to
// the `op` CLI. The vault parameter specifies which 1Password vault to use.
func NewOnePasswordBackend(vault string, opts ...OnePasswordOption) *OnePasswordBackend {
	b := &OnePasswordBackend{
		vault:   vault,
		command: "op",
		timeout: opTimeout,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Name returns "1password", the identifier used in .envref.yaml configuration
// and ref:// URIs.
func (o *OnePasswordBackend) Name() string {
	return "1password"
}

// opItem represents the relevant fields of a 1Password item returned by
// `op item get --format json`.
type opItem struct {
	ID    string    `json:"id"`
	Title string    `json:"title"`
	Fields []opField `json:"fields,omitempty"`
}

// opField represents a field within a 1Password item.
type opField struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

// opListItem represents the minimal fields returned by `op item list --format json`.
type opListItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Get retrieves the secret value for the given key from 1Password.
// Returns ErrNotFound if no item with that title exists in the vault.
func (o *OnePasswordBackend) Get(key string) (string, error) {
	args := []string{"item", "get", key, "--vault", o.vault, "--format", "json"}
	args = o.appendAccountFlag(args)

	stdout, err := o.run(args)
	if err != nil {
		if isOpNotFoundErr(err) {
			return "", ErrNotFound
		}
		return "", NewKeyError(o.Name(), key, fmt.Errorf("op get: %w", err))
	}

	var item opItem
	if err := json.Unmarshal(stdout, &item); err != nil {
		return "", NewKeyError(o.Name(), key, fmt.Errorf("parse response: %w", err))
	}

	// Extract the value from the notesPlain field.
	for _, f := range item.Fields {
		if f.ID == "notesPlain" || f.Label == "notesPlain" {
			return f.Value, nil
		}
	}

	return "", NewKeyError(o.Name(), key, fmt.Errorf("item has no notesPlain field"))
}

// Set stores a secret value under the given key in 1Password.
// If an item with that title already exists, it is updated. Otherwise,
// a new Secure Note item is created.
func (o *OnePasswordBackend) Set(key, value string) error {
	// Try to update the existing item first.
	editArgs := []string{
		"item", "edit", key,
		"--vault", o.vault,
		"notesPlain=" + value,
	}
	editArgs = o.appendAccountFlag(editArgs)

	_, err := o.run(editArgs)
	if err == nil {
		return nil
	}

	// If the item doesn't exist, create a new one.
	if !isOpNotFoundErr(err) {
		return NewKeyError(o.Name(), key, fmt.Errorf("op edit: %w", err))
	}

	createArgs := []string{
		"item", "create",
		"--category", opCategory,
		"--title", key,
		"--vault", o.vault,
		"notesPlain=" + value,
	}
	createArgs = o.appendAccountFlag(createArgs)

	if _, err := o.run(createArgs); err != nil {
		return NewKeyError(o.Name(), key, fmt.Errorf("op create: %w", err))
	}
	return nil
}

// Delete removes the secret for the given key from 1Password.
// Returns ErrNotFound if no item with that title exists in the vault.
func (o *OnePasswordBackend) Delete(key string) error {
	args := []string{"item", "delete", key, "--vault", o.vault}
	args = o.appendAccountFlag(args)

	_, err := o.run(args)
	if err != nil {
		if isOpNotFoundErr(err) {
			return ErrNotFound
		}
		return NewKeyError(o.Name(), key, fmt.Errorf("op delete: %w", err))
	}
	return nil
}

// List returns all secret keys (item titles) in the configured vault.
func (o *OnePasswordBackend) List() ([]string, error) {
	args := []string{
		"item", "list",
		"--vault", o.vault,
		"--categories", opCategory,
		"--format", "json",
	}
	args = o.appendAccountFlag(args)

	stdout, err := o.run(args)
	if err != nil {
		return nil, fmt.Errorf("1password list: %w", err)
	}

	// Handle empty vault (op may return empty string or empty array).
	trimmed := bytes.TrimSpace(stdout)
	if len(trimmed) == 0 {
		return []string{}, nil
	}

	var items []opListItem
	if err := json.Unmarshal(trimmed, &items); err != nil {
		return nil, fmt.Errorf("1password list: parse response: %w", err)
	}

	keys := make([]string, 0, len(items))
	for _, item := range items {
		keys = append(keys, item.Title)
	}
	return keys, nil
}

// appendAccountFlag adds the --account flag if an account is configured.
func (o *OnePasswordBackend) appendAccountFlag(args []string) []string {
	if o.account != "" {
		args = append(args, "--account", o.account)
	}
	return args
}

// run executes the op CLI with the given arguments and returns stdout.
// It handles timeouts and maps common error patterns.
func (o *OnePasswordBackend) run(args []string) ([]byte, error) {
	cmd := exec.Command(o.command, args...) //nolint:gosec // Command path comes from trusted config or default "op"

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start op: %w", err)
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
	case <-time.After(o.timeout):
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("op timed out after %s", o.timeout)
	}

	return stdout.Bytes(), nil
}

// isOpNotFoundErr checks whether an error from the op CLI indicates that
// the requested item was not found. The op CLI v2 prints "[ERROR] ..."
// messages to stderr with patterns like "isn't an item" or "not found".
func isOpNotFoundErr(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "isn't an item") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "could not find") ||
		strings.Contains(msg, "no item found")
}
