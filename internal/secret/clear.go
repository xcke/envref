// Package secret provides utilities for secure handling of sensitive data
// such as secret values and passphrases.
//
// The primary concern is minimizing the time that sensitive data remains
// in process memory. While Go's garbage collector makes it impossible to
// guarantee complete memory erasure (strings are immutable and may be
// copied by the runtime), clearing byte slices immediately after use
// significantly reduces the exposure window.
//
// These utilities follow the same best-effort approach used by Go's own
// crypto/subtle and x/crypto packages.
package secret

// ClearBytes overwrites a byte slice with zeros. This is a best-effort
// measure to reduce the time sensitive data (passphrases, decrypted
// secrets) remains in process memory.
//
// Callers should defer ClearBytes(b) immediately after allocating or
// receiving sensitive byte slices.
//
// Note: Go's runtime may have already copied the data during garbage
// collection or slice operations. This function clears the current
// backing array, which is the best we can do without unsafe memory
// access.
func ClearBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
