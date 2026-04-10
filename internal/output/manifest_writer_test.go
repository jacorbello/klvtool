package output

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/jacorbello/klvtool/internal/model"
)

type failingWriter struct {
	err error
}

func (w failingWriter) Write(p []byte) (int, error) {
	return 0, w.err
}

func TestManifestWriterWritesNDJSONRecord(t *testing.T) {
	var buf bytes.Buffer
	writer := NewManifestWriter(&buf)

	err := writer.WriteRecord(model.Record{
		RecordID:    "rec-001",
		PID:         136,
		PayloadPath: "payloads/rec-001.bin",
		PayloadSize: 4,
		PayloadHash: "sha256:abc",
	})
	if err != nil {
		t.Fatalf("write record: %v", err)
	}

	got := buf.String()
	want := `{"recordId":"rec-001","pid":136,"payloadPath":"payloads/rec-001.bin","payloadSize":4,"payloadHash":"sha256:abc","warnings":[]}` + "\n"
	if got != want {
		t.Fatalf("unexpected ndjson output\nwant: %q\ngot:  %q", want, got)
	}
}

func TestManifestWriterWrapsWriteErrors(t *testing.T) {
	boom := errors.New("boom")
	writer := NewManifestWriter(failingWriter{err: boom})

	err := writer.WriteRecord(model.Record{RecordID: "rec-001"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "output_write_failure") {
		t.Fatalf("expected typed output error, got %v", err)
	}
	if !errors.Is(err, &model.Error{Code: model.CodeOutputWrite}) {
		t.Fatalf("expected errors.Is to match output write failure, got %v", err)
	}
}

func TestManifestWriterWritesManifestLine(t *testing.T) {
	var buf bytes.Buffer
	writer := NewManifestWriter(&buf)

	err := writer.WriteManifest(model.Manifest{
		SchemaVersion:   "1",
		SourceInputPath: "input.ts",
		BackendName:     "ffmpeg",
		BackendVersion:  "7.1",
		Records: []model.Record{
			{
				RecordID:    "rec-001",
				PID:         256,
				PayloadPath: "payloads/rec-001.bin",
			},
		},
	})
	if err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, `"backendName":"ffmpeg"`) {
		t.Fatalf("expected manifest output to include backend name, got %q", got)
	}
	if !strings.Contains(got, `"backendVersion":"7.1"`) {
		t.Fatalf("expected manifest output to include backend version, got %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("expected manifest output to end with newline, got %q", got)
	}
}
