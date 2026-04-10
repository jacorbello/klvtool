package ts

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/jacorbello/klvtool/internal/model"
)

const (
	pidPAT     = 0x0000
	programNIT = 0x0000

	// maxDiscoveryPackets bounds the second pass of DiscoverStreams so a
	// truncated or corrupted stream where an announced PMT PID never carries
	// data does not cause a full scan to EOF.
	maxDiscoveryPackets = 10000
)

// Stream describes a single elementary stream discovered from PAT/PMT.
type Stream struct {
	PID        uint16
	StreamType uint8
	ProgramNum uint16
}

// StreamTable holds the complete program/stream inventory.
type StreamTable struct {
	Programs map[uint16][]Stream
}

// PSIParser accumulates PSI sections and builds a StreamTable.
//
// PSI sections (PAT, PMT) can span multiple TS packets. The parser buffers
// section bytes per PID until enough data has arrived to satisfy the
// section_length header, then dispatches to the table-specific parser.
type PSIParser struct {
	pmtPIDs     map[uint16]uint16 // program number → PMT PID
	pmtPIDIndex map[uint16]bool   // reverse lookup: PID → is a PMT PID
	sectionBufs map[uint16][]byte // PID → in-progress section bytes
	table       StreamTable
}

// NewPSIParser returns a fresh PSI parser ready to accept PAT/PMT packets.
func NewPSIParser() *PSIParser {
	return &PSIParser{
		pmtPIDs:     make(map[uint16]uint16),
		pmtPIDIndex: make(map[uint16]bool),
		sectionBufs: make(map[uint16][]byte),
		table:       StreamTable{Programs: make(map[uint16][]Stream)},
	}
}

// IsPMTPID reports whether pid has been identified as a PMT PID via PAT.
func (p *PSIParser) IsPMTPID(pid uint16) bool {
	return p.pmtPIDIndex[pid]
}

// Feed consumes a packet. Returns true if the parser state changed (a
// complete section was parsed). Continuation packets contribute to the
// current in-progress section on their PID; PUSI packets start a fresh
// section, discarding any prior partial accumulation.
func (p *PSIParser) Feed(pkt Packet) bool {
	if !pkt.HasPayload || pkt.Payload == nil {
		return false
	}
	if pkt.PID != pidPAT && !p.IsPMTPID(pkt.PID) {
		return false
	}

	if pkt.PayloadUnitStart {
		// Skip the pointer_field and its padding: payload[0] is the pointer
		// value N, and payload[1:1+N] is the tail of a previous section we
		// don't buffer. The new section begins at payload[1+N:].
		if len(pkt.Payload) < 1 {
			return false
		}
		pointer := int(pkt.Payload[0])
		if 1+pointer > len(pkt.Payload) {
			delete(p.sectionBufs, pkt.PID)
			return false
		}
		section := append([]byte(nil), pkt.Payload[1+pointer:]...)
		p.sectionBufs[pkt.PID] = section
	} else {
		buf, ok := p.sectionBufs[pkt.PID]
		if !ok {
			// Continuation arrived before any PUSI on this PID — ignore.
			return false
		}
		p.sectionBufs[pkt.PID] = append(buf, pkt.Payload...)
	}

	return p.tryParseSection(pkt.PID)
}

// tryParseSection attempts to parse the accumulated section for pid. If
// the section is not yet complete (fewer bytes buffered than
// 3+section_length), it returns false and keeps buffering. If the section
// is complete, it dispatches to the appropriate table parser and clears
// the buffer.
func (p *PSIParser) tryParseSection(pid uint16) bool {
	buf := p.sectionBufs[pid]
	if len(buf) < 3 {
		return false
	}
	sectionLength := int(binary.BigEndian.Uint16(buf[1:3]) & 0x0FFF)
	total := 3 + sectionLength
	if len(buf) < total {
		return false
	}

	section := buf[:total]
	var changed bool
	switch {
	case pid == pidPAT:
		changed = p.parsePAT(section)
	case p.IsPMTPID(pid):
		changed = p.parsePMT(pid, section)
	}

	delete(p.sectionBufs, pid)
	return changed
}

// Table returns a copy of the current StreamTable.
func (p *PSIParser) Table() StreamTable {
	cp := StreamTable{Programs: make(map[uint16][]Stream, len(p.table.Programs))}
	for k, v := range p.table.Programs {
		streams := make([]Stream, len(v))
		copy(streams, v)
		cp.Programs[k] = streams
	}
	return cp
}

