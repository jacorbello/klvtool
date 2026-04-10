package ts

import (
	"bytes"
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

func TestDiscoverStreamsFromSyntheticFile(t *testing.T) {
	var buf bytes.Buffer

	patSection := []byte{
		0x00, 0x00, 0xB0, 0x0D,
		0x00, 0x01, 0xC1, 0x00, 0x00,
		0x00, 0x01, 0xF0, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	patPkt := buildPacket(0x0000, 0, true, patSection)
	buf.Write(patPkt)

	pmtSection := []byte{
		0x00, 0x02, 0xB0, 0x12,
		0x00, 0x01, 0xC1, 0x00, 0x00,
		0xE1, 0x00, 0xF0, 0x00,
		0x06, 0xE3, 0x00, 0xF0, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	pmtPkt := buildPacket(0x1000, 0, true, pmtSection)
	buf.Write(pmtPkt)

	dataPkt := buildPacket(0x0300, 0, false, []byte{0xFF})
	buf.Write(dataPkt)

	r := bytes.NewReader(buf.Bytes())
	table, err := DiscoverStreams(r)
	if err != nil {
		t.Fatalf("DiscoverStreams: %v", err)
	}

	streams, ok := table.Programs[1]
	if !ok {
		t.Fatal("Program 1 not found")
	}
	if len(streams) != 1 {
		t.Fatalf("stream count = %d, want 1", len(streams))
	}
	if streams[0].PID != 0x0300 {
		t.Errorf("stream PID = 0x%04X, want 0x0300", streams[0].PID)
	}
	if streams[0].StreamType != 0x06 {
		t.Errorf("stream type = 0x%02X, want 0x06", streams[0].StreamType)
	}
}

// TestDiscoverStreamsWaitsForValidPMTParse verifies that DiscoverStreams
// does not terminate early when a payload-unit-start packet arrives on a
// PMT PID with malformed contents. Only a Feed call that actually
// updates the StreamTable should mark the PMT as complete.
func TestDiscoverStreamsWaitsForValidPMTParse(t *testing.T) {
	var file bytes.Buffer

	// PAT: program 1 → PMT PID 0x1000.
	patPkt := buildPacket(0x0000, 0, true, []byte{
		0x00, 0x00, 0xB0, 0x0D,
		0x00, 0x01, 0xC1, 0x00, 0x00,
		0x00, 0x01, 0xF0, 0x00,
		0x00, 0x00, 0x00, 0x00,
	})
	file.Write(patPkt)

	// First packet on PMT PID: PUSI=1 but malformed — wrong table_id.
	// The prior implementation would mark the PMT "parsed" from PUSI
	// alone and stop discovery. The fixed implementation must keep
	// scanning until a real PMT arrives.
	garbagePMT := buildPacket(0x1000, 0, true, []byte{
		0x00,       // pointer field
		0xFF,       // table_id = 0xFF (not PMT 0x02)
		0xB0, 0x08, // fake section_length
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	})
	file.Write(garbagePMT)

	// Real PMT section arrives later.
	realPMT := buildPacket(0x1000, 1, true, []byte{
		0x00, 0x02, 0xB0, 0x12,
		0x00, 0x01, 0xC1, 0x00, 0x00,
		0xE1, 0x00, 0xF0, 0x00,
		0x06, 0xE3, 0x00, 0xF0, 0x00,
		0x00, 0x00, 0x00, 0x00,
	})
	file.Write(realPMT)

	r := bytes.NewReader(file.Bytes())
	table, err := DiscoverStreams(r)
	if err != nil {
		t.Fatalf("DiscoverStreams: %v", err)
	}
	streams, ok := table.Programs[1]
	if !ok {
		t.Fatal("Program 1 not found after valid PMT — discovery stopped too early on garbage PMT")
	}
	if len(streams) != 1 || streams[0].PID != 0x0300 {
		t.Errorf("unexpected streams: %+v", streams)
	}
}

// TestPSIParserReassemblesMultiPacketPMT verifies that a PMT section
// which spans two TS packets (PUSI start + continuation) is correctly
// reassembled. Real MPEG-TS streams produce multi-packet PMTs once they
// carry many streams or descriptors; a parser that only looks at the
// first packet silently drops them.
func TestPSIParserReassemblesMultiPacketPMT(t *testing.T) {
	parser := NewPSIParser()

	// Feed a PAT announcing program 1 → PMT PID 0x1000.
	patSection := []byte{
		0x00,       // pointer field
		0x00,       // table_id
		0xB0, 0x0D, // section_syntax=1, length=13
		0x00, 0x01, 0xC1, 0x00, 0x00,
		0x00, 0x01, 0xF0, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	parser.Feed(Packet{
		PID: pidPAT, PayloadUnitStart: true, HasPayload: true,
		Payload: append([]byte(nil), patSection...),
	})

	// Build a PMT with 40 streams so the section exceeds a single
	// 184-byte TS payload and must span two packets.
	const numStreams = 40
	// PMT layout after table_id:
	//   [section_syntax+length] [program_number] [version/current]
	//   [section_number] [last_section_number] [PCR_PID] [program_info_length]
	//   [N × 5-byte stream entries] [CRC32]
	// Fixed overhead after table_id = 11 bytes + 4 CRC = 15 bytes.
	// section_length counts from byte 3 to end-of-CRC.
	entriesLen := numStreams * 5
	sectionBodyLen := 9 + entriesLen + 4 // 9 bytes from program_number through program_info_length, plus entries, plus CRC
	sectionLength := sectionBodyLen      // section_length = bytes after the length field itself through end of CRC

	section := make([]byte, 0, 3+sectionLength)
	section = append(section,
		0x02, // table_id = PMT
		byte(0xB0|((sectionLength>>8)&0x0F)),
		byte(sectionLength&0xFF),
		0x00, 0x01, // program_number = 1
		0xC1,       // version=0, current_next=1
		0x00, 0x00, // section_number, last_section_number
		0xE1, 0x00, // PCR_PID = 0x0100
		0xF0, 0x00, // program_info_length = 0
	)
	for i := 0; i < numStreams; i++ {
		pid := uint16(0x0200 + i)
		section = append(section,
			0x06,                      // stream_type = private data
			0xE0|byte(pid>>8)&0x1F,    // reserved + PID high
			byte(pid),                 // PID low
			0xF0, 0x00,                // ES_info_length = 0
		)
	}
	// Placeholder CRC32 (parser does not verify CRC).
	section = append(section, 0x00, 0x00, 0x00, 0x00)

	// Prepend pointer_field and split across two TS packets. TS payload
	// capacity without adaptation field is 184 bytes; the first packet's
	// payload carries pointer_field + first 183 section bytes, the second
	// packet's payload carries the rest.
	firstPayload := make([]byte, 0, 184)
	firstPayload = append(firstPayload, 0x00) // pointer_field
	firstChunk := 183
	if firstChunk > len(section) {
		firstChunk = len(section)
	}
	firstPayload = append(firstPayload, section[:firstChunk]...)

	restPayload := section[firstChunk:]
	if len(restPayload) == 0 {
		t.Fatalf("section fits in one packet (%d bytes) — test precondition failed", len(section))
	}

	parser.Feed(Packet{
		PID: 0x1000, PayloadUnitStart: true, HasPayload: true,
		Payload: firstPayload,
	})
	changed := parser.Feed(Packet{
		PID: 0x1000, PayloadUnitStart: false, HasPayload: true, ContinuityCounter: 1,
		Payload: append([]byte(nil), restPayload...),
	})
	if !changed {
		t.Fatal("second packet should complete the PMT section")
	}

	table := parser.Table()
	streams, ok := table.Programs[1]
	if !ok {
		t.Fatal("Program 1 not found")
	}
	if len(streams) != numStreams {
		t.Fatalf("stream count = %d, want %d", len(streams), numStreams)
	}
	// Spot-check a few entries.
	if streams[0].PID != 0x0200 {
		t.Errorf("streams[0].PID = 0x%04X, want 0x0200", streams[0].PID)
	}
	if streams[numStreams-1].PID != uint16(0x0200+numStreams-1) {
		t.Errorf("last stream PID = 0x%04X, want 0x%04X",
			streams[numStreams-1].PID, uint16(0x0200+numStreams-1))
	}
}
