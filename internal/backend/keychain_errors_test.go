package backend

import (
	"errors"
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

func TestKeychainError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *KeychainError
		contains []string
	}{
		{
			name: "get with key",
			err: &KeychainError{
				Kind: KeychainErrUnavailable,
				Op:   "get",
				Key:  "API_KEY",
				Hint: "install gnome-keyring",
				Err:  errors.New("dbus connection refused"),
			},
			contains: []string{"keychain get", "API_KEY", "unavailable", "hint:", "install gnome-keyring"},
		},
		{
			name: "list without key",
			err: &KeychainError{
				Kind: KeychainErrLocked,
				Op:   "list",
				Key:  "",
				Hint: "unlock the keychain",
				Err:  errors.New("locked"),
			},
			contains: []string{"keychain list", "locked", "hint:", "unlock the keychain"},
		},
		{
			name: "no hint",
			err: &KeychainError{
				Kind: KeychainErrUnknown,
				Op:   "set",
				Key:  "DB_PASS",
				Hint: "",
				Err:  errors.New("something went wrong"),
			},
			contains: []string{"keychain set", "DB_PASS", "unknown"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.Error()
			for _, s := range tt.contains {
				assert.Contains(t, msg, s)
			}
		})
	}
}

func TestKeychainError_Unwrap(t *testing.T) {
	inner := errors.New("inner error")
	err := &KeychainError{
		Kind: KeychainErrUnavailable,
		Op:   "get",
		Key:  "key",
		Err:  inner,
	}

	assert.True(t, errors.Is(err, inner))
}

func TestKeychainError_ErrorsAs(t *testing.T) {
	inner := errors.New("dbus connection refused")
	err := classifyKeychainErr("get", "key", inner)

	var kErr *KeychainError
	require.True(t, errors.As(err, &kErr))
	assert.Equal(t, "get", kErr.Op)
	assert.Equal(t, "key", kErr.Key)
	assert.NotEmpty(t, kErr.Hint)
}

func TestKeychainErrKind_String(t *testing.T) {
	tests := []struct {
		kind KeychainErrKind
		want string
	}{
		{KeychainErrUnavailable, "unavailable"},
		{KeychainErrLocked, "locked"},
		{KeychainErrPermission, "permission denied"},
		{KeychainErrDataTooBig, "data too big"},
		{KeychainErrUnknown, "unknown"},
		{KeychainErrKind(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.kind.String())
		})
	}
}

func TestClassifyRawErr_UnsupportedPlatform(t *testing.T) {
	err := errors.New("unsupported platform: plan9")
	kind, hint := classifyRawErr(err)
	assert.Equal(t, KeychainErrUnavailable, kind)
	assert.Contains(t, hint, "not supported")
}

func TestClassifyRawErr_DataTooBig(t *testing.T) {
	err := errors.New("data passed to Set was too big")
	kind, hint := classifyRawErr(err)
	assert.Equal(t, KeychainErrDataTooBig, kind)
	assert.Contains(t, hint, "size limit")
}

func TestClassifyRawErr_NilErr(t *testing.T) {
	kind, _ := classifyRawErr(nil)
	assert.Equal(t, KeychainErrUnknown, kind)
}

func TestClassifyLinuxErr(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		wantKind KeychainErrKind
	}{
		{"dbus_error", "dbus: connection refused", KeychainErrUnavailable},
		{"session_bus", "could not connect to session bus", KeychainErrUnavailable},
		{"freedesktop", "org.freedesktop.secrets not found", KeychainErrUnavailable},
		{"unlock_failed", "failed to unlock collection", KeychainErrLocked},
		{"dismissed", "prompt was dismissed by user", KeychainErrLocked},
		{"locked", "collection is locked", KeychainErrLocked},
		{"permission", "permission denied", KeychainErrPermission},
		{"access_denied", "access denied to secret service", KeychainErrPermission},
		{"unknown", "something unexpected happened", KeychainErrUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, hint := classifyLinuxErr(tt.errMsg)
			assert.Equal(t, tt.wantKind, kind)
			assert.NotEmpty(t, hint)
		})
	}
}

func TestClassifyDarwinErr(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		wantKind KeychainErrKind
	}{
		{"user_canceled", "user canceled the authorization", KeychainErrLocked},
		{"lock", "keychain is locked", KeychainErrLocked},
		{"internal", "errsecinternalcomponent", KeychainErrLocked},
		{"permission", "permission denied", KeychainErrPermission},
		{"authorization", "authorization failed", KeychainErrPermission},
		{"denied", "access denied", KeychainErrPermission},
		{"not_found_binary", "executable file not found", KeychainErrUnavailable},
		{"no_such_file", "no such file or directory", KeychainErrUnavailable},
		{"unknown", "something else went wrong", KeychainErrUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, hint := classifyDarwinErr(tt.errMsg)
			assert.Equal(t, tt.wantKind, kind)
			assert.NotEmpty(t, hint)
		})
	}
}

func TestClassifyWindowsErr(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		wantKind KeychainErrKind
	}{
		{"access_denied", "access is denied", KeychainErrPermission},
		{"permission", "permission denied", KeychainErrPermission},
		{"service", "service unavailable", KeychainErrUnavailable},
		{"unknown", "some other error", KeychainErrUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, hint := classifyWindowsErr(tt.errMsg)
			assert.Equal(t, tt.wantKind, kind)
			assert.NotEmpty(t, hint)
		})
	}
}

func TestClassifyKeychainErr_IntegrationWithOps(t *testing.T) {
	tests := []struct {
		op  string
		key string
	}{
		{"get", "API_KEY"},
		{"set", "DB_PASS"},
		{"delete", "TOKEN"},
		{"list", ""},
	}

	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			raw := errors.New("some keychain error")
			kErr := classifyKeychainErr(tt.op, tt.key, raw)
			assert.Equal(t, tt.op, kErr.Op)
			assert.Equal(t, tt.key, kErr.Key)
			assert.True(t, errors.Is(kErr, raw))
		})
	}
}

