//go:build windows

package updater

import (
	"fmt"
	"os"
)

// AtomicReplaceExecutable is unsupported on Windows because the running executable is typically locked.
func AtomicReplaceExecutable(path string, data []byte) error {
	return fmt.Errorf("in-place executable replacement is not supported on Windows")
}

// WritePendingUpdate writes newData to exePath+".new" next to the current executable path.
func WritePendingUpdate(exePath string, newData []byte) error {
	return os.WriteFile(exePath+".new", newData, 0o644)
}
