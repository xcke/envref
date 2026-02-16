package resolve_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xcke/envref/internal/backend"
	"github.com/xcke/envref/internal/envfile"
	"github.com/xcke/envref/internal/parser"
	"github.com/xcke/envref/internal/resolve"
)

// ---------------------------------------------------------------------------
// Mock backends
// ---------------------------------------------------------------------------

// mockBackend is a simple in-memory backend for testing.
type mockBackend struct {
	name    string
	secrets map[string]string
}

func newMockBackend(name string, secrets map[string]string) *mockBackend {
	return &mockBackend{name: name, secrets: secrets}
}

func (m *mockBackend) Name() string { return m.name }

func (m *mockBackend) Get(key string) (string, error) {
	val, ok := m.secrets[key]
	if !ok {
		return "", backend.ErrNotFound
	}
	return val, nil
}

func (m *mockBackend) Set(key, value string) error {
	m.secrets[key] = value
	return nil
}

func (m *mockBackend) Delete(key string) error {
	if _, ok := m.secrets[key]; !ok {
		return backend.ErrNotFound
	}
	delete(m.secrets, key)
	return nil
}

func (m *mockBackend) List() ([]string, error) {
	keys := make([]string, 0, len(m.secrets))
	for k := range m.secrets {
		keys = append(keys, k)
	}
	return keys, nil
}

// countingBackend wraps a mockBackend and counts Get calls per key.
type countingBackend struct {
	*mockBackend
	getCounts map[string]int
}

func newCountingBackend(name string, secrets map[string]string) *countingBackend {
	return &countingBackend{
		mockBackend: newMockBackend(name, secrets),
		getCounts:   make(map[string]int),
	}
}

func (c *countingBackend) Get(key string) (string, error) {
	c.getCounts[key]++
	return c.mockBackend.Get(key)
}

// errorBackend always returns an error on Get (simulates connection failures).
type errorBackend struct {
	name string
	err  error
}

func newErrorBackend(name string, err error) *errorBackend {
	return &errorBackend{name: name, err: err}
}

func (e *errorBackend) Name() string                  { return e.name }
func (e *errorBackend) Get(_ string) (string, error)   { return "", e.err }
func (e *errorBackend) Set(_, _ string) error          { return e.err }
func (e *errorBackend) Delete(_ string) error          { return e.err }
func (e *errorBackend) List() ([]string, error)        { return nil, e.err }

// ---------------------------------------------------------------------------
// Helper to build an Env with entries
// ---------------------------------------------------------------------------

func buildEnv(entries ...parser.Entry) *envfile.Env {
	env := envfile.NewEnv()
	for _, e := range entries {
		env.Set(e)
	}
	return env
}

func buildRegistry(backends ...backend.Backend) *backend.Registry {
	reg := backend.NewRegistry()
	for _, b := range backends {
		if err := reg.Register(b); err != nil {
			panic(fmt.Sprintf("buildRegistry: %v", err))
		}
	}
	return reg
}

// ---------------------------------------------------------------------------
// Input Validation Tests
// ---------------------------------------------------------------------------

func TestResolve_NilEnv(t *testing.T) {
	registry := backend.NewRegistry()
	_, err := resolve.Resolve(nil, registry, "myapp")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "env must not be nil")
}

func TestResolve_NilRegistry(t *testing.T) {
	env := envfile.NewEnv()
	_, err := resolve.Resolve(env, nil, "myapp")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "registry must not be nil")
}

func TestResolve_EmptyProject(t *testing.T) {
	env := envfile.NewEnv()
	registry := backend.NewRegistry()
	_, err := resolve.Resolve(env, registry, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "project name must not be empty")
}

// ---------------------------------------------------------------------------
// Empty / No-Ref Scenarios
// ---------------------------------------------------------------------------

func TestResolve_EmptyEnv(t *testing.T) {
	env := envfile.NewEnv()
	reg := buildRegistry(newMockBackend("keychain", map[string]string{}))

	result, err := resolve.Resolve(env, reg, "myapp")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Empty(t, result.Entries)
	assert.Empty(t, result.Errors)
}

func TestResolve_NoRefs(t *testing.T) {
	env := buildEnv(
		parser.Entry{Key: "HOST", Value: "localhost"},
		parser.Entry{Key: "PORT", Value: "5432"},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{}))

	result, err := resolve.Resolve(env, reg, "myapp")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Len(t, result.Entries, 2)
	assert.Equal(t, "HOST", result.Entries[0].Key)
	assert.Equal(t, "localhost", result.Entries[0].Value)
	assert.False(t, result.Entries[0].WasRef)
	assert.Equal(t, "PORT", result.Entries[1].Key)
	assert.Equal(t, "5432", result.Entries[1].Value)
	assert.False(t, result.Entries[1].WasRef)
}

func TestResolve_AllPlainValues(t *testing.T) {
	env := buildEnv(
		parser.Entry{Key: "A", Value: "1"},
		parser.Entry{Key: "B", Value: "2"},
		parser.Entry{Key: "C", Value: "3"},
		parser.Entry{Key: "D", Value: ""},
		parser.Entry{Key: "E", Value: "hello world"},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{}))

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Len(t, result.Entries, 5)
	for _, entry := range result.Entries {
		assert.False(t, entry.WasRef)
	}
}

// ---------------------------------------------------------------------------
// Direct Backend Match Tests
// ---------------------------------------------------------------------------

func TestResolve_DirectBackendMatch(t *testing.T) {
	env := buildEnv(
		parser.Entry{Key: "SECRET", Value: "ref://keychain/my_secret", IsRef: true},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{
		"myapp/my_secret": "hidden",
	}))

	result, err := resolve.Resolve(env, reg, "myapp")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Equal(t, "hidden", result.Entries[0].Value)
	assert.True(t, result.Entries[0].WasRef)
}

