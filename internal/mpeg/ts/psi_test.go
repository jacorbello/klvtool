package ts

import (
	"testing"
)

func TestPSIParserExtractsPMTPIDsFromPAT(t *testing.T) {
	parser := NewPSIParser()

	patSection := []byte{
		0x00,       // pointer field
		0x00,       // table_id = PAT
		0xB0, 0x0D, // section_syntax=1, length=13
		0x00, 0x01, // transport_stream_id=1
		0xC1,       // version=0, current=1
		0x00, 0x00, // section_number, last_section_number
		0x00, 0x01, // program_number=1
		0xF0, 0x00, // PMT PID=0x1000
		0x00, 0x00, 0x00, 0x00, // CRC
	}

	payload := make([]byte, len(patSection))
	copy(payload, patSection)
	packet := Packet{
		PID:              0x0000,
		PayloadUnitStart: true,
		HasPayload:       true,
		Payload:          payload,
	}

	changed := parser.Feed(packet)
	if !changed {
		t.Error("Feed returned changed=false, want true")
	}

	if !parser.IsPMTPID(0x1000) {
		t.Error("PID 0x1000 should be recognized as PMT PID")
	}
}

func TestPSIParserIgnoresNonPATPackets(t *testing.T) {
	parser := NewPSIParser()
	packet := Packet{
		PID:              0x0100,
		PayloadUnitStart: true,
		HasPayload:       true,
		Payload:          []byte{0x00, 0x00},
	}
	if parser.Feed(packet) {
		t.Error("Feed returned changed=true for non-PAT packet")
	}
}
