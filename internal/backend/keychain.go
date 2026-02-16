// Package backend provides secret storage backend implementations.
//
// This file implements the OS keychain backend using the go-keyring library,
// which provides cross-platform support for:
//   - macOS: Keychain
//   - Linux: libsecret / Secret Service (GNOME Keyring, KWallet)
//   - Windows: Credential Manager
package backend

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/zalando/go-keyring"
)

// keychainServicePrefix is the prefix used for the keyring service name
// to avoid collisions with other applications.
const keychainServicePrefix = "envref"

// keychainIndexKey is the special key used to store the list of all
// secret keys in the keychain. Since go-keyring does not support
// enumeration, we maintain a JSON-encoded index.
const keychainIndexKey = "__envref_key_index__"

// KeychainBackend stores secrets in the OS keychain using the go-keyring
// library. It works on macOS (Keychain), Linux (libsecret/Secret Service),
// and Windows (Credential Manager).
//
// Because go-keyring does not support listing keys, KeychainBackend
// maintains a separate index entry that tracks all stored keys as a
// JSON array. This index is updated atomically with each Set/Delete.
type KeychainBackend struct {
	service string
	mu      sync.Mutex
}

// keyringProvider abstracts the go-keyring functions for testing.
// In production, these point to the real go-keyring package functions.
// In tests, they can be replaced with mock implementations.
var keyringProvider = struct {
	Set    func(service, user, password string) error
	Get    func(service, user string) (string, error)
	Delete func(service, user string) error
}{
	Set:    keyring.Set,
	Get:    keyring.Get,
	Delete: keyring.Delete,
}

// NewKeychainBackend creates a new KeychainBackend. The service name
// is derived from the keychainServicePrefix to namespace envref secrets
// in the OS keychain.
func NewKeychainBackend() *KeychainBackend {
	return &KeychainBackend{
		service: keychainServicePrefix,
	}
}

// Name returns "keychain", the identifier used in .envref.yaml configuration
// and ref:// URIs.
func (k *KeychainBackend) Name() string {
	return "keychain"
}

// Get retrieves the secret value for the given key from the OS keychain.
// Returns ErrNotFound if the key does not exist.
func (k *KeychainBackend) Get(key string) (string, error) {
	val, err := keyringProvider.Get(k.service, key)
	if err != nil {
		if isNotFoundErr(err) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("keychain get %q: %w", key, err)
	}
	return val, nil
}

// Set stores a secret value under the given key in the OS keychain.
// If the key already exists, its value is overwritten. The key index
// is updated to include the new key.
func (k *KeychainBackend) Set(key, value string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if err := keyringProvider.Set(k.service, key, value); err != nil {
		return fmt.Errorf("keychain set %q: %w", key, err)
	}

	// Update the key index.
	if err := k.addToIndex(key); err != nil {
		// Best effort: the secret is stored but the index update failed.
		// Attempt to clean up by deleting the secret.
		_ = keyringProvider.Delete(k.service, key)
		return fmt.Errorf("keychain update index after set %q: %w", key, err)
	}

	return nil
}

// Delete removes the secret for the given key from the OS keychain.
// Returns ErrNotFound if the key does not exist.
func (k *KeychainBackend) Delete(key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	err := keyringProvider.Delete(k.service, key)
	if err != nil {
		if isNotFoundErr(err) {
			return ErrNotFound
		}
		return fmt.Errorf("keychain delete %q: %w", key, err)
	}

	// Update the key index.
	if err := k.removeFromIndex(key); err != nil {
		return fmt.Errorf("keychain update index after delete %q: %w", key, err)
	}

	return nil
}

// List returns all secret keys stored in this backend by reading the
// key index. The returned keys are sorted alphabetically.
func (k *KeychainBackend) List() ([]string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.loadIndex()
}

// loadIndex reads the key index from the keychain. Returns an empty slice
// if the index does not exist yet.
func (k *KeychainBackend) loadIndex() ([]string, error) {
	data, err := keyringProvider.Get(k.service, keychainIndexKey)
	if err != nil {
		if isNotFoundErr(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("keychain load index: %w", err)
	}

	var keys []string
	if err := json.Unmarshal([]byte(data), &keys); err != nil {
		return nil, fmt.Errorf("keychain parse index: %w", err)
	}
	return keys, nil
}

// saveIndex writes the key index to the keychain as a JSON array.
func (k *KeychainBackend) saveIndex(keys []string) error {
	data, err := json.Marshal(keys)
	if err != nil {
		return fmt.Errorf("keychain marshal index: %w", err)
	}
	if err := keyringProvider.Set(k.service, keychainIndexKey, string(data)); err != nil {
		return fmt.Errorf("keychain save index: %w", err)
	}
	return nil
}

// addToIndex adds a key to the index if it's not already present.
func (k *KeychainBackend) addToIndex(key string) error {
	keys, err := k.loadIndex()
	if err != nil {
		return err
	}

	// Check if already present.
	for _, existing := range keys {
		if existing == key {
			return nil
		}
	}

	keys = append(keys, key)
	sort.Strings(keys)
	return k.saveIndex(keys)
}

// removeFromIndex removes a key from the index.
func (k *KeychainBackend) removeFromIndex(key string) error {
	keys, err := k.loadIndex()
	if err != nil {
		return err
	}

	filtered := make([]string, 0, len(keys))
	for _, existing := range keys {
		if existing != key {
			filtered = append(filtered, existing)
		}
	}

	return k.saveIndex(filtered)
}

// isNotFoundErr checks whether an error from go-keyring indicates that the
// requested key was not found. go-keyring returns keyring.ErrNotFound for
// this case.
func isNotFoundErr(err error) bool {
	return err == keyring.ErrNotFound
}
