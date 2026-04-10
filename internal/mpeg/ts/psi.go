package ts

import (
	"encoding/binary"
	"fmt"
	"io"
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
type PSIParser struct {
	pmtPIDs map[uint16]uint16 // program number → PMT PID
	table   StreamTable
}

// NewPSIParser returns a fresh PSI parser ready to accept PAT/PMT packets.
func NewPSIParser() *PSIParser {
	return &PSIParser{
		pmtPIDs: make(map[uint16]uint16),
		table:   StreamTable{Programs: make(map[uint16][]Stream)},
	}
}

// IsPMTPID reports whether pid has been identified as a PMT PID via PAT.
func (p *PSIParser) IsPMTPID(pid uint16) bool {
	for _, pmtPID := range p.pmtPIDs {
		if pmtPID == pid {
			return true
		}
	}
	return false
}

// Feed consumes a packet. Returns true if the parser state changed.
func (p *PSIParser) Feed(pkt Packet) bool {
	if !pkt.HasPayload || pkt.Payload == nil {
		return false
	}
	if pkt.PID == pidPAT && pkt.PayloadUnitStart {
		return p.parsePAT(pkt.Payload)
	}
	if p.IsPMTPID(pkt.PID) && pkt.PayloadUnitStart {
		return p.parsePMT(pkt.PID, pkt.Payload)
	}
	return false
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

func (p *PSIParser) parsePAT(payload []byte) bool {
	if len(payload) < 1 {
		return false
	}
	pointer := int(payload[0])
	if 1+pointer > len(payload) {
		return false
	}
	data := payload[1+pointer:]
	if len(data) < 8 {
		return false
	}
	if data[0] != 0x00 {
		return false
	}
	sectionLength := int(binary.BigEndian.Uint16(data[1:3]) & 0x0FFF)
	if sectionLength < 5 || len(data) < 3+sectionLength {
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
			changed = true
		}
	}
	return changed
}

func (p *PSIParser) parsePMT(pmtPID uint16, payload []byte) bool {
	if len(payload) < 1 {
		return false
	}
	pointer := int(payload[0])
	if 1+pointer > len(payload) {
		return false
	}
	data := payload[1+pointer:]
	if len(data) < 12 {
		return false
	}
	if data[0] != 0x02 {
		return false
	}
	sectionLength := int(binary.BigEndian.Uint16(data[1:3]) & 0x0FFF)
	if sectionLength < 9 || len(data) < 3+sectionLength {
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
				return StreamTable{}, fmt.Errorf("PAT not found in stream")
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
		return StreamTable{}, fmt.Errorf("seek to start: %w", err)
	}

	allPIDs := map[uint16]bool{pidPAT: true}
	for pid := range pmtPIDs {
		allPIDs[pid] = true
	}
	scanner = NewPacketScanner(r, ScanConfig{PayloadPIDs: allPIDs})
	psi = NewPSIParser()

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
		psi.Feed(pkt)

		if pkt.PayloadUnitStart && pmtPIDs[pkt.PID] {
			parsedPMTs[pkt.PID] = true
		}
		if len(parsedPMTs) == len(pmtPIDs) && len(pmtPIDs) > 0 {
			break
		}
	}

	return psi.Table(), nil
}
