package clipboard

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIntegrationClipboardTTLClears(t *testing.T) {
	t.Run("TTL auto-clears after duration", func(t *testing.T) {
		var mu sync.Mutex
		var cleared bool
		var currentValue string

		set := func(s string) error {
			mu.Lock()
			currentValue = s
			mu.Unlock()
			return nil
		}
		clear := func() error {
			mu.Lock()
			cleared = true
			currentValue = ""
			mu.Unlock()
			return nil
		}

		a := New(set, clear)

		err := a.Set(context.Background(), "ttl-value", 20*time.Millisecond)
		require.NoError(t, err)

		// Value should be set immediately.
		mu.Lock()
		require.Equal(t, "ttl-value", currentValue)
		mu.Unlock()

		// Wait for TTL to clear.
		require.Eventually(t, func() bool {
			mu.Lock()
			defer mu.Unlock()
			return cleared && currentValue == ""
		}, 2*time.Second, 5*time.Millisecond, "clipboard should be cleared after TTL")
	})

	t.Run("Set second longer TTL then Clear", func(t *testing.T) {
		var mu sync.Mutex
		var currentValue string

		set := func(s string) error {
			mu.Lock()
			currentValue = s
			mu.Unlock()
			return nil
		}
		clear := func() error {
			mu.Lock()
			currentValue = ""
			mu.Unlock()
			return nil
		}

		a := New(set, clear)

		// Set with long TTL.
		err := a.Set(context.Background(), "first", 5*time.Second)
		require.NoError(t, err)

		mu.Lock()
		require.Equal(t, "first", currentValue)
		mu.Unlock()

		// Clear explicitly.
		err = a.Clear(context.Background())
		require.NoError(t, err)

		mu.Lock()
		require.Empty(t, currentValue, "value should be empty after explicit Clear")
		mu.Unlock()

		// Wait a bit to ensure the original timer doesn't re-set the value.
		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		require.Empty(t, currentValue, "value should remain empty after timer would have fired")
		mu.Unlock()
	})
}
