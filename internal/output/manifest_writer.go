package output

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/jacorbello/klvtool/internal/model"
)

// ManifestWriter writes manifest records as NDJSON.
type ManifestWriter struct {
	w *bufio.Writer
}

// NewManifestWriter creates a writer that emits one JSON record per line.
func NewManifestWriter(w io.Writer) *ManifestWriter {
	if w == nil {
		return &ManifestWriter{}
	}
	return &ManifestWriter{w: bufio.NewWriter(w)}
}

// WriteRecord writes one manifest record as a single NDJSON line.
func (mw *ManifestWriter) WriteRecord(record model.Record) error {
	if mw == nil || mw.w == nil {
		return model.OutputWrite(errors.New("manifest writer is not initialized"))
	}

	data, err := record.MarshalJSON()
	if err != nil {
		return model.OutputWrite(fmt.Errorf("marshal manifest record: %w", err))
	}

	if _, err := mw.w.Write(append(data, '\n')); err != nil {
		return model.OutputWrite(fmt.Errorf("write manifest record: %w", err))
	}
	if err := mw.w.Flush(); err != nil {
		return model.OutputWrite(fmt.Errorf("flush manifest writer: %w", err))
	}
	return nil
}
