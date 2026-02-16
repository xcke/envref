package backend

import (
	"errors"
	"fmt"
	"sort"
	"testing"
)

// memoryBackend is an in-memory Backend implementation used for testing.
type memoryBackend struct {
	name    string
	secrets map[string]string
}

func newMemoryBackend(name string) *memoryBackend {
	return &memoryBackend{
		name:    name,
		secrets: make(map[string]string),
	}
}

func (m *memoryBackend) Name() string { return m.name }

func (m *memoryBackend) Get(key string) (string, error) {
	v, ok := m.secrets[key]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (m *memoryBackend) Set(key, value string) error {
	m.secrets[key] = value
	return nil
}

func (m *memoryBackend) Delete(key string) error {
	if _, ok := m.secrets[key]; !ok {
		return ErrNotFound
	}
	delete(m.secrets, key)
	return nil
}

func (m *memoryBackend) List() ([]string, error) {
	keys := make([]string, 0, len(m.secrets))
	for k := range m.secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

// TestBackendInterface verifies that memoryBackend satisfies the Backend interface.
func TestBackendInterface(t *testing.T) {
	var _ Backend = newMemoryBackend("test")
}

func TestMemoryBackend_GetSetDelete(t *testing.T) {
	b := newMemoryBackend("test")

	// Get on missing key returns ErrNotFound.
	_, err := b.Get("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(missing): got %v, want ErrNotFound", err)
	}

	// Set and Get.
	if err := b.Set("api_key", "secret123"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	val, err := b.Get("api_key")
	if err != nil {
		t.Fatalf("Get after Set: %v", err)
	}
	if val != "secret123" {
		t.Fatalf("Get after Set: got %q, want %q", val, "secret123")
	}

	// Overwrite.
	if err := b.Set("api_key", "updated"); err != nil {
		t.Fatalf("Set overwrite: %v", err)
	}
	val, err = b.Get("api_key")
	if err != nil {
		t.Fatalf("Get after overwrite: %v", err)
	}
	if val != "updated" {
		t.Fatalf("Get after overwrite: got %q, want %q", val, "updated")
	}

	// Delete.
	if err := b.Delete("api_key"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = b.Get("api_key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete: got %v, want ErrNotFound", err)
	}

	// Delete on missing key returns ErrNotFound.
	err = b.Delete("api_key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete(missing): got %v, want ErrNotFound", err)
	}
}

func TestMemoryBackend_List(t *testing.T) {
	b := newMemoryBackend("test")

	// Empty backend.
	keys, err := b.List()
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("List empty: got %v, want empty", keys)
	}

	// Add some keys.
	for _, k := range []string{"zebra", "alpha", "middle"} {
		if err := b.Set(k, "val"); err != nil {
			t.Fatalf("Set(%s): %v", k, err)
		}
	}

	keys, err = b.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{"alpha", "middle", "zebra"}
	if len(keys) != len(want) {
		t.Fatalf("List: got %v, want %v", keys, want)
	}
	for i, k := range keys {
		if k != want[i] {
			t.Fatalf("List[%d]: got %q, want %q", i, k, want[i])
		}
	}
}

func TestMemoryBackend_Name(t *testing.T) {
	b := newMemoryBackend("keychain")
	if b.Name() != "keychain" {
		t.Fatalf("Name: got %q, want %q", b.Name(), "keychain")
	}
}

func TestKeyError(t *testing.T) {
	tests := []struct {
		name    string
		backend string
		key     string
		err     error
		want    string
	}{
		{
			name:    "not found",
			backend: "keychain",
			key:     "api_key",
			err:     ErrNotFound,
			want:    `backend "keychain": key "api_key": secret not found`,
		},
		{
			name:    "wrapped error",
			backend: "vault",
			key:     "db_pass",
			err:     fmt.Errorf("connection refused"),
			want:    `backend "vault": key "db_pass": connection refused`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kerr := NewKeyError(tt.backend, tt.key, tt.err)
			if kerr.Error() != tt.want {
				t.Fatalf("Error(): got %q, want %q", kerr.Error(), tt.want)
			}
		})
	}
}

func TestKeyError_Unwrap(t *testing.T) {
	kerr := NewKeyError("keychain", "api_key", ErrNotFound)
	if !errors.Is(kerr, ErrNotFound) {
		t.Fatal("errors.Is(KeyError, ErrNotFound) should be true")
	}

	var target *KeyError
	if !errors.As(kerr, &target) {
		t.Fatal("errors.As(KeyError, *KeyError) should be true")
	}
	if target.Backend != "keychain" {
		t.Fatalf("Backend: got %q, want %q", target.Backend, "keychain")
	}
	if target.Key != "api_key" {
		t.Fatalf("Key: got %q, want %q", target.Key, "api_key")
	}
}
