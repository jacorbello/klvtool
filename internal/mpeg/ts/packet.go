package ts

import "fmt"

// PacketSize is the fixed size of an MPEG-TS packet in bytes.
const PacketSize = 188

// SyncByte is the synchronization byte that starts every TS packet.
const SyncByte = 0x47

// Diagnostic describes a parsing warning or error.
type Diagnostic struct {
	Severity string // "warning" or "error"
	Code     string // e.g. "continuity_gap", "sync_recovery"
	Message  string
}

// AdaptationField contains parsed adaptation field data.
type AdaptationField struct {
	Length        uint8
	Discontinuity bool
	RandomAccess  bool
	PCR           *int64
}

// Packet represents a single parsed 188-byte TS packet.
type Packet struct {
	Offset            int64
	Index             int64
	PID               uint16
	ContinuityCounter uint8
	PayloadUnitStart  bool
	HasAdaptation     bool
	HasPayload        bool
	Adaptation        *AdaptationField
	Payload           []byte
}

// parseHeader extracts header fields from a 4-byte TS packet header.
// Returns a zero-value Packet if the sync byte is not 0x47.
func parseHeader(header [4]byte) Packet {
	if header[0] != SyncByte {
		return Packet{}
	}
	pusi := header[1]&0x40 != 0
	pid := uint16(header[1]&0x1F)<<8 | uint16(header[2])
	adaptationControl := (header[3] & 0x30) >> 4
	cc := header[3] & 0x0F

	return Packet{
		PID:               pid,
		PayloadUnitStart:  pusi,
		HasAdaptation:     adaptationControl == 2 || adaptationControl == 3,
		HasPayload:        adaptationControl == 1 || adaptationControl == 3,
		ContinuityCounter: cc,
	}
}

// parseAdaptationField parses an adaptation field from raw bytes.
// The input starts at the adaptation_field_length byte.
func parseAdaptationField(data []byte) (AdaptationField, error) {
	if len(data) == 0 {
		return AdaptationField{}, fmt.Errorf("empty adaptation field data")
	}

	af := AdaptationField{Length: data[0]}
	if af.Length == 0 {
		return af, nil
	}
	if int(af.Length) > len(data)-1 {
		return af, fmt.Errorf("adaptation field length %d exceeds available data %d", af.Length, len(data)-1)
	}

	flags := data[1]
	af.Discontinuity = flags&0x80 != 0
	af.RandomAccess = flags&0x40 != 0

	pcrFlag := flags&0x10 != 0
	if pcrFlag && af.Length >= 7 {
		pcr := parsePCR(data[2:8])
		af.PCR = &pcr
	}

	return af, nil
}

// parsePCR extracts the PCR value from 6 bytes.
// Returns the full PCR in 27MHz clock ticks (base*300 + extension).
func parsePCR(data []byte) int64 {
	base := int64(data[0])<<25 | int64(data[1])<<17 | int64(data[2])<<9 | int64(data[3])<<1 | int64(data[4]>>7)
	ext := int64(data[4]&0x01)<<8 | int64(data[5])
	return base*300 + ext
}
