package ts

import (
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
		return Packet{}, fmt.Errorf("sync byte mismatch at offset %d: got 0x%02X", s.offset, s.buf[0])
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

func (s *PacketScanner) shouldReadPayload(pid uint16) bool {
	if s.cfg.PayloadPIDs == nil {
		return true
	}
	return s.cfg.PayloadPIDs[pid]
}
