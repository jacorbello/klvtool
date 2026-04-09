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
	cmd.Doctor = &DoctorCommand{
		Out: &stdout,
		Err: &stderr,
		Detect: func(ctx context.Context, goos string, env map[string]string) envcheck.Report {
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
		},
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
