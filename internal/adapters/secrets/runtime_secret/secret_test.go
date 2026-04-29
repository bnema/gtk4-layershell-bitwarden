package runtime_secret

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// panicRecorder implements a simple recover wrapper for testing panics.
func panicRecorder(fn func()) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
		}
	}()
	fn()
	return
}

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

func TestBytesPanicDoesNotSkipZeroing(t *testing.T) {
	runner := NewRunner()
	original := []byte("sensitive-data")

	// Capture the buffer address so we can inspect it after the panic.
	var bufAddr *[]byte
	panicked := panicRecorder(func() {
		runner.Bytes(original, func(buf []byte) {
			bufAddr = &buf
			// Read a byte to prove the value was copied in.
			_ = buf[0]
			panic("simulated panic")
		})
	})
	require.True(t, panicked, "callback should have panicked")

	// After the panic the defer-zeroing must have run.
	if bufAddr != nil {
		for i, b := range *bufAddr {
			require.Equal(t, byte(0), b, "byte %d should be zeroed after panic", i)
		}
	}

	// Original must be unchanged.
	require.Equal(t, []byte("sensitive-data"), original,
		"original must not be mutated by panic test")
}
