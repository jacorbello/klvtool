package output

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/jacorbello/klvtool/internal/model"
)

// PacketManifestWriter writes packet checkpoint manifests as NDJSON.
type PacketManifestWriter struct {
	w *bufio.Writer
}

// NewPacketManifestWriter creates a writer that emits one packet manifest JSON line.
func NewPacketManifestWriter(w io.Writer) *PacketManifestWriter {
	if w == nil {
		return &PacketManifestWriter{}
	}
	return &PacketManifestWriter{w: bufio.NewWriter(w)}
}

// WriteManifest writes one packet manifest as a single NDJSON line.
func (mw *PacketManifestWriter) WriteManifest(manifest model.PacketManifest) error {
	if mw == nil || mw.w == nil {
		return model.OutputWrite(errors.New("packet manifest writer is not initialized"))
	}

	data, err := manifest.MarshalJSON()
	if err != nil {
		return model.OutputWrite(fmt.Errorf("marshal packet manifest: %w", err))
	}

	if _, err := mw.w.Write(append(data, '\n')); err != nil {
		return model.OutputWrite(fmt.Errorf("write packet manifest: %w", err))
	}
	if err := mw.w.Flush(); err != nil {
		return model.OutputWrite(fmt.Errorf("flush packet manifest writer: %w", err))
	}
	return nil
}
