package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jacorbello/klvtool/internal/envcheck"
)

func TestDoctorCommandRuns(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &stderr
	cmd.Doctor.IsTerminal = func() bool { return false }
	cmd.Doctor.Detect = func(ctx context.Context, goos string, env map[string]string) envcheck.Report {
		return envcheck.Report{
			Platform:        "linux",
			GuidanceSummary: "Install the backend tools with apt.",
			Guidance: []string{
				"sudo apt update && sudo apt install ffmpeg",
			},
			Backends: []envcheck.BackendHealth{
				{
					Name:    "ffmpeg",
					Healthy: true,
					Tools: []envcheck.ToolHealth{
						{
							Name:    "ffmpeg",
							Path:    "/usr/bin/ffmpeg",
							Version: "ffmpeg version 7.1 Copyright (c) 2000-2024 the FFmpeg developers",
							Healthy: true,
						},
						{
							Name:    "ffprobe",
							Path:    "/usr/bin/ffprobe",
							Version: "ffprobe version 7.1 Copyright (c) 2007-2024 the FFmpeg developers",
							Healthy: true,
						},
					},
				},
			},
		}
	}

	if got := cmd.Execute([]string{"doctor"}); got != 0 {
		t.Fatalf("expected exit code 0, got %d\nstderr: %s", got, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	text := stdout.String()

	// Header assertions
	for _, want := range []string{
		"klvtool: ",
		"platform: linux",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing header line %q in output:\n%s", want, text)
		}
	}
	if strings.Contains(text, "install guidance") {
		t.Errorf("expected no install guidance when all backends healthy, got:\n%s", text)
	}

	// Healthy backend assertions
	for _, want := range []string{
		"ffmpeg \xe2\x9c\x93 available",
		"ffmpeg              7.1",
		"/usr/bin/ffmpeg",
		"ffprobe             7.1",
		"/usr/bin/ffprobe",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing healthy backend line %q in output:\n%s", want, text)
		}
	}

	// Must NOT contain raw version banners
	if strings.Contains(text, "Copyright") {
		t.Errorf("output should not contain raw version banner, got:\n%s", text)
	}
}

func TestDoctorCommandColorizesWhenTerminal(t *testing.T) {
	var stdout bytes.Buffer

	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &bytes.Buffer{}
	cmd.Doctor.Env = map[string]string{}
	cmd.Doctor.IsTerminal = func() bool { return true }
	cmd.Doctor.Detect = func(ctx context.Context, goos string, env map[string]string) envcheck.Report {
		return envcheck.Report{
			Platform: "linux",
			Backends: []envcheck.BackendHealth{
				{
					Name:    "ffmpeg",
					Healthy: true,
					Tools: []envcheck.ToolHealth{
						{Name: "ffmpeg", Path: "/usr/bin/ffmpeg", Version: "ffmpeg version 7.1", Healthy: true},
					},
				},
			},
		}
	}

	if got := cmd.Execute([]string{"doctor"}); got != 0 {
		t.Fatalf("expected exit code 0, got %d", got)
	}

	text := stdout.String()
	if !strings.Contains(text, "\033[32m") {
		t.Errorf("expected ANSI green codes in terminal output, got:\n%s", text)
	}
}

func TestDoctorCommandHonorsNilRootWriters(t *testing.T) {
	var staleStdout bytes.Buffer
	var staleStderr bytes.Buffer

	cmd := NewRootCommand()
	cmd.Out = nil
	cmd.Err = nil
	cmd.Doctor.Out = &staleStdout
	cmd.Doctor.Err = &staleStderr
	cmd.Doctor.Detect = func(ctx context.Context, goos string, env map[string]string) envcheck.Report {
		return envcheck.Report{
			Backends: []envcheck.BackendHealth{
				{
					Name:    "ffmpeg",
					Healthy: true,
				},
			},
		}
	}

	if got := cmd.Execute([]string{"doctor"}); got != 0 {
		t.Fatalf("expected doctor command to succeed, got exit code %d", got)
	}
	if staleStdout.Len() != 0 {
		t.Fatalf("expected nil root stdout to suppress doctor output, got %q", staleStdout.String())
	}
	if staleStderr.Len() != 0 {
		t.Fatalf("expected nil root stderr to suppress doctor errors, got %q", staleStderr.String())
	}
}

