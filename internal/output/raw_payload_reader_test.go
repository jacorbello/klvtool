package output

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
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
	sum := sha256.Sum256(wantPayload)
	wantHash := "sha256:" + hex.EncodeToString(sum[:])

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
				PayloadHash: wantHash,
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

func TestReadRawPayloadManifestRejectsEscapingPayloadPaths(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "payloads"), 0o755); err != nil {
		t.Fatalf("mkdir payloads: %v", err)
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
				PayloadPath: "../escape.bin",
				PayloadSize: 2,
				PayloadHash: "sha256:test",
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

	_, err = ReadRawPayloadManifest(root)
	if err == nil {
		t.Fatal("expected escaping payload path to fail")
	}
	if !strings.Contains(err.Error(), "escapes checkpoint root") {
		t.Fatalf("expected escape validation error, got %v", err)
	}

	manifest.Records[0].PayloadPath = "/tmp/escape.bin"
	manifestFile, err = os.Create(filepath.Join(root, "manifest.ndjson"))
	if err != nil {
		t.Fatalf("recreate manifest: %v", err)
	}
	if err := NewManifestWriter(manifestFile).WriteManifest(manifest); err != nil {
		_ = manifestFile.Close()
		t.Fatalf("rewrite manifest: %v", err)
	}
	if err := manifestFile.Close(); err != nil {
		t.Fatalf("close manifest: %v", err)
	}

	_, err = ReadRawPayloadManifest(root)
	if err == nil {
		t.Fatal("expected absolute payload path to fail")
	}
	if !strings.Contains(err.Error(), "escapes checkpoint root") {
		t.Fatalf("expected escape validation error for absolute path, got %v", err)
	}
}

func TestReadRawPayloadManifestRejectsSymlinkPayloads(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "payloads"), 0o755); err != nil {
		t.Fatalf("mkdir payloads: %v", err)
	}

	outsideDir := t.TempDir()
	outsidePayload := filepath.Join(outsideDir, "escape.bin")
	wantPayload := []byte{0x09, 0x08, 0x07}
	if err := os.WriteFile(outsidePayload, wantPayload, 0o644); err != nil {
		t.Fatalf("write outside payload: %v", err)
	}
	if err := os.Symlink(outsidePayload, filepath.Join(root, "payloads", "klv-001.bin")); err != nil {
		t.Fatalf("create symlink payload: %v", err)
	}

	sum := sha256.Sum256(wantPayload)
	wantHash := "sha256:" + hex.EncodeToString(sum[:])
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
				PayloadSize: int64(len(wantPayload)),
				PayloadHash: wantHash,
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

	_, err = ReadRawPayloadManifest(root)
	if err == nil {
		t.Fatal("expected symlink payload to fail")
	}
	if !strings.Contains(err.Error(), "escapes checkpoint root") {
		t.Fatalf("expected escape validation error, got %v", err)
	}
}

func TestReadRawPayloadManifestRejectsSymlinkedParentDirectories(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outsideDir, "payloads"), 0o755); err != nil {
		t.Fatalf("mkdir outside payloads: %v", err)
	}

	wantPayload := []byte{0x0a, 0x0b}
	if err := os.WriteFile(filepath.Join(outsideDir, "payloads", "klv-001.bin"), wantPayload, 0o644); err != nil {
		t.Fatalf("write outside payload: %v", err)
	}
	if err := os.Symlink(filepath.Join(outsideDir, "payloads"), filepath.Join(root, "payloads")); err != nil {
		t.Fatalf("create symlinked payloads dir: %v", err)
	}

	sum := sha256.Sum256(wantPayload)
	wantHash := "sha256:" + hex.EncodeToString(sum[:])
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
				PayloadSize: int64(len(wantPayload)),
				PayloadHash: wantHash,
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

	_, err = ReadRawPayloadManifest(root)
	if err == nil {
		t.Fatal("expected symlinked parent directory to fail")
	}
	if !strings.Contains(err.Error(), "escapes checkpoint root") {
		t.Fatalf("expected escape validation error, got %v", err)
	}
}

func TestReadRawPayloadManifestValidatesPayloadMetadata(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "payloads"), 0o755); err != nil {
		t.Fatalf("mkdir payloads: %v", err)
	}

	wantPayload := []byte{0x01, 0x02, 0x03}
	if err := os.WriteFile(filepath.Join(root, "payloads", "klv-001.bin"), wantPayload, 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	sum := sha256.Sum256(wantPayload)
	wantHash := "sha256:" + hex.EncodeToString(sum[:])

	t.Run("size mismatch", func(t *testing.T) {
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
					PayloadSize: 4,
					PayloadHash: wantHash,
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

		_, err = ReadRawPayloadManifest(root)
		if err == nil {
			t.Fatal("expected size mismatch to fail")
		}
		if !strings.Contains(err.Error(), "payload size mismatch") {
			t.Fatalf("expected size mismatch error, got %v", err)
		}
	})

	t.Run("hash mismatch", func(t *testing.T) {
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
					PayloadSize: int64(len(wantPayload)),
					PayloadHash: "sha256:" + strings.Repeat("0", 64),
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

		_, err = ReadRawPayloadManifest(root)
		if err == nil {
			t.Fatal("expected hash mismatch to fail")
		}
		if !strings.Contains(err.Error(), "payload hash mismatch") {
			t.Fatalf("expected hash mismatch error, got %v", err)
		}
	})
}
