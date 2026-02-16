package backend

import (
	"errors"
	"fmt"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r.Len() != 0 {
		t.Fatalf("Len: got %d, want 0", r.Len())
	}
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	b1 := newMemoryBackend("keychain")
	b2 := newMemoryBackend("vault")

	if err := r.Register(b1); err != nil {
		t.Fatalf("Register(keychain): %v", err)
	}
	if err := r.Register(b2); err != nil {
		t.Fatalf("Register(vault): %v", err)
	}
	if r.Len() != 2 {
		t.Fatalf("Len: got %d, want 2", r.Len())
	}
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	r := NewRegistry()

	b1 := newMemoryBackend("keychain")
	b2 := newMemoryBackend("keychain")

	if err := r.Register(b1); err != nil {
		t.Fatalf("Register first: %v", err)
	}
	err := r.Register(b2)
	if err == nil {
		t.Fatal("Register duplicate: expected error, got nil")
	}
	if r.Len() != 1 {
		t.Fatalf("Len after dup: got %d, want 1", r.Len())
	}
}

func TestRegistry_Backend(t *testing.T) {
	r := NewRegistry()
	b := newMemoryBackend("keychain")
	_ = r.Register(b)

	got := r.Backend("keychain")
	if got != b {
		t.Fatal("Backend(keychain): did not return registered backend")
	}

	if r.Backend("nonexistent") != nil {
		t.Fatal("Backend(nonexistent): expected nil")
	}
}

func TestRegistry_Backends(t *testing.T) {
	r := NewRegistry()
	b1 := newMemoryBackend("first")
	b2 := newMemoryBackend("second")
	_ = r.Register(b1)
	_ = r.Register(b2)

	backends := r.Backends()
	if len(backends) != 2 {
		t.Fatalf("Backends: got %d, want 2", len(backends))
	}
	if backends[0].Name() != "first" {
		t.Fatalf("Backends[0]: got %q, want %q", backends[0].Name(), "first")
	}
	if backends[1].Name() != "second" {
		t.Fatalf("Backends[1]: got %q, want %q", backends[1].Name(), "second")
	}

	// Verify it's a copy by modifying the returned slice.
	backends[0] = nil
	if r.Backends()[0] == nil {
		t.Fatal("Backends: returned slice is not a copy")
	}
}

func TestRegistry_Names(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newMemoryBackend("alpha"))
	_ = r.Register(newMemoryBackend("beta"))
	_ = r.Register(newMemoryBackend("gamma"))

	names := r.Names()
	want := []string{"alpha", "beta", "gamma"}
	if len(names) != len(want) {
		t.Fatalf("Names: got %v, want %v", names, want)
	}
	for i, n := range names {
		if n != want[i] {
			t.Fatalf("Names[%d]: got %q, want %q", i, n, want[i])
		}
	}
}

func TestRegistry_Get_FallbackOrder(t *testing.T) {
	r := NewRegistry()

	b1 := newMemoryBackend("primary")
	b2 := newMemoryBackend("secondary")
	b3 := newMemoryBackend("tertiary")

	_ = r.Register(b1)
	_ = r.Register(b2)
	_ = r.Register(b3)

	// Key only in secondary.
	_ = b2.Set("api_key", "from-secondary")
	val, err := r.Get("api_key")
	if err != nil {
		t.Fatalf("Get(api_key): %v", err)
	}
	if val != "from-secondary" {
		t.Fatalf("Get(api_key): got %q, want %q", val, "from-secondary")
	}

	// Key in both primary and secondary — primary wins.
	_ = b1.Set("api_key", "from-primary")
	val, err = r.Get("api_key")
	if err != nil {
		t.Fatalf("Get(api_key) after primary set: %v", err)
	}
	if val != "from-primary" {
		t.Fatalf("Get(api_key) after primary set: got %q, want %q", val, "from-primary")
	}

	// Key only in tertiary.
	_ = b3.Set("db_pass", "from-tertiary")
	val, err = r.Get("db_pass")
	if err != nil {
		t.Fatalf("Get(db_pass): %v", err)
	}
	if val != "from-tertiary" {
		t.Fatalf("Get(db_pass): got %q, want %q", val, "from-tertiary")
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newMemoryBackend("one"))
	_ = r.Register(newMemoryBackend("two"))

	_, err := r.Get("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(missing): got %v, want ErrNotFound", err)
	}
}

func TestRegistry_Get_EmptyRegistry(t *testing.T) {
	r := NewRegistry()

	_, err := r.Get("anything")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get on empty: got %v, want ErrNotFound", err)
	}
}

// errorBackend is a Backend that returns errors for all operations.
type errorBackend struct {
	name string
	err  error
}

func (e *errorBackend) Name() string                  { return e.name }
func (e *errorBackend) Get(string) (string, error)    { return "", e.err }
func (e *errorBackend) Set(string, string) error       { return e.err }
func (e *errorBackend) Delete(string) error            { return e.err }
func (e *errorBackend) List() ([]string, error)        { return nil, e.err }

