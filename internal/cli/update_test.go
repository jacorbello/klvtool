package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestUpdateSkipsDev(t *testing.T) {
	var stdout bytes.Buffer
	cmd := NewRootCommand()
	cmd.Out = &stdout
	cmd.Version = "dev"

	if got := cmd.Execute([]string{"update"}); got != 0 {
		t.Fatalf("exit %d", got)
	}
	if !strings.Contains(stdout.String(), "update skipped") {
		t.Fatalf("got %q", stdout.String())
	}
}

func TestUpdateUpToDate_NoGoRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v1.0.0",
			"assets":   []any{},
		})
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	root := NewRootCommand()
	root.Out = &stdout
	root.Version = "v1.0.0"
	root.Update.ReleaseURL = srv.URL
	root.Update.RunGo = func(ctx context.Context, goBin string, args []string) ([]byte, []byte, error) {
		t.Fatalf("go should not run when up to date")
		return nil, nil, nil
	}

	if got := root.Execute([]string{"update"}); got != 0 {
		t.Fatalf("exit %d", got)
	}
	if !strings.Contains(stdout.String(), "up to date") {
		t.Fatalf("got %q", stdout.String())
	}
}

func TestUpdateFetchFailureExitsOne(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	var stderr bytes.Buffer
	root := NewRootCommand()
	root.Err = &stderr
	root.Out = &bytes.Buffer{}
	root.Version = "v1.0.0"
	root.Update.ReleaseURL = srv.URL

	if got := root.Execute([]string{"update"}); got != 1 {
		t.Fatalf("exit %d, want1; stderr=%q", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "error:") {
		t.Fatalf("expected error on stderr, got %q", stderr.String())
	}
}

func TestUpdateGoInstallFailureExitsOne(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v2.0.0",
			"assets":   []any{},
		})
	}))
	defer srv.Close()

	var stderr bytes.Buffer
	root := NewRootCommand()
	root.Err = &stderr
	root.Out = &bytes.Buffer{}
	root.Version = "v1.0.0"
	root.Update.ReleaseURL = srv.URL
	root.Update.LookPath = func(string) (string, error) { return "/fake/go", nil }
	root.Update.RunGo = func(context.Context, string, []string) ([]byte, []byte, error) {
		return nil, []byte("compile failed"), errors.New("exit status 1")
	}

	if got := root.Execute([]string{"update"}); got != 1 {
		t.Fatalf("exit %d, want 1; stderr=%q", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "go install failed") {
		t.Fatalf("expected go install failure, got %q", stderr.String())
	}
}

func TestUpdateRunsGoInstallWhenGoPresent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v2.0.0",
			"assets":   []any{},
		})
	}))
	defer srv.Close()

	var stdout, stderr bytes.Buffer
	root := NewRootCommand()
	root.Out = &stdout
	root.Err = &stderr
	root.Version = "v1.0.0"
	root.Update.ReleaseURL = srv.URL
	root.Update.LookPath = func(file string) (string, error) {
		if file == "go" {
			return "/fake/go", nil
		}
		return "", fmt.Errorf("not found")
	}
	var sawGo []string
	root.Update.RunGo = func(ctx context.Context, goBin string, args []string) ([]byte, []byte, error) {
		sawGo = append([]string{goBin}, args...)
		return nil, nil, nil
	}

	if got := root.Execute([]string{"update"}); got != 0 {
		t.Fatalf("exit %d stderr=%s", got, stderr.String())
	}
	if len(sawGo) < 3 || sawGo[0] != "/fake/go" || sawGo[1] != "install" || sawGo[2] != "github.com/jacorbello/klvtool/cmd/klvtool@v2.0.0" {
		t.Fatalf("unexpected go invocation: %#v", sawGo)
	}
	if !strings.Contains(stdout.String(), "updated via go install") {
		t.Fatalf("got %q", stdout.String())
	}
}

func TestUpdateDryRunNoSideEffects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v9.0.0",
			"assets":   []any{},
		})
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	root := NewRootCommand()
	root.Out = &stdout
	root.Version = "v1.0.0"
	root.Update.ReleaseURL = srv.URL
	root.Update.LookPath = func(string) (string, error) { return "/go", nil }
	root.Update.RunGo = func(context.Context, string, []string) ([]byte, []byte, error) {
		t.Fatalf("dry-run must not invoke go")
		return nil, nil, nil
	}

	if got := root.Execute([]string{"update", "--dry-run"}); got != 0 {
		t.Fatalf("exit %d", got)
	}
	if !strings.Contains(stdout.String(), "would update") || !strings.Contains(stdout.String(), "strategy: go install") {
		t.Fatalf("got %q", stdout.String())
	}
}

