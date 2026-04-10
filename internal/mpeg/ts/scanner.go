package ts

import (
	"bufio"
	"fmt"
	"io"

	"github.com/jacorbello/klvtool/internal/model"
)

// ScanConfig controls which packet payloads are read.
type ScanConfig struct {
	PayloadPIDs map[uint16]bool // nil=read all, empty=headers only
}

// PacketScanner reads sequential MPEG-TS packets from a stream.
type PacketScanner struct {
	r           *bufio.Reader
	cfg         ScanConfig
	buf         [PacketSize]byte
	offset      int64
	index       int64
	diagnostics []Diagnostic
}

// NewPacketScanner creates a scanner that reads TS packets from r.
func NewPacketScanner(r io.Reader, cfg ScanConfig) *PacketScanner {
	// Buffer big enough to hold a full packet plus the +188 lookahead
	// byte used to verify sync recovery candidates.
	return &PacketScanner{r: bufio.NewReaderSize(r, PacketSize*4), cfg: cfg}
}

// Next reads and parses the next TS packet. Returns io.EOF at end of stream.
func (s *PacketScanner) Next() (Packet, error) {
	if err := s.readAlignedPacket(); err != nil {
		return Packet{}, err
	}

	var header [4]byte
	copy(header[:], s.buf[:4])
	pkt := parseHeader(header)
	pkt.Offset = s.offset
	pkt.Index = s.index

	pos := 4
	if pkt.HasAdaptation {
		af, err := parseAdaptationField(s.buf[pos:])
		if err != nil {
			return Packet{}, model.TSParse(fmt.Errorf("adaptation field at offset %d: %w", s.offset, err))
		}
		pkt.Adaptation = &af
		pos += 1 + int(af.Length)
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

// readAlignedPacket fetches the next 188-byte TS packet into s.buf. When
// the scanner is already aligned (previous packet was clean) the fast
// path simply reads 188 bytes whose first byte is 0x47. If the first
// byte is not 0x47 the scanner is out of sync and recoverSync is
// invoked; recoverSync uses a +188 verification check to avoid locking
// onto spurious 0x47 bytes embedded in payload data.
func (s *PacketScanner) readAlignedPacket() error {
	peeked, err := s.r.Peek(PacketSize)
	if err != nil {
		if len(peeked) == 0 {
			return io.EOF
		}
		return model.TSParse(fmt.Errorf("incomplete TS packet: read %d of %d bytes", len(peeked), PacketSize))
	}

	if peeked[0] == SyncByte {
		copy(s.buf[:], peeked)
		if _, derr := s.r.Discard(PacketSize); derr != nil {
			return model.TSRead(fmt.Errorf("discard packet: %w", derr))
		}
		return nil
	}

	// Out of sync: resynchronize.
	return s.recoverSync()
}

// recoverSync scans forward one byte at a time through the bufio.Reader
// looking for a candidate sync byte whose +188 neighbor is also a sync
// byte. This guards against locking onto spurious 0x47 bytes that occur
// inside payload data. If fewer than 189 bytes remain in the stream, a
// candidate 0x47 at the start of the final 188 bytes is accepted without
// the +188 check (last-packet fallback). On success, s.buf holds the
// recovered 188-byte packet and s.offset is advanced by the number of
// skipped bytes.
func (s *PacketScanner) recoverSync() error {
	startOffset := s.offset
	var skipped int64

	for {
		peeked, err := s.r.Peek(PacketSize + 1)
		if err != nil {
			// Fewer than 189 bytes remain.
			if len(peeked) < PacketSize {
				return model.TSSync(fmt.Errorf("EOF while searching for sync byte"))
			}
			// Exactly 188 bytes left: last-packet fallback.
			if peeked[0] == SyncByte {
				copy(s.buf[:], peeked[:PacketSize])
				if _, derr := s.r.Discard(PacketSize); derr != nil {
					return model.TSRead(fmt.Errorf("sync recovery: discard: %w", derr))
				}
				break
			}
			// Advance one byte and retry (will hit EOF quickly).
			if _, derr := s.r.Discard(1); derr != nil {
				return model.TSRead(fmt.Errorf("sync recovery: discard: %w", derr))
			}
			skipped++
			continue
		}
		if peeked[0] == SyncByte && peeked[PacketSize] == SyncByte {
			copy(s.buf[:], peeked[:PacketSize])
			if _, derr := s.r.Discard(PacketSize); derr != nil {
				return model.TSRead(fmt.Errorf("sync recovery: discard: %w", derr))
			}
			break
		}
		if _, derr := s.r.Discard(1); derr != nil {
			return model.TSRead(fmt.Errorf("sync recovery: discard: %w", derr))
		}
		skipped++
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
