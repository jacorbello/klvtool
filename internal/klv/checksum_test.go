package klv

import "testing"

// TestComputeChecksumReference is the ST 0601 §6.6 reference algorithm
// ported to Go and checked against a hand-built packet.
//
// The checksum is a 16-bit running sum where each byte is shifted left by
// 8*((i+1) mod 2) before adding, i.e. even-index bytes contribute to the
// high byte and odd-index bytes to the low byte.
func TestComputeChecksumKnownVector(t *testing.T) {
	// Three bytes: 0x01, 0x02, 0x03
	// i=0: 0x01 << 8 = 0x0100
	// i=1: 0x02 << 0 = 0x0002
	// i=2: 0x03 << 8 = 0x0300
	// sum = 0x0402
	got := computeChecksum([]byte{0x01, 0x02, 0x03})
	want := uint16(0x0402)
	if got != want {
		t.Errorf("computeChecksum = 0x%04X, want 0x%04X", got, want)
	}
}

func TestComputeChecksumEmpty(t *testing.T) {
	got := computeChecksum(nil)
	if got != 0 {
		t.Errorf("computeChecksum(nil) = 0x%04X, want 0x0000", got)
	}
}

func TestComputeChecksumWraps(t *testing.T) {
	// All 0xFF bytes should wrap uint16.
	buf := make([]byte, 4)
	for i := range buf {
		buf[i] = 0xFF
	}
	// i=0: 0xFF00
	// i=1: 0x00FF
	// i=2: 0xFF00
	// i=3: 0x00FF
	// sum = 0x1FFFE & 0xFFFF = 0xFFFE
	got := computeChecksum(buf)
	want := uint16(0xFFFE)
	if got != want {
		t.Errorf("computeChecksum(4x0xFF) = 0x%04X, want 0x%04X", got, want)
	}
}
