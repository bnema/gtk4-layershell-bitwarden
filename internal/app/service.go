package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/bnema/gtk4-layershell-bitwarden/internal/core/auth"
	"github.com/bnema/gtk4-layershell-bitwarden/internal/core/cache"
	"github.com/bnema/gtk4-layershell-bitwarden/internal/core/config"
	cerrors "github.com/bnema/gtk4-layershell-bitwarden/internal/core/errors"
	coresync "github.com/bnema/gtk4-layershell-bitwarden/internal/core/sync"
	"github.com/bnema/gtk4-layershell-bitwarden/internal/core/vault"
)

// NewService creates a new Service with the given dependencies.
func NewService(deps Deps) *Service {
	cfg := deps.Config
	if cfg == nil {
		cfg = config.Default()
	}
	return &Service{
		cfg:    cfg,
		state:  auth.LockStateLocked,
		events: make(chan Event, 64),
		deps:   deps,
	}
}

// emit sends a non-blocking event to the events channel.
func (s *Service) emit(kind EventKind, message string) {
	select {
	case s.events <- Event{Kind: kind, Message: message}:
	default:
	}
}

// Unlock transitions the service from locked to unlocked.
func (s *Service) Unlock(ctx context.Context, email, password string) (retErr error) {
	s.mu.Lock()
	if s.state != auth.LockStateLocked {
		s.mu.Unlock()
		return fmt.Errorf("app: cannot unlock in state %s", s.state)
	}
	s.state = auth.LockStateUnlocking
	s.mu.Unlock()

	s.emit(Unlocking, "unlocking vault")

	// Login via remote if configured.
	if s.deps.Remote != nil {
		if err := s.deps.Remote.Login(ctx, email, password); err != nil {
			s.mu.Lock()
			s.state = auth.LockStateLocked
			s.mu.Unlock()
			return fmt.Errorf("app: login failed: %w", err)
		}
	}

	// Derive a cache key from the password.
	key := sha256.Sum256([]byte(password))

	// Load cache snapshot.
	loaded, err := s.loadCache(ctx, key[:])
	if err != nil {
		// Non-fatal: we can still unlock without cache.
		s.mu.Lock()
		s.state = auth.LockStateUnlocked
		s.mu.Unlock()
		s.emit(CacheLoaded, fmt.Sprintf("cache load skipped: %v", err))
		s.emit(IndexReady, "index ready (no cache)")
	} else if loaded {
		s.emit(CacheLoaded, "cache loaded from disk")
		s.emit(IndexReady, "search index ready")
	}

	// Start background sync worker.
	ctx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancelWorkers = cancel
	s.mu.Unlock()

	s.startMinimalSyncWorker(ctx)

	return nil
}

// loadCache attempts to load and decrypt a cached vault snapshot.
// Returns true if a valid cache was loaded, false with no error if none exists.
func (s *Service) loadCache(ctx context.Context, key []byte) (bool, error) {
	snap, err := s.deps.Cache.Load(ctx)
	if err != nil {
		return false, fmt.Errorf("cache load: %w", err)
	}

	if err := cache.ValidateSnapshot(snap); err != nil {
		return false, fmt.Errorf("cache validation: %w", err)
	}

	var ciphertext []byte
	if s.deps.SecretBox != nil {
		ciphertext, err = s.deps.SecretBox.Open(snap.VaultCiphertext, key)
		if err != nil {
			return false, fmt.Errorf("cache decrypt: %w", err)
		}
	} else {
		ciphertext = snap.VaultCiphertext
	}

	var plain cache.PlainSnapshot
	if err := json.Unmarshal(ciphertext, &plain); err != nil {
		return false, fmt.Errorf("cache decode: %w", err)
	}

	var items []vault.Item
	if err := json.Unmarshal(plain.ItemsJSON, &items); err != nil {
		return false, fmt.Errorf("cache items decode: %w", err)
	}

	var folders []vault.Folder
	if err := json.Unmarshal(plain.FoldersJSON, &folders); err != nil {
		return false, fmt.Errorf("cache folders decode: %w", err)
	}

	index := vault.BuildIndex(items)

	s.mu.Lock()
	s.items = items
	s.folders = folders
	s.index = index
	s.state = auth.LockStateUnlocked
	s.mu.Unlock()

	return true, nil
}

