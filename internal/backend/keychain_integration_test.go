//go:build integration

// Integration tests for KeychainBackend that exercise the real OS keychain.
//
// These tests are excluded from normal `go test` runs and only execute when
// the "integration" build tag is set:
//
//	go test -tags=integration -run TestIntegration ./internal/backend/...
//
// Prerequisites:
//   - macOS: Keychain must be unlocked
//   - Linux: gnome-keyring or kwallet must be running with an unlocked session
//   - Windows: Credential Manager must be available
//
// All test keys are namespaced under "__envref_inttest__/" and cleaned up
// after each test.
package backend

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// integrationTestProject is the namespace used for integration test secrets.
// This avoids collisions with real envref secrets.
const integrationTestProject = "__envref_inttest__"

// newIntegrationKeychain creates a KeychainBackend for integration testing
// and registers a cleanup function that removes all test keys.
func newIntegrationKeychain(t *testing.T) *KeychainBackend {
	t.Helper()
	kb := NewKeychainBackend()

	// Verify connectivity: attempt a no-op read to confirm keychain access.
	_, err := kb.Get(integrationTestProject + "/__probe__")
	if err != nil && !errors.Is(err, ErrNotFound) {
		t.Skipf("skipping: keychain not available: %v", err)
	}

	t.Cleanup(func() {
		// Best-effort cleanup: delete all keys under our test namespace.
		keys, err := kb.List()
		if err != nil {
			return
		}
		for _, k := range keys {
			if strings.HasPrefix(k, integrationTestProject+"/") {
				_ = kb.Delete(k)
			}
		}
	})

	return kb
}

// newIntegrationNamespaced creates a NamespacedBackend wrapping a real
// KeychainBackend for integration testing.
func newIntegrationNamespaced(t *testing.T) (*NamespacedBackend, *KeychainBackend) {
	t.Helper()
	kb := newIntegrationKeychain(t)
	nb, err := NewNamespacedBackend(kb, integrationTestProject)
	require.NoError(t, err)
	return nb, kb
}

func TestIntegrationKeychain_BasicCRUD(t *testing.T) {
	nb, _ := newIntegrationNamespaced(t)

	// Set a secret.
	err := nb.Set("test_api_key", "integration-secret-123")
	require.NoError(t, err)

	// Get the secret back.
	val, err := nb.Get("test_api_key")
	require.NoError(t, err)
	assert.Equal(t, "integration-secret-123", val)

	// Overwrite.
	err = nb.Set("test_api_key", "updated-value")
	require.NoError(t, err)

	val, err = nb.Get("test_api_key")
	require.NoError(t, err)
	assert.Equal(t, "updated-value", val)

	// Delete.
	err = nb.Delete("test_api_key")
	require.NoError(t, err)

	_, err = nb.Get("test_api_key")
	assert.True(t, errors.Is(err, ErrNotFound), "expected ErrNotFound after delete, got: %v", err)
}

func TestIntegrationKeychain_NotFound(t *testing.T) {
	nb, _ := newIntegrationNamespaced(t)

	_, err := nb.Get("nonexistent_key")
	assert.True(t, errors.Is(err, ErrNotFound))

	err = nb.Delete("nonexistent_key")
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestIntegrationKeychain_ListKeys(t *testing.T) {
	nb, _ := newIntegrationNamespaced(t)

	// Empty project namespace.
	keys, err := nb.List()
	require.NoError(t, err)
	assert.Empty(t, keys)

	// Add multiple keys.
	testKeys := map[string]string{
		"zebra_key":  "val1",
		"alpha_key":  "val2",
		"middle_key": "val3",
	}
	for k, v := range testKeys {
		require.NoError(t, nb.Set(k, v))
	}

	keys, err = nb.List()
	require.NoError(t, err)
	assert.Len(t, keys, 3)

	// Verify all keys are present (order may vary from List, but
	// NamespacedBackend inherits order from inner backend's List).
	for _, k := range keys {
		_, exists := testKeys[k]
		assert.True(t, exists, "unexpected key in list: %q", k)
	}
}

func TestIntegrationKeychain_ListAfterDelete(t *testing.T) {
	nb, _ := newIntegrationNamespaced(t)

	require.NoError(t, nb.Set("keep_a", "1"))
	require.NoError(t, nb.Set("remove_b", "2"))
	require.NoError(t, nb.Set("keep_c", "3"))

	require.NoError(t, nb.Delete("remove_b"))

	keys, err := nb.List()
	require.NoError(t, err)
	assert.Len(t, keys, 2)

	for _, k := range keys {
		assert.NotEqual(t, "remove_b", k, "deleted key should not appear in list")
	}
}

func TestIntegrationKeychain_SpecialCharValues(t *testing.T) {
	nb, _ := newIntegrationNamespaced(t)

	tests := []struct {
		name  string
		key   string
		value string
	}{
		{"empty value", "empty_val", ""},
		{"unicode", "unicode_val", "héllo wörld 日本語"},
		{"newlines", "newline_val", "line1\nline2\nline3"},
		{"json", "json_val", `{"host":"localhost","port":5432}`},
		{"special chars", "special_val", "p@ss=w0rd&key#1!"},
		{"long token", "token_val", strings.Repeat("abcdef0123456789", 64)},
		{"equals sign", "equals_val", "key=value=more"},
		{"tabs and spaces", "whitespace_val", "  tabs\there\t  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := nb.Set(tt.key, tt.value)
			require.NoError(t, err)

			got, err := nb.Get(tt.key)
			require.NoError(t, err)
			assert.Equal(t, tt.value, got)
		})
	}
}

