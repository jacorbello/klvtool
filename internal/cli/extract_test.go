package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacorbello/klvtool/internal/envcheck"
	"github.com/jacorbello/klvtool/internal/extract"
	"github.com/jacorbello/klvtool/internal/model"
)

type stubExtractor struct {
	run func(context.Context, extract.RunRequest) (extract.RunResult, error)
}

func (s stubExtractor) Run(ctx context.Context, req extract.RunRequest) (extract.RunResult, error) {
	return s.run(ctx, req)
}

func TestExtractHelpMixedWithFlags(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := &ExtractCommand{Out: &out, Err: &errBuf}
	code := cmd.Execute([]string{"--help", "--out", "/tmp"})
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

func TestExtractRequiresInputAndOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &stderr

	if got := cmd.Execute([]string{"extract"}); got != usageExitCode {
		t.Fatalf("expected usage exit code %d, got %d", usageExitCode, got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected validation failure to keep stdout empty, got %q", stdout.String())
	}
	if text := stderr.String(); !strings.Contains(text, "input path is required") {
		t.Fatalf("expected missing input error, got %q", text)
	}
}

func TestExtractStopsOnExplicitBackendFailure(t *testing.T) {
	var stderr bytes.Buffer

	cmd := NewRootCommand()
	cmd.Out = nil
	cmd.Err = &stderr
	cmd.Extract.Detect = func(ctx context.Context, goos string, env map[string]string) envcheck.Report {
		return envcheck.Report{
			Backends: []envcheck.BackendHealth{
				{Name: "ffmpeg", Healthy: false},
			},
		}
	}
	cmd.Extract.Extractor = stubExtractor{
		run: func(ctx context.Context, req extract.RunRequest) (extract.RunResult, error) {
			t.Fatal("extractor should not run when requested backend is unavailable")
			return extract.RunResult{}, nil
		},
	}

	if got := cmd.Execute([]string{"extract", "--input", "a.ts", "--out", "out"}); got != 1 {
		t.Fatalf("expected runtime failure exit code 1, got %d", got)
	}
	if text := stderr.String(); !strings.Contains(text, "missing_dependency") {
		t.Fatalf("expected missing dependency error, got %q", text)
	}
}

func TestExtractWritesManifestAndPayloads(t *testing.T) {
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
			if req.Backend.Name != "ffmpeg" {
				t.Fatalf("expected ffmpeg backend, got %q", req.Backend.Name)
			}
			return extract.RunResult{
				Backend:        extract.BackendDescriptor{Name: "ffmpeg", Healthy: true},
				BackendVersion: "7.1",
				Records: []extract.PayloadRecord{
					{
						RecordID: "rec-001",
						PID:      481,
						Payload:  []byte{0x01, 0x02, 0x03},
						Warnings: []string{"pid derived from probe"},
					},
				},
			}, nil
		},
	}

	if got := cmd.Execute([]string{"extract", "--input", "sample.ts", "--out", root}); got != 0 {
		t.Fatalf("expected successful extract exit code 0, got %d; stderr=%q", got, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected successful extract to keep stderr empty, got %q", stderr.String())
	}

	payloadBytes, err := os.ReadFile(filepath.Join(root, "payloads", "rec-001.bin"))
	if err != nil {
		t.Fatalf("read payload file: %v", err)
	}
	if string(payloadBytes) != string([]byte{0x01, 0x02, 0x03}) {
		t.Fatalf("unexpected payload bytes: %v", payloadBytes)
	}

	manifestBytes, err := os.ReadFile(filepath.Join(root, "manifest.ndjson"))
	if err != nil {
		t.Fatalf("read manifest file: %v", err)
	}

	var manifest model.Manifest
	if err := json.Unmarshal(bytes.TrimSpace(manifestBytes), &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if manifest.BackendName != "ffmpeg" || manifest.BackendVersion != "7.1" {
		t.Fatalf("unexpected backend metadata: %+v", manifest)
	}
	if got := len(manifest.Records); got != 1 {
		t.Fatalf("expected 1 manifest record, got %d", got)
	}
	if manifest.Records[0].PayloadPath != "payloads/rec-001.bin" {
		t.Fatalf("unexpected payload path %q", manifest.Records[0].PayloadPath)
	}
	if !strings.Contains(stdout.String(), "manifest:") {
		t.Fatalf("expected stdout summary, got %q", stdout.String())
	}
}

func TestExitCodeForTypedErrors(t *testing.T) {
	if got := exitCodeForError(model.InvalidUsage(errors.New("bad"))); got != usageExitCode {
		t.Fatalf("expected invalid usage exit code %d, got %d", usageExitCode, got)
	}
	if got := exitCodeForError(model.MissingDependency(errors.New("missing"))); got != 1 {
		t.Fatalf("expected runtime exit code 1, got %d", got)
	}
}