// Lock transitions the service from unlocked to locked.
func (s *Service) Lock(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cancel background workers.
	if s.cancelWorkers != nil {
		s.cancelWorkers()
		s.cancelWorkers = nil
	}

	// Clear in-memory state.
	s.items = nil
	s.folders = nil
	s.index = nil
	s.state = auth.LockStateLocked

	s.emit(Relocked, "vault relocked")

	// Notify remote if available.
	if s.deps.Remote != nil {
		if err := s.deps.Remote.Lock(ctx); err != nil {
			return fmt.Errorf("app: remote lock failed: %w", err)
		}
	}

	return nil
}

// Search searches vault items by query. Returns ErrLocked if not unlocked.
func (s *Service) Search(ctx context.Context, query string, limit int) ([]vault.ScoredItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != auth.LockStateUnlocked {
		return nil, cerrors.ErrLocked
	}

	if s.index == nil {
		return nil, nil
	}

	return s.index.Search(query, limit), nil
}

// Items returns a copy of all vault items. Returns ErrLocked if not unlocked.
func (s *Service) Items(ctx context.Context) ([]vault.Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != auth.LockStateUnlocked {
		return nil, cerrors.ErrLocked
	}

	items := make([]vault.Item, len(s.items))
	copy(items, s.items)
	return items, nil
}

// Get returns a single vault item by ID.
func (s *Service) Get(ctx context.Context, id string) (vault.Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != auth.LockStateUnlocked {
		return vault.Item{}, cerrors.ErrLocked
	}

	for _, item := range s.items {
		if item.ID == id {
			return item, nil
		}
	}

	return vault.Item{}, cerrors.ErrNotFound
}

// Config returns the current configuration.
func (s *Service) Config() *config.Config {
	return s.cfg
}

// Events returns a read-only channel of domain events.
func (s *Service) Events() <-chan Event {
	return s.events
}

// Shutdown gracefully shuts down the service.
func (s *Service) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if s.cancelWorkers != nil {
		s.cancelWorkers()
		s.cancelWorkers = nil
	}
	close(s.events)
	s.mu.Unlock()
	return nil
}

// ---------------------------------------------------------------------------
// Helper methods
// ---------------------------------------------------------------------------

// ensureUnlocked returns ErrLocked if the service is not in the unlocked state.
func (s *Service) ensureUnlocked() error {
	if s.state != auth.LockStateUnlocked {
		return cerrors.ErrLocked
	}
	return nil
}

// now returns the current time, using deps.Clock if available.
func (s *Service) now() time.Time {
	if s.deps.Clock != nil {
		return s.deps.Clock.Now()
	}
	return time.Now()
}

// rebuildIndexLocked rebuilds the search index from the current items slice.
// The caller must hold s.mu.
func (s *Service) rebuildIndexLocked() {
	s.index = vault.BuildIndex(s.items)
}

// appendOutboxLocked appends a mutation to the outbox and returns it.
// The caller must hold s.mu.
func (s *Service) appendOutboxLocked(kind coresync.MutationKind, itemID string, payload []byte) coresync.OutboxMutation {
	m := coresync.OutboxMutation{
		ID:        fmt.Sprintf("m-%d", s.now().UnixNano()),
		Kind:      kind,
		ItemID:    itemID,
		CreatedAt: s.now(),
		Payload:   payload,
	}
	s.outbox = append(s.outbox, m)
	return m
}

// saveCacheAsync can trigger an async cache save if cache + secretbox are
// available. No-op for now to avoid blocking the UI.
func (s *Service) saveCacheAsync() {
	// No-op in v0.1.0.
}

