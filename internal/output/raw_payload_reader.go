package output

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jacorbello/klvtool/internal/extract"
	"github.com/jacorbello/klvtool/internal/model"
)

// ReadRawPayloadManifest loads a raw extraction checkpoint and replays the payload bytes.
func ReadRawPayloadManifest(root string) ([]extract.RawPayloadRecord, error) {
	data, err := os.ReadFile(filepath.Join(root, "manifest.ndjson"))
	if err != nil {
		return nil, model.OutputWrite(fmt.Errorf("read raw manifest: %w", err))
	}

	var manifest model.Manifest
	if err := json.Unmarshal(bytes.TrimSpace(data), &manifest); err != nil {
		return nil, model.OutputWrite(fmt.Errorf("decode raw manifest: %w", err))
	}

	records := make([]extract.RawPayloadRecord, 0, len(manifest.Records))
	for _, rec := range manifest.Records {
		payloadPath, err := resolveCheckpointPath(root, rec.PayloadPath)
		if err != nil {
			return nil, model.OutputWrite(fmt.Errorf("resolve payload path %q: %w", rec.PayloadPath, err))
		}
		payload, err := os.ReadFile(payloadPath)
		if err != nil {
			return nil, model.OutputWrite(fmt.Errorf("read payload %q: %w", payloadPath, err))
		}

		if got, want := int64(len(payload)), rec.PayloadSize; got != want {
			return nil, model.OutputWrite(fmt.Errorf("payload size mismatch for %q: got %d, want %d", rec.RecordID, got, want))
		}

		sum := sha256.Sum256(payload)
		gotHash := "sha256:" + hex.EncodeToString(sum[:])
		if gotHash != rec.PayloadHash {
			return nil, model.OutputWrite(fmt.Errorf("payload hash mismatch for %q: got %s, want %s", rec.RecordID, gotHash, rec.PayloadHash))
		}

		records = append(records, extract.RawPayloadRecord{
			RecordID:          rec.RecordID,
			PID:               rec.PID,
			TransportStreamID: rec.TransportStreamID,
			PacketOffset:      rec.PacketOffset,
			PacketIndex:       rec.PacketIndex,
			ContinuityCounter: rec.ContinuityCounter,
			PTS:               rec.PTS,
			DTS:               rec.DTS,
			Payload:           payload,
			Warnings:          append([]string(nil), rec.Warnings...),
		})
	}

	return records, nil
}

func resolveCheckpointPath(root, payloadPath string) (string, error) {
	if filepath.IsAbs(payloadPath) {
		return "", fmt.Errorf("path escapes checkpoint root")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", err
	}

	candidate := filepath.Clean(filepath.Join(absRoot, filepath.FromSlash(payloadPath)))
	rel, err := filepath.Rel(absRoot, candidate)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes checkpoint root")
	}

	realCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	rel, err = filepath.Rel(realRoot, realCandidate)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes checkpoint root")
	}
	return realCandidate, nil
}
