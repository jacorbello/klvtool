package updater

import (
	"strings"
	"testing"
)

func TestPickAssetURL(t *testing.T) {
	assets := []ReleaseAsset{
		{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums.txt"},
		{Name: "klvtool_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux.tgz"},
		{Name: "klvtool_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/mac.tgz"},
	}
	got, err := PickAssetURL(assets, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.com/linux.tgz" {
		t.Fatalf("got %q", got)
	}

	_, err = PickAssetURL(assets, "windows", "amd64")
	if err == nil {
		t.Fatal("expected error for missing windows asset")
	}
	if !strings.Contains(err.Error(), "klvtool_windows_amd64.zip") {
		t.Fatalf("expected archive name in error, got %v", err)
	}
}

func TestChecksumsAssetURL(t *testing.T) {
	assets := []ReleaseAsset{
		{Name: "klvtool_linux_amd64.tar.gz", BrowserDownloadURL: "https://x/a.tgz"},
		{Name: "checksums.txt", BrowserDownloadURL: "https://x/sums"},
	}
	got, err := ChecksumsAssetURL(assets)
	if err != nil || got != "https://x/sums" {
		t.Fatalf("got %q %v", got, err)
	}
}