func TestResolve_WithRefs(t *testing.T) {
	env := buildEnv(
		parser.Entry{Key: "HOST", Value: "localhost"},
		parser.Entry{Key: "DB_PASS", Value: "ref://keychain/db_pass", IsRef: true},
		parser.Entry{Key: "API_KEY", Value: "ref://keychain/api_key", IsRef: true},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{
		"myapp/db_pass": "s3cret",
		"myapp/api_key": "sk-123",
	}))

	result, err := resolve.Resolve(env, reg, "myapp")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Len(t, result.Entries, 3)

	assert.Equal(t, "HOST", result.Entries[0].Key)
	assert.Equal(t, "localhost", result.Entries[0].Value)
	assert.False(t, result.Entries[0].WasRef)

	assert.Equal(t, "DB_PASS", result.Entries[1].Key)
	assert.Equal(t, "s3cret", result.Entries[1].Value)
	assert.True(t, result.Entries[1].WasRef)

	assert.Equal(t, "API_KEY", result.Entries[2].Key)
	assert.Equal(t, "sk-123", result.Entries[2].Value)
	assert.True(t, result.Entries[2].WasRef)
}

func TestResolve_MultipleBackendsDirect(t *testing.T) {
	// Each ref targets a different backend by name.
	env := buildEnv(
		parser.Entry{Key: "KEY_A", Value: "ref://keychain/key_a", IsRef: true},
		parser.Entry{Key: "KEY_B", Value: "ref://vault/key_b", IsRef: true},
		parser.Entry{Key: "KEY_C", Value: "ref://ssm/key_c", IsRef: true},
	)
	reg := buildRegistry(
		newMockBackend("keychain", map[string]string{"proj/key_a": "val_a"}),
		newMockBackend("vault", map[string]string{"proj/key_b": "val_b"}),
		newMockBackend("ssm", map[string]string{"proj/key_c": "val_c"}),
	)

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Len(t, result.Entries, 3)
	assert.Equal(t, "val_a", result.Entries[0].Value)
	assert.Equal(t, "val_b", result.Entries[1].Value)
	assert.Equal(t, "val_c", result.Entries[2].Value)
	for _, e := range result.Entries {
		assert.True(t, e.WasRef)
	}
}

// ---------------------------------------------------------------------------
// Fallback Chain Tests
// ---------------------------------------------------------------------------

func TestResolve_FallbackChain(t *testing.T) {
	env := buildEnv(
		parser.Entry{Key: "TOKEN", Value: "ref://secrets/token", IsRef: true},
	)
	reg := buildRegistry(
		newMockBackend("vault", map[string]string{}),
		newMockBackend("keychain", map[string]string{
			"myapp/token": "tok-456",
		}),
	)

	result, err := resolve.Resolve(env, reg, "myapp")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Len(t, result.Entries, 1)
	assert.Equal(t, "tok-456", result.Entries[0].Value)
	assert.True(t, result.Entries[0].WasRef)
}

func TestResolve_FallbackChainFirstBackendWins(t *testing.T) {
	// Both backends have the key; first one should win.
	env := buildEnv(
		parser.Entry{Key: "KEY", Value: "ref://secrets/shared", IsRef: true},
	)
	reg := buildRegistry(
		newMockBackend("primary", map[string]string{"app/shared": "from-primary"}),
		newMockBackend("secondary", map[string]string{"app/shared": "from-secondary"}),
	)

	result, err := resolve.Resolve(env, reg, "app")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Equal(t, "from-primary", result.Entries[0].Value)
}

func TestResolve_FallbackChainThreeBackends(t *testing.T) {
	// Secret found only in the third backend.
	env := buildEnv(
		parser.Entry{Key: "DEEP", Value: "ref://secrets/deep_key", IsRef: true},
	)
	reg := buildRegistry(
		newMockBackend("first", map[string]string{}),
		newMockBackend("second", map[string]string{}),
		newMockBackend("third", map[string]string{"proj/deep_key": "found-deep"}),
	)

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Equal(t, "found-deep", result.Entries[0].Value)
}

func TestResolve_FallbackChainNotFoundInAny(t *testing.T) {
	// Generic backend name, secret not in any backend.
	env := buildEnv(
		parser.Entry{Key: "GHOST", Value: "ref://secrets/ghost_key", IsRef: true},
	)
	reg := buildRegistry(
		newMockBackend("first", map[string]string{}),
		newMockBackend("second", map[string]string{}),
	)

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.False(t, result.Resolved())
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "GHOST", result.Errors[0].Key)
	assert.Contains(t, result.Errors[0].Err.Error(), "not found in any backend")
}

// ---------------------------------------------------------------------------
// Missing Secret / Not Found Tests
// ---------------------------------------------------------------------------

func TestResolve_MissingSecret(t *testing.T) {
	env := buildEnv(
		parser.Entry{Key: "MISSING", Value: "ref://keychain/nonexistent", IsRef: true},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{}))

	result, err := resolve.Resolve(env, reg, "myapp")
	require.NoError(t, err)

	assert.False(t, result.Resolved())
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "MISSING", result.Errors[0].Key)
	assert.Contains(t, result.Errors[0].Err.Error(), "not found")

	// The unresolved entry keeps its ref:// value.
	assert.Equal(t, "ref://keychain/nonexistent", result.Entries[0].Value)
	assert.True(t, result.Entries[0].WasRef)
}

func TestResolve_MultipleAllMissing(t *testing.T) {
	env := buildEnv(
		parser.Entry{Key: "A", Value: "ref://keychain/a", IsRef: true},
		parser.Entry{Key: "B", Value: "ref://keychain/b", IsRef: true},
		parser.Entry{Key: "C", Value: "ref://keychain/c", IsRef: true},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{}))

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.False(t, result.Resolved())
	assert.Len(t, result.Errors, 3)
	assert.Len(t, result.Entries, 3)

	for i, entry := range result.Entries {
		assert.True(t, entry.WasRef)
		assert.Equal(t, result.Errors[i].Key, entry.Key)
	}
}

// ---------------------------------------------------------------------------
// Invalid Ref URI Tests
// ---------------------------------------------------------------------------

func TestResolve_InvalidRefURI(t *testing.T) {
	env := buildEnv(
		parser.Entry{Key: "BAD", Value: "ref://", IsRef: true},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{}))

	result, err := resolve.Resolve(env, reg, "myapp")
	require.NoError(t, err)

	assert.False(t, result.Resolved())
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Err.Error(), "invalid ref:// URI")
	assert.Equal(t, "ref://", result.Entries[0].Value)
}

