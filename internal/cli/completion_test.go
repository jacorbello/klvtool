package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompletionBash(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := &CompletionCommand{Out: &out, Err: &errBuf}

	code := cmd.Execute([]string{"bash"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}

	text := out.String()
	if !strings.Contains(text, "complete -F") {
		t.Errorf("expected bash complete -F directive, got %q", text)
	}
	if !strings.Contains(text, "_klvtool") {
		t.Errorf("expected _klvtool function name, got %q", text)
	}
	for _, cmd := range []string{"version", "update", "doctor", "inspect", "extract", "decode", "packetize", "diagnose", "completion"} {
		if !strings.Contains(text, cmd) {
			t.Errorf("expected command %q in bash completions", cmd)
		}
	}
	for _, flag := range []string{"--input", "--format", "--view", "--raw", "--strict", "--step", "--pid", "--out", "--schema", "--mode", "--check"} {
		if !strings.Contains(text, flag) {
			t.Errorf("expected flag %q in bash completions", flag)
		}
	}
}

func TestCompletionZsh(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := &CompletionCommand{Out: &out, Err: &errBuf}

	code := cmd.Execute([]string{"zsh"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}

	text := out.String()
	if !strings.Contains(text, "compdef") {
		t.Errorf("expected zsh compdef directive, got %q", text)
	}
	for _, cmd := range []string{"version", "update", "doctor", "inspect", "extract", "decode", "packetize", "diagnose", "completion"} {
		if !strings.Contains(text, cmd) {
			t.Errorf("expected command %q in zsh completions", cmd)
		}
	}
}

func TestCompletionFish(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := &CompletionCommand{Out: &out, Err: &errBuf}

	code := cmd.Execute([]string{"fish"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}

	text := out.String()
	if !strings.Contains(text, "complete -c klvtool") {
		t.Errorf("expected fish complete -c directive, got %q", text)
	}
	for _, cmd := range []string{"version", "update", "doctor", "inspect", "extract", "decode", "packetize", "diagnose", "completion"} {
		if !strings.Contains(text, cmd) {
			t.Errorf("expected command %q in fish completions", cmd)
		}
	}
}

func TestCompletionMissingShell(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := &CompletionCommand{Out: &out, Err: &errBuf}

	code := cmd.Execute(nil)
	if code != usageExitCode {
		t.Fatalf("exit code = %d, want %d", code, usageExitCode)
	}
	if !strings.Contains(errBuf.String(), "shell argument required") {
		t.Errorf("expected shell-required error, got %q", errBuf.String())
	}
}

func TestCompletionInvalidShell(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := &CompletionCommand{Out: &out, Err: &errBuf}

	code := cmd.Execute([]string{"powershell"})
	if code != usageExitCode {
		t.Fatalf("exit code = %d, want %d", code, usageExitCode)
	}
	if !strings.Contains(errBuf.String(), "unsupported shell") {
		t.Errorf("expected unsupported-shell error, got %q", errBuf.String())
	}
}

func TestCompletionHelp(t *testing.T) {
	for _, arg := range []string{"--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			var out, errBuf bytes.Buffer
			cmd := &CompletionCommand{Out: &out, Err: &errBuf}

			code := cmd.Execute([]string{arg})
			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			text := out.String()
			if !strings.Contains(text, "Usage:") {
				t.Errorf("expected usage text, got %q", text)
			}
		})
	}
}

func TestCompletionNilCommand(t *testing.T) {
	var cmd *CompletionCommand
	if code := cmd.Execute(nil); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestCompletionExtraArgs(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := &CompletionCommand{Out: &out, Err: &errBuf}

	code := cmd.Execute([]string{"bash", "extra"})
	if code != usageExitCode {
		t.Fatalf("exit code = %d, want %d", code, usageExitCode)
	}
	if !strings.Contains(errBuf.String(), "unsupported arguments") {
		t.Errorf("expected unsupported-args error, got %q", errBuf.String())
	}
}
