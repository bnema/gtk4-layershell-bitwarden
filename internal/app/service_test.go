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
	coresync "github.com/bnema/gtk4-layershell-bitwarden/internal/core/sync"
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

	// Configurable Sync
	syncBlockCh chan struct{}
	syncItems   []vault.Item
	syncFolders []vault.Folder
	syncRev     string
	syncErr     error

	// Configurable Create
	createErr  error
	createItem vault.Item

	// Configurable Update
	updateErr  error
	updateItem vault.Item

	// Configurable Trash
	trashErr error

	// Configurable Restore
	restoreErr  error
	restoreItem vault.Item

	// Configurable Delete
	deleteErr error
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

func (r *fakeRemote) Sync(ctx context.Context) ([]vault.Item, []vault.Folder, string, error) {
	r.mu.Lock()
	blockCh := r.syncBlockCh
	items := r.syncItems
	folders := r.syncFolders
	rev := r.syncRev
	err := r.syncErr
	r.mu.Unlock()

	if blockCh != nil {
		select {
		case <-ctx.Done():
			return nil, nil, "", ctx.Err()
		case <-blockCh:
		}
	}

	return items, folders, rev, err
}

func (r *fakeRemote) Create(_ context.Context, item vault.Item) (vault.Item, error) {
	r.mu.Lock()
	err := r.createErr
	result := r.createItem
	r.mu.Unlock()
	if err != nil {
		return vault.Item{}, err
	}
	if result.ID == "" {
		result = item
	}
	return result, nil
}

func (r *fakeRemote) Update(_ context.Context, id string, item vault.Item) (vault.Item, error) {
	r.mu.Lock()
	err := r.updateErr
	result := r.updateItem
	r.mu.Unlock()
	if err != nil {
		return vault.Item{}, err
	}
	if result.ID == "" {
		result = item
		result.ID = id
	}
	return result, nil
}

func (r *fakeRemote) Trash(_ context.Context, _ string) error {
	r.mu.Lock()
	err := r.trashErr
	r.mu.Unlock()
	return err
}

func (r *fakeRemote) Restore(_ context.Context, id string) (vault.Item, error) {
	r.mu.Lock()
	err := r.restoreErr
	result := r.restoreItem
	r.mu.Unlock()
	if err != nil {
		return vault.Item{}, err
	}
	return result, nil
}

func (r *fakeRemote) Delete(_ context.Context, _ string) error {
	r.mu.Lock()
	err := r.deleteErr
	r.mu.Unlock()
	return err
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

// ---------------------------------------------------------------------------
// Test-only helpers
// ---------------------------------------------------------------------------

func (s *Service) pendingMutationsForTest() []coresync.OutboxMutation {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]coresync.OutboxMutation, len(s.outbox))
	copy(result, s.outbox)
	return result
}

func (s *Service) conflictsForTest() []coresync.Conflict {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]coresync.Conflict, len(s.conflicts))
	copy(result, s.conflicts)
	return result
}

// ---------------------------------------------------------------------------
// Offline mutation tests
// ---------------------------------------------------------------------------

func TestCreateQueuesPendingWhenRemoteFails(t *testing.T) {
	fr := &fakeRemote{
		createErr: context.DeadlineExceeded,
	}

	svc := NewService(Deps{Remote: fr})
	svc.mu.Lock()
	svc.state = auth.LockStateUnlocked
	svc.mu.Unlock()

	item, err := svc.Create(context.Background(), vault.Item{Name: "Offline"})
	require.NoError(t, err)
	require.Equal(t, vault.SyncStatusPending, item.SyncStatus)
	require.Contains(t, item.ID, "local-")

	pending := svc.pendingMutationsForTest()
	require.Len(t, pending, 1)
	require.Equal(t, coresync.MutationCreate, pending[0].Kind)
	require.Equal(t, item.ID, pending[0].ItemID)

	// Verify item exists in local list.
	items, err := svc.Items(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "Offline", items[0].Name)
}

func TestCreateOnlineUpdatesLocalSynced(t *testing.T) {
	fr := &fakeRemote{
		createItem: vault.Item{ID: "remote-1", Name: "SyncedItem", RevisionDate: time.Now()},
	}

	svc := NewService(Deps{Remote: fr})
	svc.mu.Lock()
	svc.state = auth.LockStateUnlocked
	svc.mu.Unlock()

	item, err := svc.Create(context.Background(), vault.Item{Name: "NewItem"})
	require.NoError(t, err)
	require.Equal(t, "remote-1", item.ID)
	require.Equal(t, vault.SyncStatusSynced, item.SyncStatus)

	// No pending mutations.
	pending := svc.pendingMutationsForTest()
	require.Len(t, pending, 0)

	// Search should find the item.
	results, err := svc.Search(context.Background(), "SyncedItem", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "remote-1", results[0].Item.ID)
}

func TestSyncConflictMarksItem(t *testing.T) {
	localItem := vault.Item{
		ID:   "item-1",
		Name: "LocalItem",
		Type: vault.ItemTypeLogin,
	}

	fr := &fakeRemote{
		revisionRev: "new-rev",
		syncItems: []vault.Item{
			{ID: "item-1", Name: "RemoteItem", RevisionDate: time.Now(), Type: vault.ItemTypeLogin},
		},
		syncFolders: nil,
		syncRev:     "new-rev",
	}

	svc := NewService(Deps{Remote: fr})
	svc.mu.Lock()
	svc.state = auth.LockStateUnlocked
	svc.items = []vault.Item{localItem}
	svc.outbox = []coresync.OutboxMutation{
		{ID: "m1", Kind: coresync.MutationUpdate, ItemID: "item-1"},
	}
	svc.rebuildIndexLocked()
	svc.mu.Unlock()

	// Run sync.
	svc.syncOnce(context.Background())

	// Check item is marked as conflict.
	svc.mu.Lock()
	require.Len(t, svc.items, 1)
	require.Equal(t, vault.SyncStatusConflict, svc.items[0].SyncStatus)
	require.NotEmpty(t, svc.items[0].ConflictID)
	conflictCount := len(svc.conflicts)
	svc.mu.Unlock()

	require.Equal(t, 1, conflictCount)
}

func TestLockCancelsSyncInstall(t *testing.T) {
	fr := &fakeRemote{
		revisionRev: "rev2",
		syncBlockCh: make(chan struct{}),
		syncItems:   []vault.Item{{ID: "remote-1", Name: "ShouldNotAppear"}},
	}

	svc := NewService(Deps{Remote: fr})
	svc.mu.Lock()
	svc.state = auth.LockStateUnlocked
	svc.items = []vault.Item{{ID: "local-1", Name: "Original"}}
	svc.rebuildIndexLocked()
	svc.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		svc.syncOnce(ctx)
	}()

	// Wait for syncOnce to reach Remote.Sync (which blocks on syncBlockCh).
	time.Sleep(50 * time.Millisecond)

	// Cancel context — simulates Lock cancelling workers.
	cancel()

	// Unblock Sync after cancel has taken effect.
	time.Sleep(10 * time.Millisecond)
	close(fr.syncBlockCh)

	wg.Wait()

	// Verify original items remain (remote items were never installed).
	svc.mu.Lock()
	require.Len(t, svc.items, 1)
	require.Equal(t, "local-1", svc.items[0].ID)
	require.Equal(t, "Original", svc.items[0].Name)
	svc.mu.Unlock()
}
