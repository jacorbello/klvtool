package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacorbello/klvtool/internal/envcheck"
	"github.com/jacorbello/klvtool/internal/extract"
)

func TestExtractManifestCloseErrorSurfaced(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &stderr
	cmd.Extract.Detect = func(ctx context.Context, goos string, env map[string]string) envcheck.Report {
		return envcheck.Report{
			Backends: []envcheck.BackendHealth{
				{Name: "ffmpeg", Healthy: true},
			},
		}
	}
	cmd.Extract.Extractor = stubExtractor{
		run: func(ctx context.Context, req extract.RunRequest) (extract.RunResult, error) {
			return extract.RunResult{
				Backend:        extract.BackendDescriptor{Name: "ffmpeg", Healthy: true},
				BackendVersion: "7.1",
				Records: []extract.PayloadRecord{
					{RecordID: "rec-001", PID: 481, Payload: []byte{0x01}},
				},
			}, nil
		},
	}
	cmd.Extract.OpenManifest = func(path string) (io.WriteCloser, error) {
		return &errCloser{closeErr: errors.New("disk full on close")}, nil
	}

	input := filepath.Join(t.TempDir(), "sample.ts")
	if err := os.WriteFile(input, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	code := cmd.Execute([]string{"extract", "--input", input, "--out", root})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "disk full on close") {
		t.Errorf("expected close error on stderr; got: %s", stderr.String())
	}
}

func TestExtractManifestCloseSuccessUnaffected(t *testing.T) {
	root := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &stderr
	cmd.Extract.Detect = func(ctx context.Context, goos string, env map[string]string) envcheck.Report {
		return envcheck.Report{
			Backends: []envcheck.BackendHealth{
				{Name: "ffmpeg", Healthy: true},
			},
		}
	}
	cmd.Extract.Extractor = stubExtractor{
		run: func(ctx context.Context, req extract.RunRequest) (extract.RunResult, error) {
			return extract.RunResult{
				Backend:        extract.BackendDescriptor{Name: "ffmpeg", Healthy: true},
				BackendVersion: "7.1",
				Records: []extract.PayloadRecord{
					{RecordID: "rec-001", PID: 481, Payload: []byte{0x01}},
				},
			}, nil
		},
	}

	input := filepath.Join(t.TempDir(), "sample.ts")
	if err := os.WriteFile(input, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	code := cmd.Execute([]string{"extract", "--input", input, "--out", root})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr; got: %s", stderr.String())
	}
}
