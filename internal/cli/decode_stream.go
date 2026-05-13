package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/jacorbello/klvtool/internal/extract"
	"github.com/jacorbello/klvtool/internal/klv"
	"github.com/jacorbello/klvtool/internal/klv/record"
	"github.com/jacorbello/klvtool/internal/model"
	"github.com/jacorbello/klvtool/internal/mpeg/ts"
	"github.com/jacorbello/klvtool/internal/packetize"
	"github.com/jacorbello/klvtool/internal/stream"
)

// streamSourceOpener matches the signature of stream.Open. Tests inject a
// fake to feed crafted MPEG-TS bytes through the streaming pipeline
// without involving real network I/O.
type streamSourceOpener func(ctx context.Context, raw string, opts stream.Options) (stream.Source, error)

// streamingDecodeOptions bundles everything newStreamingDecode needs
// beyond the registry + opener. Keeping it in a struct avoids a
// 5-positional-arg function that's easy to misorder.
type streamingDecodeOptions struct {
	SourceOptions   stream.Options
	RecordPath      string
	RecordOverwrite bool
}

// newStreamingDecode returns a DecodeStream that opens a URL-based source
// via openSource and runs the bytes through the native MPEG-TS demuxer
// + KLV decoder pipeline. No ffmpeg subprocess is involved — this is
// the live path that sidesteps the SMPTE-UL-prefix bug worked around in
// the file-based ffmpeg backend (commit bd35896).
//
// The registry function fires once per invocation so --schema can build
// a single-entry registry without mutating the shared one.
//
// When opts.RecordPath is non-empty, source bytes are tee'd to that file
// before reaching the demuxer. A write error on the record file aborts
// the run so capture is never silently lossy.
func newStreamingDecode(
	registry func() *klv.Registry,
	openSource streamSourceOpener,
	opts streamingDecodeOptions,
) func(ctx context.Context, req DecodeRequest, emit DecodeEmitter) error {
	if openSource == nil {
		openSource = stream.Open
	}
	return func(ctx context.Context, req DecodeRequest, emit DecodeEmitter) error {
		src, err := openSource(ctx, req.Path, opts.SourceOptions)
		if err != nil {
			return err
		}
		defer func() { _ = src.Close() }()

		var reader io.Reader = src
		if opts.RecordPath != "" {
			flag := os.O_CREATE | os.O_WRONLY | os.O_EXCL
			if opts.RecordOverwrite {
				flag = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
			}
			f, err := os.OpenFile(opts.RecordPath, flag, 0o644)
			if err != nil {
				return model.OutputWrite(fmt.Errorf("open --record %q: %w", opts.RecordPath, err))
			}
			// Close the record file after the demux loop returns so the
			// captured bytes are durable on disk before exit.
			defer func() { _ = f.Close() }()
			reader = io.TeeReader(src, f)
		}

		reg := registry()
		if req.Schema != "" {
			sv, ok := reg.Lookup(req.Schema)
			if !ok {
				return model.InvalidUsage(fmt.Errorf("schema %q not registered", req.Schema))
			}
			reg = klv.NewRegistry()
			reg.Register(sv)
		}
		knownULs := reg.KnownULs()
		parser := packetize.NewParser()

		// runCtx lets us stop LiveDemux when emit.Record returns an
		// abort sentinel (e.g. output write failed). LiveDemux observes
		// cancellation between packets.
		runCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		var abortErr error

		demux := ts.NewLiveDemux(reader)
		runErr := demux.Run(runCtx, true,
			func(_ ts.Packet) {
				if emit.Packet != nil {
					emit.Packet()
				}
			},
			func(unit ts.PESUnit) {
				if abortErr != nil {
					return
				}
				if req.PID != 0 && int(unit.PID) != req.PID {
					return
				}
				raw := extract.RawPayloadRecord{PID: unit.PID, Payload: unit.Payload}
				pres, err := parser.Parse(packetize.Request{
					Mode:             packetize.ModeBestEffort,
					Record:           raw,
					KnownULs:         knownULs,
					RepairStrippedUL: req.RepairStrippedUL,
				})
				if err != nil {
					// One bad PES doesn't kill the stream — surface as
					// a stream diagnostic so --strict still notices it.
					if emit.Stream != nil {
						_ = emit.Stream(record.Diagnostic{
							Severity: "error",
							Code:     "packetize_failure",
							Message:  err.Error(),
						})
					}
					return
				}
				streamDiags := liftPacketizeDiagnostics(pres.Diagnostics)
				if len(pres.Packets) == 0 {
					for _, d := range streamDiags {
						if emit.Stream != nil {
							_ = emit.Stream(d)
						}
					}
					return
				}
				parsedPayload := pres.Source.Payload
				for i, pkt := range pres.Packets {
					var lengthBytes []byte
					if pkt.LengthStart >= 0 && pkt.ValueStart >= pkt.LengthStart && pkt.ValueStart <= len(parsedPayload) {
						lengthBytes = parsedPayload[pkt.LengthStart:pkt.ValueStart]
					}
					rec, err := klv.DecodeLocalSet(reg, pkt.Key, lengthBytes, pkt.Value)
					if err != nil {
						if emit.Stream != nil {
							_ = emit.Stream(record.Diagnostic{
								Severity: "error",
								Code:     "klv_decode_failure",
								Message:  err.Error(),
							})
						}
						continue
					}
					if i == 0 && len(streamDiags) > 0 {
						// Attach packetize diagnostics to the first
						// decoded record so they flow through the
						// per-packet diagnostic path, matching the
						// file-based decode behavior at decode.go:186.
						attached := make([]record.Diagnostic, 0, len(streamDiags)+len(rec.Diagnostics))
						attached = append(attached, streamDiags...)
						attached = append(attached, rec.Diagnostics...)
						rec.Diagnostics = attached
					}
					if emit.Record != nil {
						if err := emit.Record(rec); err != nil {
							abortErr = err
							cancel()
							return
						}
					}
				}
			},
			func(diag ts.Diagnostic) {
				if emit.Stream == nil {
					return
				}
				_ = emit.Stream(record.Diagnostic{
					Severity: diag.Severity,
					Code:     diag.Code,
					Message:  diag.Message,
				})
			},
		)
		if abortErr != nil {
			return abortErr
		}
		return runErr
	}
}
