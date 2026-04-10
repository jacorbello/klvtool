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
		case "ffmpeg", "ffprobe", "gst-launch-1.0", "gst-inspect-1.0", "gst-discoverer-1.0":
			return "/usr/bin/" + name, nil
		default:
			return "", errors.New("missing")
		}
	}
	runVersion := func(ctx context.Context, name string, args ...string) (string, error) {
		if name == "/usr/bin/gst-inspect-1.0" && len(args) == 1 && args[0] == "tsdemux" {
			return "Plugin Details:\nName\ttsdemux", nil
		}
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
	if got, want := ffmpeg.Tools[0].Version, "/usr/bin/ffmpeg 1.2.3"; got != want {
		t.Fatalf("expected ffmpeg version %q, got %q", want, got)
	}

	gstreamer := report.BackendsByName()["gstreamer"]
	if gstreamer == nil {
		t.Fatal("expected gstreamer backend report")
	}
	if !gstreamer.Healthy {
		t.Fatal("expected gstreamer backend to be healthy")
	}
	if got, want := len(gstreamer.Tools), 3; got != want {
		t.Fatalf("expected %d gstreamer tools, got %d", want, got)
	}
	if got, want := gstreamer.Tools[2].Name, "gst-discoverer-1.0"; got != want {
		t.Fatalf("expected third gstreamer tool name %q, got %q", want, got)
	}
	if got, want := len(gstreamer.Modules), 1; got != want {
		t.Fatalf("expected %d gstreamer modules, got %d", want, got)
	}
	if got, want := gstreamer.Modules[0].Name, "tsdemux"; got != want {
		t.Fatalf("expected required gstreamer module %q, got %q", want, got)
	}
	if !gstreamer.Modules[0].Healthy {
		t.Fatal("expected required gstreamer module to be healthy")
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
	if got := gstreamer.MissingTools; len(got) != 3 {
		t.Fatalf("expected 3 missing gstreamer tools, got %d", len(got))
	}
	if got := gstreamer.MissingModules; len(got) != 1 {
		t.Fatalf("expected 1 missing gstreamer module, got %d", len(got))
	}
}

func TestDetectUsesResolvedPathForVersionProbe(t *testing.T) {
	var probed []string
	var probedArgs [][]string
	lookPath := func(name string) (string, error) {
		return "/opt/bin/" + name, nil
	}
	runVersion := func(ctx context.Context, name string, args ...string) (string, error) {
		probed = append(probed, name)
		probedArgs = append(probedArgs, append([]string(nil), args...))
		return name + " 9.0", nil
	}

	report := Detect(context.Background(), "linux", map[string]string{"ID": "ubuntu"}, lookPath, runVersion)
	ffmpeg := report.BackendsByName()["ffmpeg"]
	if ffmpeg == nil {
		t.Fatal("expected ffmpeg backend report")
	}
	if got, want := probed[0], "/opt/bin/ffmpeg"; got != want {
		t.Fatalf("expected version probe to use resolved path %q, got %q", want, got)
	}
	if got, want := strings.Join(probedArgs[0], " "), "-version"; got != want {
		t.Fatalf("expected ffmpeg version probe args %q, got %q", want, got)
	}
	if got, want := ffmpeg.Tools[0].Path, "/opt/bin/ffmpeg"; got != want {
		t.Fatalf("expected resolved tool path %q, got %q", want, got)
	}
}

func TestDetectReportsMissingGstreamerModuleWhenInspectSucceeds(t *testing.T) {
	lookPath := func(name string) (string, error) {
		switch name {
		case "gst-launch-1.0", "gst-inspect-1.0", "gst-discoverer-1.0":
			return "/usr/bin/" + name, nil
		default:
			return "", errors.New("missing")
		}
	}
	runVersion := func(ctx context.Context, name string, args ...string) (string, error) {
		if name == "/usr/bin/gst-inspect-1.0" && len(args) == 1 && args[0] == "tsdemux" {
			return "", errors.New("No such element or plugin 'tsdemux'")
		}
		return name + " 1.2.3", nil
	}

	report := Detect(context.Background(), "linux", nil, lookPath, runVersion)
	gstreamer := report.BackendsByName()["gstreamer"]
	if gstreamer == nil {
		t.Fatal("expected gstreamer backend report")
	}
	if gstreamer.Healthy {
		t.Fatal("expected gstreamer backend to be unhealthy when module is missing")
	}
	if got, want := len(gstreamer.MissingModules), 1; got != want {
		t.Fatalf("expected %d missing gstreamer module, got %d", want, got)
	}
	if got, want := gstreamer.MissingModules[0], "tsdemux"; got != want {
		t.Fatalf("expected missing module %q, got %q", want, got)
	}
	if got, want := gstreamer.Modules[0].Error, "No such element or plugin 'tsdemux'"; got != want {
		t.Fatalf("expected module error %q, got %q", want, got)
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

func TestDetectGenericLinuxDoesNotAssumeDebianUbuntu(t *testing.T) {
	guidance := installGuidance("linux", nil, func() (string, error) {
		return "NAME=\"Arch Linux\"\nID=arch\n", nil
	})

	if guidance.Platform != "unsupported" {
		t.Fatalf("expected unsupported platform for generic linux, got %q", guidance.Platform)
	}
	if containsString(guidance.Steps, "apt install") {
		t.Fatalf("expected generic linux guidance to avoid apt instructions, got %v", guidance.Steps)
	}
}

func TestInstallGuidanceUsesOSReleaseEvidence(t *testing.T) {
	guidance := installGuidance("linux", nil, func() (string, error) {
		return "NAME=\"Ubuntu\"\nID=ubuntu\nID_LIKE=debian\n", nil
	})

	if guidance.Platform != "debian_ubuntu" {
		t.Fatalf("expected debian_ubuntu platform, got %q", guidance.Platform)
	}
	if !containsString(guidance.Steps, "apt install ffmpeg gstreamer1.0-tools") {
		t.Fatalf("expected apt guidance, got %v", guidance.Steps)
	}
}

func TestInstallGuidanceUsesOSReleaseEvidenceForWSL(t *testing.T) {
	guidance := installGuidance("linux", map[string]string{"WSL_INTEROP": "1"}, func() (string, error) {
		return "NAME=\"Ubuntu\"\nID=ubuntu\nID_LIKE=debian\n", nil
	})

	if guidance.Platform != "wsl" {
		t.Fatalf("expected wsl platform, got %q", guidance.Platform)
	}
	if !containsString(guidance.Steps, "apt install ffmpeg gstreamer1.0-tools") {
		t.Fatalf("expected apt guidance, got %v", guidance.Steps)
	}
}

func TestInstallGuidanceDoesNotUseAptForWSLWithoutOSReleaseEvidence(t *testing.T) {
	guidance := installGuidance("linux", map[string]string{"WSL_INTEROP": "1"}, func() (string, error) {
		return "NAME=\"Arch Linux\"\nID=arch\n", nil
	})

	if guidance.Platform != "unsupported" {
		t.Fatalf("expected unsupported platform, got %q", guidance.Platform)
	}
	if containsString(guidance.Steps, "apt install") {
		t.Fatalf("expected guidance to avoid apt instructions, got %v", guidance.Steps)
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
		if name == "/usr/bin/ffmpeg" {
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
		name        string
		goos        string
		env         map[string]string
		osRelease   string
		contains    string
		notContains string
	}{
		{name: "macos", goos: "darwin", contains: "brew install ffmpeg gstreamer"},
		{name: "wsl with distro evidence", goos: "linux", env: map[string]string{"WSL_INTEROP": "1"}, osRelease: "NAME=\"Ubuntu\"\nID=ubuntu\nID_LIKE=debian\n", contains: "apt install ffmpeg gstreamer1.0-tools"},
		{name: "wsl without distro evidence", goos: "linux", env: map[string]string{"WSL_INTEROP": "1"}, osRelease: "NAME=\"Arch Linux\"\nID=arch\n", contains: "native package manager", notContains: "apt install"},
		{name: "unsupported", goos: "windows", contains: "native package manager"},
		{name: "debian ubuntu", goos: "linux", osRelease: "NAME=\"Ubuntu\"\nID=ubuntu\nID_LIKE=debian\n", contains: "apt install ffmpeg gstreamer1.0-tools"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guidance := InstallGuidance(tt.goos, tt.env)
			if tt.osRelease != "" {
				guidance = installGuidance(tt.goos, tt.env, func() (string, error) {
					return tt.osRelease, nil
				})
			}
			if len(guidance.Steps) == 0 {
				t.Fatal("expected install guidance steps")
			}
			if !containsString(guidance.Steps, tt.contains) {
				t.Fatalf("expected guidance to contain %q, got %v", tt.contains, guidance.Steps)
			}
			if tt.notContains != "" && containsString(guidance.Steps, tt.notContains) {
				t.Fatalf("expected guidance to avoid %q, got %v", tt.notContains, guidance.Steps)
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
