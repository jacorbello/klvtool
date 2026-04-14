package updater

import (
	"strings"
	"testing"
)

func TestVerifyChecksumInFile(t *testing.T) {
	const sums = `a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3  klvtool_linux_amd64.tar.gz
deadbeef  other.zip`
	data := []byte("123")
	// SHA256 of "123"
	if err := VerifyChecksumInFile(sums, "klvtool_linux_amd64.tar.gz", data); err != nil {
		t.Fatal(err)
	}

	err := VerifyChecksumInFile(sums, "klvtool_linux_amd64.tar.gz", []byte("nope"))
	if err == nil {
		t.Fatal("expected checksum mismatch")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("got %v", err)
	}

	err = VerifyChecksumInFile(sums, "missing.tar.gz", data)
	if err == nil {
		t.Fatal("expected missing entry")
	}
}