// ---------------------------------------------------------------------------
// Mutation methods
// ---------------------------------------------------------------------------

// Create creates a new vault item. If remote is available, it tries to create
// online first. On failure or offline, it queues a pending mutation.
func (s *Service) Create(ctx context.Context, item vault.Item) (vault.Item, error) {
	s.mu.Lock()
	if err := s.ensureUnlocked(); err != nil {
		s.mu.Unlock()
		return vault.Item{}, err
	}
	s.mu.Unlock()

	// Try remote if available.
	if s.deps.Remote != nil {
		remoteItem, err := s.deps.Remote.Create(ctx, item)
		if err == nil {
			s.mu.Lock()
			remoteItem.SyncStatus = vault.SyncStatusSynced
			s.items = append(s.items, remoteItem)
			s.rebuildIndexLocked()
			s.mu.Unlock()
			s.emit(SyncUpdated, "item created remotely")
			return remoteItem, nil
		}
	}

	// Remote missing or error: queue pending locally.
	s.mu.Lock()
	defer s.mu.Unlock()

	if item.ID == "" {
		item.ID = fmt.Sprintf("local-%d", s.now().UnixNano())
	}
	item.SyncStatus = vault.SyncStatusPending
	item.RevisionDate = s.now()

	payload, _ := json.Marshal(item)
	s.appendOutboxLocked(coresync.MutationCreate, item.ID, payload)

	s.items = append(s.items, item)
	s.rebuildIndexLocked()
	s.emit(MutationPending, "item queued for creation")
	return item, nil
}

