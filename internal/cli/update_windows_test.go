//go:build windows

package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jacorbello/klvtool/internal/updater"
)

func TestUpdateBinaryFallbackWindows(t *testing.T) {
	binPayload := []byte("new-binary")
	archive := mustZipWin(t, "klvtool.exe", binPayload)
	archName := updater.ArchiveFileName("windows", "amd64")
	sumLine := fmt.Sprintf("%s  %s\n", sha256HexWin(archive), archName)
	sumsBody := sumLine + "deadbeef  other.tgz\n"

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
		_, _ = w.Write(archive)
	})
	mux.HandleFunc("/dl/sums", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sumsBody))
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	dir := t.TempDir()
	exe := filepath.Join(dir, "klvtool.exe")
	if err := os.WriteFile(exe, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	root := NewRootCommand()
	root.Err = &stderr
	root.Version = "v1.0.0"
	root.Update.ReleaseURL = srv.URL + "/rel"
	root.Update.GOOS = "windows"
	root.Update.GOARCH = "amd64"
	root.Update.LookPath = func(string) (string, error) { return "", fmt.Errorf("no go") }
	root.Update.Executable = func() (string, error) { return exe, nil }
	root.Update.RunGo = func(context.Context, string, []string) ([]byte, []byte, error) {
		t.Fatalf("go should not run in binary fallback")
		return nil, nil, nil
	}

	if got := root.Execute([]string{"update"}); got != 0 {
		t.Fatalf("exit %d stderr=%s", got, stderr.String())
	}
	pending := exe + ".new"
	gotBytes, err := os.ReadFile(pending)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotBytes) != string(binPayload) {
		t.Fatalf("got %q want %q", gotBytes, binPayload)
	}
}

func sha256HexWin(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func mustZipWin(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