func TestDoctorCommandResyncsCachedWritersAcrossInvocations(t *testing.T) {
	var firstStdout bytes.Buffer
	var firstStderr bytes.Buffer
	var secondStdout bytes.Buffer
	var secondStderr bytes.Buffer

	cmd := NewRootCommand()
	cmd.Doctor.Detect = func(ctx context.Context, goos string, env map[string]string) envcheck.Report {
		return envcheck.Report{
			Backends: []envcheck.BackendHealth{
				{
					Name:    "ffmpeg",
					Healthy: true,
				},
			},
		}
	}

	cmd.Out = &firstStdout
	cmd.Err = &firstStderr
	if got := cmd.Execute([]string{"doctor"}); got != 0 {
		t.Fatalf("expected first doctor invocation to succeed, got exit code %d", got)
	}

	cmd.Out = &secondStdout
	cmd.Err = &secondStderr
	if got := cmd.Execute([]string{"doctor"}); got != 0 {
		t.Fatalf("expected second doctor invocation to succeed, got exit code %d", got)
	}

	if firstStdout.Len() == 0 {
		t.Fatal("expected first invocation to write to first stdout")
	}
	if firstStderr.Len() != 0 {
		t.Fatalf("expected first invocation to keep first stderr empty, got %q", firstStderr.String())
	}
	if secondStdout.Len() == 0 {
		t.Fatal("expected second invocation to write to updated stdout")
	}
	if secondStderr.Len() != 0 {
		t.Fatalf("expected second invocation to keep second stderr empty, got %q", secondStderr.String())
	}
	if secondStdout.String() != firstStdout.String() {
		t.Fatalf("expected doctor output to be stable across invocations, got first %q and second %q", firstStdout.String(), secondStdout.String())
	}
}

