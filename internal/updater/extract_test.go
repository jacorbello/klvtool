package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"strings"
	"testing"
)

func TestExtractPublishedBinary_tarGz(t *testing.T) {
	payload := []byte("binary-bytes")
	archive := mustTarGz(t, "klvtool", payload)

	got, err := ExtractPublishedBinary(archive, "linux")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("got %q, want %q", got, payload)
	}
}

func TestExtractPublishedBinary_zipWindows(t *testing.T) {
	payload := []byte("exe-bytes")
	archive := mustZip(t, "klvtool.exe", payload)

	got, err := ExtractPublishedBinary(archive, "windows")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("got %q, want %q", got, payload)
	}
}

func TestExtractPublishedBinary_wrongMemberSkipped(t *testing.T) {
	// GoReleaser adds README; first member may not be the binary.
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	_ = tw.WriteHeader(&tar.Header{Name: "README.md", Mode: 0o644, Size: 3})
	_, _ = tw.Write([]byte("hi\n"))
	_ = tw.WriteHeader(&tar.Header{Name: "klvtool", Mode: 0o755, Size: 4})
	_, _ = tw.Write([]byte("exec"))
	_ = tw.Close()
	_ = gw.Close()

	got, err := ExtractPublishedBinary(buf.Bytes(), "linux")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "exec" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractPublishedBinary_missingBinary(t *testing.T) {
	archive := mustTarGz(t, "README.md", []byte("x"))
	_, err := ExtractPublishedBinary(archive, "linux")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractPublishedBinary_tarGzRejectsSymlink(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	_ = tw.WriteHeader(&tar.Header{
		Name:     "klvtool",
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
	})
	_ = tw.Close()
	_ = gw.Close()

	_, err := ExtractPublishedBinary(buf.Bytes(), "linux")
	if err == nil {
		t.Fatal("expected error for symlink entry")
	}
	if !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtractPublishedBinary_zipRejectsDirectory(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	// Create a directory entry named "klvtool"
	h := &zip.FileHeader{Name: "klvtool/"}
	h.SetMode(os.ModeDir | 0o755)
	_, err := zw.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	_ = zw.Close()

	_, err = ExtractPublishedBinary(buf.Bytes(), "linux")
	if err == nil {
		t.Fatal("expected error for directory entry")
	}
}

func TestExtractPublishedBinary_PKNotZipLocalHeaderUsesGzip(t *testing.T) {
	// ZIP end-of-central-directory signature starts with PK but is not a local file header.
	archive := []byte{'P', 'K', 0x05, 0x06, 0, 0, 0, 0}
	_, err := ExtractPublishedBinary(archive, "linux")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.HasPrefix(err.Error(), "gzip:") {
		t.Fatalf("expected gzip parse error, got %v", err)
	}
	if strings.HasPrefix(err.Error(), "zip:") {
		t.Fatalf("should not treat PK\\x05\\x06 as zip archive, got %v", err)
	}
}

func mustTarGz(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))})
	_, _ = tw.Write(body)
	_ = tw.Close()
	_ = gw.Close()
	return buf.Bytes()
}

func mustZip(t *testing.T, name string, body []byte) []byte {
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
