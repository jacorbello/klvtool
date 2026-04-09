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
	cmd.Doctor.Detect = func(ctx context.Context, goos string, env map[string]string) envcheck.Report {
		return envcheck.Report{
			Platform:        "linux",
			GuidanceSummary: "Install the backend tools with apt.",
			Guidance: []string{
				"sudo apt update && sudo apt install ffmpeg gstreamer1.0-tools",
			},
			Backends: []envcheck.BackendHealth{
				{
					Name:    "ffmpeg",
					Healthy: true,
					Tools: []envcheck.ToolHealth{
						{
							Name:    "ffmpeg",
							Path:    "/usr/bin/ffmpeg",
							Version: "ffmpeg version 7.1",
							Healthy: true,
						},
						{
							Name:    "ffprobe",
							Path:    "/usr/bin/ffprobe",
							Version: "ffprobe version 7.1",
							Healthy: true,
						},
					},
				},
				{
					Name:         "gstreamer",
					Healthy:      false,
					MissingTools: []string{"gst-launch-1.0", "gst-inspect-1.0"},
					Tools: []envcheck.ToolHealth{
						{
							Name:  "gst-launch-1.0",
							Error: "missing",
						},
						{
							Name:  "gst-inspect-1.0",
							Error: "missing",
						},
					},
				},
			},
		}
	}

	if got := cmd.Execute([]string{"doctor"}); got != 0 {
		t.Fatalf("expected doctor command to succeed, got exit code %d", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected doctor command to keep stderr empty, got %q", stderr.String())
	}

	text := stdout.String()
	for _, want := range []string{
		"backend resolution preference: auto",
		"ffmpeg: available",
		"version: ffmpeg version 7.1",
		"gstreamer: unavailable",
		"install guidance: Install the backend tools with apt.",
		"install: sudo apt update && sudo apt install ffmpeg gstreamer1.0-tools",
		"ffmpeg:",
		"gstreamer:",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected doctor output to contain %q, got %q", want, text)
		}
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