// TestKeychainBackend_GetError verifies that non-ErrNotFound errors from
// the keychain are returned as *KeychainError with classification.
func TestKeychainBackend_GetError(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	// Override Get to return a non-ErrNotFound error.
	origGet := keyringProvider.Get
	keyringProvider.Get = func(service, user string) (string, error) {
		return "", errors.New("dbus: connection refused")
	}
	defer func() { keyringProvider.Get = origGet }()

	kb := NewKeychainBackend()
	_, err := kb.Get("some_key")

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrNotFound))

	var kErr *KeychainError
	require.True(t, errors.As(err, &kErr))
	assert.Equal(t, "get", kErr.Op)
	assert.Equal(t, "some_key", kErr.Key)
	assert.NotEmpty(t, kErr.Hint)
}

// TestKeychainBackend_SetError verifies that errors from Set are returned
// as *KeychainError.
func TestKeychainBackend_SetError(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	// Override Set to return an error.
	origSet := keyringProvider.Set
	keyringProvider.Set = func(service, user, password string) error {
		return errors.New("unsupported platform: plan9")
	}
	defer func() { keyringProvider.Set = origSet }()

	kb := NewKeychainBackend()
	err := kb.Set("some_key", "value")

	require.Error(t, err)

	var kErr *KeychainError
	require.True(t, errors.As(err, &kErr))
	assert.Equal(t, "set", kErr.Op)
	assert.Equal(t, "some_key", kErr.Key)
	assert.Equal(t, KeychainErrUnavailable, kErr.Kind)
}

// TestKeychainBackend_DeleteError verifies that non-ErrNotFound errors from
// Delete are returned as *KeychainError.
func TestKeychainBackend_DeleteError(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	// Override Delete to return a non-ErrNotFound error.
	origDelete := keyringProvider.Delete
	keyringProvider.Delete = func(service, user string) error {
		return errors.New("access is denied")
	}
	defer func() { keyringProvider.Delete = origDelete }()

	kb := NewKeychainBackend()
	err := kb.Delete("some_key")

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrNotFound))

	var kErr *KeychainError
	require.True(t, errors.As(err, &kErr))
	assert.Equal(t, "delete", kErr.Op)
	assert.Equal(t, "some_key", kErr.Key)
}

// TestKeychainBackend_ListError verifies that errors from List are returned
// as *KeychainError.
func TestKeychainBackend_ListError(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	// First store a corrupted index value.
	_ = keyring.Set("envref", keychainIndexKey, "not-valid-json")

	kb := NewKeychainBackend()
	_, err := kb.List()

	require.Error(t, err)

	var kErr *KeychainError
	require.True(t, errors.As(err, &kErr))
	assert.Equal(t, "list", kErr.Op)
}

// TestKeychainBackend_SetIndexError verifies that errors during index update
// in Set are returned as *KeychainError.
func TestKeychainBackend_SetIndexError(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	kb := NewKeychainBackend()

	// First Set succeeds, then override Get to fail when reading index.
	callCount := 0
	origGet := keyringProvider.Get
	keyringProvider.Get = func(service, user string) (string, error) {
		callCount++
		// First call is for Set (which succeeds), second is for reading index.
		if callCount > 0 && user == keychainIndexKey {
			return "", errors.New("dbus: connection refused")
		}
		return origGet(service, user)
	}
	defer func() { keyringProvider.Get = origGet }()

	err := kb.Set("key1", "val1")
	require.Error(t, err)

	var kErr *KeychainError
	require.True(t, errors.As(err, &kErr))
	assert.Equal(t, "set", kErr.Op)
}

// TestClassifyRawErr_PlatformSpecific ensures the current platform's
// classifier is exercised.
func TestClassifyRawErr_PlatformSpecific(t *testing.T) {
	err := errors.New("some random error for this platform")
	kind, hint := classifyRawErr(err)

	// On any supported platform, the generic unknown classifier should
	// produce a non-empty hint.
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
		assert.NotEmpty(t, hint)
	default:
		// Fallback platforms get a generic hint.
		assert.Equal(t, KeychainErrUnknown, kind)
		assert.NotEmpty(t, hint)
	}
	_ = kind // used in switch above
}

// TestKeychainBackend_GetErrorStillReturnsNotFound verifies that ErrNotFound
// is still returned correctly (not wrapped as KeychainError).
func TestKeychainBackend_GetErrorStillReturnsNotFound(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	kb := NewKeychainBackend()
	_, err := kb.Get("nonexistent")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))

	var kErr *KeychainError
	assert.False(t, errors.As(err, &kErr), "ErrNotFound should not be wrapped as KeychainError")
}

// TestKeychainBackend_DeleteErrorStillReturnsNotFound verifies that ErrNotFound
// from Delete is still returned correctly.
func TestKeychainBackend_DeleteErrorStillReturnsNotFound(t *testing.T) {
	cleanup := setupMockKeyring()
	defer cleanup()

	kb := NewKeychainBackend()
	err := kb.Delete("nonexistent")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))

	var kErr *KeychainError
	assert.False(t, errors.As(err, &kErr), "ErrNotFound should not be wrapped as KeychainError")
}

// TestKeychainError_HintFormatting verifies the hint includes the newline
// correctly in multi-line hints.
func TestKeychainError_HintFormatting(t *testing.T) {
	err := classifyKeychainErr("get", "KEY", fmt.Errorf("dbus: connection refused"))

	msg := err.Error()
	assert.Contains(t, msg, "\nhint: ")
}
