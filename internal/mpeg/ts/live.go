package ts

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// IsKLVCandidateStreamType reports whether streamType (from a PMT entry)
// is a plausible carrier for MISB KLV metadata. The predicate covers:
//
//   - 0x06 — PES_private_data (the common case for MISB ST 1402 data streams)
//   - 0x15 — Metadata in PES packets (some encoders use this for KLV)
//   - 0xC0..0xFF — user-private stream types some integrators use for KLV
//
// Inspect uses the same predicate to identify likely data PIDs; the live
// demux uses it to decide which PIDs to reassemble into PES units when
// klvOnly=true.
func IsKLVCandidateStreamType(streamType uint8) bool {
	return streamType == 0x06 || streamType == 0x15 || streamType >= 0xC0
}

// LiveDemux orchestrates a single-pass MPEG-TS demux suitable for
// non-seekable inputs: read packets, parse PSI, track which PIDs carry
// KLV-candidate data, reassemble PES units, and surface diagnostics
// through caller-provided callbacks.
//
// LiveDemux is pure: io.Reader in, callbacks out. It owns no I/O
// lifecycle and installs no signal handlers — that's the caller's job.
// Cancellation is observed between packet reads via the provided context.
type LiveDemux struct {
	scanner *PacketScanner
	psi     *PSIParser
	pes     *PESAssembler
}

// NewLiveDemux constructs a live demux reading from r. The scanner reads
// all packet payloads (PayloadPIDs=nil) because the KLV-candidate PID set
// is only known after PMTs arrive, and re-configuring the scanner mid-run
// is not supported. Reading every payload at typical MPEG-TS bitrates
// (tens of Mbit/s) is well within budget for a Go single-pass demuxer.
func NewLiveDemux(r io.Reader) *LiveDemux {
	return &LiveDemux{
		scanner: NewPacketScanner(r, ScanConfig{}),
		psi:     NewPSIParser(),
		pes:     NewPESAssembler(),
	}
}

// Table returns a snapshot of the PMT/PAT inventory accumulated so far.
// Callers typically read this after Run returns to learn what programs
// and elementary streams the live source advertised during the window.
// Calling Table mid-run is safe but the result is a value type — callers
// see a frozen view, not a live reference.
func (d *LiveDemux) Table() StreamTable {
	return d.psi.Table()
}

// Run consumes packets from the underlying reader until ctx is cancelled
// or the reader returns io.EOF. Callbacks may be nil; nil callbacks are
// skipped. On exit (cancel or EOF), any in-progress PES units are flushed
// through onPES so the caller doesn't lose the tail of a stream.
//
// When klvOnly is true, onPES fires only for PIDs whose PMT stream type
// passes IsKLVCandidateStreamType. When klvOnly is false, onPES fires for
// every PMT-declared data PID (the predicate still gates so audio/video
// PESes don't overwhelm a metadata-oriented caller). PSI packets (PAT,
// PMT) never produce PES units.
//
// Returns nil for graceful stops (cancel or EOF), or a wrapped scanner
// error otherwise.
func (d *LiveDemux) Run(
	ctx context.Context,
	klvOnly bool,
	onPacket func(Packet),
	onPES func(PESUnit),
	onDiag func(Diagnostic),
) error {
	// Recompute the KLV-candidate PID set lazily whenever PSI state changes.
	// Stale until the first PMT arrives.
	var klvPIDs map[uint16]bool
	refreshKLVPIDs := func() {
		t := d.psi.Table()
		next := make(map[uint16]bool, len(klvPIDs))
		for _, streams := range t.Programs {
			for _, s := range streams {
				if IsKLVCandidateStreamType(s.StreamType) {
					next[s.PID] = true
				}
			}
		}
		klvPIDs = next
	}

	drainDiagnostics := func() {
		if onDiag == nil {
			d.scanner.Diagnostics() // drain even if no consumer
			return
		}
		for _, diag := range d.scanner.Diagnostics() {
			onDiag(diag)
		}
	}

	flush := func() {
		if onPES == nil {
			return
		}
		for _, unit := range d.pes.Flush() {
			onPES(unit)
		}
	}

	for {
		// Honor cancellation between packets. A blocked Read inside
		// scanner.Next() can only unblock when the source's Close fires
		// from the caller's context plumbing — LiveDemux deliberately
		// doesn't reach into the reader.
		if err := ctx.Err(); err != nil {
			flush()
			drainDiagnostics()
			return nil
		}
		pkt, err := d.scanner.Next()
		if errors.Is(err, io.EOF) {
			flush()
			drainDiagnostics()
			return nil
		}
		if err != nil {
			// Reader closed mid-stream (typical when context cancels and
			// the source closes the underlying socket) — treat as a clean
			// stop, not an error, when the context has been cancelled.
			// Surface the underlying error via onDiag first so a real
			// corruption that happens to coincide with Ctrl-C isn't
			// silently dropped from the run summary.
			if ctx.Err() != nil {
				if onDiag != nil && !errors.Is(err, io.EOF) {
					onDiag(Diagnostic{
						Severity: "warning",
						Code:     "scanner_error_at_cancel",
						Message:  fmt.Sprintf("scanner returned %v after context cancel", err),
					})
				}
				flush()
				drainDiagnostics()
				return nil
			}
			drainDiagnostics()
			return err
		}

		if onPacket != nil {
			onPacket(pkt)
		}
		drainDiagnostics()

		// PSI tracking. PSIParser silently ignores non-PAT/PMT packets,
		// so this is safe to call on every packet.
		if d.psi.Feed(pkt) {
			refreshKLVPIDs()
		}

		// Skip PSI packets at the PES layer.
		if pkt.PID == pidPAT || d.psi.IsPMTPID(pkt.PID) {
			continue
		}
		// klvOnly=true: only reassemble PES for KLV-candidate PIDs.
		// klvOnly=false: reassemble PES for every non-PSI PID — useful
		// for callers that want a complete elementary-stream picture
		// (e.g. inspect when it accumulates per-PID PES counts).
		if klvOnly && !klvPIDs[pkt.PID] {
			continue
		}
		unit := d.pes.Feed(pkt)
		if unit != nil && onPES != nil {
			onPES(*unit)
		}
	}
}
