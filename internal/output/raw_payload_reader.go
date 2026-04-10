package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
		payloadPath := filepath.Join(root, filepath.FromSlash(rec.PayloadPath))
		payload, err := os.ReadFile(payloadPath)
		if err != nil {
			return nil, model.OutputWrite(fmt.Errorf("read payload %q: %w", payloadPath, err))
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
