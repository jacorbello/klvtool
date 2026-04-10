package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacorbello/klvtool/internal/model"
	"github.com/jacorbello/klvtool/internal/output"
)

func TestPacketizeRequiresInputAndOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &stderr

	if got := cmd.Execute([]string{"packetize"}); got != usageExitCode {
		t.Fatalf("expected usage exit code %d, got %d", usageExitCode, got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected validation failure to keep stdout empty, got %q", stdout.String())
	}
	if text := stderr.String(); !strings.Contains(text, "input directory is required") {
		t.Fatalf("expected missing input error, got %q", text)
	}
}

func TestPacketizeRejectsSameInputAndOutputDirectory(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	dir := t.TempDir()
	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &stderr

	if got := cmd.Execute([]string{"packetize", "--input", dir, "--out", dir}); got != usageExitCode {
		t.Fatalf("expected usage exit code %d, got %d", usageExitCode, got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected validation failure to keep stdout empty, got %q", stdout.String())
	}
	if text := stderr.String(); !strings.Contains(text, "input and output directories must be different") {
		t.Fatalf("expected same-directory validation error, got %q", text)
	}
}

func TestPacketizeWritesPacketCheckpointOutputs(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()

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

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &stderr

	if got := cmd.Execute([]string{"packetize", "--input", inputDir, "--out", outputDir, "--mode", "best-effort"}); got != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", got, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected successful packetize to keep stderr empty, got %q", stderr.String())
	}

	manifestBytes, err := os.ReadFile(filepath.Join(outputDir, "manifest.ndjson"))
	if err != nil {
		t.Fatalf("read packet manifest: %v", err)
	}
	if !bytes.Contains(manifestBytes, []byte(`"packetPath":"packets/klv-001.json"`)) {
		t.Fatalf("expected packet manifest to reference packet checkpoint path, got %s", manifestBytes)
	}

	packetBytes, err := os.ReadFile(filepath.Join(outputDir, "packets", "klv-001.json"))
	if err != nil {
		t.Fatalf("read packet checkpoint: %v", err)
	}
	if !bytes.Contains(packetBytes, []byte(`"mode":"best-effort"`)) {
		t.Fatalf("expected packet checkpoint to record mode, got %s", packetBytes)
	}
	if !bytes.Contains(packetBytes, []byte(`"packets":[`)) {
		t.Fatalf("expected packet checkpoint to include packet array, got %s", packetBytes)
	}
	if !bytes.Contains(packetBytes, []byte(`"diagnostics":[]`)) {
		t.Fatalf("expected packet checkpoint to normalize diagnostics array, got %s", packetBytes)
	}
	if !bytes.Contains(packetBytes, []byte(`"parsedCount":1`)) {
		t.Fatalf("expected packet checkpoint to include parsed count, got %s", packetBytes)
	}
	for _, want := range [][]byte{
		[]byte(`"packetEnd":20`),
		[]byte(`"rawKeyHex":"060e2b34000000000000000000000000"`),
		[]byte(`"rawValueHex":"aabbcc"`),
	} {
		if !bytes.Contains(packetBytes, want) {
			t.Fatalf("expected packet checkpoint to include %s, got %s", want, packetBytes)
		}
	}
	for _, legacy := range [][]byte{
		[]byte(`"packetEndExclusive"`),
		[]byte(`"key":`),
		[]byte(`"value":`),
	} {
		if bytes.Contains(packetBytes, legacy) {
			t.Fatalf("did not expect legacy field %s in %s", legacy, packetBytes)
		}
	}
	if !strings.Contains(stdout.String(), "packets: 1") {
		t.Fatalf("expected stdout summary, got %q", stdout.String())
	}
}
