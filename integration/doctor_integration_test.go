package integration

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/jacorbello/klvtool/internal/cli"
)

func TestDoctorIntegration(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available: %v", tool, err)
		}
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := cli.NewRootCommand()
	cmd.Out = &stdout
	cmd.Err = &stderr

	if got := cmd.Execute([]string{"doctor"}); got != 0 {
		t.Fatalf("expected doctor command to succeed, got %d with stderr %q", got, stderr.String())
	}

	text := stdout.String()
	for _, want := range []string{"ffmpeg"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected doctor output to contain %q, got %q", want, text)
		}
	}
}
