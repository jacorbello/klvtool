package ts

import "testing"

func TestParsePESHeaderExtractsPTS(t *testing.T) {
	pesHeader := []byte{
		0x00, 0x00, 0x01, 0xBD,
		0x00, 0x08,
		0x80, 0x80, 0x05,
		0x21, 0x00, 0x01, 0x00, 0x01, // PTS = 0
	}
	pts, dts, headerLen, err := parsePESHeader(pesHeader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pts == nil || *pts != 0 {
		t.Errorf("PTS = %v, want 0", pts)
	}
	if dts != nil {
		t.Errorf("DTS = %d, want nil", *dts)
	}
	if headerLen <= 0 {
		t.Errorf("headerLen = %d, want > 0", headerLen)
	}
}

func TestParsePESHeaderExtractsPTSAndDTS(t *testing.T) {
	pesHeader := []byte{
		0x00, 0x00, 0x01, 0xBD,
		0x00, 0x0D,
		0x80, 0xC0, 0x0A,
		0x31, 0x00, 0x01, 0x00, 0x01, // PTS=0
		0x11, 0x00, 0x01, 0x00, 0x01, // DTS=0
	}
	pts, dts, _, err := parsePESHeader(pesHeader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pts == nil || *pts != 0 {
		t.Errorf("PTS = %v", pts)
	}
	if dts == nil || *dts != 0 {
		t.Errorf("DTS = %v", dts)
	}
}

func TestParsePESHeaderNoPTSNoDTS(t *testing.T) {
	pesHeader := []byte{0x00, 0x00, 0x01, 0xBD, 0x00, 0x03, 0x80, 0x00, 0x00}
	pts, dts, headerLen, err := parsePESHeader(pesHeader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pts != nil || dts != nil {
		t.Error("expected nil PTS/DTS")
	}
	if headerLen != 9 {
		t.Errorf("headerLen = %d, want 9", headerLen)
	}
}

func TestParsePESHeaderInvalidStartCode(t *testing.T) {
	pesHeader := []byte{0x00, 0x00, 0x00, 0xBD, 0x00, 0x03, 0x80, 0x00, 0x00}
	if _, _, _, err := parsePESHeader(pesHeader); err == nil {
		t.Fatal("expected error for invalid start code")
	}
}

func TestParsePESHeaderTooShort(t *testing.T) {
	if _, _, _, err := parsePESHeader([]byte{0x00, 0x00, 0x01}); err == nil {
		t.Fatal("expected error for truncated PES header")
	}
}

// TestParsePESHeaderRejectsOverreachingHeaderDataLength verifies that a
// PES header whose declared pes_header_data_length extends beyond the
// bytes actually available fails cleanly rather than returning a
// headerLen that exceeds the buffer.
func TestParsePESHeaderRejectsOverreachingHeaderDataLength(t *testing.T) {
	// 9-byte header claiming 200 bytes of optional header data follow.
	data := []byte{
		0x00, 0x00, 0x01, 0xBD, // start code + stream_id
		0x00, 0xD0, // PES packet length (not used for bounds)
		0x80,       // flags
		0x00,       // PTS/DTS flags = none
		0xC8,       // pes_header_data_length = 200
	}
	_, _, _, err := parsePESHeader(data)
	if err == nil {
		t.Fatal("expected error for overreaching header_data_length")
	}
}

func TestPESAssemblerEmitsUnitOnNextPUSI(t *testing.T) {
	asm := NewPESAssembler()

	pesHeader1 := []byte{
		0x00, 0x00, 0x01, 0xBD,
		0x00, 0x0B,
		0x80, 0x80, 0x05,
		0x21, 0x00, 0x01, 0x00, 0x01,
		0xAA, 0xBB,
	}
	pkt1 := Packet{
		PID: 0x0300, PayloadUnitStart: true, HasPayload: true, Payload: pesHeader1,
		Offset: 0, Index: 0, ContinuityCounter: 0,
	}
	if asm.Feed(pkt1) != nil {
		t.Error("first PUSI should not emit a unit")
	}

	pkt2 := Packet{
		PID: 0x0300, HasPayload: true, Payload: []byte{0xCC, 0xDD},
		Offset: 188, Index: 1, ContinuityCounter: 1,
	}
	if asm.Feed(pkt2) != nil {
		t.Error("continuation should not emit a unit")
	}

	pesHeader2 := []byte{
		0x00, 0x00, 0x01, 0xBD,
		0x00, 0x05, 0x80, 0x00, 0x00,
		0xEE,
	}
	pkt3 := Packet{
		PID: 0x0300, PayloadUnitStart: true, HasPayload: true, Payload: pesHeader2,
		Offset: 376, Index: 2, ContinuityCounter: 2,
	}
	unit := asm.Feed(pkt3)
	if unit == nil {
		t.Fatal("second PUSI should emit the previous unit")
	}
	if unit.PID != 0x0300 {
		t.Errorf("PID = 0x%04X", unit.PID)
	}
	if unit.PTS == nil || *unit.PTS != 0 {
		t.Errorf("PTS = %v, want 0", unit.PTS)
	}
	if unit.PacketCount != 2 {
		t.Errorf("PacketCount = %d, want 2", unit.PacketCount)
	}
	if unit.PacketStart != 0 {
		t.Errorf("PacketStart = %d, want 0", unit.PacketStart)
	}
	if unit.PacketIndex != 0 {
		t.Errorf("PacketIndex = %d, want 0", unit.PacketIndex)
	}
	if len(unit.Payload) != 4 {
		t.Fatalf("Payload length = %d, want 4", len(unit.Payload))
	}
	if unit.Payload[0] != 0xAA || unit.Payload[3] != 0xDD {
		t.Errorf("Payload = %X", unit.Payload)
	}
}

func TestPESAssemblerFlushEmitsFinalUnit(t *testing.T) {
	asm := NewPESAssembler()
	pesHeader := []byte{
		0x00, 0x00, 0x01, 0xBD, 0x00, 0x05, 0x80, 0x00, 0x00,
		0xFF, 0xFE,
	}
	asm.Feed(Packet{PID: 0x0300, PayloadUnitStart: true, HasPayload: true, Payload: pesHeader, ContinuityCounter: 0})
	units := asm.Flush()
	if len(units) != 1 {
		t.Fatalf("Flush returned %d, want 1", len(units))
	}
}

func TestPESAssemblerMultiPIDInterleaving(t *testing.T) {
	asm := NewPESAssembler()
	pesHeaderA := []byte{0x00, 0x00, 0x01, 0xBD, 0x00, 0x05, 0x80, 0x00, 0x00, 0xAA}
	pesHeaderB := []byte{0x00, 0x00, 0x01, 0xBD, 0x00, 0x05, 0x80, 0x00, 0x00, 0xBB}
	asm.Feed(Packet{PID: 0x0300, PayloadUnitStart: true, HasPayload: true, Payload: pesHeaderA, ContinuityCounter: 0})
	asm.Feed(Packet{PID: 0x0400, PayloadUnitStart: true, HasPayload: true, Payload: pesHeaderB, ContinuityCounter: 0})
	units := asm.Flush()
	if len(units) != 2 {
		t.Fatalf("Flush returned %d, want 2", len(units))
	}
	pids := map[uint16]bool{}
	for _, u := range units {
		pids[u.PID] = true
	}
	if !pids[0x0300] || !pids[0x0400] {
		t.Errorf("expected PIDs 0x0300 and 0x0400, got %v", pids)
	}
}

func TestPESAssemblerContinuityGapDiagnostic(t *testing.T) {
	asm := NewPESAssembler()
	pesHeader := []byte{0x00, 0x00, 0x01, 0xBD, 0x00, 0x05, 0x80, 0x00, 0x00, 0xAA}
	asm.Feed(Packet{PID: 0x0300, PayloadUnitStart: true, HasPayload: true, Payload: pesHeader, ContinuityCounter: 0})
	asm.Feed(Packet{PID: 0x0300, HasPayload: true, Payload: []byte{0xBB}, ContinuityCounter: 2})

	pesHeader2 := []byte{0x00, 0x00, 0x01, 0xBD, 0x00, 0x05, 0x80, 0x00, 0x00, 0xCC}
	unit := asm.Feed(Packet{PID: 0x0300, PayloadUnitStart: true, HasPayload: true, Payload: pesHeader2, ContinuityCounter: 3})
	if unit == nil {
		t.Fatal("expected unit emitted")
	}
	found := false
	for _, d := range unit.Diagnostics {
		if d.Code == "continuity_gap" {
			found = true
		}
	}
	if !found {
		t.Error("expected continuity_gap diagnostic")
	}
}
