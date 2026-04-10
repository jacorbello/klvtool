package output

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jacorbello/klvtool/internal/model"
)

func TestReadRawPayloadManifestLoadsPayloadBytes(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "payloads"), 0o755); err != nil {
		t.Fatalf("mkdir payloads: %v", err)
	}

	wantPayload := []byte{0x01, 0x02}
	if err := os.WriteFile(filepath.Join(root, "payloads", "klv-001.bin"), wantPayload, 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	manifest := model.Manifest{
		SchemaVersion:   "1",
		SourceInputPath: "input.ts",
		BackendName:     "ffmpeg",
		BackendVersion:  "7.1",
		Records: []model.Record{
			{
				RecordID:    "klv-001",
				PID:         256,
				PayloadPath: "payloads/klv-001.bin",
				PayloadSize: 2,
				PayloadHash: "sha256:test",
				Warnings:    []string{"from extract"},
			},
		},
	}

	manifestBytes, err := manifest.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.ndjson"), append(manifestBytes, '\n'), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	records, err := ReadRawPayloadManifest(root)
	if err != nil {
		t.Fatalf("ReadRawPayloadManifest error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	got := records[0]
	if got.RecordID != "klv-001" {
		t.Fatalf("unexpected record id %q", got.RecordID)
	}
	if got.PID != 256 {
		t.Fatalf("unexpected pid %d", got.PID)
	}
	if got.Payload == nil || string(got.Payload) != string(wantPayload) {
		t.Fatalf("unexpected payload bytes %v", got.Payload)
	}
	if len(got.Warnings) != 1 || got.Warnings[0] != "from extract" {
		t.Fatalf("unexpected warnings %v", got.Warnings)
	}
}
