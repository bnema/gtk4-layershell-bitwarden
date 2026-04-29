package file

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"

	"github.com/bnema/gtk4-layershell-bitwarden/internal/adapters/fileutil"
	"github.com/bnema/gtk4-layershell-bitwarden/internal/core/cache"
)

// Store persists encrypted cache snapshots to a JSON file on disk.
type Store struct {
	path string
}

// NewStore creates a Store that reads/writes the file at path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Load reads and returns the Snapshot from the JSON file.
// Returns zero Snapshot and os.ErrNotExist if the file does not exist.
func (s *Store) Load(ctx context.Context) (cache.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return cache.Snapshot{}, err
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cache.Snapshot{}, os.ErrNotExist
		}
		return cache.Snapshot{}, err
	}

	if err := ctx.Err(); err != nil {
		return cache.Snapshot{}, err
	}

	var snap cache.Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return cache.Snapshot{}, err
	}
	return snap, nil
}

// Save marshals the snapshot to JSON and writes it atomically to the file.
// Creates parent directories with mode 0700 if needed. Final file is mode 0600.
func (s *Store) Save(ctx context.Context, snapshot cache.Snapshot) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}

	return fileutil.AtomicWriteFile(ctx, s.path, data, 0600)
}

// Clear removes the store file. No error if the file does not exist.
func (s *Store) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	return fileutil.RemoveIfExists(s.path)
}

// Path returns the file path this store uses.
func (s *Store) Path() string {
	return s.path
}
