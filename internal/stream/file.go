package stream

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jacorbello/klvtool/internal/model"
)

type fileSource struct {
	*os.File
	path string
}

func (f *fileSource) Scheme() string     { return "file" }
func (f *fileSource) RemoteAddr() string { return f.path }

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
