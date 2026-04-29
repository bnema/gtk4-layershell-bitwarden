package file

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

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

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}

	// Write atomically: unique temp file in same directory, fsync, rename.
	// Unique names avoid overlapping async Save calls clobbering one shared .tmp.
	f, err := os.CreateTemp(dir, "."+filepath.Base(s.path)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpFile := f.Name()
	if err := f.Chmod(0600); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpFile)
		return err
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			_ = f.Close()
		}
	}()

	if _, err := f.Write(data); err != nil {
		_ = os.Remove(tmpFile)
		return err
	}

	if err := f.Sync(); err != nil {
		_ = os.Remove(tmpFile)
		return err
	}

	if err := ctx.Err(); err != nil {
		_ = os.Remove(tmpFile)
		return err
	}

	closeOnError = false
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpFile)
		return err
	}

	return os.Rename(tmpFile, s.path)
}

// Clear removes the store file. No error if the file does not exist.
func (s *Store) Clear(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	err := os.Remove(s.path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Path returns the file path this store uses.
func (s *Store) Path() string {
	return s.path
}
