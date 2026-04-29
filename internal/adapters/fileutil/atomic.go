// Package fileutil contains small filesystem helpers shared by adapters.
package fileutil

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// AtomicWriteFile writes data to path using a uniquely named temporary file in
// the target directory, then renames it into place. The parent directory is
// created with mode 0700 and the final file is chmodded to fileMode.
func AtomicWriteFile(ctx context.Context, path string, data []byte, fileMode fs.FileMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := f.Name()
	if err := f.Chmod(fileMode); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	closeOnError := true
	defer func() {
		if closeOnError {
			_ = f.Close()
		}
	}()

	if _, err := f.Write(data); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	if err := f.Sync(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	if err := ctx.Err(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	closeOnError = false
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

// RemoveIfExists removes path, ignoring not-exist errors.
func RemoveIfExists(path string) error {
	err := os.Remove(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
