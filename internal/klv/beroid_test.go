package klv

import "testing"

func TestDecodeBEROIDShortForm(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantTag int
		wantN   int
	}{
		{"tag 1", []byte{0x01}, 1, 1},
		{"tag 65", []byte{0x41}, 65, 1},
		{"tag 127", []byte{0x7F}, 127, 1},
		{"trailing bytes ignored", []byte{0x05, 0xFF, 0xFF}, 5, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, n, err := decodeBEROID(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantTag {
				t.Errorf("tag = %d, want %d", got, tt.wantTag)
			}
			if n != tt.wantN {
				t.Errorf("n = %d, want %d", n, tt.wantN)
			}
		})
	}
}

func TestDecodeBEROIDLongForm(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantTag int
		wantN   int
	}{
		// 128 = 0x80 → 0x81 0x00
		{"tag 128", []byte{0x81, 0x00}, 128, 2},
		// 143 = 0x8F → 0x81 0x0F
		{"tag 143", []byte{0x81, 0x0F}, 143, 2},
		// 16384 = 0x4000 → 0x81 0x80 0x00 (three bytes)
		{"tag 16384", []byte{0x81, 0x80, 0x00}, 16384, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, n, err := decodeBEROID(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantTag {
				t.Errorf("tag = %d, want %d", got, tt.wantTag)
			}
			if n != tt.wantN {
				t.Errorf("n = %d, want %d", n, tt.wantN)
			}
		})
	}
}

func TestDecodeBEROIDErrors(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"empty", []byte{}},
		{"continuation without follow-up", []byte{0x81}},
		{"two-byte continuation truncated", []byte{0x81, 0x81}},
		{"9-byte with high-bit continuation", []byte{0x81, 0x81, 0x81, 0x81, 0x81, 0x81, 0x81, 0x81, 0x81}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := decodeBEROID(tt.input)
			if err == nil {
				t.Errorf("expected error for %v", tt.input)
			}
		})
	}
}
