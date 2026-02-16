package backend

import (
	"errors"
	"fmt"
	"strings"
)

// Registry manages an ordered collection of secret backends and provides
// fallback resolution: when getting a secret, backends are tried in order
// until one returns a value.
type Registry struct {
	backends []Backend
	byName   map[string]Backend
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		byName: make(map[string]Backend),
	}
}

// Register adds a backend to the registry. Backends are tried in the order
// they are registered when resolving secrets. Returns an error if a backend
// with the same name is already registered.
func (r *Registry) Register(b Backend) error {
	name := b.Name()
	if _, exists := r.byName[name]; exists {
		return fmt.Errorf("backend %q is already registered", name)
	}
	r.backends = append(r.backends, b)
	r.byName[name] = b
	return nil
}

// Len returns the number of registered backends.
func (r *Registry) Len() int {
	return len(r.backends)
}

// Backend returns the backend with the given name, or nil if not found.
func (r *Registry) Backend(name string) Backend {
	return r.byName[name]
}

// Backends returns the ordered list of registered backends.
// The returned slice is a copy; modifying it does not affect the registry.
func (r *Registry) Backends() []Backend {
	out := make([]Backend, len(r.backends))
	copy(out, r.backends)
	return out
}

// Get retrieves a secret by trying each backend in registration order.
// Returns the value from the first backend that has the key.
// If no backend has the key, returns ErrNotFound.
// If a backend returns an error other than ErrNotFound, that error is
// returned immediately (wrapped in a KeyError).
func (r *Registry) Get(key string) (string, error) {
	for _, b := range r.backends {
		val, err := b.Get(key)
		if err == nil {
			return val, nil
		}
		if errors.Is(err, ErrNotFound) {
			continue
		}
		// Non-ErrNotFound error: stop and report.
		return "", NewKeyError(b.Name(), key, err)
	}
	return "", ErrNotFound
}

// GetFrom retrieves a secret from a specific named backend.
// Returns an error if the backend is not registered.
func (r *Registry) GetFrom(backendName, key string) (string, error) {
	b := r.byName[backendName]
	if b == nil {
		return "", fmt.Errorf("backend %q is not registered", backendName)
	}
	val, err := b.Get(key)
	if err != nil {
		return "", NewKeyError(backendName, key, err)
	}
	return val, nil
}

// SetIn stores a secret in a specific named backend.
// Returns an error if the backend is not registered.
func (r *Registry) SetIn(backendName, key, value string) error {
	b := r.byName[backendName]
	if b == nil {
		return fmt.Errorf("backend %q is not registered", backendName)
	}
	if err := b.Set(key, value); err != nil {
		return NewKeyError(backendName, key, err)
	}
	return nil
}

// DeleteFrom removes a secret from a specific named backend.
// Returns an error if the backend is not registered.
func (r *Registry) DeleteFrom(backendName, key string) error {
	b := r.byName[backendName]
	if b == nil {
		return fmt.Errorf("backend %q is not registered", backendName)
	}
	if err := b.Delete(key); err != nil {
		return NewKeyError(backendName, key, err)
	}
	return nil
}

// ListFrom returns all secret keys from a specific named backend.
// Returns an error if the backend is not registered.
func (r *Registry) ListFrom(backendName string) ([]string, error) {
	b := r.byName[backendName]
	if b == nil {
		return nil, fmt.Errorf("backend %q is not registered", backendName)
	}
	keys, err := b.List()
	if err != nil {
		return nil, fmt.Errorf("backend %q: list: %w", backendName, err)
	}
	return keys, nil
}

// Names returns the names of all registered backends in registration order.
func (r *Registry) Names() []string {
	names := make([]string, len(r.backends))
	for i, b := range r.backends {
		names[i] = b.Name()
	}
	return names
}

// String returns a human-readable description of the registry.
func (r *Registry) String() string {
	if len(r.backends) == 0 {
		return "Registry(empty)"
	}
	names := r.Names()
	return fmt.Sprintf("Registry(%s)", strings.Join(names, " → "))
}
