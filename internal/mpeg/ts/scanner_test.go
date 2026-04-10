package ts

import (
	"bytes"
	"io"
	"testing"
)

// buildPacket creates a valid 188-byte TS packet with the given PID, CC, PUSI,
// and payload. The adaptation control is set to payload-only (0x01).
func buildPacket(pid uint16, cc uint8, pusi bool, payload []byte) []byte {
	pkt := make([]byte, PacketSize)
	pkt[0] = SyncByte
	pkt[1] = byte(pid>>8) & 0x1F
	if pusi {
		pkt[1] |= 0x40
	}
	pkt[2] = byte(pid & 0xFF)
	pkt[3] = 0x10 | (cc & 0x0F) // adaptation_control=01 (payload only)
	copy(pkt[4:], payload)
	return pkt
}

func TestScannerReadsSequentialPackets(t *testing.T) {
	p1 := buildPacket(0x100, 0, true, []byte{0xAA, 0xBB})
	p2 := buildPacket(0x200, 3, false, []byte{0xCC})
	r := bytes.NewReader(append(p1, p2...))

	s := NewPacketScanner(r, ScanConfig{})
	pkt1, err := s.Next()
	if err != nil {
		t.Fatalf("packet 1: unexpected error: %v", err)
	}
	if pkt1.PID != 0x100 {
		t.Errorf("packet 1 PID = 0x%X, want 0x100", pkt1.PID)
	}
	if pkt1.Offset != 0 {
		t.Errorf("packet 1 Offset = %d, want 0", pkt1.Offset)
	}
	if pkt1.Index != 0 {
		t.Errorf("packet 1 Index = %d, want 0", pkt1.Index)
	}
	if !pkt1.PayloadUnitStart {
		t.Error("packet 1 PayloadUnitStart = false, want true")
	}

	pkt2, err := s.Next()
	if err != nil {
		t.Fatalf("packet 2: unexpected error: %v", err)
	}
	if pkt2.PID != 0x200 {
		t.Errorf("packet 2 PID = 0x%X, want 0x200", pkt2.PID)
	}
	if pkt2.Offset != PacketSize {
		t.Errorf("packet 2 Offset = %d, want %d", pkt2.Offset, PacketSize)
	}
	if pkt2.Index != 1 {
		t.Errorf("packet 2 Index = %d, want 1", pkt2.Index)
	}
	if pkt2.PayloadUnitStart {
		t.Error("packet 2 PayloadUnitStart = true, want false")
	}

	_, err = s.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF after last packet, got %v", err)
	}
}

func TestScannerEmptyReader(t *testing.T) {
	s := NewPacketScanner(bytes.NewReader(nil), ScanConfig{})
	_, err := s.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestScannerTrailingBytesError(t *testing.T) {
	data := append(buildPacket(0x100, 0, false, nil), 0x00, 0x00, 0x00)
	s := NewPacketScanner(bytes.NewReader(data), ScanConfig{})

	_, err := s.Next()
	if err != nil {
		t.Fatalf("first packet: unexpected error: %v", err)
	}

	_, err = s.Next()
	if err == nil {
		t.Fatal("expected error for trailing bytes")
	}
	if err == io.EOF {
		t.Error("expected non-EOF error for trailing bytes, got io.EOF")
	}
}