func TestIntegrationKeychain_ProjectIsolation(t *testing.T) {
	kb := newIntegrationKeychain(t)

	proj1, err := NewNamespacedBackend(kb, integrationTestProject+"/proj1")
	require.NoError(t, err)
	proj2, err := NewNamespacedBackend(kb, integrationTestProject+"/proj2")
	require.NoError(t, err)

	// Set the same key name in both projects.
	require.NoError(t, proj1.Set("shared_key", "proj1_value"))
	require.NoError(t, proj2.Set("shared_key", "proj2_value"))

	// Values are isolated.
	val1, err := proj1.Get("shared_key")
	require.NoError(t, err)
	assert.Equal(t, "proj1_value", val1)

	val2, err := proj2.Get("shared_key")
	require.NoError(t, err)
	assert.Equal(t, "proj2_value", val2)

	// Delete from proj1 does not affect proj2.
	require.NoError(t, proj1.Delete("shared_key"))

	_, err = proj1.Get("shared_key")
	assert.True(t, errors.Is(err, ErrNotFound))

	val2, err = proj2.Get("shared_key")
	require.NoError(t, err)
	assert.Equal(t, "proj2_value", val2)
}

func TestIntegrationKeychain_ProfileIsolation(t *testing.T) {
	kb := newIntegrationKeychain(t)

	dev, err := NewProfileNamespacedBackend(kb, integrationTestProject, "development")
	require.NoError(t, err)
	prod, err := NewProfileNamespacedBackend(kb, integrationTestProject, "production")
	require.NoError(t, err)

	require.NoError(t, dev.Set("db_pass", "dev_password"))
	require.NoError(t, prod.Set("db_pass", "prod_password"))

	devVal, err := dev.Get("db_pass")
	require.NoError(t, err)
	assert.Equal(t, "dev_password", devVal)

	prodVal, err := prod.Get("db_pass")
	require.NoError(t, err)
	assert.Equal(t, "prod_password", prodVal)

	// Lists are profile-scoped.
	devKeys, err := dev.List()
	require.NoError(t, err)
	assert.Equal(t, []string{"db_pass"}, devKeys)

	prodKeys, err := prod.List()
	require.NoError(t, err)
	assert.Equal(t, []string{"db_pass"}, prodKeys)
}

func TestIntegrationKeychain_RegistryFallback(t *testing.T) {
	kb := newIntegrationKeychain(t)
	nb, err := NewNamespacedBackend(kb, integrationTestProject)
	require.NoError(t, err)

	// Set up a registry with a memory backend (primary) and keychain (secondary).
	// Register the raw keychain (not the namespaced wrapper) so the registry
	// doesn't double-prefix keys — NamespacedBackend.Get prepends the project
	// namespace, but registry.Get passes the full key to each backend as-is.
	mem := newMemoryBackend("memory")
	reg := NewRegistry()
	require.NoError(t, reg.Register(mem))
	require.NoError(t, reg.Register(kb))

	// Store a secret only in keychain.
	require.NoError(t, nb.Set("kc_only", "from_keychain"))

	// Registry should fall through memory (not found) to keychain.
	val, err := reg.Get(integrationTestProject + "/kc_only")
	require.NoError(t, err)
	assert.Equal(t, "from_keychain", val)

	// Store in memory — memory takes priority.
	require.NoError(t, mem.Set(integrationTestProject+"/kc_only", "from_memory"))
	val, err = reg.Get(integrationTestProject + "/kc_only")
	require.NoError(t, err)
	assert.Equal(t, "from_memory", val)
}