// Update updates an existing vault item. Tries remote first, falls back to
// local pending mutation.
func (s *Service) Update(ctx context.Context, id string, item vault.Item) (vault.Item, error) {
	s.mu.Lock()
	if err := s.ensureUnlocked(); err != nil {
		s.mu.Unlock()
		return vault.Item{}, err
	}
	s.mu.Unlock()

	if s.deps.Remote != nil {
		remoteItem, err := s.deps.Remote.Update(ctx, id, item)
		if err == nil {
			s.mu.Lock()
			remoteItem.SyncStatus = vault.SyncStatusSynced
			for i, existing := range s.items {
				if existing.ID == id {
					s.items[i] = remoteItem
					break
				}
			}
			s.rebuildIndexLocked()
			s.mu.Unlock()
			s.emit(SyncUpdated, "item updated remotely")
			return remoteItem, nil
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	item.ID = id
	item.SyncStatus = vault.SyncStatusPending
	item.RevisionDate = s.now()

	payload, _ := json.Marshal(item)
	s.appendOutboxLocked(coresync.MutationUpdate, id, payload)

	found := false
	for i, existing := range s.items {
		if existing.ID == id {
			s.items[i] = item
			found = true
			break
		}
	}
	if !found {
		s.items = append(s.items, item)
	}
	s.rebuildIndexLocked()
	s.emit(MutationPending, "item queued for update")
	return item, nil
}

// Trash moves an item to the trash. Tries remote first, falls back to local pending.
func (s *Service) Trash(ctx context.Context, id string) error {
	s.mu.Lock()
	if err := s.ensureUnlocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()

	if s.deps.Remote != nil {
		err := s.deps.Remote.Trash(ctx, id)
		if err == nil {
			s.mu.Lock()
			for i, existing := range s.items {
				if existing.ID == id {
					s.items[i].Deleted = true
					s.items[i].SyncStatus = vault.SyncStatusSynced
					break
				}
			}
			s.rebuildIndexLocked()
			s.mu.Unlock()
			s.emit(SyncUpdated, "item trashed remotely")
			return nil
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	payload, _ := json.Marshal(map[string]string{"id": id})
	s.appendOutboxLocked(coresync.MutationTrash, id, payload)

	for i, existing := range s.items {
		if existing.ID == id {
			s.items[i].Deleted = true
			s.items[i].SyncStatus = vault.SyncStatusPending
			break
		}
	}

	s.rebuildIndexLocked()
	s.emit(MutationPending, "item queued for trash")
	return nil
}

// Restore restores an item from the trash. Tries remote first, falls back to local pending.
func (s *Service) Restore(ctx context.Context, id string) (vault.Item, error) {
	s.mu.Lock()
	if err := s.ensureUnlocked(); err != nil {
		s.mu.Unlock()
		return vault.Item{}, err
	}
	s.mu.Unlock()

	if s.deps.Remote != nil {
		remoteItem, err := s.deps.Remote.Restore(ctx, id)
		if err == nil {
			s.mu.Lock()
			remoteItem.Deleted = false
			remoteItem.SyncStatus = vault.SyncStatusSynced
			for i, existing := range s.items {
				if existing.ID == id {
					s.items[i] = remoteItem
					break
				}
			}
			s.rebuildIndexLocked()
			s.mu.Unlock()
			s.emit(SyncUpdated, "item restored remotely")
			return remoteItem, nil
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	payload, _ := json.Marshal(map[string]string{"id": id})
	s.appendOutboxLocked(coresync.MutationRestore, id, payload)

	var restored vault.Item
	for i, existing := range s.items {
		if existing.ID == id {
			s.items[i].Deleted = false
			s.items[i].SyncStatus = vault.SyncStatusPending
			restored = s.items[i]
			break
		}
	}

	s.rebuildIndexLocked()
	s.emit(MutationPending, "item queued for restore")
	return restored, nil
}

// Delete permanently deletes a vault item. Tries remote first, falls back to local pending.
func (s *Service) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	if err := s.ensureUnlocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()

	if s.deps.Remote != nil {
		err := s.deps.Remote.Delete(ctx, id)
		if err == nil {
			s.mu.Lock()
			for i, existing := range s.items {
				if existing.ID == id {
					s.items = append(s.items[:i], s.items[i+1:]...)
					break
				}
			}
			s.rebuildIndexLocked()
			s.mu.Unlock()
			s.emit(SyncUpdated, "item deleted remotely")
			return nil
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	payload, _ := json.Marshal(map[string]string{"id": id})
	s.appendOutboxLocked(coresync.MutationDelete, id, payload)

	for i, existing := range s.items {
		if existing.ID == id {
			s.items = append(s.items[:i], s.items[i+1:]...)
			break
		}
	}

	s.rebuildIndexLocked()
	s.emit(MutationPending, "item queued for deletion")
	return nil
}

// ListAttachments is not yet supported.
func (s *Service) ListAttachments(ctx context.Context, itemID string) ([]vault.Attachment, error) {
	return nil, cerrors.ErrUnsupported
}

// DownloadAttachment is not yet supported.
func (s *Service) DownloadAttachment(ctx context.Context, itemID, attachmentID string, dst io.Writer) error {
	return cerrors.ErrUnsupported
}

// UploadAttachment is not yet supported.
func (s *Service) UploadAttachment(ctx context.Context, itemID, fileName string, size int64, src io.Reader) (vault.Attachment, error) {
	return vault.Attachment{}, cerrors.ErrUnsupported
}

// DeleteAttachment is not yet supported.
func (s *Service) DeleteAttachment(ctx context.Context, itemID, attachmentID string) error {
	return cerrors.ErrUnsupported
}

// ResolveConflict resolves a sync conflict by applying the given resolution.
func (s *Service) ResolveConflict(ctx context.Context, conflictID string, resolution coresync.ConflictResolution) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureUnlocked(); err != nil {
		return err
	}

	// Find and remove the conflict.
	idx := -1
	for i, c := range s.conflicts {
		if c.ID == conflictID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return cerrors.ErrNotFound
	}
	conflict := s.conflicts[idx]
	s.conflicts = append(s.conflicts[:idx], s.conflicts[idx+1:]...)

	switch resolution {
	case coresync.ResolutionKeepRemote:
		// No local change; remote data wins (installed during sync).
	case coresync.ResolutionKeepLocal:
		// Mark the local item as pending so it will be replayed.
		for i, item := range s.items {
			if item.ID == conflict.ItemID {
				s.items[i].SyncStatus = vault.SyncStatusPending
				s.items[i].ConflictID = ""
				break
			}
		}
	case coresync.ResolutionDuplicateLocal:
		// Clone the conflicting item with a new local ID.
		for _, item := range s.items {
			if item.ID == conflict.ItemID {
				dup := item
				dup.ID = fmt.Sprintf("local-%d", s.now().UnixNano())
				dup.SyncStatus = vault.SyncStatusPending
				dup.ConflictID = ""
				s.items = append(s.items, dup)
				break
			}
		}
	}

	s.rebuildIndexLocked()
	s.emit(SyncUpdated, "conflict resolved")
	return nil
}

// ---------------------------------------------------------------------------
// Sync
// ---------------------------------------------------------------------------

// syncOnce performs a single sync cycle: checks remote revision, pushes local
// mutations, pulls remote changes, and detects conflicts.
func (s *Service) syncOnce(ctx context.Context) {
	s.emit(SyncChecking, "checking remote revision")

	if s.deps.Remote == nil {
		return
	}

	rev, err := s.deps.Remote.Revision(ctx)
	if err != nil {
		s.emit(SyncFailed, fmt.Sprintf("revision check failed: %v", err))
		return
	}

	// Snapshot the outbox under lock.
	s.mu.Lock()
	outboxSnapshot := make([]coresync.OutboxMutation, len(s.outbox))
	copy(outboxSnapshot, s.outbox)
	s.mu.Unlock()

	// If nothing to sync, return early.
	if len(outboxSnapshot) == 0 && rev == "" {
		s.emit(SyncUpdated, "already up to date")
		return
	}

	// Fetch remote changes.
	remoteItems, remoteFolders, remoteRev, err := s.deps.Remote.Sync(ctx)
	if err != nil {
		s.emit(SyncFailed, fmt.Sprintf("remote sync failed: %v", err))
		return
	}

	// Build remote change list for conflict detection.
	remoteChanges := make([]coresync.RemoteChange, 0, len(remoteItems))
	for _, ritem := range remoteItems {
		rc := coresync.RemoteChange{
			ItemID:   ritem.ID,
			Revision: ritem.RevisionDate.Format(time.RFC3339),
			Deleted:  ritem.Deleted,
		}
		remoteChanges = append(remoteChanges, rc)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check context cancellation before installing state.
	if ctx.Err() != nil {
		return
	}

	// Detect conflicts.
	conflicts := coresync.DetectConflicts(outboxSnapshot, remoteChanges)
	if len(conflicts) > 0 {
		s.conflicts = append(s.conflicts, conflicts...)
		for _, c := range conflicts {
			for i, item := range s.items {
				if item.ID == c.ItemID {
					s.items[i].SyncStatus = vault.SyncStatusConflict
					s.items[i].ConflictID = c.ID
					break
				}
			}
		}
		s.rebuildIndexLocked()
		s.emit(ConflictDetected, fmt.Sprintf("%d conflict(s) detected", len(conflicts)))
		return
	}

	// No conflicts: install remote state.
	s.items = remoteItems
	s.folders = remoteFolders
	for i := range s.items {
		s.items[i].SyncStatus = vault.SyncStatusSynced
	}
	s.outbox = nil
	s.rebuildIndexLocked()
	s.emit(SyncUpdated, fmt.Sprintf("sync complete (rev: %s)", remoteRev))
}

// startMinimalSyncWorker starts a background goroutine that runs syncOnce.
func (s *Service) startMinimalSyncWorker(ctx context.Context) {
	go func() {
		s.syncOnce(ctx)
	}()
}