func TestResolve_InvalidRefVariants(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"empty_ref", "ref://"},
		{"backend_only_no_slash", "ref://keychain"},
		{"backend_only_trailing_slash", "ref://keychain/"},
		{"empty_backend", "ref:///path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := buildEnv(
				parser.Entry{Key: "BAD", Value: tt.value, IsRef: true},
			)
			reg := buildRegistry(newMockBackend("keychain", map[string]string{}))

			result, err := resolve.Resolve(env, reg, "proj")
			require.NoError(t, err)

			assert.False(t, result.Resolved())
			assert.Len(t, result.Errors, 1)
			assert.Contains(t, result.Errors[0].Err.Error(), "invalid ref:// URI")
			// Original value preserved in entries.
			assert.Equal(t, tt.value, result.Entries[0].Value)
			assert.True(t, result.Entries[0].WasRef)
		})
	}
}

// ---------------------------------------------------------------------------
// Mixed Resolution (partial success) Tests
// ---------------------------------------------------------------------------

func TestResolve_MixedResolution(t *testing.T) {
	env := buildEnv(
		parser.Entry{Key: "HOST", Value: "localhost"},
		parser.Entry{Key: "FOUND", Value: "ref://keychain/found_key", IsRef: true},
		parser.Entry{Key: "MISSING", Value: "ref://keychain/missing_key", IsRef: true},
		parser.Entry{Key: "PORT", Value: "8080"},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{
		"myapp/found_key": "resolved_value",
	}))

	result, err := resolve.Resolve(env, reg, "myapp")
	require.NoError(t, err)

	assert.False(t, result.Resolved())
	assert.Len(t, result.Entries, 4)
	assert.Len(t, result.Errors, 1)

	// Non-ref entries pass through.
	assert.Equal(t, "localhost", result.Entries[0].Value)
	assert.Equal(t, "8080", result.Entries[3].Value)

	// Found ref resolved.
	assert.Equal(t, "resolved_value", result.Entries[1].Value)
	assert.True(t, result.Entries[1].WasRef)

	// Missing ref kept as-is with error recorded.
	assert.Equal(t, "ref://keychain/missing_key", result.Entries[2].Value)
	assert.Equal(t, "MISSING", result.Errors[0].Key)
}

func TestResolve_MixedRefsAndPlainInterleaved(t *testing.T) {
	// Interleave plain values and refs to verify order is maintained.
	env := buildEnv(
		parser.Entry{Key: "A", Value: "plain_a"},
		parser.Entry{Key: "B", Value: "ref://keychain/b_secret", IsRef: true},
		parser.Entry{Key: "C", Value: "plain_c"},
		parser.Entry{Key: "D", Value: "ref://keychain/d_secret", IsRef: true},
		parser.Entry{Key: "E", Value: "plain_e"},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{
		"proj/b_secret": "resolved_b",
		"proj/d_secret": "resolved_d",
	}))

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Len(t, result.Entries, 5)

	expected := []struct {
		key    string
		value  string
		wasRef bool
	}{
		{"A", "plain_a", false},
		{"B", "resolved_b", true},
		{"C", "plain_c", false},
		{"D", "resolved_d", true},
		{"E", "plain_e", false},
	}
	for i, exp := range expected {
		assert.Equal(t, exp.key, result.Entries[i].Key)
		assert.Equal(t, exp.value, result.Entries[i].Value)
		assert.Equal(t, exp.wasRef, result.Entries[i].WasRef)
	}
}

func TestResolve_MixedSomeInvalidRefs(t *testing.T) {
	// One valid ref, one invalid ref, one missing ref.
	env := buildEnv(
		parser.Entry{Key: "GOOD", Value: "ref://keychain/good", IsRef: true},
		parser.Entry{Key: "BAD_URI", Value: "ref://", IsRef: true},
		parser.Entry{Key: "MISSING", Value: "ref://keychain/missing", IsRef: true},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{
		"proj/good": "good_val",
	}))

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.False(t, result.Resolved())
	assert.Len(t, result.Entries, 3)
	assert.Len(t, result.Errors, 2)

	// Good ref resolved.
	assert.Equal(t, "good_val", result.Entries[0].Value)
	assert.True(t, result.Entries[0].WasRef)

	// Bad URI error.
	assert.Equal(t, "BAD_URI", result.Errors[0].Key)
	assert.Contains(t, result.Errors[0].Err.Error(), "invalid ref:// URI")

	// Missing secret error.
	assert.Equal(t, "MISSING", result.Errors[1].Key)
	assert.Contains(t, result.Errors[1].Err.Error(), "not found")
}

// ---------------------------------------------------------------------------
// Project Namespace Tests
// ---------------------------------------------------------------------------

func TestResolve_ProjectNamespaceSeparation(t *testing.T) {
	// Secrets stored for "proj-a" should not be accessible from "proj-b".
	mock := newMockBackend("keychain", map[string]string{
		"proj-a/secret": "secret-for-a",
		"proj-b/secret": "secret-for-b",
	})

	env := buildEnv(
		parser.Entry{Key: "SECRET", Value: "ref://keychain/secret", IsRef: true},
	)

	// Resolve as proj-a.
	regA := buildRegistry(mock)
	resultA, err := resolve.Resolve(env, regA, "proj-a")
	require.NoError(t, err)
	assert.True(t, resultA.Resolved())
	assert.Equal(t, "secret-for-a", resultA.Entries[0].Value)

	// Resolve as proj-b — must get proj-b's value, not proj-a's.
	// Need a fresh registry since NamespacedBackend wraps are different.
	mock2 := newMockBackend("keychain", map[string]string{
		"proj-a/secret": "secret-for-a",
		"proj-b/secret": "secret-for-b",
	})
	regB := buildRegistry(mock2)
	resultB, err := resolve.Resolve(env, regB, "proj-b")
	require.NoError(t, err)
	assert.True(t, resultB.Resolved())
	assert.Equal(t, "secret-for-b", resultB.Entries[0].Value)
}

func TestResolve_ProjectNamespaceMissing(t *testing.T) {
	// Secret exists for "other-proj" but not for "this-proj".
	mock := newMockBackend("keychain", map[string]string{
		"other-proj/api_key": "should-not-see",
	})
	env := buildEnv(
		parser.Entry{Key: "API_KEY", Value: "ref://keychain/api_key", IsRef: true},
	)
	reg := buildRegistry(mock)

	result, err := resolve.Resolve(env, reg, "this-proj")
	require.NoError(t, err)

	assert.False(t, result.Resolved())
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Err.Error(), "not found")
}

