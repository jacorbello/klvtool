package envcheck

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
)

func TestDetectBackendsReportsToolHealth(t *testing.T) {
	lookPath := func(name string) (string, error) {
		switch name {
		case "ffmpeg", "ffprobe", "gst-launch-1.0", "gst-inspect-1.0":
			return "/usr/bin/" + name, nil
		default:
			return "", errors.New("missing")
		}
	}
	runVersion := func(ctx context.Context, name string, args ...string) (string, error) {
		return name + " 1.2.3", nil
	}

	report := Detect(context.Background(), runtime.GOOS, nil, lookPath, runVersion)

	if len(report.Backends) != 2 {
		t.Fatalf("expected 2 backends, got %d", len(report.Backends))
	}

	ffmpeg := report.BackendsByName()["ffmpeg"]
	if ffmpeg == nil {
		t.Fatal("expected ffmpeg backend report")
	}
	if !ffmpeg.Healthy {
		t.Fatal("expected ffmpeg backend to be healthy")
	}
	if got, want := len(ffmpeg.Tools), 2; got != want {
		t.Fatalf("expected %d ffmpeg tools, got %d", want, got)
	}
	if got, want := ffmpeg.Tools[0].Name, "ffmpeg"; got != want {
		t.Fatalf("expected first ffmpeg tool name %q, got %q", want, got)
	}
	if got, want := ffmpeg.Tools[0].Version, "ffmpeg 1.2.3"; got != want {
		t.Fatalf("expected ffmpeg version %q, got %q", want, got)
	}

	gstreamer := report.BackendsByName()["gstreamer"]
	if gstreamer == nil {
		t.Fatal("expected gstreamer backend report")
	}
	if !gstreamer.Healthy {
		t.Fatal("expected gstreamer backend to be healthy")
	}
}

func TestDetectBackendsReportsMissingTools(t *testing.T) {
	lookPath := func(name string) (string, error) {
		switch name {
		case "ffmpeg", "ffprobe":
			return "/usr/bin/" + name, nil
		default:
			return "", errors.New("missing")
		}
	}
	runVersion := func(ctx context.Context, name string, args ...string) (string, error) {
		return name + " 7.1", nil
	}

	report := Detect(context.Background(), "linux", nil, lookPath, runVersion)
	gstreamer := report.BackendsByName()["gstreamer"]
	if gstreamer == nil {
		t.Fatal("expected gstreamer backend report")
	}
	if gstreamer.Healthy {
		t.Fatal("expected gstreamer backend to be unhealthy")
	}
	if got := gstreamer.MissingTools; len(got) != 2 {
		t.Fatalf("expected 2 missing gstreamer tools, got %d", len(got))
	}
}

func TestDetectPreservesGuidanceSummaryForUnsupportedOS(t *testing.T) {
	report := Detect(context.Background(), "windows", nil, func(name string) (string, error) {
		return "", errors.New("unexpected")
	}, func(ctx context.Context, name string, args ...string) (string, error) {
		return "", errors.New("unexpected")
	})

	if report.Platform != "unsupported" {
		t.Fatalf("expected unsupported platform, got %q", report.Platform)
	}
	if report.GuidanceSummary == "" {
		t.Fatal("expected guidance summary to be preserved")
	}
	if containsString(report.Guidance, "apt install") {
		t.Fatalf("expected unsupported guidance to avoid apt instructions, got %v", report.Guidance)
	}
}

func TestDetectReportsVersionFailureWhenToolIsInstalled(t *testing.T) {
	lookPath := func(name string) (string, error) {
		switch name {
		case "ffmpeg", "ffprobe":
			return "/usr/bin/" + name, nil
		default:
			return "", errors.New("missing")
		}
	}
	runVersion := func(ctx context.Context, name string, args ...string) (string, error) {
		if name == "ffmpeg" {
			return "", errors.New("exec failed")
		}
		return name + " 7.1", nil
	}

	report := Detect(context.Background(), "linux", nil, lookPath, runVersion)
	ffmpeg := report.BackendsByName()["ffmpeg"]
	if ffmpeg == nil {
		t.Fatal("expected ffmpeg backend report")
	}
	if ffmpeg.Healthy {
		t.Fatal("expected ffmpeg backend to be unhealthy")
	}
	if len(ffmpeg.Tools) == 0 {
		t.Fatal("expected ffmpeg tools to be reported")
	}
	if ffmpeg.Tools[0].Path != "/usr/bin/ffmpeg" {
		t.Fatalf("expected installed tool path to be preserved, got %q", ffmpeg.Tools[0].Path)
	}
	if ffmpeg.Tools[0].Healthy {
		t.Fatal("expected version failure to mark tool unhealthy")
	}
	if ffmpeg.Tools[0].Error == "" {
		t.Fatal("expected version failure error to be recorded")
	}
}

func TestPlatformGuidance(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		env      map[string]string
		contains string
	}{
		{name: "macos", goos: "darwin", contains: "brew install ffmpeg gstreamer"},
		{name: "debian ubuntu", goos: "linux", contains: "apt install ffmpeg gstreamer1.0-tools"},
		{name: "wsl", goos: "linux", env: map[string]string{"WSL_DISTRO_NAME": "Ubuntu"}, contains: "WSL"},
		{name: "unsupported", goos: "windows", contains: "native package manager"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guidance := InstallGuidance(tt.goos, tt.env)
			if len(guidance.Steps) == 0 {
				t.Fatal("expected install guidance steps")
			}
			if !containsString(guidance.Steps, tt.contains) {
				t.Fatalf("expected guidance to contain %q, got %v", tt.contains, guidance.Steps)
			}
		})
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if strings.Contains(item, want) {
			return true
		}
	}
	return false
}
