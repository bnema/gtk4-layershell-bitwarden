package clipboard

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSetCallsSetter(t *testing.T) {
	var got string
	set := func(s string) error {
		got = s
		return nil
	}
	a := New(set, nil)
	err := a.Set(context.Background(), "hello", 0)
	require.NoError(t, err)
	require.Equal(t, "hello", got)
}

func TestTTLClearsAfterDuration(t *testing.T) {
	var mu sync.Mutex
	var cleared bool
	clear := func() error {
		mu.Lock()
		cleared = true
		mu.Unlock()
		return nil
	}
	a := New(nil, clear)

	err := a.Set(context.Background(), "ttl-test", 50*time.Millisecond)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return cleared
	}, 2*time.Second, 10*time.Millisecond, "clipboard should be cleared after TTL")
}

func TestSecondSetCancelsFirstTimer(t *testing.T) {
	var mu sync.Mutex
	var clearCount int
	clear := func() error {
		mu.Lock()
		clearCount++
		mu.Unlock()
		return nil
	}
	a := New(nil, clear)

	// Set with long TTL.
	err := a.Set(context.Background(), "first", 5*time.Second)
	require.NoError(t, err)

	// Overwrite with short TTL – first timer is cancelled.
	err = a.Set(context.Background(), "second", 50*time.Millisecond)
	require.NoError(t, err)

	// Wait for short TTL to fire.
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	count := clearCount
	mu.Unlock()

	require.Equal(t, 1, count, "clear should be called exactly once (from second timer)")
}

func TestClearCancelsTimerAndClears(t *testing.T) {
	var mu sync.Mutex
	var clearCount int
	clear := func() error {
		mu.Lock()
		clearCount++
		mu.Unlock()
		return nil
	}
	a := New(nil, clear)

	err := a.Set(context.Background(), "hello", 5*time.Second)
	require.NoError(t, err)

	err = a.Clear(context.Background())
	require.NoError(t, err)

	// Timer should have been cancelled and clear called.
	mu.Lock()
	require.Equal(t, 1, clearCount, "clear should be called once")
	mu.Unlock()

	// Wait a bit to ensure the timer doesn't fire again.
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	require.Equal(t, 1, clearCount, "clear should only be called once, timer should not fire again")
	mu.Unlock()
}

func TestNilFuncsDoNotPanic(t *testing.T) {
	a := New(nil, nil)
	require.NotNil(t, a)

	err := a.Set(context.Background(), "test", 0)
	require.NoError(t, err)

	err = a.Clear(context.Background())
	require.NoError(t, err)
}

func TestContextCancelledBeforeSet(t *testing.T) {
	a := New(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := a.Set(ctx, "should-not-set", 0)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestContextCancelledBeforeClear(t *testing.T) {
	a := New(nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := a.Clear(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}
