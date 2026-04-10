package gstreamer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacorbello/klvtool/internal/model"
)

func TestParseVersion(t *testing.T) {
	if got, want := ParseVersion("gst-launch-1.0 version 1.24.3"), "1.24.3"; got != want {
		t.Fatalf("expected parsed version %q, got %q", want, got)
	}
}

func TestBuildExtractCommand(t *testing.T) {
	cmd := BuildExtractCommand("/tmp/in.ts", "/tmp/out.bin", "private_01bd")
	if got, want := cmd.Path, "gst-launch-1.0"; got != want {
		t.Fatalf("expected command path %q, got %q", want, got)
	}
	if got := strings.Join(cmd.Args, " "); !strings.Contains(got, "filesrc location=/tmp/in.ts") {
		t.Fatalf("expected filesrc input in args, got %q", got)
	}
	if got := strings.Join(cmd.Args, " "); !strings.Contains(got, "demux.private_01bd") {
		t.Fatalf("expected extract args to target stream %q, got %q", "private_01bd", got)
	}
}

func TestBuildDiscoverCommand(t *testing.T) {
	cmd := BuildDiscoverCommand("/tmp/in.ts")
	if got, want := cmd.Path, "gst-discoverer-1.0"; got != want {
		t.Fatalf("expected command path %q, got %q", want, got)
	}
	if got := strings.Join(cmd.Args, " "); !strings.Contains(got, "-v /tmp/in.ts") {
		t.Fatalf("expected verbose discover args for %q, got %q", "/tmp/in.ts", got)
	}
}

func TestVersionUsesNormalizedParser(t *testing.T) {
	backend := &Backend{
		Run: func(ctx context.Context, path string, args ...string) ([]byte, error) {
			return []byte("gst-launch-1.0 version 1.24.3"), nil
		},
	}

	got, err := backend.Version(context.Background())
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if got != "1.24.3" {
		t.Fatalf("expected parsed version, got %q", got)
	}
}

func TestParseDiscoverStreams(t *testing.T) {
	streams, err := parseDiscoverStreams([]byte(`
Analyzing file:///tmp/input.ts
Done discovering file:///tmp/input.ts

  private_01bd: KLV Metadata
    PID: 0x01bd
  private_01be: KLV Metadata
`))
	if err != nil {
		t.Fatalf("parse discover output: %v", err)
	}
	if len(streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(streams))
	}
	if got, want := streams[0].StreamID, "private_01bd"; got != want {
		t.Fatalf("expected first stream id %q, got %q", want, got)
	}
	if got, want := streams[0].PID, uint16(0x01bd); got != want {
		t.Fatalf("expected first stream pid %d, got %d", want, got)
	}
	if got, want := streams[1].PID, uint16(0x01be); got != want {
		t.Fatalf("expected second stream pid %d from stream id, got %d", want, got)
	}
	if got := streams[1].Warning; got != "" {
		t.Fatalf("expected second stream to avoid warnings when stream id is parseable, got %q", got)
	}
}

func TestExtractBuildsRecordsFromDiscoveryAndPayloads(t *testing.T) {
	root := t.TempDir()
	backend := &Backend{
		Run: func(ctx context.Context, path string, args ...string) ([]byte, error) {
			switch path {
			case "gst-discoverer-1.0":
				return []byte(`
Analyzing file:///tmp/input.ts
Done discovering file:///tmp/input.ts

  private_01bd: KLV Metadata
    PID: 0x01bd
  private_01be: KLV Metadata
    PID: 446
`), nil
			case "gst-launch-1.0":
				outPath := extractOutputPath(args)
				if outPath == "" {
					t.Fatalf("expected filesink output path in args %q", strings.Join(args, " "))
				}
				if err := os.WriteFile(outPath, []byte(filepath.Base(outPath)), 0o600); err != nil {
					t.Fatalf("write extracted payload: %v", err)
				}
				return []byte(""), nil
			default:
				t.Fatalf("unexpected command path %q", path)
				return nil, nil
			}
		},
	}

	records, err := backend.Extract(context.Background(), filepath.Join(root, "input.ts"))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].PID != 0x01bd {
		t.Fatalf("expected first record pid 0x01bd, got %d", records[0].PID)
	}
	for i, record := range records {
		if record.RecordID != "" {
			t.Fatalf("expected record %d id to be left for canonicalization, got %q", i, record.RecordID)
		}
	}
	if string(records[1].Payload) != "klv-002.bin" {
		t.Fatalf("expected second payload bytes derived from filename, got %q", string(records[1].Payload))
	}
}

func TestExtractWrapsDiscoverParseErrors(t *testing.T) {
	backend := &Backend{
		Run: func(ctx context.Context, path string, args ...string) ([]byte, error) {
			if path == "gst-discoverer-1.0" {
				return []byte("not discoverer output"), nil
			}
			return nil, errors.New("unexpected")
		},
	}

	_, err := backend.Extract(context.Background(), "/tmp/input.ts")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, &model.Error{Code: model.CodeBackendParse}) {
		t.Fatalf("expected backend parse error, got %v", err)
	}
}

func extractOutputPath(args []string) string {
	for i := len(args) - 1; i >= 0; i-- {
		if strings.HasPrefix(args[i], "location=") {
			return strings.TrimPrefix(args[i], "location=")
		}
	}
	return ""
}
