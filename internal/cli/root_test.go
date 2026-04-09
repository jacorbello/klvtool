package cli

import (
	"bytes"
	"strings"
	"testing"

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
		})
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