func TestUpdateErrorsUseModelErrorCodes(t *testing.T) {
	var stderr bytes.Buffer
	root := NewRootCommand()
	root.Err = &stderr
	if got := root.Execute([]string{"update", "nope"}); got != usageExitCode {
		t.Fatalf("exit %d", got)
	}
	if !strings.Contains(stderr.String(), "invalid_usage") {
		t.Fatalf("got %q", stderr.String())
	}
}

func TestUpdateHelpFlag(t *testing.T) {
	var stdout bytes.Buffer
	cmd := NewRootCommand()
	cmd.Out = &stdout
	if got := cmd.Execute([]string{"update", "--help"}); got != 0 {
		t.Fatalf("exit %d", got)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("got %q", stdout.String())
	}
}

func TestUpdateBadFlagUsesInvalidUsage(t *testing.T) {
	var stderr strings.Builder
	root := NewRootCommand()
	root.Err = &stderr
	if got := root.Execute([]string{"update", "--nope"}); got != usageExitCode {
		t.Fatalf("exit %d", got)
	}
	if !strings.Contains(stderr.String(), "invalid_usage") {
		t.Fatalf("got %q", stderr.String())
	}
}

func TestUpdateUsesAdequateTimeout(t *testing.T) {
	// Verify the update command uses a timeout longer than 60s to handle
	// slow networks and go install builds.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v2.0.0",
			"assets":   []any{},
		})
	}))
	defer srv.Close()

	var capturedDeadline time.Time
	root := NewRootCommand()
	root.Out = &bytes.Buffer{}
	root.Err = &bytes.Buffer{}
	root.Version = "v1.0.0"
	root.Update.ReleaseURL = srv.URL
	root.Update.LookPath = func(string) (string, error) { return "/fake/go", nil }
	root.Update.RunGo = func(ctx context.Context, _ string, _ []string) ([]byte, []byte, error) {
		dl, ok := ctx.Deadline()
		if ok {
			capturedDeadline = dl
		}
		return nil, nil, nil
	}
	root.Update.Executable = func() (string, error) { return "/fake/go/bin/klvtool", nil }

	root.Execute([]string{"update"})

	if capturedDeadline.IsZero() {
		t.Fatal("expected context deadline to be set")
	}
	remaining := time.Until(capturedDeadline)
	// The timeout should be at least 4 minutes (allowing for test overhead).
	if remaining < 4*time.Minute {
		t.Fatalf("timeout too short: %v remaining", remaining)
	}
}

func TestUpdateGoInstallWarnsWhenExeOutsideGopath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v2.0.0",
			"assets":   []any{},
		})
	}))
	defer srv.Close()

	var stdout bytes.Buffer
	root := NewRootCommand()
	root.Out = &stdout
	root.Err = &bytes.Buffer{}
	root.Version = "v1.0.0"
	root.Update.ReleaseURL = srv.URL
	root.Update.LookPath = func(file string) (string, error) {
		if file == "go" {
			return "/usr/local/go/bin/go", nil
		}
		return "", fmt.Errorf("not found")
	}
	root.Update.Executable = func() (string, error) {
		return "/usr/local/bin/klvtool", nil
	}
	root.Update.RunGo = func(ctx context.Context, goBin string, args []string) ([]byte, []byte, error) {
		return nil, nil, nil
	}

	if got := root.Execute([]string{"update"}); got != 0 {
		t.Fatalf("exit %d", got)
	}
	out := stdout.String()
	if !strings.Contains(out, "note:") {
		t.Fatalf("expected warning about install location, got %q", out)
	}
}

func TestUpdatePreferBinarySkipsGo(t *testing.T) {
	archName := "klvtool_linux_amd64.tar.gz"
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/rel", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v2.0.0",
			"assets": []map[string]string{
				{"name": archName, "browser_download_url": srv.URL + "/dl/archive"},
				{"name": "checksums.txt", "browser_download_url": srv.URL + "/dl/sums"},
			},
		})
	})
	mux.HandleFunc("/dl/archive", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not a real archive", http.StatusInternalServerError)
	})
	mux.HandleFunc("/dl/sums", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("abc " + archName + "\n"))
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	var stdout, stderr strings.Builder
	root := NewRootCommand()
	root.Out = &stdout
	root.Err = &stderr
	root.Version = "v1.0.0"
	root.Update.ReleaseURL = srv.URL + "/rel"
	root.Update.GOOS = "linux"
	root.Update.GOARCH = "amd64"
	root.Update.LookPath = func(string) (string, error) { return "/bin/go", nil }
	root.Update.RunGo = func(context.Context, string, []string) ([]byte, []byte, error) {
		t.Fatalf("go must not run when --prefer-binary is set")
		return nil, nil, nil
	}

	if got := root.Execute([]string{"update", "--prefer-binary"}); got == 0 {
		t.Fatal("expected non-zero exit when archive download fails")
	}
}