// parsePAT parses a complete PAT section (pointer_field already stripped,
// length already validated by tryParseSection).
func (p *PSIParser) parsePAT(data []byte) bool {
	if len(data) < 8 || data[0] != 0x00 {
		return false
	}
	sectionLength := int(binary.BigEndian.Uint16(data[1:3]) & 0x0FFF)
	if sectionLength < 5 {
		return false
	}
	entryStart := 8
	entryEnd := 3 + sectionLength - 4

	changed := false
	for i := entryStart; i+4 <= entryEnd; i += 4 {
		programNum := binary.BigEndian.Uint16(data[i : i+2])
		pid := binary.BigEndian.Uint16(data[i+2:i+4]) & 0x1FFF
		if programNum == programNIT {
			continue
		}
		if existing, ok := p.pmtPIDs[programNum]; !ok || existing != pid {
			p.pmtPIDs[programNum] = pid
			p.pmtPIDIndex[pid] = true
			changed = true
		}
	}
	return changed
}

// parsePMT parses a complete PMT section.
func (p *PSIParser) parsePMT(pmtPID uint16, data []byte) bool {
	if len(data) < 12 || data[0] != 0x02 {
		return false
	}
	sectionLength := int(binary.BigEndian.Uint16(data[1:3]) & 0x0FFF)
	if sectionLength < 9 {
		return false
	}
	programNum := binary.BigEndian.Uint16(data[3:5])
	programInfoLength := int(binary.BigEndian.Uint16(data[10:12]) & 0x0FFF)
	entryStart := 12 + programInfoLength
	entryEnd := 3 + sectionLength - 4

	var streams []Stream
	for i := entryStart; i+5 <= entryEnd; {
		streamType := data[i]
		elementaryPID := binary.BigEndian.Uint16(data[i+1:i+3]) & 0x1FFF
		esInfoLength := int(binary.BigEndian.Uint16(data[i+3:i+5]) & 0x0FFF)
		streams = append(streams, Stream{
			PID:        elementaryPID,
			StreamType: streamType,
			ProgramNum: programNum,
		})
		i += 5 + esInfoLength
	}

	p.table.Programs[programNum] = streams
	return len(streams) > 0
}

// DiscoverStreams reads enough of the source to parse PAT/PMT and returns
// the complete StreamTable.
func DiscoverStreams(r io.ReadSeeker) (StreamTable, error) {
	scanner := NewPacketScanner(r, ScanConfig{PayloadPIDs: map[uint16]bool{pidPAT: true}})
	psi := NewPSIParser()

	var pmtPIDs map[uint16]bool
	for {
		pkt, err := scanner.Next()
		if err != nil {
			if err == io.EOF {
				return StreamTable{}, model.TSParse(fmt.Errorf("PAT not found in stream"))
			}
			return StreamTable{}, err
		}
		if psi.Feed(pkt) {
			pmtPIDs = make(map[uint16]bool)
			for _, pid := range psi.pmtPIDs {
				pmtPIDs[pid] = true
			}
			break
		}
	}

	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return StreamTable{}, model.TSRead(fmt.Errorf("seek to start: %w", err))
	}

	allPIDs := map[uint16]bool{pidPAT: true}
	for pid := range pmtPIDs {
		allPIDs[pid] = true
	}
	scanner = NewPacketScanner(r, ScanConfig{PayloadPIDs: allPIDs})
	psi = NewPSIParser()

	// Track which PMT PIDs have been fully parsed. A PMT is only marked
	// complete when psi.Feed reports that a section actually changed the
	// table — PUSI alone is insufficient because a malformed PMT start
	// or a not-yet-complete multi-packet section would otherwise end
	// discovery early with an incomplete table.
	parsedPMTs := make(map[uint16]bool)
	packetsScanned := 0
	for packetsScanned < maxDiscoveryPackets {
		pkt, err := scanner.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return StreamTable{}, err
		}
		packetsScanned++
		if psi.Feed(pkt) && pmtPIDs[pkt.PID] {
			parsedPMTs[pkt.PID] = true
		}
		if len(parsedPMTs) == len(pmtPIDs) && len(pmtPIDs) > 0 {
			break
		}
	}

	return psi.Table(), nil
}
