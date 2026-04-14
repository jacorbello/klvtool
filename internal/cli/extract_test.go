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

func TestExtractRequiresOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &stderr

	input := filepath.Join(t.TempDir(), "sample.ts")
	if err := os.WriteFile(input, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if got := cmd.Execute([]string{"extract", "--input", input}); got != usageExitCode {
		t.Fatalf("expected usage exit code %d, got %d", usageExitCode, got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected validation failure to keep stdout empty, got %q", stdout.String())
	}
	if text := stderr.String(); !strings.Contains(text, "output directory is required") {
		t.Fatalf("expected missing output error, got %q", text)
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

	input := filepath.Join(t.TempDir(), "a.ts")
	if err := os.WriteFile(input, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := cmd.Execute([]string{"extract", "--input", input, "--out", "out"}); got != 1 {
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

	input := filepath.Join(t.TempDir(), "sample.ts")
	if err := os.WriteFile(input, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := cmd.Execute([]string{"extract", "--input", input, "--out", root}); got != 0 {
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

func TestExtractRejectsNonExistentInput(t *testing.T) {
	var stderr bytes.Buffer
	cmd := NewRootCommand()
	cmd.Out = nil
	cmd.Err = &stderr
	cmd.Extract.Detect = func(ctx context.Context, goos string, env map[string]string) envcheck.Report {
		t.Fatal("detect should not be called for non-existent input")
		return envcheck.Report{}
	}
	cmd.Extract.Extractor = stubExtractor{
		run: func(ctx context.Context, req extract.RunRequest) (extract.RunResult, error) {
			t.Fatal("extractor should not run for non-existent input")
			return extract.RunResult{}, nil
		},
	}

	missing := filepath.Join(t.TempDir(), "missing.ts")
	got := cmd.Execute([]string{"extract", "--input", missing, "--out", t.TempDir()})
	if got != 1 {
		t.Fatalf("exit code = %d, want 1", got)
	}
	text := stderr.String()
	if !strings.Contains(text, "ts_read_failure") {
		t.Fatalf("expected ts_read_failure error code, got %q", text)
	}
	if !strings.Contains(text, missing) {
		t.Fatalf("expected file path in error, got %q", text)
	}
}

func TestExtractRejectsDirectory(t *testing.T) {
	var stderr bytes.Buffer
	cmd := NewRootCommand()
	cmd.Out = nil
	cmd.Err = &stderr
	cmd.Extract.Detect = func(ctx context.Context, goos string, env map[string]string) envcheck.Report {
		t.Fatal("detect should not be called for directory input")
		return envcheck.Report{}
	}

	got := cmd.Execute([]string{"extract", "--input", t.TempDir(), "--out", t.TempDir()})
	if got != 1 {
		t.Fatalf("exit code = %d, want 1", got)
	}
	text := stderr.String()
	if !strings.Contains(text, "ts_read_failure") {
		t.Fatalf("expected ts_read_failure error code, got %q", text)
	}
	if !strings.Contains(text, "not a regular file") {
		t.Fatalf("expected 'not a regular file' message, got %q", text)
	}
}

func TestExtractWarnsWhenOutputDirExists(t *testing.T) {
	healthyDetect := func(ctx context.Context, goos string, env map[string]string) envcheck.Report {
		return envcheck.Report{
			Backends: []envcheck.BackendHealth{
				{Name: "ffmpeg", Healthy: true},
			},
		}
	}
	successExtractor := stubExtractor{
		run: func(ctx context.Context, req extract.RunRequest) (extract.RunResult, error) {
			return extract.RunResult{
				Backend:        extract.BackendDescriptor{Name: "ffmpeg", Healthy: true},
				BackendVersion: "7.1",
				Records:        nil,
			}, nil
		},
	}

	// Helper to create a real input file for os.Stat validation
	makeInput := func(t *testing.T) string {
		t.Helper()
		p := filepath.Join(t.TempDir(), "input.ts")
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	t.Run("fresh output dir emits no warning", func(t *testing.T) {
		root := t.TempDir()
		outDir := filepath.Join(root, "fresh")

		var stdout, stderr bytes.Buffer
		cmd := NewRootCommand()
		cmd.Out = &stdout
		cmd.Err = &stderr
		cmd.Extract.Detect = healthyDetect
		cmd.Extract.Extractor = successExtractor

		code := cmd.Execute([]string{"extract", "--input", makeInput(t), "--out", outDir})
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
		}
		if strings.Contains(stderr.String(), "warning: output directory already exists") {
			t.Errorf("expected no overwrite warning for fresh dir, got stderr=%q", stderr.String())
		}
	})

	t.Run("existing dir with manifest emits warning", func(t *testing.T) {
		outDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(outDir, "manifest.ndjson"), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		cmd := NewRootCommand()
		cmd.Out = &stdout
		cmd.Err = &stderr
		cmd.Extract.Detect = healthyDetect
		cmd.Extract.Extractor = successExtractor

		code := cmd.Execute([]string{"extract", "--input", makeInput(t), "--out", outDir})
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "warning: output directory already exists") {
			t.Errorf("expected overwrite warning on stderr, got %q", stderr.String())
		}
		if !strings.Contains(stderr.String(), outDir) {
			t.Errorf("expected warning to include output dir path %q, got %q", outDir, stderr.String())
		}
	})

	t.Run("existing dir with non-manifest files emits warning", func(t *testing.T) {
		outDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(outDir, "somefile.bin"), []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		cmd := NewRootCommand()
		cmd.Out = &stdout
		cmd.Err = &stderr
		cmd.Extract.Detect = healthyDetect
		cmd.Extract.Extractor = successExtractor

		code := cmd.Execute([]string{"extract", "--input", makeInput(t), "--out", outDir})
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "warning: output directory already exists") {
			t.Errorf("expected overwrite warning for non-empty dir, got stderr=%q", stderr.String())
		}
	})

	t.Run("existing empty dir emits no warning", func(t *testing.T) {
		outDir := t.TempDir()

		var stdout, stderr bytes.Buffer
		cmd := NewRootCommand()
		cmd.Out = &stdout
		cmd.Err = &stderr
		cmd.Extract.Detect = healthyDetect
		cmd.Extract.Extractor = successExtractor

		code := cmd.Execute([]string{"extract", "--input", makeInput(t), "--out", outDir})
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
		}
		if strings.Contains(stderr.String(), "warning: output directory already exists") {
			t.Errorf("expected no overwrite warning for empty dir, got stderr=%q", stderr.String())
		}
	})
}

func TestExtractRejectsStrayArgs(t *testing.T) {
	var stderr bytes.Buffer
	cmd := &ExtractCommand{Out: &bytes.Buffer{}, Err: &stderr}
	if got := cmd.Execute([]string{"stray"}); got != 2 {
		t.Fatalf("exit code = %d, want 2", got)
	}
	if !strings.Contains(stderr.String(), "unsupported arguments") {
		t.Fatalf("expected unsupported arguments error, got %q", stderr.String())
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

func TestExtractOutputUsesStreamsLabel(t *testing.T) {
	var stdout, stderr bytes.Buffer
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
			}, nil
		},
	}

	p := filepath.Join(t.TempDir(), "input.ts")
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(t.TempDir(), "out")

	code := cmd.Execute([]string{"extract", "--input", p, "--out", outDir})
	if code != 0 {
		t.Fatalf("exit code = %d; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "streams:") {
		t.Errorf("expected 'streams:' in output; got %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "records:") {
		t.Errorf("output should use 'streams:' not 'records:'; got %q", stdout.String())
	}
}
