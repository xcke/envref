// Package backend provides secret storage backend implementations.
//
// This file defines error types and classification logic for keychain backend
// failures. It maps raw go-keyring errors into user-friendly categories with
// actionable hints for resolving common issues.
package backend

import (
	"fmt"
	"runtime"
	"strings"
)

// KeychainErrKind categorizes the type of keychain access failure.
type KeychainErrKind int

const (
	// KeychainErrUnavailable means no keychain service is available on the system.
	// This typically happens when there is no D-Bus session (Linux), the
	// security binary is missing (macOS), or the platform is unsupported.
	KeychainErrUnavailable KeychainErrKind = iota

	// KeychainErrLocked means the keychain is present but locked and could
	// not be unlocked (e.g., user dismissed the unlock prompt).
	KeychainErrLocked

	// KeychainErrPermission means the keychain denied access to the secret,
	// for example due to code signing or ACL restrictions.
	KeychainErrPermission

	// KeychainErrDataTooBig means the secret value exceeds the platform's
	// size limit.
	KeychainErrDataTooBig

	// KeychainErrUnknown means the error did not match any known category.
	KeychainErrUnknown
)

// String returns the human-readable name for this error kind.
func (k KeychainErrKind) String() string {
	switch k {
	case KeychainErrUnavailable:
		return "unavailable"
	case KeychainErrLocked:
		return "locked"
	case KeychainErrPermission:
		return "permission denied"
	case KeychainErrDataTooBig:
		return "data too big"
	case KeychainErrUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// KeychainError wraps a raw keychain error with a classification and
// a user-friendly hint for how to resolve the issue.
type KeychainError struct {
	// Kind classifies the error into a known category.
	Kind KeychainErrKind
	// Op is the operation that failed (e.g., "get", "set", "delete", "list").
	Op string
	// Key is the secret key involved, if applicable (empty for list operations).
	Key string
	// Hint provides actionable advice for the user to resolve the error.
	Hint string
	// Err is the underlying error from go-keyring.
	Err error
}

// Error returns a user-friendly error message including the operation,
// error kind, and actionable hint.
func (e *KeychainError) Error() string {
	var msg string
	if e.Key != "" {
		msg = fmt.Sprintf("keychain %s %q: %s", e.Op, e.Key, e.Kind)
	} else {
		msg = fmt.Sprintf("keychain %s: %s", e.Op, e.Kind)
	}
	if e.Hint != "" {
		msg += "\nhint: " + e.Hint
	}
	return msg
}

// Unwrap returns the underlying error for use with errors.Is and errors.As.
func (e *KeychainError) Unwrap() error {
	return e.Err
}

// classifyKeychainErr inspects a raw error from go-keyring and returns a
// KeychainError with the appropriate kind and hint. If the error is nil
// or ErrNotFound, it is returned as-is (not wrapped).
func classifyKeychainErr(op, key string, err error) *KeychainError {
	kind, hint := classifyRawErr(err)
	return &KeychainError{
		Kind: kind,
		Op:   op,
		Key:  key,
		Hint: hint,
		Err:  err,
	}
}

// classifyRawErr maps a raw go-keyring error to a KeychainErrKind and hint.
func classifyRawErr(err error) (KeychainErrKind, string) {
	if err == nil {
		return KeychainErrUnknown, ""
	}

	msg := err.Error()
	lower := strings.ToLower(msg)

	// Check for platform-unsupported errors.
	if strings.Contains(lower, "unsupported platform") {
		return KeychainErrUnavailable, fmt.Sprintf(
			"keychain is not supported on %s; consider using a different backend", runtime.GOOS)
	}

	// Check for data-too-big errors.
	if strings.Contains(lower, "data passed to set was too big") ||
		strings.Contains(lower, "too big") {
		return KeychainErrDataTooBig, "the secret value exceeds the platform's size limit; try a shorter value"
	}

	// Platform-specific classification.
	switch runtime.GOOS {
	case "linux":
		return classifyLinuxErr(lower)
	case "darwin":
		return classifyDarwinErr(lower)
	case "windows":
		return classifyWindowsErr(lower)
	default:
		return KeychainErrUnknown, "check that a supported keychain service is available on your system"
	}
}

// classifyLinuxErr classifies errors from the Linux Secret Service (D-Bus).
func classifyLinuxErr(lower string) (KeychainErrKind, string) {
	// D-Bus connection failures (no session bus, no daemon).
	if strings.Contains(lower, "dbus") ||
		strings.Contains(lower, "session bus") ||
		strings.Contains(lower, "org.freedesktop.secrets") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "no such host") {
		return KeychainErrUnavailable,
			"no secret service daemon found; install and start gnome-keyring or kwallet:\n" +
				"  sudo apt install gnome-keyring  # Debian/Ubuntu\n" +
				"  sudo dnf install gnome-keyring  # Fedora\n" +
				"  eval $(gnome-keyring-daemon --start --components=secrets)"
	}

	// Unlock failures (keyring locked, user dismissed prompt).
	if strings.Contains(lower, "unlock") ||
		strings.Contains(lower, "dismissed") ||
		strings.Contains(lower, "locked") {
		return KeychainErrLocked,
			"the keyring is locked; unlock it by logging into your desktop session or running:\n" +
				"  gnome-keyring-daemon --unlock"
	}

	// Permission / access denied.
	if strings.Contains(lower, "permission") ||
		strings.Contains(lower, "access denied") ||
		strings.Contains(lower, "not allowed") {
		return KeychainErrPermission,
			"access to the secret service was denied; check your D-Bus permissions"
	}

	return KeychainErrUnknown,
		"an unexpected keychain error occurred; ensure gnome-keyring or kwallet is running"
}

// classifyDarwinErr classifies errors from macOS Keychain.
func classifyDarwinErr(lower string) (KeychainErrKind, string) {
	// Keychain locked / user cancelled.
	if strings.Contains(lower, "user canceled") ||
		strings.Contains(lower, "user cancelled") ||
		strings.Contains(lower, "errsecinternalcomponent") ||
		strings.Contains(lower, "lock") {
		return KeychainErrLocked,
			"the macOS Keychain is locked; unlock it in Keychain Access.app or run:\n" +
				"  security unlock-keychain ~/Library/Keychains/login.keychain-db"
	}

	// Permission denied (code signing, ACL).
	if strings.Contains(lower, "permission") ||
		strings.Contains(lower, "authorization") ||
		strings.Contains(lower, "denied") ||
		strings.Contains(lower, "not allowed") ||
		strings.Contains(lower, "errsecrequiredentitlement") {
		return KeychainErrPermission,
			"macOS Keychain denied access; you may need to grant access in Keychain Access.app → Get Info → Access Control"
	}

	// security binary not found.
	if strings.Contains(lower, "executable file not found") ||
		strings.Contains(lower, "no such file") {
		return KeychainErrUnavailable,
			"the macOS security command was not found; this is unexpected — is /usr/bin/security present?"
	}

	return KeychainErrUnknown,
		"an unexpected macOS Keychain error occurred; try unlocking the keychain in Keychain Access.app"
}

// classifyWindowsErr classifies errors from Windows Credential Manager.
func classifyWindowsErr(lower string) (KeychainErrKind, string) {
	// Access denied.
	if strings.Contains(lower, "access is denied") ||
		strings.Contains(lower, "permission") {
		return KeychainErrPermission,
			"Windows Credential Manager denied access; try running the application as the same user who stored the secret"
	}

	// Service unavailable.
	if strings.Contains(lower, "service") ||
		strings.Contains(lower, "unavailable") {
		return KeychainErrUnavailable,
			"Windows Credential Manager is not available; ensure the Credential Manager service is running"
	}

	return KeychainErrUnknown,
		"an unexpected Windows Credential Manager error occurred; check Event Viewer for details"
}
