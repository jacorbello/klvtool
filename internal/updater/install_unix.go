//go:build !windows

package updater

import (
	"fmt"
	"os"
	"path/filepath"
)

// WritePendingUpdate is a compile-time stub for non-Windows builds.
// It always returns an error; call sites must guard with a GOOS check.
func WritePendingUpdate(_ string, _ []byte) error {
	return fmt.Errorf("WritePendingUpdate called on non-Windows build")
}

// AtomicReplaceExecutable replaces path with data using a rename dance in the same directory.
func AtomicReplaceExecutable(path string, data []byte) error {
	if eval, err := filepath.EvalSymlinks(path); err == nil {
		path = eval
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".klvtool-new-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	// Any pre-existing backup from a prior attempt is removed so we do not
	// accumulate stale backups across repeated updates.
	backup := path + ".bak"
	_ = os.Remove(backup)
	if err := os.Rename(path, backup); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("backup existing binary: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Rename(backup, path)
		_ = os.Remove(tmpPath)
		return fmt.Errorf("install new binary: %w", err)
	}
	// Best-effort cleanup; if Remove fails the .bak is harmless and may be deleted manually.
	_ = os.Remove(backup)
	return nil
}
