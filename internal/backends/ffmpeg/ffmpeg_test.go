package ffmpeg

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
	if got, want := ParseVersion("ffmpeg version 7.1 Copyright"), "7.1"; got != want {
		t.Fatalf("expected parsed version %q, got %q", want, got)
	}
}

func TestBuildExtractCommand(t *testing.T) {
	cmd := BuildExtractCommand("/tmp/in.ts", "/tmp/out.bin", 4)
	if got, want := cmd.Path, "ffmpeg"; got != want {
		t.Fatalf("expected command path %q, got %q", want, got)
	}
	if got := strings.Join(cmd.Args, " "); !strings.Contains(got, "-map 0:4") {
		t.Fatalf("expected extract args to target stream 4, got %q", got)
	}
}

func TestVersionUsesNormalizedParser(t *testing.T) {
	backend := &Backend{
		Run: func(ctx context.Context, path string, args ...string) ([]byte, error) {
			return []byte("ffmpeg version 7.1.1-static"), nil
		},
	}

	got, err := backend.Version(context.Background())
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if got != "7.1.1-static" {
		t.Fatalf("expected parsed version, got %q", got)
	}
}

func TestExtractBuildsRecordsFromProbeAndPayloads(t *testing.T) {
	root := t.TempDir()
	backend := &Backend{
		Run: func(ctx context.Context, path string, args ...string) ([]byte, error) {
			switch path {
			case "ffprobe":
				return []byte(`{"streams":[{"index":2,"id":"0x1bd"},{"index":3,"id":"445"}]}`), nil
			case "ffmpeg":
				outPath := args[len(args)-1]
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
	if records[0].PID != 0x1bd {
		t.Fatalf("expected first record pid 0x1bd, got %d", records[0].PID)
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

func TestExtractNormalizesMissingPIDWarning(t *testing.T) {
	root := t.TempDir()
	backend := &Backend{
		Run: func(ctx context.Context, path string, args ...string) ([]byte, error) {
			switch path {
			case "ffprobe":
				return []byte(`{"streams":[{"index":2,"id":""}]}`), nil
			case "ffmpeg":
				outPath := args[len(args)-1]
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
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if got, want := records[0].Warnings, []string{"pid unavailable from backend metadata"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("expected normalized warning %q, got %#v", want[0], got)
	}
}

func TestExtractPreservesMalformedPIDWarning(t *testing.T) {
	root := t.TempDir()
	backend := &Backend{
		Run: func(ctx context.Context, path string, args ...string) ([]byte, error) {
			switch path {
			case "ffprobe":
				return []byte(`{"streams":[{"index":2,"id":"abc"}]}`), nil
			case "ffmpeg":
				outPath := args[len(args)-1]
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
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if got := records[0].Warnings; len(got) != 1 || !strings.Contains(got[0], `abc`) {
		t.Fatalf("expected malformed id warning to preserve raw value, got %#v", got)
	}
}

func TestExtractWrapsProbeParseErrors(t *testing.T) {
	backend := &Backend{
		Run: func(ctx context.Context, path string, args ...string) ([]byte, error) {
			if path == "ffprobe" {
				return []byte("{"), nil
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
