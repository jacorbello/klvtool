package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchLatestRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/releases/latest" {
			http.NotFound(w, r)
			return
		}
		resp := map[string]any{
			"tag_name": "v2.0.0",
			"html_url": "https://github.com/o/r/releases/tag/v2.0.0",
			"assets": []map[string]string{
				{"name": "klvtool_linux_amd64.tar.gz", "browser_download_url": "https://example.invalid/dl/linux.tgz"},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	rel, err := FetchLatestRelease(context.Background(), http.DefaultClient, srv.URL+"/releases/latest", "test/1")
	if err != nil {
		t.Fatal(err)
	}
	if rel.TagName != "v2.0.0" {
		t.Fatalf("tag %q", rel.TagName)
	}
	if len(rel.Assets) != 1 {
		t.Fatal(rel.Assets)
	}
}

func TestFetchLatestRelease_badStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := FetchLatestRelease(context.Background(), http.DefaultClient, srv.URL+"/releases/latest", "x")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDownloadBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("payload"))
	}))
	defer srv.Close()

	b, err := DownloadBytes(context.Background(), http.DefaultClient, srv.URL, "x/1")
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "payload" {
		t.Fatalf("got %q", b)
	}
}
