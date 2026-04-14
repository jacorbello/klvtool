package updater

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// VerifyChecksumInFile checks that checksumsText (checksums.txt body) contains an SHA256
// line for archiveFileName that matches the hash of data.
func VerifyChecksumInFile(checksumsText, archiveFileName string, data []byte) error {
	want := hex.EncodeToString(sha256sum(data))
	lines := bytes.Split([]byte(checksumsText), []byte{'\n'})
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		fields := bytes.Fields(line)
		if len(fields) < 2 {
			continue
		}
		hash := string(fields[0])
		name := string(fields[len(fields)-1])
		if name != archiveFileName {
			continue
		}
		if !strings.EqualFold(hash, want) {
			return fmt.Errorf("checksum mismatch for %q", archiveFileName)
		}
		return nil
	}
	return fmt.Errorf("no checksum entry for %q", archiveFileName)
}

func sha256sum(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}
