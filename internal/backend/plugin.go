// Package backend provides the plugin backend, which delegates secret
// operations to an external executable using a JSON-over-stdin/stdout protocol.
//
// # Plugin Protocol
//
// Plugins are external executables that implement the envref secret backend
// interface. The CLI communicates with the plugin via JSON messages sent to
// stdin and JSON responses read from stdout.
//
// Each operation launches the plugin executable with the argument "serve".
// A single JSON request is written to stdin, and a single JSON response is
// read from stdout. The plugin process exits after each request.
//
// # Request Format
//
//	{
//	  "operation": "get" | "set" | "delete" | "list",
//	  "key": "secret-key",         // present for get, set, delete
//	  "value": "secret-value"      // present for set only
//	}
//
// # Response Format
//
//	{
//	  "value": "secret-value",     // present for get
//	  "keys": ["key1", "key2"],    // present for list
//	  "error": "message"           // present on failure (empty on success)
//	}
//
// If the response contains a non-empty "error" field, the operation is
// considered failed. For "get", a response error of "not found" is mapped
// to ErrNotFound.
//
// # Plugin Discovery
//
// Plugins are discovered by convention: an executable named
// "envref-backend-<name>" on $PATH. Alternatively, the full path to the
// plugin executable can be specified in the backend config:
//
//	backends:
//	  - name: my-vault
//	    type: plugin
//	    config:
//	      command: /usr/local/bin/envref-backend-my-vault
//
// If no "command" config is specified, the plugin is discovered by searching
// $PATH for "envref-backend-<name>".
package backend

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Default timeout for plugin operations.
const pluginTimeout = 30 * time.Second

// pluginRequest is the JSON request sent to a plugin's stdin.
type pluginRequest struct {
	Operation string `json:"operation"`
	Key       string `json:"key,omitempty"`
	Value     string `json:"value,omitempty"`
}

// pluginResponse is the JSON response read from a plugin's stdout.
type pluginResponse struct {
	Value string   `json:"value,omitempty"`
	Keys  []string `json:"keys,omitempty"`
	Error string   `json:"error,omitempty"`
}

// PluginBackend delegates secret operations to an external executable.
type PluginBackend struct {
	name    string        // Backend name (used in ref:// URIs and config)
	command string        // Path to the plugin executable
	timeout time.Duration // Per-operation timeout
}

// NewPluginBackend creates a new PluginBackend that delegates to the given
// executable. The name is the backend identifier used in config and ref:// URIs.
func NewPluginBackend(name, command string) *PluginBackend {
	return &PluginBackend{
		name:    name,
		command: command,
		timeout: pluginTimeout,
	}
}

// Name returns the backend identifier.
func (p *PluginBackend) Name() string {
	return p.name
}

// Get retrieves a secret value from the plugin.
func (p *PluginBackend) Get(key string) (string, error) {
	resp, err := p.execute(pluginRequest{Operation: "get", Key: key})
	if err != nil {
		return "", err
	}
	return resp.Value, nil
}

// Set stores a secret value via the plugin.
func (p *PluginBackend) Set(key, value string) error {
	_, err := p.execute(pluginRequest{Operation: "set", Key: key, Value: value})
	return err
}

// Delete removes a secret via the plugin.
func (p *PluginBackend) Delete(key string) error {
	_, err := p.execute(pluginRequest{Operation: "delete", Key: key})
	return err
}

// List returns all secret keys from the plugin.
func (p *PluginBackend) List() ([]string, error) {
	resp, err := p.execute(pluginRequest{Operation: "list"})
	if err != nil {
		return nil, err
	}
	if resp.Keys == nil {
		return []string{}, nil
	}
	return resp.Keys, nil
}

// execute runs the plugin executable with the given request and returns the
// parsed response. It handles timeouts, exit codes, and error mapping.
func (p *PluginBackend) execute(req pluginRequest) (*pluginResponse, error) {
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("plugin %q: marshal request: %w", p.name, err)
	}

	cmd := exec.Command(p.command, "serve") //nolint:gosec // Plugin path comes from trusted config
	cmd.Stdin = bytes.NewReader(reqBytes)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Use a channel-based timeout since exec.CommandContext kills with SIGKILL.
	done := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("plugin %q: start: %w", p.name, err)
	}

	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		// Process completed.
		if err != nil {
			stderrMsg := strings.TrimSpace(stderr.String())
			if stderrMsg != "" {
				return nil, fmt.Errorf("plugin %q: %s", p.name, stderrMsg)
			}
			return nil, fmt.Errorf("plugin %q: %w", p.name, err)
		}
	case <-time.After(p.timeout):
		// Kill on timeout; ignore kill error since we're already failing.
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("plugin %q: timed out after %s", p.name, p.timeout)
	}

	// Parse response.
	var resp pluginResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("plugin %q: invalid JSON response: %w", p.name, err)
	}

	// Map plugin errors.
	if resp.Error != "" {
		if isNotFoundError(resp.Error) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("plugin %q: %s", p.name, resp.Error)
	}

	return &resp, nil
}

// isNotFoundError checks whether a plugin error message indicates a
// not-found condition. Plugins should return "not found" but we accept
// common variations.
func isNotFoundError(msg string) bool {
	lower := strings.ToLower(msg)
	return lower == "not found" ||
		lower == "secret not found" ||
		lower == "key not found" ||
		strings.Contains(lower, "not found")
}

// DiscoverPlugin searches $PATH for an executable named
// "envref-backend-<name>" and returns its absolute path.
// Returns an error if the plugin cannot be found.
func DiscoverPlugin(name string) (string, error) {
	binName := "envref-backend-" + name
	path, err := exec.LookPath(binName)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("plugin %q: executable %q not found on $PATH", name, binName)
		}
		return "", fmt.Errorf("plugin %q: %w", name, err)
	}
	return path, nil
}
