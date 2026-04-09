package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
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

func TestExecuteEmptyArgs(t *testing.T) {
	if got := NewRootCommand().Execute(nil); got != 0 {
		t.Fatalf("expected success exit code for empty args, got %d", got)
	}
}

func TestMainEmptyArgs(t *testing.T) {
	origArgs := os.Args
	t.Cleanup(func() {
		os.Args = origArgs
	})

	os.Args = []string{"klvtool"}
	if got := Main(); got != 0 {
		t.Fatalf("expected Main to succeed for empty args, got %d", got)
	}
}

func TestHelpArgs(t *testing.T) {
	for _, arg := range []string{"--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			var out bytes.Buffer
			cmd := NewRootCommand()
			cmd.Out = &out
			cmd.Err = &out

			if got := cmd.Execute([]string{arg}); got != 0 {
				t.Fatalf("expected help exit code 0, got %d", got)
			}
			text := out.String()
			if !strings.Contains(text, "klvtool") {
				t.Fatalf("expected help text to include klvtool, got %q", text)
			}
			if !strings.Contains(text, "Usage:") {
				t.Fatalf("expected help text to include usage text, got %q", text)
			}
		})
	}
}

func TestExecuteUnsupportedArgs(t *testing.T) {
	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.Out = &out
	cmd.Err = &out

	if got := cmd.Execute([]string{"bogus"}); got != usageExitCode {
		t.Fatalf("expected usage exit code %d for unsupported args, got %d", usageExitCode, got)
	}
	text := out.String()
	if !strings.Contains(text, "error: unsupported arguments") {
		t.Fatalf("expected unsupported-args diagnostic, got %q", text)
	}
	if !strings.Contains(text, "Usage:") {
		t.Fatalf("expected usage text for unsupported args, got %q", text)
	}
}
