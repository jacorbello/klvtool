package h264

import (
	"reflect"
	"testing"
)

func TestIterateNALUnitsBehavior(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  []uint8 // nal unit types in order
	}{
		{
			name:  "empty input yields no units",
			input: nil,
			want:  nil,
		},
		{
			name:  "no start code yields no units",
			input: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
			want:  nil,
		},
		{
			name:  "single 4-byte start code then IDR",
			input: []byte{0x00, 0x00, 0x00, 0x01, 0x65, 0xAA, 0xBB},
			want:  []uint8{NALIDR},
		},
		{
			name:  "single 3-byte start code then AUD",
			input: []byte{0x00, 0x00, 0x01, 0x09, 0xF0},
			want:  []uint8{NALAUD},
		},
		{
			name: "mixed 3 and 4 byte start codes with SPS, PPS, IDR",
			input: []byte{
				0x00, 0x00, 0x00, 0x01, 0x67, 0x42, 0xE0, 0x1E, // SPS
				0x00, 0x00, 0x01, 0x68, 0xCE, 0x3C, 0x80, // PPS
				0x00, 0x00, 0x00, 0x01, 0x65, 0x88, 0x80, // IDR
			},
			want: []uint8{NALSPS, NALPPS, NALIDR},
		},
		{
			name: "non-IDR slice + AUD",
			input: []byte{
				0x00, 0x00, 0x00, 0x01, 0x09, 0xF0, // AUD
				0x00, 0x00, 0x00, 0x01, 0x41, 0x9A, 0x24, // non-IDR slice (type 1)
			},
			want: []uint8{NALAUD, NALSlice},
		},
		{
			name:  "leading bytes before start code are skipped",
			input: []byte{0xFF, 0xFF, 0x00, 0x00, 0x00, 0x01, 0x67, 0x42},
			want:  []uint8{NALSPS},
		},
		{
			name: "two NAL units back to back, type taken from first byte",
			input: []byte{
				0x00, 0x00, 0x01, 0x67, 0x42, // SPS
				0x00, 0x00, 0x01, 0x68, 0xCE, // PPS
			},
			want: []uint8{NALSPS, NALPPS},
		},
		{
			name:  "forbidden bit set still parses nal_unit_type from low 5 bits",
			input: []byte{0x00, 0x00, 0x00, 0x01, 0xE5, 0xAA}, // 0xE5 & 0x1F = 5 (IDR)
			want:  []uint8{NALIDR},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []uint8
			IterateNALUnits(tt.input, func(nalType uint8, body []byte) {
				got = append(got, nalType)
			})
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NAL types = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIterateNALUnitsBodyExcludesHeaderByte(t *testing.T) {
	// Start code + header byte 0x65 (IDR) + body bytes 0xAA 0xBB.
	input := []byte{0x00, 0x00, 0x00, 0x01, 0x65, 0xAA, 0xBB}

	var gotType uint8
	var gotBody []byte
	IterateNALUnits(input, func(nalType uint8, body []byte) {
		gotType = nalType
		gotBody = append([]byte(nil), body...)
	})

	if gotType != NALIDR {
		t.Fatalf("nalType = %d, want %d", gotType, NALIDR)
	}
	wantBody := []byte{0xAA, 0xBB}
	if !reflect.DeepEqual(gotBody, wantBody) {
		t.Errorf("body = %v, want %v", gotBody, wantBody)
	}
}

func TestIterateNALUnitsStopsCleanlyAtEOF(t *testing.T) {
	// Two units; the second has no trailing start code.
	input := []byte{
		0x00, 0x00, 0x00, 0x01, 0x67, 0x42, 0xE0,
		0x00, 0x00, 0x00, 0x01, 0x65, 0x88, 0x80, 0xAA,
	}
	var calls int
	IterateNALUnits(input, func(nalType uint8, body []byte) {
		calls++
	})
	if calls != 2 {
		t.Errorf("calls = %d, want 2", calls)
	}
}