func TestRegistry_Get_StopsOnNonNotFoundError(t *testing.T) {
	r := NewRegistry()

	b1 := newMemoryBackend("primary")
	errBroken := fmt.Errorf("connection refused")
	b2 := &errorBackend{name: "broken", err: errBroken}
	b3 := newMemoryBackend("tertiary")
	_ = b3.Set("key", "from-tertiary")

	_ = r.Register(b1)
	_ = r.Register(b2)
	_ = r.Register(b3)

	// primary doesn't have it, broken returns a real error → stop, don't reach tertiary.
	_, err := r.Get("key")
	if err == nil {
		t.Fatal("Get: expected error, got nil")
	}

	var kerr *KeyError
	if !errors.As(err, &kerr) {
		t.Fatalf("Get: expected *KeyError, got %T: %v", err, err)
	}
	if kerr.Backend != "broken" {
		t.Fatalf("KeyError.Backend: got %q, want %q", kerr.Backend, "broken")
	}
	if kerr.Key != "key" {
		t.Fatalf("KeyError.Key: got %q, want %q", kerr.Key, "key")
	}
	if !errors.Is(kerr, errBroken) {
		t.Fatalf("KeyError.Err: got %v, want %v", kerr.Err, errBroken)
	}
}

func TestRegistry_GetFrom(t *testing.T) {
	r := NewRegistry()
	b := newMemoryBackend("keychain")
	_ = b.Set("api_key", "secret")
	_ = r.Register(b)

	val, err := r.GetFrom("keychain", "api_key")
	if err != nil {
		t.Fatalf("GetFrom: %v", err)
	}
	if val != "secret" {
		t.Fatalf("GetFrom: got %q, want %q", val, "secret")
	}
}

func TestRegistry_GetFrom_NotRegistered(t *testing.T) {
	r := NewRegistry()

	_, err := r.GetFrom("nonexistent", "key")
	if err == nil {
		t.Fatal("GetFrom(nonexistent): expected error, got nil")
	}
}

func TestRegistry_GetFrom_KeyNotFound(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newMemoryBackend("keychain"))

	_, err := r.GetFrom("keychain", "missing")
	if err == nil {
		t.Fatal("GetFrom(missing key): expected error, got nil")
	}

	var kerr *KeyError
	if !errors.As(err, &kerr) {
		t.Fatalf("GetFrom: expected *KeyError, got %T: %v", err, err)
	}
	if !errors.Is(kerr, ErrNotFound) {
		t.Fatalf("GetFrom: expected ErrNotFound, got %v", kerr.Err)
	}
}

func TestRegistry_SetIn(t *testing.T) {
	r := NewRegistry()
	b := newMemoryBackend("keychain")
	_ = r.Register(b)

	if err := r.SetIn("keychain", "api_key", "secret"); err != nil {
		t.Fatalf("SetIn: %v", err)
	}

	// Verify via direct backend access.
	val, err := b.Get("api_key")
	if err != nil {
		t.Fatalf("Get after SetIn: %v", err)
	}
	if val != "secret" {
		t.Fatalf("Get after SetIn: got %q, want %q", val, "secret")
	}
}

func TestRegistry_SetIn_NotRegistered(t *testing.T) {
	r := NewRegistry()

	err := r.SetIn("nonexistent", "key", "val")
	if err == nil {
		t.Fatal("SetIn(nonexistent): expected error, got nil")
	}
}

func TestRegistry_DeleteFrom(t *testing.T) {
	r := NewRegistry()
	b := newMemoryBackend("keychain")
	_ = b.Set("api_key", "secret")
	_ = r.Register(b)

	if err := r.DeleteFrom("keychain", "api_key"); err != nil {
		t.Fatalf("DeleteFrom: %v", err)
	}

	_, err := b.Get("api_key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after DeleteFrom: got %v, want ErrNotFound", err)
	}
}

func TestRegistry_DeleteFrom_NotRegistered(t *testing.T) {
	r := NewRegistry()

	err := r.DeleteFrom("nonexistent", "key")
	if err == nil {
		t.Fatal("DeleteFrom(nonexistent): expected error, got nil")
	}
}

func TestRegistry_DeleteFrom_KeyNotFound(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(newMemoryBackend("keychain"))

	err := r.DeleteFrom("keychain", "missing")
	if err == nil {
		t.Fatal("DeleteFrom(missing): expected error, got nil")
	}

	var kerr *KeyError
	if !errors.As(err, &kerr) {
		t.Fatalf("DeleteFrom: expected *KeyError, got %T: %v", err, err)
	}
}

func TestRegistry_ListFrom(t *testing.T) {
	r := NewRegistry()
	b := newMemoryBackend("keychain")
	_ = b.Set("alpha", "1")
	_ = b.Set("beta", "2")
	_ = r.Register(b)

	keys, err := r.ListFrom("keychain")
	if err != nil {
		t.Fatalf("ListFrom: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("ListFrom: got %d keys, want 2", len(keys))
	}
	if keys[0] != "alpha" || keys[1] != "beta" {
		t.Fatalf("ListFrom: got %v, want [alpha beta]", keys)
	}
}

func TestRegistry_ListFrom_NotRegistered(t *testing.T) {
	r := NewRegistry()

	_, err := r.ListFrom("nonexistent")
	if err == nil {
		t.Fatal("ListFrom(nonexistent): expected error, got nil")
	}
}

func TestRegistry_String(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		r := NewRegistry()
		if r.String() != "Registry(empty)" {
			t.Fatalf("String: got %q, want %q", r.String(), "Registry(empty)")
		}
	})

	t.Run("with backends", func(t *testing.T) {
		r := NewRegistry()
		_ = r.Register(newMemoryBackend("keychain"))
		_ = r.Register(newMemoryBackend("vault"))
		_ = r.Register(newMemoryBackend("ssm"))

		want := "Registry(keychain → vault → ssm)"
		if r.String() != want {
			t.Fatalf("String: got %q, want %q", r.String(), want)
		}
	})
}
