package ts

import (
	"testing"
)

func TestParseHeaderExtractsPIDAndFlags(t *testing.T) {
	tests := []struct {
		name   string
		header [4]byte
		want   Packet
	}{
		{
			name:   "null packet PID 0x1FFF",
			header: [4]byte{0x47, 0x1F, 0xFF, 0x10},
			want: Packet{
				PID:                0x1FFF,
				ContinuityCounter:  0,
				PayloadUnitStart:   false,
				HasAdaptation:      false,
				HasPayload:         true,
			},
		},
		{
			name:   "PAT PID 0 with PUSI",
			header: [4]byte{0x47, 0x40, 0x00, 0x10},
			want: Packet{
				PID:                0x0000,
				ContinuityCounter:  0,
				PayloadUnitStart:   true,
				HasAdaptation:      false,
				HasPayload:         true,
			},
		},
		{
			name:   "PID 256 with adaptation and payload, CC=5",
			header: [4]byte{0x47, 0x01, 0x00, 0x35},
			want: Packet{
				PID:                0x0100,
				ContinuityCounter:  5,
				HasAdaptation:      true,
				HasPayload:         true,
			},
		},
		{
			name:   "adaptation only no payload",
			header: [4]byte{0x47, 0x01, 0x00, 0x20},
			want: Packet{
				PID:                0x0100,
				HasAdaptation:      true,
				HasPayload:         false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseHeader(tt.header)
			if got.PID != tt.want.PID {
				t.Errorf("PID = %d, want %d", got.PID, tt.want.PID)
			}
			if got.PayloadUnitStart != tt.want.PayloadUnitStart {
				t.Errorf("PayloadUnitStart = %v, want %v", got.PayloadUnitStart, tt.want.PayloadUnitStart)
			}
			if got.HasAdaptation != tt.want.HasAdaptation {
				t.Errorf("HasAdaptation = %v, want %v", got.HasAdaptation, tt.want.HasAdaptation)
			}
			if got.HasPayload != tt.want.HasPayload {
				t.Errorf("HasPayload = %v, want %v", got.HasPayload, tt.want.HasPayload)
			}
			if got.ContinuityCounter != tt.want.ContinuityCounter {
				t.Errorf("ContinuityCounter = %d, want %d", got.ContinuityCounter, tt.want.ContinuityCounter)
			}
		})
	}
}

func TestParseHeaderRejectsBadSyncByte(t *testing.T) {
	header := [4]byte{0x00, 0x40, 0x00, 0x10}
	got := parseHeader(header)
	if got.PID != 0 || got.HasPayload {
		t.Errorf("expected zero-value Packet for bad sync byte, got PID=%d HasPayload=%v", got.PID, got.HasPayload)
	}
}
