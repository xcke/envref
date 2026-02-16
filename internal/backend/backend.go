// Package backend defines the interface for secret storage backends and
// provides common error types used across all backend implementations.
//
// A Backend stores and retrieves secret values keyed by string identifiers.
// Backends are configured in .envref.yaml and resolved in order when
// processing ref:// references in .env files.
//
// Example backends include OS keychain, local encrypted vault, 1Password CLI,
// and AWS SSM Parameter Store.
package backend

import (
	"errors"
	"fmt"
)

// Backend is the interface that secret storage backends must implement.
// Each backend manages secrets identified by string keys within a given
// project namespace.
type Backend interface {
	// Name returns the unique identifier for this backend (e.g., "keychain",
	// "vault", "1password"). This must match the backend name used in
	// .envref.yaml configuration and ref:// URIs.
	Name() string

	// Get retrieves the secret value for the given key.
	// Returns ErrNotFound if the key does not exist in this backend.
	Get(key string) (string, error)

	// Set stores a secret value under the given key, creating or overwriting
	// as needed.
	Set(key, value string) error

	// Delete removes the secret for the given key.
	// Returns ErrNotFound if the key does not exist in this backend.
	Delete(key string) error

	// List returns all secret keys stored in this backend.
	// The returned keys are in no guaranteed order.
	List() ([]string, error)
}

// ErrNotFound is returned when a requested secret key does not exist
// in a backend.
var ErrNotFound = errors.New("secret not found")

// KeyError records an error associated with a specific secret key.
type KeyError struct {
	// Backend is the name of the backend that produced the error.
	Backend string
	// Key is the secret key that caused the error.
	Key string
	// Err is the underlying error.
	Err error
}

// Error returns a human-readable error message including backend and key context.
func (e *KeyError) Error() string {
	return fmt.Sprintf("backend %q: key %q: %v", e.Backend, e.Key, e.Err)
}

// Unwrap returns the underlying error for use with errors.Is and errors.As.
func (e *KeyError) Unwrap() error {
	return e.Err
}

// NewKeyError creates a KeyError for the given backend, key, and underlying error.
func NewKeyError(backend, key string, err error) *KeyError {
	return &KeyError{
		Backend: backend,
		Key:     key,
		Err:     err,
	}
}