// ---------------------------------------------------------------------------
// Nested Path Tests
// ---------------------------------------------------------------------------

func TestResolve_NestedPath(t *testing.T) {
	env := buildEnv(
		parser.Entry{Key: "DB_PASS", Value: "ref://ssm/prod/db/password", IsRef: true},
	)
	reg := buildRegistry(newMockBackend("ssm", map[string]string{
		"myapp/prod/db/password": "db-secret-123",
	}))

	result, err := resolve.Resolve(env, reg, "myapp")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Equal(t, "db-secret-123", result.Entries[0].Value)
}

func TestResolve_DeeplyNestedPaths(t *testing.T) {
	tests := []struct {
		name     string
		refValue string
		storeKey string
	}{
		{"two_levels", "ref://ssm/prod/db", "proj/prod/db"},
		{"three_levels", "ref://ssm/prod/db/password", "proj/prod/db/password"},
		{"four_levels", "ref://ssm/us-east/prod/db/password", "proj/us-east/prod/db/password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := buildEnv(
				parser.Entry{Key: "SECRET", Value: tt.refValue, IsRef: true},
			)
			reg := buildRegistry(newMockBackend("ssm", map[string]string{
				tt.storeKey: "resolved",
			}))

			result, err := resolve.Resolve(env, reg, "proj")
			require.NoError(t, err)
			assert.True(t, result.Resolved())
			assert.Equal(t, "resolved", result.Entries[0].Value)
		})
	}
}

// ---------------------------------------------------------------------------
// Backend Error Handling Tests
// ---------------------------------------------------------------------------

func TestResolve_BackendConnectionError_DirectMatch(t *testing.T) {
	// A backend that errors on Get (not ErrNotFound) for a direct match.
	connErr := fmt.Errorf("connection refused")
	env := buildEnv(
		parser.Entry{Key: "SECRET", Value: "ref://broken/key", IsRef: true},
	)
	reg := buildRegistry(newErrorBackend("broken", connErr))

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err) // Resolve itself doesn't fail.

	assert.False(t, result.Resolved())
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "SECRET", result.Errors[0].Key)
	assert.Contains(t, result.Errors[0].Err.Error(), "broken")
}

func TestResolve_BackendConnectionError_Fallback(t *testing.T) {
	// Generic backend ref with error backend in the chain.
	connErr := fmt.Errorf("timeout")
	env := buildEnv(
		parser.Entry{Key: "KEY", Value: "ref://secrets/key", IsRef: true},
	)
	// The error backend should cause immediate failure in the fallback chain
	// (non-ErrNotFound errors stop the chain).
	reg := buildRegistry(newErrorBackend("failback", connErr))

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.False(t, result.Resolved())
	assert.Len(t, result.Errors, 1)
}

func TestResolve_MixedBackendHealthy_And_Broken(t *testing.T) {
	// One healthy backend works, another entry hits a broken backend.
	env := buildEnv(
		parser.Entry{Key: "GOOD", Value: "ref://healthy/good", IsRef: true},
		parser.Entry{Key: "BROKEN", Value: "ref://broken/bad", IsRef: true},
	)
	reg := buildRegistry(
		newMockBackend("healthy", map[string]string{"proj/good": "yay"}),
		newErrorBackend("broken", fmt.Errorf("disk full")),
	)

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.False(t, result.Resolved())
	assert.Len(t, result.Entries, 2)
	assert.Len(t, result.Errors, 1)

	assert.Equal(t, "yay", result.Entries[0].Value)
	assert.True(t, result.Entries[0].WasRef)
	assert.Equal(t, "BROKEN", result.Errors[0].Key)
}

// ---------------------------------------------------------------------------
// Order Preservation Tests
// ---------------------------------------------------------------------------

func TestResolve_OrderPreserved(t *testing.T) {
	// Ensure the result maintains the insertion order of the input Env.
	env := buildEnv(
		parser.Entry{Key: "Z_LAST", Value: "z"},
		parser.Entry{Key: "A_FIRST", Value: "a"},
		parser.Entry{Key: "M_MIDDLE", Value: "ref://keychain/m", IsRef: true},
		parser.Entry{Key: "B_SECOND", Value: "b"},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{
		"proj/m": "resolved_m",
	}))

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	keys := make([]string, len(result.Entries))
	for i, e := range result.Entries {
		keys[i] = e.Key
	}
	assert.Equal(t, []string{"Z_LAST", "A_FIRST", "M_MIDDLE", "B_SECOND"}, keys)
}

// ---------------------------------------------------------------------------
// Special Value Tests
// ---------------------------------------------------------------------------

func TestResolve_EmptySecretValue(t *testing.T) {
	// A backend can legitimately store an empty string as a secret.
	env := buildEnv(
		parser.Entry{Key: "EMPTY_SECRET", Value: "ref://keychain/empty", IsRef: true},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{
		"proj/empty": "",
	}))

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Equal(t, "", result.Entries[0].Value)
	assert.True(t, result.Entries[0].WasRef)
}

func TestResolve_SecretWithSpecialCharacters(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"spaces", "my secret value"},
		{"newlines", "line1\nline2\nline3"},
		{"tabs", "col1\tcol2"},
		{"quotes", `he said "hello"`},
		{"single_quotes", "it's working"},
		{"unicode", "p\u00e4ssw\u00f6rd"},
		{"equals_sign", "key=value=pair"},
		{"dollar_sign", "costs $100"},
		{"hash", "color #ff0000"},
		{"backslash", `path\to\file`},
		{"multiline_json", `{"key":"value","nested":{"a":1}}`},
		{"long_value", string(make([]byte, 4096))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := buildEnv(
				parser.Entry{Key: "SECRET", Value: "ref://keychain/secret", IsRef: true},
			)
			reg := buildRegistry(newMockBackend("keychain", map[string]string{
				"proj/secret": tt.value,
			}))

			result, err := resolve.Resolve(env, reg, "proj")
			require.NoError(t, err)

			assert.True(t, result.Resolved())
			assert.Equal(t, tt.value, result.Entries[0].Value)
		})
	}
}

