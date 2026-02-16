package secret

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClearBytes(t *testing.T) {
	t.Run("clears non-empty slice", func(t *testing.T) {
		b := []byte("super-secret-passphrase-123")
		ClearBytes(b)
		for i, v := range b {
			assert.Equal(t, byte(0), v, "byte at index %d should be zero", i)
		}
	})

	t.Run("handles nil slice", func(t *testing.T) {
		assert.NotPanics(t, func() {
			ClearBytes(nil)
		})
	})

	t.Run("handles empty slice", func(t *testing.T) {
		assert.NotPanics(t, func() {
			ClearBytes([]byte{})
		})
	})

	t.Run("clears single byte", func(t *testing.T) {
		b := []byte{0xFF}
		ClearBytes(b)
		assert.Equal(t, byte(0), b[0])
	})

	t.Run("clears large slice", func(t *testing.T) {
		b := make([]byte, 4096)
		for i := range b {
			b[i] = 0xAA
		}
		ClearBytes(b)
		for i, v := range b {
			assert.Equal(t, byte(0), v, "byte at index %d should be zero", i)
		}
	})
}
