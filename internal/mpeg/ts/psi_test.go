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

func TestPSIParserExtractsStreamsFromPMT(t *testing.T) {
	parser := NewPSIParser()

	patSection := []byte{
		0x00, 0x00, 0xB0, 0x0D,
		0x00, 0x01, 0xC1, 0x00, 0x00,
		0x00, 0x01, 0xF0, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	patPkt := Packet{PID: 0x0000, PayloadUnitStart: true, HasPayload: true}
	patPayload := make([]byte, len(patSection))
	copy(patPayload, patSection)
	patPkt.Payload = patPayload
	parser.Feed(patPkt)

	pmtSection := []byte{
		0x00, 0x02, 0xB0, 0x17,
		0x00, 0x01, 0xC1, 0x00, 0x00,
		0xE1, 0x00, 0xF0, 0x00,
		0x06, 0xE3, 0x00, 0xF0, 0x00,
		0x1B, 0xE1, 0x00, 0xF0, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	pmtPkt := Packet{PID: 0x1000, PayloadUnitStart: true, HasPayload: true}
	pmtPayload := make([]byte, len(pmtSection))
	copy(pmtPayload, pmtSection)
	pmtPkt.Payload = pmtPayload

	if !parser.Feed(pmtPkt) {
		t.Error("Feed returned changed=false for PMT, want true")
	}

	table := parser.Table()
	streams, ok := table.Programs[1]
	if !ok {
		t.Fatal("Program 1 not found")
	}
	if len(streams) != 2 {
		t.Fatalf("stream count = %d, want 2", len(streams))
	}
	if streams[0].StreamType != 0x06 {
		t.Errorf("stream[0].StreamType = 0x%02X, want 0x06", streams[0].StreamType)
	}
	if streams[0].PID != 0x0300 {
		t.Errorf("stream[0].PID = 0x%04X, want 0x0300", streams[0].PID)
	}
	if streams[0].ProgramNum != 1 {
		t.Errorf("stream[0].ProgramNum = %d, want 1", streams[0].ProgramNum)
	}
	if streams[1].StreamType != 0x1B {
		t.Errorf("stream[1].StreamType = 0x%02X, want 0x1B", streams[1].StreamType)
	}
	if streams[1].PID != 0x0100 {
		t.Errorf("stream[1].PID = 0x%04X, want 0x0100", streams[1].PID)
	}
}

func TestPSIParserMultiProgramPAT(t *testing.T) {
	parser := NewPSIParser()
	patSection := []byte{
		0x00, 0x00, 0xB0, 0x11,
		0x00, 0x01, 0xC1, 0x00, 0x00,
		0x00, 0x01, 0xF0, 0x00,
		0x00, 0x02, 0xF1, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	pkt := Packet{PID: 0x0000, PayloadUnitStart: true, HasPayload: true}
	payload := make([]byte, len(patSection))
	copy(payload, patSection)
	pkt.Payload = payload
	parser.Feed(pkt)

	if !parser.IsPMTPID(0x1000) {
		t.Error("0x1000 should be PMT PID")
	}
	if !parser.IsPMTPID(0x1100) {
		t.Error("0x1100 should be PMT PID")
	}
}
