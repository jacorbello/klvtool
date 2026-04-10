package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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
