package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// ExtractPublishedBinary returns the klvtool executable bytes from a GoReleaser archive
// (tar.gz or zip) for the given target goos.
func ExtractPublishedBinary(archive []byte, goos string) ([]byte, error) {
	want := BinaryFileName(goos)
	if len(archive) >= 4 && archive[0] == 'P' && archive[1] == 'K' &&
		archive[2] == 0x03 && archive[3] == 0x04 {
		return extractFromZip(archive, want)
	}
	return extractFromTarGz(archive, want)
}

func extractFromTarGz(archive []byte, wantName string) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar: %w", err)
		}
		if h.Typeflag == tar.TypeDir {
			continue
		}
		base := filepath.Base(strings.TrimPrefix(h.Name, "./"))
		if base != wantName {
			continue
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != '\x00' { // '\x00' is legacy regular file flag
			return nil, fmt.Errorf("archive member %q is not a regular file", wantName)
		}
		if h.Size < 0 {
			return nil, fmt.Errorf("invalid tar header size for %q", wantName)
		}
		// Guard obviously huge entries (zip/tar bombs) without reading unbounded memory.
		const maxBinary = 128 << 20
		if h.Size > maxBinary {
			return nil, fmt.Errorf("archive member %q too large", wantName)
		}
		return io.ReadAll(io.LimitReader(tr, h.Size))
	}
	return nil, fmt.Errorf("archive missing %q", wantName)
}

func extractFromZip(archive []byte, wantName string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("zip: %w", err)
	}
	const maxBinary = 128 << 20
	for _, f := range zr.File {
		base := filepath.Base(f.Name)
		if base != wantName {
			continue
		}
		if !f.FileInfo().Mode().IsRegular() {
			return nil, fmt.Errorf("archive member %q is not a regular file", wantName)
		}
		if f.UncompressedSize64 > maxBinary {
			return nil, fmt.Errorf("archive member %q too large", wantName)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(io.LimitReader(rc, maxBinary+1))
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		if int64(len(b)) > maxBinary {
			return nil, fmt.Errorf("archive member %q too large", wantName)
		}
		return b, nil
	}
	return nil, fmt.Errorf("archive missing %q", wantName)
}
