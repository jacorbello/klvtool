package integration

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jacorbello/klvtool/internal/cli"
)

func TestDoctorIntegration(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := cli.NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &stderr

	if got := cmd.Execute([]string{"doctor"}); got != 0 {
		t.Fatalf("expected doctor command to succeed, got %d with stderr %q", got, stderr.String())
	}

	text := stdout.String()
	for _, want := range []string{"ffmpeg", "gstreamer", "backend resolution preference: auto"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected doctor output to contain %q, got %q", want, text)
		}
	}
}
