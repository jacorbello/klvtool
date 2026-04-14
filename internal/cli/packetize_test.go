package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacorbello/klvtool/internal/extract"
	"github.com/jacorbello/klvtool/internal/model"
	"github.com/jacorbello/klvtool/internal/output"
	"github.com/jacorbello/klvtool/internal/packetize"
)

func TestPacketizeHelpMixedWithFlags(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := &PacketizeCommand{Out: &out, Err: &errBuf}
	code := cmd.Execute([]string{"--help", "--input", "/tmp/in"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Errorf("expected usage on stdout, got %q", out.String())
	}
	if errBuf.Len() != 0 {
		t.Errorf("expected empty stderr, got %q", errBuf.String())
	}
}

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

func TestPacketizeRequiresOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &stderr

	if got := cmd.Execute([]string{"packetize", "--input", t.TempDir()}); got != usageExitCode {
		t.Fatalf("expected usage exit code %d, got %d", usageExitCode, got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected validation failure to keep stdout empty, got %q", stdout.String())
	}
	if text := stderr.String(); !strings.Contains(text, "output directory is required") {
		t.Fatalf("expected missing output error, got %q", text)
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

func TestPacketizeValidatesInputDirectoryExistence(t *testing.T) {
	t.Run("non-existent directory", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		cmd := NewRootCommand()
		cmd.Out = &stdout
		cmd.Err = &stderr

		missingDir := filepath.Join(t.TempDir(), "missing")
		code := cmd.Execute([]string{"packetize", "--input", missingDir, "--out", t.TempDir()})
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if text := stderr.String(); !strings.Contains(text, "input directory does not exist: "+missingDir) {
			t.Fatalf("expected clear error about missing directory, got %q", text)
		}
	})

	t.Run("file instead of directory", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		cmd := NewRootCommand()
		cmd.Out = &stdout
		cmd.Err = &stderr

		tmpFile := filepath.Join(t.TempDir(), "notadir.txt")
		if err := os.WriteFile(tmpFile, []byte("hello"), 0o644); err != nil {
			t.Fatalf("create temp file: %v", err)
		}

		code := cmd.Execute([]string{"packetize", "--input", tmpFile, "--out", t.TempDir()})
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if text := stderr.String(); !strings.Contains(text, "input path is not a directory") {
			t.Fatalf("expected clear error about non-directory input, got %q", text)
		}
	})
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
	if !bytes.Contains(manifestBytes, []byte(`"schemaVersion":"3"`)) {
		t.Fatalf("expected packet manifest schema version 2, got %s", manifestBytes)
	}

	packetBytes, err := os.ReadFile(filepath.Join(outputDir, "packets", "klv-001.json"))
	if err != nil {
		t.Fatalf("read packet checkpoint: %v", err)
	}
	if !bytes.Contains(packetBytes, []byte(`"schemaVersion":"3"`)) {
		t.Fatalf("expected packet checkpoint schema version 2, got %s", packetBytes)
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
		[]byte(`"packetEndInclusive":19`),
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
	if !strings.Contains(stdout.String(), "records: 1") {
		t.Fatalf("expected stdout summary, got %q", stdout.String())
	}
}

func TestPacketizeRejectsStrayArgs(t *testing.T) {
	var stderr bytes.Buffer
	cmd := &PacketizeCommand{Out: &bytes.Buffer{}, Err: &stderr}
	if got := cmd.Execute([]string{"stray"}); got != 2 {
		t.Fatalf("exit code = %d, want 2", got)
	}
	if !strings.Contains(stderr.String(), "unsupported arguments") {
		t.Fatalf("expected unsupported arguments error, got %q", stderr.String())
	}
}

func TestPacketizeOverwriteWarningBehavior(t *testing.T) {
	// Helper to set up a valid input dir with manifest and payload.
	setupInput := func(t *testing.T) string {
		t.Helper()
		inputDir := t.TempDir()
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
			if closeErr := manifestFile.Close(); closeErr != nil {
				t.Fatalf("write manifest: %v; close manifest: %v", err, closeErr)
			}
			t.Fatalf("write manifest: %v", err)
		}
		if err := manifestFile.Close(); err != nil {
			t.Fatalf("close manifest: %v", err)
		}
		return inputDir
	}

	t.Run("fresh output dir emits no warning", func(t *testing.T) {
		inputDir := setupInput(t)
		outDir := t.TempDir()

		var stdout, stderr bytes.Buffer
		cmd := NewRootCommand()
		cmd.Out = &stdout
		cmd.Err = &stderr

		code := cmd.Execute([]string{"packetize", "--input", inputDir, "--out", outDir})
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
		}
		if stderr.Len() != 0 {
			t.Fatalf("expected empty stderr on fresh dir, got %q", stderr.String())
		}
	})

	t.Run("empty record id returns model.OutputWrite error", func(t *testing.T) {
		_, err := packetCheckpointFilename("")
		if err == nil {
			t.Fatal("expected error for empty record id")
		}
		var mErr *model.Error
		if !errors.As(err, &mErr) {
			t.Fatalf("expected model.Error, got %T: %v", err, err)
		}
		if mErr.Code != model.CodeOutputWrite {
			t.Fatalf("expected code %q, got %q", model.CodeOutputWrite, mErr.Code)
		}
	})

	t.Run("existing output dir with manifest emits warning", func(t *testing.T) {
		inputDir := setupInput(t)
		outDir := t.TempDir()

		// Pre-populate output dir with manifest.ndjson
		if err := os.WriteFile(filepath.Join(outDir, "manifest.ndjson"), []byte("{}"), 0o644); err != nil {
			t.Fatalf("seed manifest: %v", err)
		}

		var stdout, stderr bytes.Buffer
		cmd := NewRootCommand()
		cmd.Out = &stdout
		cmd.Err = &stderr

		code := cmd.Execute([]string{"packetize", "--input", inputDir, "--out", outDir})
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
		}
		want := "warning: output directory already exists, files will be overwritten: " + outDir
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("expected overwrite warning on stderr, got %q", stderr.String())
		}
	})

}

