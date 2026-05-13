package stream

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jacorbello/klvtool/internal/model"
)

type fileSource struct {
	*os.File
	path string
}

func (f *fileSource) Scheme() string     { return "file" }
func (f *fileSource) RemoteAddr() string { return f.path }

// fileURLToLocalPath maps a parsed file:// URL to a host OS path.
// On Windows, file:///C:/foo yields URL Path "/C:/foo" which must drop the
// leading slash before os.Open. UNC paths use file://host/share/... .
func fileURLToLocalPath(u *url.URL) (string, error) {
	if u == nil {
		return "", errors.New("nil file URL")
	}
	path := u.Path
	path, err := url.PathUnescape(path)
	if err != nil {
		return "", fmt.Errorf("file URL path: %w", err)
	}

	host := u.Host
	if host != "" && !strings.EqualFold(host, "localhost") {
		if runtime.GOOS != "windows" {
			return "", fmt.Errorf("file URL with host %q is only supported on Windows (UNC)", host)
		}
		// file://server/share -> \\server\share
		unc := `\\` + host + strings.ReplaceAll(filepath.ToSlash(path), "/", `\`)
		return filepath.Clean(unc), nil
	}

	if runtime.GOOS == "windows" {
		// Drive letter: /C:/Users/... -> C:/Users/...
		if len(path) >= 4 && path[0] == '/' && path[1] != '/' && path[2] == ':' && (path[3] == '/' || path[3] == '\\') {
			path = path[1:]
		}
		return filepath.Clean(filepath.FromSlash(path)), nil
	}

	if path == "" {
		return "", errors.New("file URL has empty path")
	}
	return filepath.Clean(path), nil
}

func openFile(_ context.Context, path string) (Source, error) {
	if path == "" {
		return nil, model.InvalidUsage(errors.New("file:// input is missing a path"))
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, model.TSRead(fmt.Errorf("input file does not exist: %s", path))
		}
		return nil, model.TSRead(fmt.Errorf("failed to stat input file %q: %w", path, err))
	}
	if !info.Mode().IsRegular() {
		return nil, model.TSRead(fmt.Errorf("input path must be a regular file: %s", path))
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, model.TSRead(fmt.Errorf("open %q: %w", path, err))
	}
	return &fileSource{File: f, path: path}, nil
}
