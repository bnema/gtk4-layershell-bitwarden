package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bnema/gtk4-layershell-bitwarden/internal/core/auth"
	"github.com/bnema/gtk4-layershell-bitwarden/internal/core/cache"
	coreerrors "github.com/bnema/gtk4-layershell-bitwarden/internal/core/errors"
	"github.com/bnema/gtk4-layershell-bitwarden/internal/core/vault"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Fake implementations for testing.
// ---------------------------------------------------------------------------

type fakeRemote struct {
	mu          sync.Mutex
	loginCalled bool
	lockCalled  bool
	revisionRev string
	revisionErr error
	syncStarted atomic.Bool
}

func (r *fakeRemote) Login(_ context.Context, email, password string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loginCalled = true
	_ = email
	_ = password
	return nil
}

func (r *fakeRemote) CompleteTwoFactor(_ context.Context, _, _ string, _ bool) error {
	return nil
}

func (r *fakeRemote) Lock(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lockCalled = true
	return nil
}

func (r *fakeRemote) Revision(_ context.Context) (string, error) {
	r.syncStarted.Store(true)
	return r.revisionRev, r.revisionErr
}

func (r *fakeRemote) Sync(_ context.Context) ([]vault.Item, []vault.Folder, string, error) {
	return nil, nil, "", nil
}

func (r *fakeRemote) Create(_ context.Context, _ vault.Item) (vault.Item, error) {
	return vault.Item{}, nil
}

func (r *fakeRemote) Update(_ context.Context, _ string, _ vault.Item) (vault.Item, error) {
	return vault.Item{}, nil
}

func (r *fakeRemote) Trash(_ context.Context, _ string) error {
	return nil
}

func (r *fakeRemote) Restore(_ context.Context, _ string) (vault.Item, error) {
	return vault.Item{}, nil
}

func (r *fakeRemote) Delete(_ context.Context, _ string) error {
	return nil
}

func (r *fakeRemote) ListAttachments(_ context.Context, _ string) ([]vault.Attachment, error) {
	return nil, nil
}

func (r *fakeRemote) DownloadAttachment(_ context.Context, _, _ string, _ io.Writer) error {
	return nil
}

func (r *fakeRemote) UploadAttachment(_ context.Context, _ string, _ string, _ int64, _ io.Reader) (vault.Attachment, error) {
	return vault.Attachment{}, nil
}

func (r *fakeRemote) DeleteAttachment(_ context.Context, _, _ string) error {
	return nil
}

type fakeCache struct {
	mu       sync.Mutex
	data     *cache.Snapshot
	loadErr  error
	loadCall int
}

func (c *fakeCache) Load(_ context.Context) (cache.Snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadCall++
	if c.loadErr != nil {
		return cache.Snapshot{}, c.loadErr
	}
	if c.data != nil {
		return *c.data, nil
	}
	return cache.Snapshot{}, nil
}

func (c *fakeCache) Save(_ context.Context, _ cache.Snapshot) error {
	return nil
}

func (c *fakeCache) Clear(_ context.Context) error {
	return nil
}

func (c *fakeCache) Path() string {
	return "/fake/path"
}

type fakeSecretBox struct{}

func (f *fakeSecretBox) Seal(plaintext, key []byte) ([]byte, error) {
	return plaintext, nil
}

func (f *fakeSecretBox) Open(ciphertext, key []byte) ([]byte, error) {
	return ciphertext, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildValidSnapshot creates a cache.Snapshot containing items as a PlainSnapshot,
// encrypted (via secretbox) with a key derived from the given password.
func buildValidSnapshot(t *testing.T, password string, items []vault.Item, folders []vault.Folder) cache.Snapshot {
	t.Helper()

	itemsJSON, err := json.Marshal(items)
	require.NoError(t, err)

	foldersJSON, err := json.Marshal(folders)
	require.NoError(t, err)

	plain := cache.PlainSnapshot{
		AccountHash:  "test-account-hash",
		LastRevision: "rev-1",
		ItemsJSON:    itemsJSON,
		FoldersJSON:  foldersJSON,
	}

	plainJSON, err := json.Marshal(plain)
	require.NoError(t, err)

	key := sha256.Sum256([]byte(password))

	box := &fakeSecretBox{}
	ciphertext, err := box.Seal(plainJSON, key[:])
	require.NoError(t, err)

	return cache.Snapshot{
		Version:         cache.Version,
		AccountHash:     "test-account-hash",
		LastRevision:    "rev-1",
		SavedAt:         time.Now(),
		VaultCiphertext: ciphertext,
	}
}

// consumeEvents reads all events from a channel until no more arrive within
// a short timeout, returning them in order.
func consumeEvents(t *testing.T, ch <-chan Event, timeout time.Duration) []Event {
	t.Helper()
	var result []Event
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return result
			}
			result = append(result, evt)
		case <-time.After(timeout):
			return result
		}
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestSearchLockedReturnsError(t *testing.T) {
	svc := NewService(Deps{})
	_, err := svc.Search(context.Background(), "git", 10)
	require.ErrorIs(t, err, coreerrors.ErrLocked)
}

