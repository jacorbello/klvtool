package output

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jacorbello/klvtool/internal/model"
)

// PayloadResult captures the deterministic output path and content hash for a payload write.
type PayloadResult struct {
	Path string
	Size int64
	Hash string
}

// WritePayload writes payload bytes into dir using a deterministic filename based on recordID.
func WritePayload(dir, recordID string, payload []byte) (PayloadResult, error) {
	var result PayloadResult

	if strings.TrimSpace(dir) == "" {
		return result, model.OutputWrite(errors.New("payload directory is required"))
	}

	filename, err := payloadFilename(recordID)
	if err != nil {
		return result, model.OutputWrite(fmt.Errorf("derive payload filename: %w", err))
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return result, model.OutputWrite(fmt.Errorf("create payload directory %q: %w", dir, err))
	}

	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		return result, model.OutputWrite(fmt.Errorf("write payload %q: %w", path, err))
	}

	sum := sha256.Sum256(payload)
	result.Path = path
	result.Size = int64(len(payload))
	result.Hash = "sha256:" + hex.EncodeToString(sum[:])
	return result, nil
}

func payloadFilename(recordID string) (string, error) {
	var b strings.Builder
	for _, r := range recordID {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}

	name := strings.Trim(b.String(), "._-")
	if name == "" {
		return "", errors.New("record id is required")
	}
	return name + ".bin", nil
}
