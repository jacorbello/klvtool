package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jacorbello/klvtool/internal/cli"
	"github.com/jacorbello/klvtool/internal/model"
)

func TestExtractIntegrationFFmpeg(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	fixture := filepath.Join("..", "testdata", "fixtures", "sample.ts")
	if _, err := os.Stat(fixture); err != nil {
		t.Skipf("fixture not available: %v", err)
	}
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available: %v", tool, err)
		}
	}

	outDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := cli.NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &stderr

	if got := cmd.Execute([]string{"extract", "--input", fixture, "--out", outDir, "--backend", "ffmpeg"}); got != 0 {
		t.Fatalf("expected extract command to succeed, got %d with stderr %q", got, stderr.String())
	}

	manifest, err := readManifest(filepath.Join(outDir, "manifest.ndjson"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if manifest.BackendName != "ffmpeg" {
		t.Fatalf("expected manifest backend name ffmpeg, got %q", manifest.BackendName)
	}
}

func TestExtractIntegrationParityAcrossBackends(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	fixture := filepath.Join("..", "testdata", "fixtures", "sample.ts")
	if _, err := os.Stat(fixture); err != nil {
		t.Skipf("fixture not available: %v", err)
	}
	for _, tool := range []string{"ffmpeg", "ffprobe", "gst-launch-1.0", "gst-inspect-1.0", "gst-discoverer-1.0"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available: %v", tool, err)
		}
	}

	ffmpegOut := t.TempDir()
	gstreamerOut := t.TempDir()

	ffmpegManifest := runExtractIntegration(t, fixture, ffmpegOut, "ffmpeg")
	gstreamerManifest := runExtractIntegration(t, fixture, gstreamerOut, "gstreamer")

	if ffmpegManifest.SchemaVersion != gstreamerManifest.SchemaVersion {
		t.Fatalf("schema version mismatch: %q vs %q", ffmpegManifest.SchemaVersion, gstreamerManifest.SchemaVersion)
	}
	if ffmpegManifest.SourceInputPath != gstreamerManifest.SourceInputPath {
		t.Fatalf("source input path mismatch: %q vs %q", ffmpegManifest.SourceInputPath, gstreamerManifest.SourceInputPath)
	}
	if ffmpegManifest.BackendName != "ffmpeg" {
		t.Fatalf("expected ffmpeg manifest backend name, got %q", ffmpegManifest.BackendName)
	}
	if gstreamerManifest.BackendName != "gstreamer" {
		t.Fatalf("expected gstreamer manifest backend name, got %q", gstreamerManifest.BackendName)
	}

	if !reflect.DeepEqual(ffmpegManifest.Records, gstreamerManifest.Records) {
		t.Fatalf("manifest record mismatch\nffmpeg: %#v\ngstreamer: %#v", ffmpegManifest.Records, gstreamerManifest.Records)
	}

	ffmpegPayloads, err := collectPayloadBytes(ffmpegOut, ffmpegManifest)
	if err != nil {
		t.Fatalf("collect ffmpeg payloads: %v", err)
	}
	gstreamerPayloads, err := collectPayloadBytes(gstreamerOut, gstreamerManifest)
	if err != nil {
		t.Fatalf("collect gstreamer payloads: %v", err)
	}
	if !reflect.DeepEqual(ffmpegPayloads, gstreamerPayloads) {
		t.Fatalf("payload mismatch\nffmpeg: %#v\ngstreamer: %#v", ffmpegPayloads, gstreamerPayloads)
	}
}

func runExtractIntegration(t *testing.T, fixture, outDir, backend string) model.Manifest {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := cli.NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &stderr

	if got := cmd.Execute([]string{"extract", "--input", fixture, "--out", outDir, "--backend", backend}); got != 0 {
		t.Fatalf("%s extract failed: code=%d stderr=%q", backend, got, stderr.String())
	}

	manifest, err := readManifest(filepath.Join(outDir, "manifest.ndjson"))
	if err != nil {
		t.Fatalf("read %s manifest: %v", backend, err)
	}
	return manifest
}

func readManifest(path string) (model.Manifest, error) {
	var manifest model.Manifest

	data, err := os.ReadFile(path)
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(bytes.TrimSpace(data), &manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func collectPayloadBytes(root string, manifest model.Manifest) (map[string][]byte, error) {
	payloads := make(map[string][]byte, len(manifest.Records))
	for _, record := range manifest.Records {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(record.PayloadPath)))
		if err != nil {
			return nil, err
		}
		payloads[record.PayloadPath] = data
	}
	return payloads, nil
}