func TestPacketCheckpointUsesPacketEndInclusive(t *testing.T) {
	stream := packetize.PacketizedStream{
		Source:        extract.RawPayloadRecord{RecordID: "klv-001"},
		Mode:          packetize.ModeStrict,
		ParserVersion: "1",
		ParsedCount:   1,
		Packets: []packetize.Packet{
			{
				PacketIndex:        0,
				PacketStart:        0,
				KeyStart:           0,
				LengthStart:        16,
				ValueStart:         18,
				PacketEndExclusive: 259,
				Key:                make([]byte, 16),
				Length:             241,
				Value:              make([]byte, 241),
				Classification:     packetize.ClassificationUniversalSet,
			},
		},
	}

	checkpoint := toPacketCheckpoint(stream)
	data, err := json.Marshal(checkpoint)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"packetEndInclusive"`) {
		t.Errorf("expected JSON field 'packetEndInclusive'; got: %s", string(data))
	}
	if strings.Contains(string(data), `"packetEnd"`) && !strings.Contains(string(data), `"packetEndInclusive"`) {
		t.Errorf("expected 'packetEndInclusive' not 'packetEnd'; got: %s", string(data))
	}
}

func TestPacketizeRejectsInvalidMode(t *testing.T) {
	inputDir := t.TempDir()
	outDir := t.TempDir()

	var stdout, stderr bytes.Buffer
	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &stderr

	code := cmd.Execute([]string{"packetize", "--input", inputDir, "--out", outDir, "--mode", "invalid"})
	if code != usageExitCode {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, usageExitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "unsupported packetization mode") {
		t.Errorf("expected mode error on stderr; got: %s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout, got %q", stdout.String())
	}
}
