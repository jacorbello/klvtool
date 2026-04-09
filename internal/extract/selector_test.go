package extract

import (
	"errors"
	"testing"
)

func TestSelectBackendAutoPrefersGStreamerWhenHealthy(t *testing.T) {
	req := ExtractionRequest{
		Backend: BackendAuto,
		Backends: []BackendDescriptor{
			{Name: BackendFFmpeg, Healthy: true},
			{Name: BackendGStreamer, Healthy: true},
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

func TestSelectBackendAutoFailsWhenNoBackendIsHealthy(t *testing.T) {
	req := ExtractionRequest{
		Backend: BackendAuto,
		Backends: []BackendDescriptor{
			{Name: BackendGStreamer, Healthy: false},
			{Name: BackendFFmpeg, Healthy: false},
		},
	}

	resp, err := SelectBackend(req)
	if err == nil {
		t.Fatal("expected auto selection to fail when no backend is healthy")
	}
	if !errors.Is(err, ErrNoHealthyBackend) {
		t.Fatalf("expected ErrNoHealthyBackend, got %v", err)
	}
	if resp.Selected.Name != "" {
		t.Fatalf("expected no backend selection on error, got %q", resp.Selected.Name)
	}
}

func TestSelectBackendExplicitBackendDoesNotFallback(t *testing.T) {
	t.Run("gstreamer", func(t *testing.T) {
		req := ExtractionRequest{
			Backend: BackendGStreamer,
			Backends: []BackendDescriptor{
				{Name: BackendGStreamer, Healthy: false},
				{Name: BackendFFmpeg, Healthy: true},
			},
		}

		resp, err := SelectBackend(req)
		if err == nil {
			t.Fatal("expected explicit gstreamer selection to fail when gstreamer is unhealthy")
		}
		if !errors.Is(err, ErrBackendUnavailable) {
			t.Fatalf("expected ErrBackendUnavailable, got %v", err)
		}
		if resp.Selected.Name != "" {
			t.Fatalf("expected no backend selection on error, got %q", resp.Selected.Name)
		}
	})

	t.Run("ffmpeg", func(t *testing.T) {
		req := ExtractionRequest{
			Backend: BackendFFmpeg,
			Backends: []BackendDescriptor{
				{Name: BackendGStreamer, Healthy: true},
				{Name: BackendFFmpeg, Healthy: false},
			},
		}

		resp, err := SelectBackend(req)
		if err == nil {
			t.Fatal("expected explicit ffmpeg selection to fail when ffmpeg is unhealthy")
		}
		if !errors.Is(err, ErrBackendUnavailable) {
			t.Fatalf("expected ErrBackendUnavailable, got %v", err)
		}
		if resp.Selected.Name != "" {
			t.Fatalf("expected no backend selection on error, got %q", resp.Selected.Name)
		}
	})
}

func TestSelectBackendRejectsUnsupportedBackendRequests(t *testing.T) {
	req := ExtractionRequest{
		Backend: BackendName("v4l2"),
		Backends: []BackendDescriptor{
			{Name: BackendGStreamer, Healthy: true},
		},
	}

	resp, err := SelectBackend(req)
	if err == nil {
		t.Fatal("expected unsupported backend request to fail")
	}
	if !errors.Is(err, ErrUnsupportedBackend) {
		t.Fatalf("expected ErrUnsupportedBackend, got %v", err)
	}
	if resp.Selected.Name != "" {
		t.Fatalf("expected no backend selection on error, got %q", resp.Selected.Name)
	}
}

func TestSelectBackendNormalizesRequestAndBackendDescriptors(t *testing.T) {
	req := ExtractionRequest{
		Backend: BackendName("  AUTO "),
		Backends: []BackendDescriptor{
			{Name: BackendName(" ffmpeg "), Healthy: true},
			{Name: BackendName(" GStReAmEr "), Healthy: true},
		},
	}

	resp, err := SelectBackend(req)
	if err != nil {
		t.Fatalf("expected normalized auto selection to succeed, got error: %v", err)
	}
	if got, want := resp.Selected.Name, BackendGStreamer; got != want {
		t.Fatalf("expected normalized selection to prefer %q, got %q", want, got)
	}
	if got, want := len(resp.Backends), 2; got != want {
		t.Fatalf("expected 2 backend descriptors in response, got %d", got)
	}
	if got, want := resp.Backends[0].Name, BackendFFmpeg; got != want {
		t.Fatalf("expected first backend to normalize to %q, got %q", want, got)
	}
	if got, want := resp.Backends[1].Name, BackendGStreamer; got != want {
		t.Fatalf("expected second backend to normalize to %q, got %q", want, got)
	}
}
