package output

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/jacorbello/klvtool/internal/model"
)

func TestPacketManifestWriterWritesManifestLine(t *testing.T) {
	var buf bytes.Buffer
	writer := NewPacketManifestWriter(&buf)

	err := writer.WriteManifest(model.PacketManifest{
		SchemaVersion: "1",
		SourcePath:    "/tmp/raw",
		Records: []model.PacketManifestEntry{
			{
				RecordID:   "klv-001",
				Mode:       "strict",
				PacketPath: "packets/klv-001.json",
			},
		},
	})
	if err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, `"schemaVersion":"1"`) {
		t.Fatalf("expected schema version in output, got %q", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("expected manifest output to end with newline, got %q", got)
	}
}

func TestPacketManifestWriterWrapsWriteErrors(t *testing.T) {
	boom := errors.New("boom")
	writer := NewPacketManifestWriter(failingWriter{err: boom})

	err := writer.WriteManifest(model.PacketManifest{SchemaVersion: "1"})
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