func TestResolve_PlainValueContainingRefPrefix(t *testing.T) {
	// A plain (non-ref) value that happens to contain "ref://" text
	// should pass through without resolution attempt.
	env := buildEnv(
		parser.Entry{Key: "DOCS", Value: "See ref://docs for info", IsRef: false},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{}))

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Equal(t, "See ref://docs for info", result.Entries[0].Value)
	assert.False(t, result.Entries[0].WasRef)
}

// ---------------------------------------------------------------------------
// Large Env Tests
// ---------------------------------------------------------------------------

func TestResolve_LargeEnv(t *testing.T) {
	// Test with a large number of entries to verify scalability.
	const n = 500
	env := envfile.NewEnv()
	secrets := make(map[string]string)

	for i := 0; i < n; i++ {
		key := fmt.Sprintf("KEY_%04d", i)
		if i%3 == 0 {
			// Every 3rd entry is a ref.
			refValue := fmt.Sprintf("ref://keychain/secret_%04d", i)
			env.Set(parser.Entry{Key: key, Value: refValue, IsRef: true})
			secrets[fmt.Sprintf("proj/secret_%04d", i)] = fmt.Sprintf("value_%04d", i)
		} else {
			env.Set(parser.Entry{Key: key, Value: fmt.Sprintf("plain_%04d", i)})
		}
	}

	reg := buildRegistry(newMockBackend("keychain", secrets))

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Len(t, result.Entries, n)
	assert.Empty(t, result.Errors)

	// Spot-check a few entries.
	assert.Equal(t, "value_0000", result.Entries[0].Value)
	assert.True(t, result.Entries[0].WasRef)
	assert.Equal(t, "plain_0001", result.Entries[1].Value)
	assert.False(t, result.Entries[1].WasRef)
}

// ---------------------------------------------------------------------------
// All Refs Resolved Successfully Tests
// ---------------------------------------------------------------------------

func TestResolve_AllRefsResolved(t *testing.T) {
	env := buildEnv(
		parser.Entry{Key: "A", Value: "ref://keychain/a", IsRef: true},
		parser.Entry{Key: "B", Value: "ref://keychain/b", IsRef: true},
		parser.Entry{Key: "C", Value: "ref://keychain/c", IsRef: true},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{
		"proj/a": "va",
		"proj/b": "vb",
		"proj/c": "vc",
	}))

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Len(t, result.Entries, 3)
	for _, e := range result.Entries {
		assert.True(t, e.WasRef)
	}
}

// ---------------------------------------------------------------------------
// Result and KeyErr Type Tests
// ---------------------------------------------------------------------------

func TestKeyErr_Error(t *testing.T) {
	e := resolve.KeyErr{
		Key: "API_KEY",
		Ref: "ref://keychain/api_key",
		Err: backend.ErrNotFound,
	}
	msg := e.Error()
	assert.Contains(t, msg, "API_KEY")
	assert.Contains(t, msg, "ref://keychain/api_key")
	assert.Contains(t, msg, "not found")
}

func TestKeyErr_ErrorFormat(t *testing.T) {
	e := resolve.KeyErr{
		Key: "DB_PASS",
		Ref: "ref://vault/db_pass",
		Err: fmt.Errorf("connection timeout"),
	}
	msg := e.Error()
	assert.Equal(t, "DB_PASS: failed to resolve ref://vault/db_pass: connection timeout", msg)
}

func TestResult_Resolved(t *testing.T) {
	t.Run("no errors", func(t *testing.T) {
		r := &resolve.Result{}
		assert.True(t, r.Resolved())
	})

	t.Run("with errors", func(t *testing.T) {
		r := &resolve.Result{
			Errors: []resolve.KeyErr{{Key: "X", Ref: "ref://a/b", Err: backend.ErrNotFound}},
		}
		assert.False(t, r.Resolved())
	})

	t.Run("entries but no errors", func(t *testing.T) {
		r := &resolve.Result{
			Entries: []resolve.Entry{
				{Key: "A", Value: "1", WasRef: false},
			},
		}
		assert.True(t, r.Resolved())
	})

	t.Run("multiple errors", func(t *testing.T) {
		r := &resolve.Result{
			Errors: []resolve.KeyErr{
				{Key: "X", Ref: "ref://a/x", Err: backend.ErrNotFound},
				{Key: "Y", Ref: "ref://a/y", Err: backend.ErrNotFound},
			},
		}
		assert.False(t, r.Resolved())
	})
}

// ---------------------------------------------------------------------------
// Error Isolation Tests (errors don't stop other resolutions)
// ---------------------------------------------------------------------------

func TestResolve_ErrorsDoNotStopResolution(t *testing.T) {
	// Multiple refs: first fails, second succeeds, third fails, fourth succeeds.
	env := buildEnv(
		parser.Entry{Key: "FAIL_1", Value: "ref://keychain/missing1", IsRef: true},
		parser.Entry{Key: "OK_1", Value: "ref://keychain/found1", IsRef: true},
		parser.Entry{Key: "FAIL_2", Value: "ref://", IsRef: true},
		parser.Entry{Key: "OK_2", Value: "ref://keychain/found2", IsRef: true},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{
		"proj/found1": "val1",
		"proj/found2": "val2",
	}))

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.False(t, result.Resolved())
	assert.Len(t, result.Entries, 4)
	assert.Len(t, result.Errors, 2)

	// Successful resolutions.
	assert.Equal(t, "val1", result.Entries[1].Value)
	assert.Equal(t, "val2", result.Entries[3].Value)

	// Failed entries preserve original values.
	assert.Equal(t, "ref://keychain/missing1", result.Entries[0].Value)
	assert.Equal(t, "ref://", result.Entries[2].Value)
}

// ---------------------------------------------------------------------------
// Ref Preservation Tests (unresolved refs keep original value)
// ---------------------------------------------------------------------------

