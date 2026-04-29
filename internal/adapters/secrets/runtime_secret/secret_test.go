package runtime_secret

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDoRunsCallback(t *testing.T) {
	runner := NewRunner()
	called := false
	runner.Do(func() {
		called = true
	})
	require.True(t, called, "Do should invoke the callback")
}

func TestBytesPassesCopyAndDoesNotMutateOriginal(t *testing.T) {
	runner := NewRunner()
	original := []byte("secret-value-123")

	var received []byte
	runner.Bytes(original, func(buf []byte) {
		// Copy what we received so we can inspect it after the callback returns.
		received = make([]byte, len(buf))
		copy(received, buf)
		// Mutate the callback slice to prove it's an owned copy.
		for i := range buf {
			buf[i] = 0
		}
	})

	// Original must be unchanged.
	require.Equal(t, []byte("secret-value-123"), original,
		"mutating the callback slice must not affect original")
	// The received copy must contain the original value.
	require.Equal(t, []byte("secret-value-123"), received,
		"callback must receive the original value")
}

func TestStringPassesValue(t *testing.T) {
	runner := NewRunner()
	var result string
	runner.String("hello-world", func(s string) {
		result = s
	})
	require.Equal(t, "hello-world", result)
}
