package fileutil

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAtomicWriteFileCreatesPrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "cache.json")

	require.NoError(t, AtomicWriteFile(context.Background(), path, []byte("secret"), 0600))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, []byte("secret"), data)

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestAtomicWriteFileHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	path := filepath.Join(t.TempDir(), "cache.json")
	err := AtomicWriteFile(ctx, path, []byte("secret"), 0600)
	require.ErrorIs(t, err, context.Canceled)
	_, statErr := os.Stat(path)
	require.True(t, errors.Is(statErr, os.ErrNotExist))
}

func TestRemoveIfExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing")
	require.NoError(t, RemoveIfExists(path))

	require.NoError(t, os.WriteFile(path, []byte("x"), 0600))
	require.NoError(t, RemoveIfExists(path))
	_, err := os.Stat(path)
	require.True(t, errors.Is(err, os.ErrNotExist))
}
