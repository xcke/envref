package resolve_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xcke/envref/internal/backend"
	"github.com/xcke/envref/internal/envfile"
	"github.com/xcke/envref/internal/parser"
	"github.com/xcke/envref/internal/resolve"
)

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

func TestResolve_NoRefs(t *testing.T) {
	env := envfile.NewEnv()
	env.Set(parser.Entry{Key: "HOST", Value: "localhost"})
	env.Set(parser.Entry{Key: "PORT", Value: "5432"})

	registry := backend.NewRegistry()
	mock := newMockBackend("keychain", map[string]string{})
	require.NoError(t, registry.Register(mock))

	result, err := resolve.Resolve(env, registry, "myapp")
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

func TestResolve_WithRefs(t *testing.T) {
	env := envfile.NewEnv()
	env.Set(parser.Entry{Key: "HOST", Value: "localhost"})
	env.Set(parser.Entry{Key: "DB_PASS", Value: "ref://keychain/db_pass", IsRef: true})
	env.Set(parser.Entry{Key: "API_KEY", Value: "ref://keychain/api_key", IsRef: true})

	// The mock backend stores keys with project namespace prefix, since
	// NamespacedBackend wraps it with "myapp/" prefix.
	registry := backend.NewRegistry()
	mock := newMockBackend("keychain", map[string]string{
		"myapp/db_pass": "s3cret",
		"myapp/api_key": "sk-123",
	})
	require.NoError(t, registry.Register(mock))

	result, err := resolve.Resolve(env, registry, "myapp")
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

func TestResolve_FallbackChain(t *testing.T) {
	env := envfile.NewEnv()
	// Use "secrets" as backend name — not a registered backend name,
	// so fallback chain should be used.
	env.Set(parser.Entry{Key: "TOKEN", Value: "ref://secrets/token", IsRef: true})

	registry := backend.NewRegistry()
	// First backend doesn't have it.
	mock1 := newMockBackend("vault", map[string]string{})
	require.NoError(t, registry.Register(mock1))
	// Second backend has it.
	mock2 := newMockBackend("keychain", map[string]string{
		"myapp/token": "tok-456",
	})
	require.NoError(t, registry.Register(mock2))

	result, err := resolve.Resolve(env, registry, "myapp")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Len(t, result.Entries, 1)
	assert.Equal(t, "tok-456", result.Entries[0].Value)
	assert.True(t, result.Entries[0].WasRef)
}

func TestResolve_DirectBackendMatch(t *testing.T) {
	env := envfile.NewEnv()
	// ref backend name matches registered backend "keychain" directly.
	env.Set(parser.Entry{Key: "SECRET", Value: "ref://keychain/my_secret", IsRef: true})

	registry := backend.NewRegistry()
	mock := newMockBackend("keychain", map[string]string{
		"myapp/my_secret": "hidden",
	})
	require.NoError(t, registry.Register(mock))

	result, err := resolve.Resolve(env, registry, "myapp")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Equal(t, "hidden", result.Entries[0].Value)
}

func TestResolve_MissingSecret(t *testing.T) {
	env := envfile.NewEnv()
	env.Set(parser.Entry{Key: "MISSING", Value: "ref://keychain/nonexistent", IsRef: true})

	registry := backend.NewRegistry()
	mock := newMockBackend("keychain", map[string]string{})
	require.NoError(t, registry.Register(mock))

	result, err := resolve.Resolve(env, registry, "myapp")
	require.NoError(t, err)

	assert.False(t, result.Resolved())
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "MISSING", result.Errors[0].Key)
	assert.Contains(t, result.Errors[0].Err.Error(), "not found")

	// The unresolved entry keeps its ref:// value.
	assert.Equal(t, "ref://keychain/nonexistent", result.Entries[0].Value)
	assert.True(t, result.Entries[0].WasRef)
}

func TestResolve_InvalidRefURI(t *testing.T) {
	env := envfile.NewEnv()
	env.Set(parser.Entry{Key: "BAD", Value: "ref://", IsRef: true})

	registry := backend.NewRegistry()
	mock := newMockBackend("keychain", map[string]string{})
	require.NoError(t, registry.Register(mock))

	result, err := resolve.Resolve(env, registry, "myapp")
	require.NoError(t, err)

	assert.False(t, result.Resolved())
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Err.Error(), "invalid ref:// URI")
	// Original value preserved.
	assert.Equal(t, "ref://", result.Entries[0].Value)
}

func TestResolve_MixedResolution(t *testing.T) {
	env := envfile.NewEnv()
	env.Set(parser.Entry{Key: "HOST", Value: "localhost"})
	env.Set(parser.Entry{Key: "FOUND", Value: "ref://keychain/found_key", IsRef: true})
	env.Set(parser.Entry{Key: "MISSING", Value: "ref://keychain/missing_key", IsRef: true})
	env.Set(parser.Entry{Key: "PORT", Value: "8080"})

	registry := backend.NewRegistry()
	mock := newMockBackend("keychain", map[string]string{
		"myapp/found_key": "resolved_value",
	})
	require.NoError(t, registry.Register(mock))

	result, err := resolve.Resolve(env, registry, "myapp")
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

func TestResolve_EmptyEnv(t *testing.T) {
	env := envfile.NewEnv()
	registry := backend.NewRegistry()
	mock := newMockBackend("keychain", map[string]string{})
	require.NoError(t, registry.Register(mock))

	result, err := resolve.Resolve(env, registry, "myapp")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Empty(t, result.Entries)
	assert.Empty(t, result.Errors)
}

func TestResolve_NestedPath(t *testing.T) {
	env := envfile.NewEnv()
	env.Set(parser.Entry{Key: "DB_PASS", Value: "ref://ssm/prod/db/password", IsRef: true})

	registry := backend.NewRegistry()
	mock := newMockBackend("ssm", map[string]string{
		"myapp/prod/db/password": "db-secret-123",
	})
	require.NoError(t, registry.Register(mock))

	result, err := resolve.Resolve(env, registry, "myapp")
	require.NoError(t, err)

	assert.True(t, result.Resolved())
	assert.Equal(t, "db-secret-123", result.Entries[0].Value)
}

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
}
