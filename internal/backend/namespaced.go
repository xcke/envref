package backend

import (
	"fmt"
	"strings"
)

// NamespacedBackend wraps a Backend and prefixes all keys with a project
// namespace to avoid collisions when multiple projects share the same
// backend (e.g., the OS keychain).
//
// Keys are stored as "<project>/<key>" in the underlying backend.
// For example, with project "myapp" and key "api_key", the underlying
// backend stores "myapp/api_key".
//
// List() returns only keys belonging to this project's namespace,
// with the prefix stripped.
type NamespacedBackend struct {
	inner   Backend
	project string
	profile string
	prefix  string
}

// NewNamespacedBackend creates a NamespacedBackend that wraps the given backend
// and scopes all operations to the specified project namespace.
//
// The project name must not be empty.
func NewNamespacedBackend(inner Backend, project string) (*NamespacedBackend, error) {
	if project == "" {
		return nil, fmt.Errorf("project name must not be empty")
	}
	if inner == nil {
		return nil, fmt.Errorf("inner backend must not be nil")
	}
	return &NamespacedBackend{
		inner:   inner,
		project: project,
		prefix:  project + "/",
	}, nil
}

// Name returns the name of the underlying backend.
func (n *NamespacedBackend) Name() string {
	return n.inner.Name()
}

// Project returns the project namespace used for key prefixing.
func (n *NamespacedBackend) Project() string {
	return n.project
}

// Get retrieves the secret value for the namespaced key.
func (n *NamespacedBackend) Get(key string) (string, error) {
	return n.inner.Get(n.prefix + key)
}

// Set stores a secret value under the namespaced key.
func (n *NamespacedBackend) Set(key, value string) error {
	return n.inner.Set(n.prefix+key, value)
}

// Delete removes the secret for the namespaced key.
func (n *NamespacedBackend) Delete(key string) error {
	return n.inner.Delete(n.prefix + key)
}

// Profile returns the profile scope, if any. Returns empty string for
// project-scoped backends.
func (n *NamespacedBackend) Profile() string {
	return n.profile
}

// List returns all secret keys in this project's namespace, with the
// project prefix stripped. Keys from other projects are excluded.
func (n *NamespacedBackend) List() ([]string, error) {
	allKeys, err := n.inner.List()
	if err != nil {
		return nil, err
	}

	var keys []string
	for _, k := range allKeys {
		if strings.HasPrefix(k, n.prefix) {
			keys = append(keys, k[len(n.prefix):])
		}
	}
	return keys, nil
}

// NewProfileNamespacedBackend creates a NamespacedBackend that scopes all
// operations to a specific project and profile. Keys are stored as
// "<project>/<profile>/<key>" in the underlying backend.
//
// Both project and profile must be non-empty.
func NewProfileNamespacedBackend(inner Backend, project, profile string) (*NamespacedBackend, error) {
	if project == "" {
		return nil, fmt.Errorf("project name must not be empty")
	}
	if profile == "" {
		return nil, fmt.Errorf("profile name must not be empty")
	}
	if inner == nil {
		return nil, fmt.Errorf("inner backend must not be nil")
	}
	return &NamespacedBackend{
		inner:   inner,
		project: project,
		profile: profile,
		prefix:  project + "/" + profile + "/",
	}, nil
}
