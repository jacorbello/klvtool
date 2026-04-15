package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestSpinnerWritesFrames(t *testing.T) {
	var buf bytes.Buffer
	stop := startSpinner(&buf, newColorizer(false), true, "working...")
	// Let at least one frame render.
	time.Sleep(150 * time.Millisecond)
	stop()

	text := buf.String()
	if !strings.Contains(text, "working...") {
		t.Errorf("expected spinner message, got %q", text)
	}
}

func TestSpinnerClearsOnStop(t *testing.T) {
	var buf bytes.Buffer
	stop := startSpinner(&buf, newColorizer(false), true, "loading")
	time.Sleep(150 * time.Millisecond)
	stop()

	// After stop, the last thing written should be a clear line (spaces + \r).
	text := buf.String()
	if !strings.HasSuffix(text, "\r") {
		t.Errorf("expected spinner to end with carriage return, got %q", text)
	}
}

func TestSpinnerNoopInRawMode(t *testing.T) {
	var buf bytes.Buffer
	stop := startSpinner(&buf, newColorizer(false), false, "should not appear")
	time.Sleep(100 * time.Millisecond)
	stop()

	if buf.Len() != 0 {
		t.Errorf("expected no output in raw mode, got %q", buf.String())
	}
}

func TestSpinnerStopIdempotent(t *testing.T) {
	var buf bytes.Buffer
	stop := startSpinner(&buf, newColorizer(false), true, "test")
	time.Sleep(100 * time.Millisecond)
	stop()
	stop() // should not panic
}

func TestSpinnerNilWriter(t *testing.T) {
	stop := startSpinner(nil, newColorizer(false), true, "test")
	stop() // should not panic
}
