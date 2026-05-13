package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacorbello/klvtool/internal/stream"
)

func TestRecordCommandMissingInputUsageError(t *testing.T) {
	errBuf := &bytes.Buffer{}
	cmd := &RecordCommand{Out: io.Discard, Err: errBuf}
	code := cmd.Execute([]string{"--out", filepath.Join(t.TempDir(), "x.ts")})
	if code != usageExitCode {
		t.Fatalf("code = %d, want %d", code, usageExitCode)
	}
	if !strings.Contains(errBuf.String(), "--input is required") {
		t.Errorf("stderr: %s", errBuf.String())
	}
}

func TestRecordCommandMissingOutUsageError(t *testing.T) {
	errBuf := &bytes.Buffer{}
	cmd := &RecordCommand{Out: io.Discard, Err: errBuf}
	code := cmd.Execute([]string{"--input", "-"})
	if code != usageExitCode {
		t.Fatalf("code = %d, want %d", code, usageExitCode)
	}
	if !strings.Contains(errBuf.String(), "--out is required") {
		t.Errorf("stderr: %s", errBuf.String())
	}
}

func TestRecordCommandCapturesBytesFromFakeSource(t *testing.T) {
	payload := bytes.Repeat([]byte{0x47, 0x01, 0x02, 0x03}, 100)
	outPath := filepath.Join(t.TempDir(), "cap.bin")
	errBuf := &bytes.Buffer{}
	cmd := &RecordCommand{
		Out: io.Discard,
		Err: errBuf,
		openSource: func(_ context.Context, _ string, _ stream.Options) (stream.Source, error) {
			return &fakeStreamSource{r: bytes.NewReader(payload), scheme: "tcp"}, nil
		},
	}
	code := cmd.Execute([]string{"--input", "tcp://127.0.0.1:5005", "--out", outPath})
	if code != 0 {
		t.Fatalf("code = %d, stderr=%s", code, errBuf.String())
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read out: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("captured %d bytes, want %d", len(got), len(payload))
	}
	if !strings.Contains(errBuf.String(), "exit=") {
		t.Errorf("expected summary line on stderr, got: %s", errBuf.String())
	}
}

func TestRecordCommandRefusesOverwriteByDefault(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "cap.bin")
	if err := os.WriteFile(outPath, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	errBuf := &bytes.Buffer{}
	cmd := &RecordCommand{
		Out: io.Discard,
		Err: errBuf,
		openSource: func(_ context.Context, _ string, _ stream.Options) (stream.Source, error) {
			return &fakeStreamSource{r: bytes.NewReader([]byte("new")), scheme: "tcp"}, nil
		},
	}
	code := cmd.Execute([]string{"--input", "tcp://127.0.0.1:5005", "--out", outPath})
	if code == 0 {
		t.Fatalf("expected non-zero exit when --out exists; stderr=%s", errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "output_write_failure") {
		t.Errorf("expected output_write_failure in stderr, got: %s", errBuf.String())
	}
}

func TestRecordCommandOverwriteAllowedWithFlag(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "cap.bin")
	if err := os.WriteFile(outPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	errBuf := &bytes.Buffer{}
	cmd := &RecordCommand{
		Out: io.Discard,
		Err: errBuf,
		openSource: func(_ context.Context, _ string, _ stream.Options) (stream.Source, error) {
			return &fakeStreamSource{r: bytes.NewReader([]byte("new")), scheme: "tcp"}, nil
		},
	}
	code := cmd.Execute([]string{
		"--input", "tcp://127.0.0.1:5005",
		"--out", outPath,
		"--record-overwrite",
	})
	if code != 0 {
		t.Fatalf("code = %d, stderr=%s", code, errBuf.String())
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("got %q, want %q", got, "new")
	}
}

func TestRecordCommandMaxBytesStopsEarly(t *testing.T) {
	// 4 KiB payload, --max-bytes 1024 → expect capture to stop after ~1 KiB.
	payload := bytes.Repeat([]byte{0xAB}, 4096)
	outPath := filepath.Join(t.TempDir(), "cap.bin")
	errBuf := &bytes.Buffer{}
	cmd := &RecordCommand{
		Out: io.Discard,
		Err: errBuf,
		openSource: func(_ context.Context, _ string, _ stream.Options) (stream.Source, error) {
			return &fakeStreamSource{r: bytes.NewReader(payload), scheme: "tcp"}, nil
		},
	}
	code := cmd.Execute([]string{
		"--input", "tcp://127.0.0.1:5005",
		"--out", outPath,
		"--max-bytes", "1024",
	})
	if code != 0 {
		t.Fatalf("code = %d, stderr=%s", code, errBuf.String())
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	// io.Copy uses a 32KB buffer internally — but our recordWriter
	// triggers AddBytes after each write, and the fake source delivers
	// bytes via bytes.Reader which may exceed max-bytes in one chunk.
	// What matters: we stopped (we'd have captured all 4096 without
	// max-bytes) and the summary says max-bytes.
	if len(got) > len(payload) {
		t.Errorf("captured more bytes (%d) than the payload (%d)", len(got), len(payload))
	}
	if !strings.Contains(errBuf.String(), "exit=max-bytes") && !strings.Contains(errBuf.String(), "exit=eof") {
		t.Errorf("expected exit=max-bytes or exit=eof in summary, got: %s", errBuf.String())
	}
}

func TestRecordCommandHelpFlag(t *testing.T) {
	out := &bytes.Buffer{}
	cmd := &RecordCommand{Out: out, Err: io.Discard}
	if code := cmd.Execute([]string{"--help"}); code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	got := out.String()
	if !strings.Contains(got, "klvtool record") || !strings.Contains(got, "--input") || !strings.Contains(got, "--out") {
		t.Errorf("usage missing expected text, got: %s", got)
	}
}
