package ts

import "fmt"

// PESUnit represents a reassembled Packetized Elementary Stream unit.
type PESUnit struct {
	PID               uint16
	PTS               *int64
	DTS               *int64
	Payload           []byte
	PacketStart       int64
	PacketIndex       int64
	PacketCount       int
	ContinuityCounter *uint8
	Diagnostics       []Diagnostic
}

// parsePESHeader parses PTS/DTS from a PES header.
// Returns PTS, DTS (nil if absent), total header length (bytes to skip
// before payload data), and any error.
func parsePESHeader(data []byte) (pts *int64, dts *int64, headerLen int, err error) {
	if len(data) < 9 {
		return nil, nil, 0, fmt.Errorf("PES header too short: %d bytes", len(data))
	}
	if data[0] != 0x00 || data[1] != 0x00 || data[2] != 0x01 {
		return nil, nil, 0, fmt.Errorf("invalid PES start code: %02X%02X%02X", data[0], data[1], data[2])
	}

	ptsDTSFlags := (data[7] >> 6) & 0x03
	pesHeaderDataLength := int(data[8])
	headerLen = 9 + pesHeaderDataLength
	if len(data) < headerLen {
		return nil, nil, 0, fmt.Errorf("PES header too short for declared optional header length: have %d bytes, need %d", len(data), headerLen)
	}

	switch ptsDTSFlags {
	case 0x02:
		if len(data) < 14 {
			return nil, nil, 0, fmt.Errorf("PES header too short for PTS: %d bytes", len(data))
		}
		ptsVal := parseTimestamp(data[9:14])
		pts = &ptsVal
	case 0x03:
		if len(data) < 19 {
			return nil, nil, 0, fmt.Errorf("PES header too short for PTS+DTS: %d bytes", len(data))
		}
		ptsVal := parseTimestamp(data[9:14])
		dtsVal := parseTimestamp(data[14:19])
		pts = &ptsVal
		dts = &dtsVal
	}

	return pts, dts, headerLen, nil
}

type pesAccumulator struct {
	pid         uint16
	pts         *int64
	dts         *int64
	payload     []byte
	packetStart int64
	packetIndex int64
	packetCount int
	firstCC     uint8
	lastCC      int
	diagnostics []Diagnostic
}

// PESAssembler reassembles TS packets into complete PES units.
type PESAssembler struct {
	pending map[uint16]*pesAccumulator
}

// NewPESAssembler returns an empty PESAssembler.
func NewPESAssembler() *PESAssembler {
	return &PESAssembler{pending: make(map[uint16]*pesAccumulator)}
}

// Feed processes a TS packet. If a PUSI flag starts a new PES unit and
// there was a previous in-progress unit on the same PID, the completed
// unit is returned.
func (a *PESAssembler) Feed(pkt Packet) *PESUnit {
	if !pkt.HasPayload || pkt.Payload == nil {
		return nil
	}

	var emitted *PESUnit

	if pkt.PayloadUnitStart {
		if acc, ok := a.pending[pkt.PID]; ok {
			emitted = acc.toPESUnit()
		}
		pts, dts, headerLen, err := parsePESHeader(pkt.Payload)
		acc := &pesAccumulator{
			pid:         pkt.PID,
			packetStart: pkt.Offset,
			packetIndex: pkt.Index,
			packetCount: 1,
			firstCC:     pkt.ContinuityCounter,
			lastCC:      int(pkt.ContinuityCounter),
		}
		if err != nil {
			acc.diagnostics = append(acc.diagnostics, Diagnostic{
				Severity: "warning",
				Code:     "missing_pes_header",
				Message:  fmt.Sprintf("PES header parse error: %v", err),
			})
			acc.payload = append(acc.payload, pkt.Payload...)
		} else {
			acc.pts = pts
			acc.dts = dts
			if headerLen < len(pkt.Payload) {
				acc.payload = append(acc.payload, pkt.Payload[headerLen:]...)
			}
		}
		a.pending[pkt.PID] = acc
	} else {
		acc, ok := a.pending[pkt.PID]
		if !ok {
			return nil
		}
		expected := (acc.lastCC + 1) & 0x0F
		if int(pkt.ContinuityCounter) != expected {
			if int(pkt.ContinuityCounter) == acc.lastCC {
				acc.diagnostics = append(acc.diagnostics, Diagnostic{
					Severity: "warning",
					Code:     "continuity_duplicate",
					Message:  fmt.Sprintf("duplicate CC=%d on PID 0x%04X", pkt.ContinuityCounter, pkt.PID),
				})
				return nil
			}
			acc.diagnostics = append(acc.diagnostics, Diagnostic{
				Severity: "warning",
				Code:     "continuity_gap",
				Message:  fmt.Sprintf("CC gap on PID 0x%04X: expected %d, got %d", pkt.PID, expected, pkt.ContinuityCounter),
			})
		}
		acc.lastCC = int(pkt.ContinuityCounter)
		acc.packetCount++
		acc.payload = append(acc.payload, pkt.Payload...)
	}

	return emitted
}

// Flush returns any in-progress PES units.
func (a *PESAssembler) Flush() []PESUnit {
	var units []PESUnit
	for _, acc := range a.pending {
		units = append(units, *acc.toPESUnit())
	}
	a.pending = make(map[uint16]*pesAccumulator)
	return units
}

func (acc *pesAccumulator) toPESUnit() *PESUnit {
	cc := acc.firstCC
	return &PESUnit{
		PID:               acc.pid,
		PTS:               acc.pts,
		DTS:               acc.dts,
		Payload:           acc.payload,
		PacketStart:       acc.packetStart,
		PacketIndex:       acc.packetIndex,
		PacketCount:       acc.packetCount,
		ContinuityCounter: &cc,
		Diagnostics:       acc.diagnostics,
	}
}

// parseTimestamp extracts a 33-bit PTS or DTS value from 5 bytes.
func parseTimestamp(data []byte) int64 {
	ts := int64(data[0]>>1&0x07) << 30
	ts |= int64(data[1]) << 22
	ts |= int64(data[2]>>1) << 15
	ts |= int64(data[3]) << 7
	ts |= int64(data[4] >> 1)
	return ts
}
