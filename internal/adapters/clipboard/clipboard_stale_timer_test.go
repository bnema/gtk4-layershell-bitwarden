package clipboard

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRapidSetOnlyLatestTimerClears(t *testing.T) {
	var mu sync.Mutex
	clearCount := 0
	adapter := New(nil, func() error {
		mu.Lock()
		clearCount++
		mu.Unlock()
		return nil
	})

	for i := 0; i < 100; i++ {
		require.NoError(t, adapter.Set(context.Background(), "value", 20*time.Millisecond))
	}

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return clearCount == 1
	}, time.Second, 5*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, clearCount, "stale timer callbacks must not clear newer clipboard generations")
}