func TestResolve_UnresolvedRefsPreserveOriginalValue(t *testing.T) {
	refs := []string{
		"ref://keychain/missing",
		"ref://vault/gone",
		"ref://ssm/prod/db/nonexistent",
	}

	for _, refVal := range refs {
		t.Run(refVal, func(t *testing.T) {
			env := buildEnv(
				parser.Entry{Key: "KEY", Value: refVal, IsRef: true},
			)
			reg := buildRegistry(
				newMockBackend("keychain", map[string]string{}),
				newMockBackend("vault", map[string]string{}),
				newMockBackend("ssm", map[string]string{}),
			)

			result, err := resolve.Resolve(env, reg, "proj")
			require.NoError(t, err)

			assert.Len(t, result.Entries, 1)
			assert.Equal(t, refVal, result.Entries[0].Value,
				"unresolved ref should preserve original value")
			assert.True(t, result.Entries[0].WasRef)

			// Error should also carry the original ref.
			assert.Len(t, result.Errors, 1)
			assert.Equal(t, refVal, result.Errors[0].Ref)
		})
	}
}

// ---------------------------------------------------------------------------
// WasRef Flag Tests
// ---------------------------------------------------------------------------

func TestResolve_WasRefFlag(t *testing.T) {
	env := buildEnv(
		parser.Entry{Key: "PLAIN", Value: "plain_value"},
		parser.Entry{Key: "REF_RESOLVED", Value: "ref://keychain/key", IsRef: true},
		parser.Entry{Key: "REF_MISSING", Value: "ref://keychain/missing", IsRef: true},
		parser.Entry{Key: "REF_INVALID", Value: "ref://", IsRef: true},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{
		"proj/key": "resolved",
	}))

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.Len(t, result.Entries, 4)

	// Plain value: WasRef = false.
	assert.False(t, result.Entries[0].WasRef)

	// Resolved ref: WasRef = true.
	assert.True(t, result.Entries[1].WasRef)

	// Missing ref: WasRef = true (it was a ref, even though unresolved).
	assert.True(t, result.Entries[2].WasRef)

	// Invalid ref: WasRef = true (it was a ref, even though parse failed).
	assert.True(t, result.Entries[3].WasRef)
}

// ---------------------------------------------------------------------------
// Duplicate Ref Paths (same secret referenced by different env vars)
// ---------------------------------------------------------------------------

func TestResolve_DuplicateRefPaths(t *testing.T) {
	// Two different env vars referencing the same secret path.
	env := buildEnv(
		parser.Entry{Key: "DB_PASS", Value: "ref://keychain/shared_secret", IsRef: true},
		parser.Entry{Key: "DB_PASS_COPY", Value: "ref://keychain/shared_secret", IsRef: true},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{
		"proj/shared_secret": "the_value",
	}))

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Len(t, result.Entries, 2)
	assert.Equal(t, "the_value", result.Entries[0].Value)
	assert.Equal(t, "the_value", result.Entries[1].Value)
}

// ---------------------------------------------------------------------------
// Resolution Cache Tests
// ---------------------------------------------------------------------------

func TestResolve_CacheAvoidsDuplicateBackendHits(t *testing.T) {
	// Three env vars reference the same secret. The backend should only be hit once.
	cb := newCountingBackend("keychain", map[string]string{
		"proj/shared": "the_value",
	})
	env := buildEnv(
		parser.Entry{Key: "A", Value: "ref://keychain/shared", IsRef: true},
		parser.Entry{Key: "B", Value: "ref://keychain/shared", IsRef: true},
		parser.Entry{Key: "C", Value: "ref://keychain/shared", IsRef: true},
	)
	reg := buildRegistry(cb)

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Len(t, result.Entries, 3)
	for _, entry := range result.Entries {
		assert.Equal(t, "the_value", entry.Value)
		assert.True(t, entry.WasRef)
	}

	// The backend should have been queried only once for the namespaced key.
	assert.Equal(t, 1, cb.getCounts["proj/shared"],
		"expected 1 backend hit for duplicate refs, got %d", cb.getCounts["proj/shared"])
}

func TestResolve_CacheDistinctRefsHitBackendSeparately(t *testing.T) {
	// Different ref URIs should each hit the backend once.
	cb := newCountingBackend("keychain", map[string]string{
		"proj/key_a": "val_a",
		"proj/key_b": "val_b",
	})
	env := buildEnv(
		parser.Entry{Key: "A", Value: "ref://keychain/key_a", IsRef: true},
		parser.Entry{Key: "B", Value: "ref://keychain/key_b", IsRef: true},
	)
	reg := buildRegistry(cb)

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Equal(t, 1, cb.getCounts["proj/key_a"])
	assert.Equal(t, 1, cb.getCounts["proj/key_b"])
}

func TestResolve_CacheErrorsAreAlsoCached(t *testing.T) {
	// If a ref fails, the error should be cached so duplicate refs don't retry.
	cb := newCountingBackend("keychain", map[string]string{})
	env := buildEnv(
		parser.Entry{Key: "A", Value: "ref://keychain/missing", IsRef: true},
		parser.Entry{Key: "B", Value: "ref://keychain/missing", IsRef: true},
	)
	reg := buildRegistry(cb)

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.False(t, result.Resolved())
	assert.Len(t, result.Errors, 2)
	assert.Equal(t, "A", result.Errors[0].Key)
	assert.Equal(t, "B", result.Errors[1].Key)

	// Backend should have been queried only once.
	assert.Equal(t, 1, cb.getCounts["proj/missing"],
		"expected 1 backend hit for cached error, got %d", cb.getCounts["proj/missing"])
}

func TestResolve_CacheMixedSuccessAndDuplicates(t *testing.T) {
	// Mix of unique refs and duplicates. Each unique ref hits the backend once.
	cb := newCountingBackend("keychain", map[string]string{
		"proj/secret_a": "val_a",
		"proj/secret_b": "val_b",
	})
	env := buildEnv(
		parser.Entry{Key: "A1", Value: "ref://keychain/secret_a", IsRef: true},
		parser.Entry{Key: "B1", Value: "ref://keychain/secret_b", IsRef: true},
		parser.Entry{Key: "A2", Value: "ref://keychain/secret_a", IsRef: true},
		parser.Entry{Key: "B2", Value: "ref://keychain/secret_b", IsRef: true},
		parser.Entry{Key: "A3", Value: "ref://keychain/secret_a", IsRef: true},
	)
	reg := buildRegistry(cb)

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Len(t, result.Entries, 5)

	// Each unique secret queried exactly once.
	assert.Equal(t, 1, cb.getCounts["proj/secret_a"])
	assert.Equal(t, 1, cb.getCounts["proj/secret_b"])

	// All entries get correct values.
	assert.Equal(t, "val_a", result.Entries[0].Value)
	assert.Equal(t, "val_b", result.Entries[1].Value)
	assert.Equal(t, "val_a", result.Entries[2].Value)
	assert.Equal(t, "val_b", result.Entries[3].Value)
	assert.Equal(t, "val_a", result.Entries[4].Value)
}