func TestIntegrationKeychain_ConcurrentAccess(t *testing.T) {
	nb, _ := newIntegrationNamespaced(t)

	const numGoroutines = 10
	var wg sync.WaitGroup

	// Concurrent writes to different keys.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("concurrent_key_%d", idx)
			val := fmt.Sprintf("value_%d", idx)
			err := nb.Set(key, val)
			assert.NoError(t, err, "concurrent Set(%q)", key)
		}(i)
	}
	wg.Wait()

	// Verify all keys were written.
	for i := 0; i < numGoroutines; i++ {
		key := fmt.Sprintf("concurrent_key_%d", i)
		expected := fmt.Sprintf("value_%d", i)
		val, err := nb.Get(key)
		require.NoError(t, err, "Get(%q) after concurrent write", key)
		assert.Equal(t, expected, val)
	}

	// Concurrent reads.
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("concurrent_key_%d", idx)
			expected := fmt.Sprintf("value_%d", idx)
			val, err := nb.Get(key)
			assert.NoError(t, err, "concurrent Get(%q)", key)
			assert.Equal(t, expected, val)
		}(i)
	}
	wg.Wait()
}

func TestIntegrationKeychain_OverwritePreservesIndex(t *testing.T) {
	nb, _ := newIntegrationNamespaced(t)

	// Set a key multiple times — the index should not accumulate duplicates.
	for i := 0; i < 5; i++ {
		require.NoError(t, nb.Set("overwrite_key", fmt.Sprintf("v%d", i)))
	}

	keys, err := nb.List()
	require.NoError(t, err)

	count := 0
	for _, k := range keys {
		if k == "overwrite_key" {
			count++
		}
	}
	assert.Equal(t, 1, count, "key should appear exactly once in list after multiple overwrites")

	// Final value should be the last one written.
	val, err := nb.Get("overwrite_key")
	require.NoError(t, err)
	assert.Equal(t, "v4", val)
}

func TestIntegrationKeychain_DeleteAndRecreate(t *testing.T) {
	nb, _ := newIntegrationNamespaced(t)

	// Create, delete, recreate.
	require.NoError(t, nb.Set("recreate_key", "first"))
	require.NoError(t, nb.Delete("recreate_key"))

	_, err := nb.Get("recreate_key")
	assert.True(t, errors.Is(err, ErrNotFound))

	require.NoError(t, nb.Set("recreate_key", "second"))
	val, err := nb.Get("recreate_key")
	require.NoError(t, err)
	assert.Equal(t, "second", val)

	// Key appears once in list.
	keys, err := nb.List()
	require.NoError(t, err)
	count := 0
	for _, k := range keys {
		if k == "recreate_key" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestIntegrationKeychain_ManyKeys(t *testing.T) {
	nb, _ := newIntegrationNamespaced(t)

	const numKeys = 50

	// Store many keys.
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("bulk_key_%03d", i)
		val := fmt.Sprintf("bulk_value_%03d", i)
		require.NoError(t, nb.Set(key, val), "Set(%q)", key)
	}

	// List should return all keys.
	keys, err := nb.List()
	require.NoError(t, err)
	assert.Len(t, keys, numKeys)

	// Verify each key has the correct value.
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("bulk_key_%03d", i)
		expected := fmt.Sprintf("bulk_value_%03d", i)
		val, err := nb.Get(key)
		require.NoError(t, err, "Get(%q)", key)
		assert.Equal(t, expected, val)
	}

	// Delete every other key.
	for i := 0; i < numKeys; i += 2 {
		key := fmt.Sprintf("bulk_key_%03d", i)
		require.NoError(t, nb.Delete(key), "Delete(%q)", key)
	}

	keys, err = nb.List()
	require.NoError(t, err)
	assert.Len(t, keys, numKeys/2)
}
