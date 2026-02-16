package backend

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xcke/envref/internal/audit"
)

// mockBackend is a simple in-memory backend for testing.
type mockBackend struct {
	name    string
	secrets map[string]string
}

func newMockBackend(name string) *mockBackend {
	return &mockBackend{name: name, secrets: make(map[string]string)}
}

func (m *mockBackend) Name() string { return m.name }

func (m *mockBackend) Get(key string) (string, error) {
	val, ok := m.secrets[key]
	if !ok {
		return "", ErrNotFound
	}
	return val, nil
}

func (m *mockBackend) Set(key, value string) error {
	m.secrets[key] = value
	return nil
}

func (m *mockBackend) Delete(key string) error {
	if _, ok := m.secrets[key]; !ok {
		return ErrNotFound
	}
	delete(m.secrets, key)
	return nil
}

func (m *mockBackend) List() ([]string, error) {
	var keys []string
	for k := range m.secrets {
		keys = append(keys, k)
	}
	return keys, nil
}

func TestAuditBackend_Set_LogsEntry(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")
	logger := audit.NewLogger(logPath)

	inner := newMockBackend("test")
	ab := NewAuditBackend(inner, logger, "myproject")

	err := ab.Set("API_KEY", "secret123")
	require.NoError(t, err)

	// Verify the value was stored.
	val, err := inner.Get("API_KEY")
	require.NoError(t, err)
	assert.Equal(t, "secret123", val)

	// Verify audit entry was logged.
	entries, err := logger.Read()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, audit.OpSet, entries[0].Operation)
	assert.Equal(t, "API_KEY", entries[0].Key)
	assert.Equal(t, "test", entries[0].Backend)
	assert.Equal(t, "myproject", entries[0].Project)
}

func TestAuditBackend_Delete_LogsEntry(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")
	logger := audit.NewLogger(logPath)

	inner := newMockBackend("test")
	inner.secrets["API_KEY"] = "old"

	ab := NewAuditBackend(inner, logger, "myproject")

	err := ab.Delete("API_KEY")
	require.NoError(t, err)

	// Verify the value was deleted.
	_, err = inner.Get("API_KEY")
	assert.ErrorIs(t, err, ErrNotFound)

	// Verify audit entry was logged.
	entries, err := logger.Read()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, audit.OpDelete, entries[0].Operation)
	assert.Equal(t, "API_KEY", entries[0].Key)
}

func TestAuditBackend_Get_DoesNotLog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")
	logger := audit.NewLogger(logPath)

	inner := newMockBackend("test")
	inner.secrets["KEY"] = "val"

	ab := NewAuditBackend(inner, logger, "myproject")

	val, err := ab.Get("KEY")
	require.NoError(t, err)
	assert.Equal(t, "val", val)

	// No audit entry for Get.
	entries, err := logger.Read()
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestAuditBackend_List_DoesNotLog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")
	logger := audit.NewLogger(logPath)

	inner := newMockBackend("test")
	inner.secrets["A"] = "1"
	inner.secrets["B"] = "2"

	ab := NewAuditBackend(inner, logger, "myproject")

	keys, err := ab.List()
	require.NoError(t, err)
	assert.Len(t, keys, 2)

	// No audit entry for List.
	entries, err := logger.Read()
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestAuditBackend_WithProfile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")
	logger := audit.NewLogger(logPath)

	inner := newMockBackend("test")
	ab := NewAuditBackend(inner, logger, "myproject", WithAuditProfile("staging"))

	require.NoError(t, ab.Set("KEY", "val"))

	entries, err := logger.Read()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "staging", entries[0].Profile)
}

func TestAuditBackend_WithOperation(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")
	logger := audit.NewLogger(logPath)

	inner := newMockBackend("test")
	ab := NewAuditBackend(inner, logger, "myproject", WithAuditOperation(audit.OpRotate))

	require.NoError(t, ab.Set("KEY", "val"))

	entries, err := logger.Read()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, audit.OpRotate, entries[0].Operation)
}

func TestAuditBackend_Name(t *testing.T) {
	inner := newMockBackend("mybackend")
	ab := NewAuditBackend(inner, audit.NewLogger("/dev/null"), "p")
	assert.Equal(t, "mybackend", ab.Name())
}

func TestAuditBackend_Set_InnerError_NoLog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")
	logger := audit.NewLogger(logPath)

	inner := &failingBackend{name: "fail"}
	ab := NewAuditBackend(inner, logger, "myproject")

	err := ab.Set("KEY", "val")
	assert.Error(t, err)

	// No log entry on failure.
	entries, err := logger.Read()
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestAuditBackend_Delete_InnerError_NoLog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")
	logger := audit.NewLogger(logPath)

	inner := newMockBackend("test")
	// KEY doesn't exist, so Delete returns ErrNotFound.
	ab := NewAuditBackend(inner, logger, "myproject")

	err := ab.Delete("MISSING")
	assert.Error(t, err)

	// No log entry on failure.
	_, readErr := os.ReadFile(logPath)
	assert.True(t, os.IsNotExist(readErr))
}

// failingBackend always returns errors for mutation operations.
type failingBackend struct {
	name string
}

func (f *failingBackend) Name() string                  { return f.name }
func (f *failingBackend) Get(key string) (string, error) { return "", ErrNotFound }
func (f *failingBackend) Set(key, value string) error    { return assert.AnError }
func (f *failingBackend) Delete(key string) error        { return assert.AnError }
func (f *failingBackend) List() ([]string, error)        { return nil, nil }
