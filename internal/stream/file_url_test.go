package stream

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFileURLToLocalPath(t *testing.T) {
	switch runtime.GOOS {
	case "windows":
		u, err := url.Parse("file:///C:/Windows/System32/drivers/etc/hosts")
		if err != nil {
			t.Fatal(err)
		}
		got, err := fileURLToLocalPath(u)
		if err != nil {
			t.Fatalf("fileURLToLocalPath: %v", err)
		}
		wantPrefix := filepath.VolumeName(got)
		if wantPrefix == "" || !strings.HasPrefix(strings.ToUpper(got), strings.ToUpper(wantPrefix+`\`)) {
			t.Fatalf("unexpected path %q", got)
		}
	default:
		u, err := url.Parse("file:///tmp/klvtool-file-url-test")
		if err != nil {
			t.Fatal(err)
		}
		got, err := fileURLToLocalPath(u)
		if err != nil {
			t.Fatalf("fileURLToLocalPath: %v", err)
		}
		if want := "/tmp/klvtool-file-url-test"; got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
		u2, err := url.Parse("file://localhost/tmp/with%20space")
		if err != nil {
			t.Fatal(err)
		}
		got2, err := fileURLToLocalPath(u2)
		if err != nil {
			t.Fatalf("localhost file URL: %v", err)
		}
		if want2 := "/tmp/with space"; got2 != want2 {
			t.Fatalf("got %q, want %q", got2, want2)
		}
	}
}

func TestFileURLToLocalPath_uncOnlyOnWindows(t *testing.T) {
	u, err := url.Parse("file://fileserver/share/docs/readme.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, err = fileURLToLocalPath(u)
	if runtime.GOOS == "windows" {
		if err != nil {
			t.Fatalf("UNC on Windows: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatal("expected error for UNC file URL on non-Windows")
	}
}
