package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"sync/atomic"

	"github.com/bnema/gtk4-layershell-bitwarden/internal/core/auth"
	"github.com/bnema/gtk4-layershell-bitwarden/internal/core/cache"
	"github.com/bnema/gtk4-layershell-bitwarden/internal/core/config"
	cerrors "github.com/bnema/gtk4-layershell-bitwarden/internal/core/errors"
	"github.com/bnema/gtk4-layershell-bitwarden/internal/core/sync"
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
// Stub methods — replaced with real implementations in the next sub-task.
// ---------------------------------------------------------------------------

// Create stubs item creation.
func (s *Service) Create(ctx context.Context, item vault.Item) (vault.Item, error) {
	return vault.Item{}, cerrors.ErrUnsupported
}

// Update stubs item update.
func (s *Service) Update(ctx context.Context, id string, item vault.Item) (vault.Item, error) {
	return vault.Item{}, cerrors.ErrUnsupported
}

// Trash stubs item trash.
func (s *Service) Trash(ctx context.Context, id string) error {
	return cerrors.ErrUnsupported
}

// Restore stubs item restore.
func (s *Service) Restore(ctx context.Context, id string) (vault.Item, error) {
	return vault.Item{}, cerrors.ErrUnsupported
}

// Delete stubs item deletion.
func (s *Service) Delete(ctx context.Context, id string) error {
	return cerrors.ErrUnsupported
}

// ListAttachments stubs listing attachments.
func (s *Service) ListAttachments(ctx context.Context, itemID string) ([]vault.Attachment, error) {
	return nil, cerrors.ErrUnsupported
}

// DownloadAttachment stubs attachment download.
func (s *Service) DownloadAttachment(ctx context.Context, itemID, attachmentID string, dst io.Writer) error {
	return cerrors.ErrUnsupported
}

// UploadAttachment stubs attachment upload.
func (s *Service) UploadAttachment(ctx context.Context, itemID, fileName string, size int64, src io.Reader) (vault.Attachment, error) {
	return vault.Attachment{}, cerrors.ErrUnsupported
}

// DeleteAttachment stubs attachment deletion.
func (s *Service) DeleteAttachment(ctx context.Context, itemID, attachmentID string) error {
	return cerrors.ErrUnsupported
}

// ResolveConflict stubs conflict resolution.
func (s *Service) ResolveConflict(ctx context.Context, conflictID string, resolution sync.ConflictResolution) error {
	return cerrors.ErrUnsupported
}

// startMinimalSyncWorker starts a minimal background worker that checks the
// remote revision. Full sync logic comes in a later sub-task.
func (s *Service) startMinimalSyncWorker(ctx context.Context) {
	go func() {
		s.emit(SyncChecking, "checking remote revision")

		if s.deps.Remote == nil {
			return
		}

		revCh := make(chan string, 1)
		var syncStarted atomic.Bool

		go func() {
			rev, err := s.deps.Remote.Revision(ctx)
			if err != nil {
				s.emit(SyncFailed, fmt.Sprintf("revision check failed: %v", err))
				return
			}
			syncStarted.Store(true)
			revCh <- rev
		}()

		select {
		case <-ctx.Done():
			return
		case rev := <-revCh:
			s.emit(SyncUpdated, fmt.Sprintf("remote revision: %s", rev))
			_ = syncStarted.Load() // used to signal caller in tests
		}
	}()
}
