package ts

import (
	"bytes"
	"fmt"
	"io"
)

// ScanConfig controls which packet payloads are read.
type ScanConfig struct {
	PayloadPIDs map[uint16]bool // nil=read all, empty=headers only
}

// PacketScanner reads sequential MPEG-TS packets from a stream.
type PacketScanner struct {
	r           io.Reader
	cfg         ScanConfig
	buf         [PacketSize]byte
	offset      int64
	index       int64
	diagnostics []Diagnostic
}

// NewPacketScanner creates a scanner that reads TS packets from r.
func NewPacketScanner(r io.Reader, cfg ScanConfig) *PacketScanner {
	return &PacketScanner{r: r, cfg: cfg}
}

// Next reads and parses the next TS packet. Returns io.EOF at end of stream.
func (s *PacketScanner) Next() (Packet, error) {
	n, err := io.ReadFull(s.r, s.buf[:])
	if err != nil {
		if err == io.EOF || (n == 0 && err == io.ErrUnexpectedEOF) {
			return Packet{}, io.EOF
		}
		return Packet{}, fmt.Errorf("incomplete TS packet: read %d of %d bytes", n, PacketSize)
	}

	if s.buf[0] != SyncByte {
		if err := s.recoverSync(); err != nil {
			return Packet{}, err
		}
	}

	var header [4]byte
	copy(header[:], s.buf[:4])
	pkt := parseHeader(header)
	pkt.Offset = s.offset
	pkt.Index = s.index

	pos := 4
	if pkt.HasAdaptation {
		af, err := parseAdaptationField(s.buf[pos:])
		if err == nil {
			pkt.Adaptation = &af
			pos += 1 + int(af.Length)
		}
	}

	if pkt.HasPayload && s.shouldReadPayload(pkt.PID) && pos < PacketSize {
		payload := make([]byte, PacketSize-pos)
		copy(payload, s.buf[pos:])
		pkt.Payload = payload
	}

	s.offset += PacketSize
	s.index++
	return pkt, nil
}

// Diagnostics returns and drains accumulated diagnostics.
func (s *PacketScanner) Diagnostics() []Diagnostic {
	d := s.diagnostics
	s.diagnostics = nil
	return d
}

// recoverSync attempts to resynchronize after a sync byte mismatch.
// On entry, s.buf contains 188 bytes not starting with 0x47.
// On success, s.buf is refilled starting at the next 0x47, a diagnostic is
// recorded, and s.offset is advanced by the number of skipped bytes.
func (s *PacketScanner) recoverSync() error {
	startOffset := s.offset
	skipped := int64(0)

	// Search the current 188-byte buffer for a 0x47.
	idx := bytes.IndexByte(s.buf[:], SyncByte)
	if idx > 0 {
		skipped = int64(idx)
		// Shift buffer left by idx.
		copy(s.buf[:], s.buf[idx:])
		// Fill the remaining idx bytes.
		if _, err := io.ReadFull(s.r, s.buf[PacketSize-idx:]); err != nil {
			return fmt.Errorf("sync recovery: failed to refill buffer: %w", err)
		}
	} else if idx < 0 {
		// No 0x47 in buffer: scan forward one byte at a time.
		skipped = int64(PacketSize)
		var one [1]byte
		for {
			n, err := io.ReadFull(s.r, one[:])
			if err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF && n == 0 {
					return fmt.Errorf("sync recovery: EOF while searching for sync byte")
				}
				return fmt.Errorf("sync recovery: %w", err)
			}
			if one[0] == SyncByte {
				break
			}
			skipped++
		}
		// Place 0x47 at buf[0] and read the remaining 187 bytes.
		s.buf[0] = SyncByte
		if _, err := io.ReadFull(s.r, s.buf[1:]); err != nil {
			return fmt.Errorf("sync recovery: failed to read packet body: %w", err)
		}
	}

	s.offset += skipped
	s.diagnostics = append(s.diagnostics, Diagnostic{
		Severity: "warning",
		Code:     "sync_recovery",
		Message:  fmt.Sprintf("resynced after skipping %d bytes at offset %d", skipped, startOffset),
	})
	return nil
}

func (s *PacketScanner) shouldReadPayload(pid uint16) bool {
	if s.cfg.PayloadPIDs == nil {
		return true
	}
	return s.cfg.PayloadPIDs[pid]
}
