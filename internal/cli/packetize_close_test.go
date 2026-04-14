package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacorbello/klvtool/internal/model"
	"github.com/jacorbello/klvtool/internal/output"
)

func TestPacketizeManifestCloseErrorSurfaced(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	setupPacketizeInput(t, inputDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &stderr
	cmd.Packetize.OpenManifest = func(path string) (io.WriteCloser, error) {
		return &errCloser{closeErr: errors.New("disk full on close")}, nil
	}

	code := cmd.Execute([]string{"packetize", "--input", inputDir, "--out", outputDir, "--mode", "best-effort"})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "disk full on close") {
		t.Errorf("expected close error on stderr; got: %s", stderr.String())
	}
}

func TestPacketizeManifestCloseSuccessUnaffected(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

	setupPacketizeInput(t, inputDir)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &stderr

	code := cmd.Execute([]string{"packetize", "--input", inputDir, "--out", outputDir, "--mode", "best-effort"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr; got: %s", stderr.String())
	}
}

// setupPacketizeInput creates the raw payload checkpoint directory structure
// needed by the packetize command.
func setupPacketizeInput(t *testing.T, inputDir string) {
	t.Helper()

	payloadDir := filepath.Join(inputDir, "payloads")
	if err := os.MkdirAll(payloadDir, 0o755); err != nil {
		t.Fatalf("mkdir payloads: %v", err)
	}

	payload := append([]byte{0x06, 0x0e, 0x2b, 0x34}, bytes.Repeat([]byte{0x00}, 12)...)
	payload = append(payload, 0x03, 0xaa, 0xbb, 0xcc)
	payloadResult, err := output.WritePayload(payloadDir, "klv-001", payload)
	if err != nil {
		t.Fatalf("write payload: %v", err)
	}

	sum := sha256.Sum256(payload)
	manifest := model.Manifest{
		SchemaVersion:   "1",
		SourceInputPath: "sample.ts",
		BackendName:     "ffmpeg",
		BackendVersion:  "7.1",
		Records: []model.Record{
			{
				RecordID:    "klv-001",
				PID:         256,
				PayloadPath: filepath.ToSlash(filepath.Join("payloads", filepath.Base(payloadResult.Path))),
				PayloadSize: payloadResult.Size,
				PayloadHash: "sha256:" + hex.EncodeToString(sum[:]),
			},
		},
	}

	manifestFile, err := os.Create(filepath.Join(inputDir, "manifest.ndjson"))
	if err != nil {
		t.Fatalf("create manifest: %v", err)
	}
	if err := output.NewManifestWriter(manifestFile).WriteManifest(manifest); err != nil {
		_ = manifestFile.Close()
		t.Fatalf("write manifest: %v", err)
	}
	if err := manifestFile.Close(); err != nil {
		t.Fatalf("close manifest: %v", err)
	}
}
