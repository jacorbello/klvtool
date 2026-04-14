package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVersionPrintsVersion(t *testing.T) {
	var stdout bytes.Buffer
	cmd := NewRootCommand()
	cmd.Out = &stdout

	if got := cmd.Execute([]string{"version"}); got != 0 {
		t.Fatalf("expected exit 0, got %d", got)
	}
	if !strings.Contains(stdout.String(), "klvtool "+cmd.Version) {
		t.Fatalf("expected output to contain version, got %q", stdout.String())
	}
}

func TestVersionCheckUpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{"tag_name": "v1.0.0"}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Version = "v1.0.0"
	cmd.VersionCmd.ReleaseURL = srv.URL

	if got := cmd.Execute([]string{"version", "--check"}); got != 0 {
		t.Fatalf("expected exit 0, got %d", got)
	}
	out := stdout.String()
	if !strings.Contains(out, "up to date") {
		t.Fatalf("expected 'up to date', got %q", out)
	}
}

func TestVersionCheckUpdateAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{
			"tag_name": "v2.0.0",
			"html_url": "https://github.com/jacorbello/klvtool/releases/tag/v2.0.0",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Version = "v1.0.0"
	cmd.VersionCmd.ReleaseURL = srv.URL

	if got := cmd.Execute([]string{"version", "--check"}); got != 0 {
		t.Fatalf("expected exit 0, got %d", got)
	}
	out := stdout.String()
	if !strings.Contains(out, "v2.0.0 available") {
		t.Fatalf("expected update available message, got %q", out)
	}
	if !strings.Contains(out, "https://github.com/jacorbello/klvtool/releases/tag/v2.0.0") {
		t.Fatalf("expected release URL in output, got %q", out)
	}
}

func TestVersionCheckAPIFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Version = "v1.0.0"
	cmd.VersionCmd.ReleaseURL = srv.URL

	if got := cmd.Execute([]string{"version", "--check"}); got != 0 {
		t.Fatalf("expected exit 0 even on API failure, got %d", got)
	}
	out := stdout.String()
	if !strings.Contains(out, "update check failed") {
		t.Fatalf("expected failure message, got %q", out)
	}
}

func TestVersionCheckDevBuild(t *testing.T) {
	var stdout bytes.Buffer
	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Version = "dev"

	if got := cmd.Execute([]string{"version", "--check"}); got != 0 {
		t.Fatalf("expected exit 0, got %d", got)
	}
	out := stdout.String()
	if !strings.Contains(out, "update check skipped") {
		t.Fatalf("expected skip message for dev build, got %q", out)
	}
}

func TestVersionRejectsStrayArgs(t *testing.T) {
	var stderr bytes.Buffer
	cmd := NewRootCommand()
	cmd.Err = &stderr

	if got := cmd.Execute([]string{"version", "bogus"}); got != 2 {
		t.Fatalf("expected exit 2, got %d", got)
	}
	if !strings.Contains(stderr.String(), "unsupported arguments") {
		t.Fatalf("expected unsupported arguments error, got %q", stderr.String())
	}
}

func TestVersionHelpFlag(t *testing.T) {
	var stdout bytes.Buffer
	cmd := NewRootCommand()
	cmd.Out = &stdout

	if got := cmd.Execute([]string{"version", "--help"}); got != 0 {
		t.Fatalf("expected exit 0, got %d", got)
	}
	out := stdout.String()
	if !strings.Contains(out, "Usage:") {
		t.Fatalf("expected usage text, got %q", out)
	}
}
