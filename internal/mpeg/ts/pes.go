package ts

import "fmt"

// PESUnit represents a reassembled Packetized Elementary Stream unit.
type PESUnit struct {
	PID         uint16
	PTS         *int64
	DTS         *int64
	Payload     []byte
	PacketStart int64
	PacketIndex int64
	PacketCount int
	Diagnostics []Diagnostic
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

	if ptsDTSFlags == 0x02 {
		if len(data) < 14 {
			return nil, nil, 0, fmt.Errorf("PES header too short for PTS: %d bytes", len(data))
		}
		ptsVal := parseTimestamp(data[9:14])
		pts = &ptsVal
	} else if ptsDTSFlags == 0x03 {
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

// parseTimestamp extracts a 33-bit PTS or DTS value from 5 bytes.
func parseTimestamp(data []byte) int64 {
	ts := int64(data[0]>>1&0x07) << 30
	ts |= int64(data[1]) << 22
	ts |= int64(data[2]>>1) << 15
	ts |= int64(data[3]) << 7
	ts |= int64(data[4] >> 1)
	return ts
}
