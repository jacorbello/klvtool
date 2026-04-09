package cli

import (
	"os"
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

func TestExecuteUnsupportedArgs(t *testing.T) {
	if got := NewRootCommand().Execute([]string{"bogus"}); got == 0 {
		t.Fatal("expected non-zero exit code for unsupported args")
	}
}