// ---------------------------------------------------------------------------
// Cross-Backend Resolution (different refs → different backends)
// ---------------------------------------------------------------------------

func TestResolve_CrossBackendResolution(t *testing.T) {
	// Each ref targets a different backend explicitly.
	env := buildEnv(
		parser.Entry{Key: "FROM_KEYCHAIN", Value: "ref://keychain/key1", IsRef: true},
		parser.Entry{Key: "FROM_VAULT", Value: "ref://vault/key2", IsRef: true},
		parser.Entry{Key: "FROM_SSM", Value: "ref://ssm/path/key3", IsRef: true},
		parser.Entry{Key: "FALLBACK", Value: "ref://secrets/key4", IsRef: true},
	)
	reg := buildRegistry(
		newMockBackend("keychain", map[string]string{"proj/key1": "kc_val", "proj/key4": "fb_val"}),
		newMockBackend("vault", map[string]string{"proj/key2": "vlt_val"}),
		newMockBackend("ssm", map[string]string{"proj/path/key3": "ssm_val"}),
	)

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Equal(t, "kc_val", result.Entries[0].Value)
	assert.Equal(t, "vlt_val", result.Entries[1].Value)
	assert.Equal(t, "ssm_val", result.Entries[2].Value)
	// Fallback "secrets" not a registered backend, so falls through chain; keychain has key4.
	assert.Equal(t, "fb_val", result.Entries[3].Value)
}

// ---------------------------------------------------------------------------
// Edge Cases
// ---------------------------------------------------------------------------

func TestResolve_SingleEntryRef(t *testing.T) {
	env := buildEnv(
		parser.Entry{Key: "ONLY", Value: "ref://keychain/only", IsRef: true},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{
		"proj/only": "sole_value",
	}))

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Len(t, result.Entries, 1)
	assert.Equal(t, "sole_value", result.Entries[0].Value)
}

func TestResolve_SingleEntryPlain(t *testing.T) {
	env := buildEnv(
		parser.Entry{Key: "ONLY", Value: "plain"},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{}))

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Len(t, result.Entries, 1)
	assert.Equal(t, "plain", result.Entries[0].Value)
	assert.False(t, result.Entries[0].WasRef)
}

func TestResolve_EmptyPlainValue(t *testing.T) {
	env := buildEnv(
		parser.Entry{Key: "EMPTY", Value: ""},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{}))

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Equal(t, "", result.Entries[0].Value)
	assert.False(t, result.Entries[0].WasRef)
}

func TestResolve_RefToUnregisteredBackendViaFallback(t *testing.T) {
	// ref://unknown/key where "unknown" isn't registered;
	// fallback chain doesn't have the key either → error.
	env := buildEnv(
		parser.Entry{Key: "KEY", Value: "ref://unknown/key", IsRef: true},
	)
	reg := buildRegistry(
		newMockBackend("keychain", map[string]string{}),
	)

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.False(t, result.Resolved())
	assert.Len(t, result.Errors, 1)
}

func TestResolve_RefBackendNameMatchesButKeyMissing(t *testing.T) {
	// Backend "keychain" is registered but doesn't have the specific key.
	env := buildEnv(
		parser.Entry{Key: "KEY", Value: "ref://keychain/nonexistent", IsRef: true},
	)
	reg := buildRegistry(
		newMockBackend("keychain", map[string]string{
			"proj/other_key": "other_value",
		}),
	)

	result, err := resolve.Resolve(env, reg, "proj")
	require.NoError(t, err)

	assert.False(t, result.Resolved())
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Err.Error(), "not found")
	assert.Contains(t, result.Errors[0].Err.Error(), "keychain")
}

// ---------------------------------------------------------------------------
// Table-driven: Comprehensive Resolution Scenarios
// ---------------------------------------------------------------------------

