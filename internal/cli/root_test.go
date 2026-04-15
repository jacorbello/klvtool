package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ts "github.com/jacorbello/klvtool/internal/mpeg/ts"
	"github.com/jacorbello/klvtool/internal/version"
)

func TestNewRootCommand(t *testing.T) {
	cmd := NewRootCommand()
	if cmd == nil {
		t.Fatal("expected root command")
	}
	if cmd.Use != "klvtool" {
		t.Fatalf("expected command use klvtool, got %q", cmd.Use)
	}
}

func TestNewRootCommandVersion(t *testing.T) {
	cmd := NewRootCommand()
	if cmd.Version == "" {
		t.Fatal("expected root command version to be non-empty")
	}
	if cmd.Version != version.String() {
		t.Fatalf("expected root command version %q, got %q", version.String(), cmd.Version)
	}
}

func TestExecuteEmptyArgs(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &stderr

	if got := cmd.Execute(nil); got != usageExitCode {
		t.Fatalf("expected usage exit code %d for empty args, got %d", usageExitCode, got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty args to keep stdout empty, got %q", stdout.String())
	}
	text := stderr.String()
	if !strings.Contains(text, "Usage:") {
		t.Fatalf("expected usage text on stderr for empty args, got %q", text)
	}
	if strings.Contains(text, "error:") {
		t.Fatalf("expected no error prefix for empty args, got %q", text)
	}
}

func TestHelpArgs(t *testing.T) {
	for _, arg := range []string{"--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			cmd := NewRootCommand()
			cmd.Out = &stdout
			cmd.Err = &stderr

			if got := cmd.Execute([]string{arg}); got != 0 {
				t.Fatalf("expected help exit code 0, got %d", got)
			}
			if stderr.Len() != 0 {
				t.Fatalf("expected help to keep stderr empty, got %q", stderr.String())
			}
			text := stdout.String()
			if !strings.Contains(text, "Usage:") {
				t.Fatalf("expected help text to include usage text, got %q", text)
			}
			if !strings.Contains(text, "klvtool") {
				t.Fatalf("expected help text to include klvtool, got %q", text)
			}
			if !strings.Contains(text, version.String()) {
				t.Fatalf("expected help text to include version %q, got %q", version.String(), text)
			}
			if !strings.Contains(text, "Common workflows:") {
				t.Fatalf("expected task-oriented workflow section, got %q", text)
			}
			if !strings.Contains(text, "inspect -> decode") {
				t.Fatalf("expected inspect/decode workflow hint, got %q", text)
			}
		})
	}
}

func TestRootRoutesToInspect(t *testing.T) {
	var out, errBuf bytes.Buffer

	root := &RootCommand{
		Out: &out,
		Err: &errBuf,
		Inspect: &InspectCommand{
			Out: &out,
			Err: &errBuf,
			Inspect: func(path string) (ts.StreamTable, InspectStats, error) {
				return ts.StreamTable{Programs: map[uint16][]ts.Stream{}}, InspectStats{
					PacketCounts:  map[uint16]int64{},
					PESUnitCounts: map[uint16]int{},
					FirstPTS:      map[uint16]int64{},
					LastPTS:       map[uint16]int64{},
				}, nil
			},
		},
	}

	input := filepath.Join(t.TempDir(), "test.ts")
	if err := os.WriteFile(input, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	code := root.Execute([]string{"inspect", "--input", input})
	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
}

func TestHelpSubcommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &stderr

	if got := cmd.Execute([]string{"help"}); got != 0 {
		t.Fatalf("expected exit code 0, got %d", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected stderr empty, got %q", stderr.String())
	}
	text := stdout.String()
	if !strings.Contains(text, "Usage:") {
		t.Fatalf("expected usage text on stdout, got %q", text)
	}
}

func TestHelpSubcommandWithExtraArgs(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &stderr

	if got := cmd.Execute([]string{"help", "extraarg"}); got != usageExitCode {
		t.Fatalf("expected exit code %d, got %d", usageExitCode, got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected stdout empty, got %q", stdout.String())
	}
	text := stderr.String()
	if !strings.Contains(text, "error: unsupported arguments") {
		t.Fatalf("expected unsupported-args diagnostic on stderr, got %q", text)
	}
	if !strings.Contains(text, "extraarg") {
		t.Fatalf("expected unsupported argument name in stderr, got %q", text)
	}
}

func TestExecuteUnsupportedArgs(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &stderr

	if got := cmd.Execute([]string{"bogus"}); got != usageExitCode {
		t.Fatalf("expected usage exit code %d for unsupported args, got %d", usageExitCode, got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected unsupported args to keep stdout empty, got %q", stdout.String())
	}
	text := stderr.String()
	if !strings.Contains(text, "error: unsupported arguments") {
		t.Fatalf("expected unsupported-args diagnostic, got %q", text)
	}
	if !strings.Contains(text, "Usage:") {
		t.Fatalf("expected usage text for unsupported args, got %q", text)
	}
}