func TestUnlockInstallsCacheIndexBeforeSync(t *testing.T) {
	gitItem := vault.Item{
		ID:   "item-1",
		Name: "GitHub",
		Type: vault.ItemTypeLogin,
		Login: &vault.Login{
			Username: "user@github.com",
		},
	}

	snap := buildValidSnapshot(t, "mypassword", []vault.Item{gitItem}, nil)

	fakeR := &fakeRemote{}
	fakeR.revisionRev = "rev-2"

	svc := NewService(Deps{
		Remote:    fakeR,
		Cache:     &fakeCache{data: &snap},
		SecretBox: &fakeSecretBox{},
	})

	err := svc.Unlock(context.Background(), "user@test.com", "mypassword")
	require.NoError(t, err)

	// Search should immediately return the cached GitHub item.
	results, err := svc.Search(context.Background(), "git", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "GitHub", results[0].Item.Name)

	// Eventually the sync worker should have checked revision.
	require.Eventually(t, func() bool {
		return fakeR.syncStarted.Load()
	}, 1*time.Second, 10*time.Millisecond, "sync worker should have started")
}

func TestLockClearsState(t *testing.T) {
	gitItem := vault.Item{
		ID:   "item-1",
		Name: "GitHub",
		Type: vault.ItemTypeLogin,
	}
	snap := buildValidSnapshot(t, "pw", []vault.Item{gitItem}, nil)

	svc := NewService(Deps{
		Cache:     &fakeCache{data: &snap},
		SecretBox: &fakeSecretBox{},
	})

	// Unlock
	err := svc.Unlock(context.Background(), "user@test.com", "pw")
	require.NoError(t, err)

	// Verify unlocked state
	_, err = svc.Search(context.Background(), "git", 10)
	require.NoError(t, err)

	// Lock
	err = svc.Lock(context.Background())
	require.NoError(t, err)

	// Search after lock returns error
	_, err = svc.Search(context.Background(), "git", 10)
	require.ErrorIs(t, err, coreerrors.ErrLocked)

	// Items after lock returns error
	_, err = svc.Items(context.Background())
	require.ErrorIs(t, err, coreerrors.ErrLocked)

	// Verify state is locked
	svc.mu.Lock()
	require.Equal(t, auth.LockStateLocked, svc.state)
	require.Nil(t, svc.items)
	require.Nil(t, svc.index)
	svc.mu.Unlock()
}

func TestEventsEmittedForUnlock(t *testing.T) {
	gitItem := vault.Item{
		ID:   "item-1",
		Name: "GitHub",
		Type: vault.ItemTypeLogin,
	}
	snap := buildValidSnapshot(t, "pw", []vault.Item{gitItem}, nil)

	fakeR := &fakeRemote{}
	fakeR.revisionRev = "rev-2"

	svc := NewService(Deps{
		Remote:    fakeR,
		Cache:     &fakeCache{data: &snap},
		SecretBox: &fakeSecretBox{},
	})

	err := svc.Unlock(context.Background(), "user@test.com", "pw")
	require.NoError(t, err)

	// Collect events with a generous timeout.
	events := consumeEvents(t, svc.events, 200*time.Millisecond)

	// Build a set of observed kinds.
	seen := make(map[EventKind]bool)
	for _, e := range events {
		seen[e.Kind] = true
	}

	require.True(t, seen[Unlocking], "expected Unlocking event")
	require.True(t, seen[CacheLoaded], "expected CacheLoaded event")
	require.True(t, seen[IndexReady], "expected IndexReady event")
}
