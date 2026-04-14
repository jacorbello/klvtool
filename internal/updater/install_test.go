//go:build !windows

package updater

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicReplaceExecutable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "klvtool")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := AtomicReplaceExecutable(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "new" {
		t.Fatalf("got %q", b)
	}
}
