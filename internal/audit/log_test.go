package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogger_Log_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	logger := NewLogger(path)
	err := logger.Log(Entry{
		Timestamp: "2025-01-15T10:30:00Z",
		User:      "alice",
		Operation: OpSet,
		Key:       "API_KEY",
		Backend:   "keychain",
		Project:   "myapp",
	})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"operation":"set"`)
	assert.Contains(t, string(data), `"key":"API_KEY"`)
	assert.Contains(t, string(data), `"user":"alice"`)
}

func TestLogger_Log_AppendsEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewLogger(path)

	require.NoError(t, logger.Log(Entry{
		Timestamp: "2025-01-15T10:00:00Z",
		User:      "alice",
		Operation: OpSet,
		Key:       "KEY_A",
		Backend:   "keychain",
		Project:   "myapp",
	}))
	require.NoError(t, logger.Log(Entry{
		Timestamp: "2025-01-15T10:01:00Z",
		User:      "bob",
		Operation: OpDelete,
		Key:       "KEY_B",
		Backend:   "vault",
		Project:   "myapp",
	}))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 2)
	assert.Contains(t, lines[0], `"key":"KEY_A"`)
	assert.Contains(t, lines[1], `"key":"KEY_B"`)
}

func TestLogger_Log_AutoFillsTimestampAndUser(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewLogger(path)

	err := logger.Log(Entry{
		Operation: OpGenerate,
		Key:       "SECRET",
		Backend:   "keychain",
		Project:   "test",
	})
	require.NoError(t, err)

	entries, err := logger.Read()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.NotEmpty(t, entries[0].Timestamp)
	assert.NotEmpty(t, entries[0].User)
}

func TestLogger_Read_EmptyFileReturnsNil(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	// File doesn't exist.
	logger := NewLogger(path)

	entries, err := logger.Read()
	require.NoError(t, err)
	assert.Nil(t, entries)
}

func TestLogger_Read_ParsesEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewLogger(path)

	// Write multiple entries.
	for _, op := range []Operation{OpSet, OpDelete, OpRotate, OpCopy, OpGenerate, OpImport} {
		require.NoError(t, logger.Log(Entry{
			Timestamp: "2025-01-15T10:00:00Z",
			User:      "dev",
			Operation: op,
			Key:       "K",
			Backend:   "keychain",
			Project:   "p",
		}))
	}

	entries, err := logger.Read()
	require.NoError(t, err)
	assert.Len(t, entries, 6)
	assert.Equal(t, OpSet, entries[0].Operation)
	assert.Equal(t, OpImport, entries[5].Operation)
}

func TestLogger_Read_WithProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewLogger(path)

	require.NoError(t, logger.Log(Entry{
		Timestamp: "2025-01-15T10:00:00Z",
		User:      "dev",
		Operation: OpSet,
		Key:       "DB_PASS",
		Backend:   "keychain",
		Project:   "myapp",
		Profile:   "staging",
	}))

	entries, err := logger.Read()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "staging", entries[0].Profile)
}

func TestLogger_Read_WithDetail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewLogger(path)

	require.NoError(t, logger.Log(Entry{
		Timestamp: "2025-01-15T10:00:00Z",
		User:      "dev",
		Operation: OpCopy,
		Key:       "API_KEY",
		Backend:   "keychain",
		Project:   "myapp",
		Detail:    "from project \"other\"",
	}))

	entries, err := logger.Read()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, `from project "other"`, entries[0].Detail)
}

func TestParseEntries_SkipsBlankLines(t *testing.T) {
	data := `{"timestamp":"2025-01-15T10:00:00Z","user":"a","operation":"set","key":"K","backend":"b","project":"p"}

{"timestamp":"2025-01-15T10:01:00Z","user":"a","operation":"delete","key":"K","backend":"b","project":"p"}
`
	entries, err := ParseEntries([]byte(data))
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}

func TestParseEntries_MalformedLineReturnsError(t *testing.T) {
	data := `{"timestamp":"2025-01-15T10:00:00Z","user":"a","operation":"set","key":"K","backend":"b","project":"p"}
not json
`
	_, err := ParseEntries([]byte(data))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing audit entry")
}

func TestParseEntries_EmptyInput(t *testing.T) {
	entries, err := ParseEntries([]byte(""))
	require.NoError(t, err)
	assert.Nil(t, entries)
}

func TestLogger_Path(t *testing.T) {
	logger := NewLogger("/some/path/audit.log")
	assert.Equal(t, "/some/path/audit.log", logger.Path())
}

func TestDefaultFileName(t *testing.T) {
	assert.Equal(t, ".envref.audit.log", DefaultFileName)
}

func TestLogger_Log_OmitsEmptyProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewLogger(path)

	require.NoError(t, logger.Log(Entry{
		Timestamp: "2025-01-15T10:00:00Z",
		User:      "dev",
		Operation: OpSet,
		Key:       "K",
		Backend:   "keychain",
		Project:   "p",
	}))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	// Profile should be omitted (omitempty) when empty.
	assert.NotContains(t, string(data), `"profile"`)
}

func TestLogger_Log_OmitsEmptyDetail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	logger := NewLogger(path)

	require.NoError(t, logger.Log(Entry{
		Timestamp: "2025-01-15T10:00:00Z",
		User:      "dev",
		Operation: OpSet,
		Key:       "K",
		Backend:   "keychain",
		Project:   "p",
	}))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	// Detail should be omitted (omitempty) when empty.
	assert.NotContains(t, string(data), `"detail"`)
}
