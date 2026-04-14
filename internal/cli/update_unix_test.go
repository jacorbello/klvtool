//go:build !windows

package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
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

func TestUpdateBinaryFallbackUnix(t *testing.T) {
	binPayload := []byte("new-binary")
	archive := mustTarGzFile(t, "klvtool", binPayload)
	archName := updater.ArchiveFileName("linux", "amd64")
	sumLine := fmt.Sprintf("%s  %s\n", sha256Hex(archive), archName)
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
	exe := filepath.Join(dir, "klvtool")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	root := NewRootCommand()
	root.Out = &stdout
	root.Err = &stderr
	root.Version = "v1.0.0"
	root.Update.ReleaseURL = srv.URL + "/rel"
	root.Update.GOOS = "linux"
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
	gotBytes, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotBytes) != string(binPayload) {
		t.Fatalf("got %q want %q", gotBytes, binPayload)
	}
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func mustTarGzFile(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))})
	_, _ = tw.Write(body)
	_ = tw.Close()
	_ = gw.Close()
	return buf.Bytes()
}
