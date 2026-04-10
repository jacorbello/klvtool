package output

import (
	"bytes"
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

	manifestFile, err := os.Create(filepath.Join(root, "manifest.ndjson"))
	if err != nil {
		t.Fatalf("create manifest: %v", err)
	}
	if err := NewManifestWriter(manifestFile).WriteManifest(manifest); err != nil {
		_ = manifestFile.Close()
		t.Fatalf("write manifest: %v", err)
	}
	if err := manifestFile.Close(); err != nil {
		t.Fatalf("close manifest: %v", err)
	}
	if contents, err := os.ReadFile(filepath.Join(root, "manifest.ndjson")); err != nil {
		t.Fatalf("read manifest: %v", err)
	} else if !bytes.Contains(contents, []byte(`"payloads/klv-001.bin"`)) {
		t.Fatalf("expected writer output to include payload path, got %s", contents)
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
