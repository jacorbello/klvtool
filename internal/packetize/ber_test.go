package packetize

import "testing"

func TestDecodeBERLengthHandlesShortAndLongForm(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		wantLen   int
		wantRead  int
		wantError bool
	}{
		{name: "short form", input: []byte{0x7f}, wantLen: 127, wantRead: 1},
		{name: "long form one byte", input: []byte{0x81, 0x80}, wantLen: 128, wantRead: 2},
		{name: "long form two bytes", input: []byte{0x82, 0x01, 0x00}, wantLen: 256, wantRead: 3},
		{name: "truncated long form", input: []byte{0x82, 0x01}, wantError: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotLen, gotRead, err := decodeBERLength(tc.input)
			if tc.wantError {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeBERLength error: %v", err)
			}
			if gotLen != tc.wantLen || gotRead != tc.wantRead {
				t.Fatalf("want (%d,%d), got (%d,%d)", tc.wantLen, tc.wantRead, gotLen, gotRead)
			}
		})
	}
}

func TestDecodeBERLengthRejectsOverflowingLongForm(t *testing.T) {
	overflowing := []byte{0x88, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	if _, _, err := decodeBERLength(overflowing); err == nil {
		t.Fatal("expected overflow error")
	}
}