func TestDoctorHelpMixedWithFlags(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := &DoctorCommand{Out: &out, Err: &errBuf}
	code := cmd.Execute([]string{"--help", "--extra"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Errorf("expected usage on stdout, got %q", out.String())
	}
	if errBuf.Len() != 0 {
		t.Errorf("expected empty stderr, got %q", errBuf.String())
	}
}

func TestDoctorCommandHonorsNilRootStderrOnErrorPath(t *testing.T) {
	var staleStdout bytes.Buffer
	var staleStderr bytes.Buffer

	cmd := NewRootCommand()
	cmd.Out = &staleStdout
	cmd.Err = nil
	cmd.Doctor.Out = &staleStdout
	cmd.Doctor.Err = &staleStderr

	if got := cmd.Execute([]string{"doctor", "bogus"}); got != usageExitCode {
		t.Fatalf("expected doctor error path to return usage exit code %d, got %d", usageExitCode, got)
	}
	if staleStdout.Len() != 0 {
		t.Fatalf("expected nil root stderr to suppress doctor stdout, got %q", staleStdout.String())
	}
	if staleStderr.Len() != 0 {
		t.Fatalf("expected nil root stderr to suppress doctor error output, got %q", staleStderr.String())
	}
}

func TestDoctorCommandResyncsCachedStderrAcrossInvocations(t *testing.T) {
	var firstStderr bytes.Buffer
	var secondStderr bytes.Buffer

	cmd := NewRootCommand()
	cmd.Out = nil
	cmd.Doctor.Out = nil
	cmd.Doctor.Detect = func(ctx context.Context, goos string, env map[string]string) envcheck.Report {
		t.Fatal("detect should not run for unsupported doctor arguments")
		return envcheck.Report{}
	}

	cmd.Err = &firstStderr
	if got := cmd.Execute([]string{"doctor", "bogus"}); got != usageExitCode {
		t.Fatalf("expected first doctor error path to return usage exit code %d, got %d", usageExitCode, got)
	}

	cmd.Err = &secondStderr
	if got := cmd.Execute([]string{"doctor", "bogus"}); got != usageExitCode {
		t.Fatalf("expected second doctor error path to return usage exit code %d, got %d", usageExitCode, got)
	}

	if firstStderr.Len() == 0 {
		t.Fatal("expected first invocation to write doctor error output")
	}
	if secondStderr.Len() == 0 {
		t.Fatal("expected second invocation to write doctor error output to updated stderr")
	}
	if firstStderr.String() != secondStderr.String() {
		t.Fatalf("expected doctor error output to stay stable across invocations, got first %q and second %q", firstStderr.String(), secondStderr.String())
	}
}

func TestDoctorCommandShowsUnhealthyWhenToolsInstalledButFailing(t *testing.T) {
	var stdout bytes.Buffer

	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &bytes.Buffer{}
	cmd.Doctor.IsTerminal = func() bool { return false }
	cmd.Doctor.Detect = func(ctx context.Context, goos string, env map[string]string) envcheck.Report {
		return envcheck.Report{
			Platform: "linux",
			Guidance: []string{"sudo apt install ffmpeg"},
			Backends: []envcheck.BackendHealth{
				{
					Name:    "ffmpeg",
					Healthy: false,
					Tools: []envcheck.ToolHealth{
						{Name: "ffmpeg", Path: "/usr/bin/ffmpeg", Error: "exec format error", Healthy: false},
						{Name: "ffprobe", Path: "/usr/bin/ffprobe", Version: "7.1", Healthy: true},
					},
				},
			},
		}
	}

	if got := cmd.Execute([]string{"doctor"}); got != 1 {
		t.Fatalf("expected exit code 1 for unhealthy backend, got %d", got)
	}

	text := stdout.String()
	if !strings.Contains(text, "unhealthy") {
		t.Errorf("expected 'unhealthy' label when tools installed but failing, got:\n%s", text)
	}
	if strings.Contains(text, "not installed") {
		t.Errorf("expected no 'not installed' label when tools are present, got:\n%s", text)
	}
	if !strings.Contains(text, "ffmpeg:") || !strings.Contains(text, "exec format error") {
		t.Errorf("expected tool error details in output, got:\n%s", text)
	}
	if strings.Contains(text, "install:") {
		t.Errorf("expected no install guidance for unhealthy (not missing) backend, got:\n%s", text)
	}
}

func TestDoctorCommandExitsNonZeroWhenBackendMissing(t *testing.T) {
	var stdout bytes.Buffer

	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &bytes.Buffer{}
	cmd.Doctor.IsTerminal = func() bool { return false }
	cmd.Doctor.Detect = func(ctx context.Context, goos string, env map[string]string) envcheck.Report {
		return envcheck.Report{
			Platform: "linux",
			Guidance: []string{"sudo apt install ffmpeg"},
			Backends: []envcheck.BackendHealth{
				{
					Name:         "ffmpeg",
					Healthy:      false,
					MissingTools: []string{"ffmpeg", "ffprobe"},
				},
			},
		}
	}

	if got := cmd.Execute([]string{"doctor"}); got != 1 {
		t.Fatalf("expected exit code 1 for missing backend, got %d", got)
	}

	text := stdout.String()
	if !strings.Contains(text, "not installed") {
		t.Errorf("expected 'not installed' label, got:\n%s", text)
	}
}

func TestDoctorCommandShowsGuidanceWhenUnhealthy(t *testing.T) {
	var stdout bytes.Buffer

	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &bytes.Buffer{}
	cmd.Doctor.IsTerminal = func() bool { return false }
	cmd.Doctor.Detect = func(ctx context.Context, goos string, env map[string]string) envcheck.Report {
		return envcheck.Report{
			Platform:        "linux",
			GuidanceSummary: "Install the backend tools with apt.",
			Guidance:        []string{"sudo apt update && sudo apt install ffmpeg"},
			Backends: []envcheck.BackendHealth{
				{Name: "ffmpeg", Healthy: false, MissingTools: []string{"ffmpeg", "ffprobe"}},
			},
		}
	}

	if got := cmd.Execute([]string{"doctor"}); got != 1 {
		t.Fatalf("expected exit code 1 for unhealthy backend, got %d", got)
	}

	text := stdout.String()
	if !strings.Contains(text, "install guidance: Install the backend tools with apt.") {
		t.Errorf("expected install guidance when backend unhealthy, got:\n%s", text)
	}
}

func TestDoctorCommandRespectsNoColor(t *testing.T) {
	var stdout bytes.Buffer

	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &bytes.Buffer{}
	cmd.Doctor.IsTerminal = func() bool { return true }
	cmd.Doctor.Env = map[string]string{"NO_COLOR": "1"}
	cmd.Doctor.Detect = func(ctx context.Context, goos string, env map[string]string) envcheck.Report {
		return envcheck.Report{
			Platform: "linux",
			Backends: []envcheck.BackendHealth{
				{
					Name:    "ffmpeg",
					Healthy: true,
					Tools: []envcheck.ToolHealth{
						{Name: "ffmpeg", Path: "/usr/bin/ffmpeg", Version: "ffmpeg version 7.1", Healthy: true},
					},
				},
			},
		}
	}

	if got := cmd.Execute([]string{"doctor"}); got != 0 {
		t.Fatalf("expected exit code 0, got %d", got)
	}

	text := stdout.String()
	if strings.Contains(text, "\033[") {
		t.Errorf("expected no ANSI codes with NO_COLOR set, got:\n%s", text)
	}
}
