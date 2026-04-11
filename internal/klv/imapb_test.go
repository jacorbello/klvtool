package klv

import (
	"math"
	"testing"
)

// Test vectors derived from MISB ST 1201 worked examples. IMAPB maps a
// floating-point value in [a, b] to an unsigned integer in [0, 2^(8L)-1]
// with the lowest bit representing a quantum computed from the range.
func TestIMAPBRoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		a, b       float64
		length     int
		val        float64
		tolerance  float64
	}{
		{"0 to 360 degrees, 2 bytes", 0, 360, 2, 159.9744, 0.01},
		{"-90 to 90 degrees, 4 bytes", -90, 90, 4, 42.123456, 0.00001},
		{"-900 to 19000 meters, 2 bytes", -900, 19000, 2, 1500.0, 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := toIMAPB(tt.a, tt.b, tt.length, tt.val)
			if err != nil {
				t.Fatalf("toIMAPB error: %v", err)
			}
			decoded, err := fromIMAPB(tt.a, tt.b, tt.length, encoded)
			if err != nil {
				t.Fatalf("fromIMAPB error: %v", err)
			}
			if math.Abs(decoded-tt.val) > tt.tolerance {
				t.Errorf("round-trip = %v, want %v (tolerance %v)", decoded, tt.val, tt.tolerance)
			}
		})
	}
}

func TestIMAPBEndpoints(t *testing.T) {
	// IMAPB(0..360, L=2): encoded 0 → 0.0, encoded maxUInt → 360.0 (approx).
	lo, err := fromIMAPB(0, 360, 2, []byte{0x00, 0x00})
	if err != nil {
		t.Fatalf("fromIMAPB lo: %v", err)
	}
	if math.Abs(lo-0.0) > 1e-9 {
		t.Errorf("lo endpoint = %v, want 0.0", lo)
	}
	hi, err := fromIMAPB(0, 360, 2, []byte{0xFF, 0xFF})
	if err != nil {
		t.Fatalf("fromIMAPB hi: %v", err)
	}
	if math.Abs(hi-360.0) > 0.01 {
		t.Errorf("hi endpoint = %v, want ~360.0", hi)
	}
}