func TestResolve_Scenarios(t *testing.T) {
	tests := []struct {
		name        string
		entries     []parser.Entry
		backends    map[string]map[string]string // backend name → (namespaced key → value)
		project     string
		wantLen     int
		wantErrors  int
		wantValues  map[string]string // key → expected value
		wantWasRef  map[string]bool   // key → expected WasRef flag
	}{
		{
			name: "all_plain",
			entries: []parser.Entry{
				{Key: "A", Value: "1"},
				{Key: "B", Value: "2"},
			},
			backends:   map[string]map[string]string{"keychain": {}},
			project:    "p",
			wantLen:    2,
			wantErrors: 0,
			wantValues: map[string]string{"A": "1", "B": "2"},
			wantWasRef: map[string]bool{"A": false, "B": false},
		},
		{
			name: "all_refs_resolved",
			entries: []parser.Entry{
				{Key: "X", Value: "ref://keychain/x", IsRef: true},
				{Key: "Y", Value: "ref://keychain/y", IsRef: true},
			},
			backends:   map[string]map[string]string{"keychain": {"p/x": "vx", "p/y": "vy"}},
			project:    "p",
			wantLen:    2,
			wantErrors: 0,
			wantValues: map[string]string{"X": "vx", "Y": "vy"},
			wantWasRef: map[string]bool{"X": true, "Y": true},
		},
		{
			name: "all_refs_missing",
			entries: []parser.Entry{
				{Key: "X", Value: "ref://keychain/x", IsRef: true},
			},
			backends:   map[string]map[string]string{"keychain": {}},
			project:    "p",
			wantLen:    1,
			wantErrors: 1,
			wantValues: map[string]string{"X": "ref://keychain/x"},
			wantWasRef: map[string]bool{"X": true},
		},
		{
			name: "mixed_with_fallback",
			entries: []parser.Entry{
				{Key: "CONFIG", Value: "localhost"},
				{Key: "SECRET", Value: "ref://secrets/token", IsRef: true},
			},
			backends: map[string]map[string]string{
				"primary":   {},
				"secondary": {"p/token": "found-it"},
			},
			project:    "p",
			wantLen:    2,
			wantErrors: 0,
			wantValues: map[string]string{"CONFIG": "localhost", "SECRET": "found-it"},
			wantWasRef: map[string]bool{"CONFIG": false, "SECRET": true},
		},
		{
			name: "ref_to_first_of_two_backends",
			entries: []parser.Entry{
				{Key: "S", Value: "ref://secrets/s", IsRef: true},
			},
			backends: map[string]map[string]string{
				"a": {"proj/s": "from-a"},
				"b": {"proj/s": "from-b"},
			},
			project:    "proj",
			wantLen:    1,
			wantErrors: 0,
			wantValues: map[string]string{"S": "from-a"},
			wantWasRef: map[string]bool{"S": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := buildEnv(tt.entries...)

			// We need deterministic order for backend registration.
			// Since map iteration is non-deterministic, sort names first.
			reg := backend.NewRegistry()
			// Sort the backend names for deterministic registration order.
			// For the "ref_to_first_of_two_backends" test, order matters.
			orderedNames := make([]string, 0, len(tt.backends))
			for name := range tt.backends {
				orderedNames = append(orderedNames, name)
			}
			// Simple sort for determinism.
			for i := 0; i < len(orderedNames); i++ {
				for j := i + 1; j < len(orderedNames); j++ {
					if orderedNames[i] > orderedNames[j] {
						orderedNames[i], orderedNames[j] = orderedNames[j], orderedNames[i]
					}
				}
			}
			for _, name := range orderedNames {
				require.NoError(t, reg.Register(newMockBackend(name, tt.backends[name])))
			}

			result, err := resolve.Resolve(env, reg, tt.project)
			require.NoError(t, err)

			assert.Len(t, result.Entries, tt.wantLen)
			assert.Len(t, result.Errors, tt.wantErrors)

			for _, entry := range result.Entries {
				if expected, ok := tt.wantValues[entry.Key]; ok {
					assert.Equal(t, expected, entry.Value, "value mismatch for key %s", entry.Key)
				}
				if expectedRef, ok := tt.wantWasRef[entry.Key]; ok {
					assert.Equal(t, expectedRef, entry.WasRef, "WasRef mismatch for key %s", entry.Key)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Error Wrapping and Message Tests
// ---------------------------------------------------------------------------

func TestResolve_ErrorMessages(t *testing.T) {
	t.Run("missing_in_direct_backend", func(t *testing.T) {
		env := buildEnv(
			parser.Entry{Key: "KEY", Value: "ref://keychain/missing", IsRef: true},
		)
		reg := buildRegistry(newMockBackend("keychain", map[string]string{}))

		result, err := resolve.Resolve(env, reg, "proj")
		require.NoError(t, err)

		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Err.Error(), "not found")
		assert.Contains(t, result.Errors[0].Err.Error(), "keychain")
		assert.Contains(t, result.Errors[0].Err.Error(), "missing")
	})

	t.Run("missing_in_fallback_chain", func(t *testing.T) {
		env := buildEnv(
			parser.Entry{Key: "KEY", Value: "ref://secrets/missing", IsRef: true},
		)
		reg := buildRegistry(
			newMockBackend("keychain", map[string]string{}),
			newMockBackend("vault", map[string]string{}),
		)

		result, err := resolve.Resolve(env, reg, "proj")
		require.NoError(t, err)

		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Err.Error(), "not found in any backend")
	})

	t.Run("backend_error_direct", func(t *testing.T) {
		env := buildEnv(
			parser.Entry{Key: "KEY", Value: "ref://broken/key", IsRef: true},
		)
		reg := buildRegistry(newErrorBackend("broken", errors.New("permission denied")))

		result, err := resolve.Resolve(env, reg, "proj")
		require.NoError(t, err)

		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Err.Error(), "broken")
	})

	t.Run("invalid_ref_uri", func(t *testing.T) {
		env := buildEnv(
			parser.Entry{Key: "KEY", Value: "ref://keychain/", IsRef: true},
		)
		reg := buildRegistry(newMockBackend("keychain", map[string]string{}))

		result, err := resolve.Resolve(env, reg, "proj")
		require.NoError(t, err)

		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Err.Error(), "invalid ref:// URI")
	})
}

// ---------------------------------------------------------------------------
// Concurrent-Safe: Resolve can be called multiple times on same env
// ---------------------------------------------------------------------------

func TestResolve_MultipleCallsSameEnv(t *testing.T) {
	// Calling Resolve multiple times on the same Env should produce consistent results.
	env := buildEnv(
		parser.Entry{Key: "HOST", Value: "localhost"},
		parser.Entry{Key: "SECRET", Value: "ref://keychain/secret", IsRef: true},
	)
	reg := buildRegistry(newMockBackend("keychain", map[string]string{
		"proj/secret": "the_secret",
	}))

	for i := 0; i < 5; i++ {
		result, err := resolve.Resolve(env, reg, "proj")
		require.NoError(t, err)
		assert.True(t, result.Resolved())
		assert.Len(t, result.Entries, 2)
		assert.Equal(t, "localhost", result.Entries[0].Value)
		assert.Equal(t, "the_secret", result.Entries[1].Value)
	}
}

// ---------------------------------------------------------------------------
// Entry and Result Struct Tests
// ---------------------------------------------------------------------------

func TestEntry_Fields(t *testing.T) {
	entry := resolve.Entry{
		Key:    "MY_KEY",
		Value:  "my_value",
		WasRef: true,
	}
	assert.Equal(t, "MY_KEY", entry.Key)
	assert.Equal(t, "my_value", entry.Value)
	assert.True(t, entry.WasRef)
}

func TestKeyErr_Fields(t *testing.T) {
	kerr := resolve.KeyErr{
		Key: "KEY",
		Ref: "ref://backend/path",
		Err: errors.New("something failed"),
	}
	assert.Equal(t, "KEY", kerr.Key)
	assert.Equal(t, "ref://backend/path", kerr.Ref)
	assert.Equal(t, "something failed", kerr.Err.Error())
}

func TestResult_ResolvedWithEmptyResult(t *testing.T) {
	r := &resolve.Result{}
	assert.True(t, r.Resolved())
	assert.Nil(t, r.Entries)
	assert.Nil(t, r.Errors)
}
