package extract

import (
	"testing"

	"github.com/jacorbello/klvtool/internal/envcheck"
)

func TestSelectBackendAutoPrefersGStreamerWhenHealthy(t *testing.T) {
	req := ExtractionRequest{
		Backend: BackendAuto,
		Report: envcheck.Report{
			Backends: []envcheck.BackendHealth{
				{Name: string(BackendFFmpeg), Healthy: true},
				{Name: string(BackendGStreamer), Healthy: true},
			},
		},
	}

	resp, err := SelectBackend(req)
	if err != nil {
		t.Fatalf("expected auto selection to succeed, got error: %v", err)
	}
	if got, want := resp.Selected.Name, BackendGStreamer; got != want {
		t.Fatalf("expected auto to prefer %q, got %q", want, got)
	}
}

func TestSelectBackendExplicitBackendDoesNotFallback(t *testing.T) {
	t.Run("gstreamer", func(t *testing.T) {
		req := ExtractionRequest{
			Backend: BackendGStreamer,
			Report: envcheck.Report{
				Backends: []envcheck.BackendHealth{
					{Name: string(BackendGStreamer), Healthy: false},
					{Name: string(BackendFFmpeg), Healthy: true},
				},
			},
		}

		resp, err := SelectBackend(req)
		if err == nil {
			t.Fatal("expected explicit gstreamer selection to fail when gstreamer is unhealthy")
		}
		if resp.Selected.Name != "" {
			t.Fatalf("expected no backend selection on error, got %q", resp.Selected.Name)
		}
	})

	t.Run("ffmpeg", func(t *testing.T) {
		req := ExtractionRequest{
			Backend: BackendFFmpeg,
			Report: envcheck.Report{
				Backends: []envcheck.BackendHealth{
					{Name: string(BackendGStreamer), Healthy: true},
					{Name: string(BackendFFmpeg), Healthy: false},
				},
			},
		}

		resp, err := SelectBackend(req)
		if err == nil {
			t.Fatal("expected explicit ffmpeg selection to fail when ffmpeg is unhealthy")
		}
		if resp.Selected.Name != "" {
			t.Fatalf("expected no backend selection on error, got %q", resp.Selected.Name)
		}
	})
}

