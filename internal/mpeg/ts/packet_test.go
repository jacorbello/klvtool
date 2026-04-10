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
				PID:               0x1FFF,
				ContinuityCounter: 0,
				PayloadUnitStart:  false,
				HasAdaptation:     false,
				HasPayload:        true,
			},
		},
		{
			name:   "PAT PID 0 with PUSI",
			header: [4]byte{0x47, 0x40, 0x00, 0x10},
			want: Packet{
				PID:               0x0000,
				ContinuityCounter: 0,
				PayloadUnitStart:  true,
				HasAdaptation:     false,
				HasPayload:        true,
			},
		},
		{
			name:   "PID 256 with adaptation and payload, CC=5",
			header: [4]byte{0x47, 0x01, 0x00, 0x35},
			want: Packet{
				PID:               0x0100,
				ContinuityCounter: 5,
				HasAdaptation:     true,
				HasPayload:        true,
			},
		},
		{
			name:   "adaptation only no payload",
			header: [4]byte{0x47, 0x01, 0x00, 0x20},
			want: Packet{
				PID:           0x0100,
				HasAdaptation: true,
				HasPayload:    false,
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

func TestParseAdaptationFieldBasic(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want AdaptationField
	}{
		{
			name: "empty adaptation field length=0",
			data: []byte{0x00},
			want: AdaptationField{Length: 0},
		},
		{
			name: "discontinuity flag set",
			data: []byte{0x01, 0x80},
			want: AdaptationField{Length: 1, Discontinuity: true},
		},
		{
			name: "random access flag set",
			data: []byte{0x01, 0x40},
			want: AdaptationField{Length: 1, RandomAccess: true},
		},
		{
			name: "both flags set",
			data: []byte{0x01, 0xC0},
			want: AdaptationField{Length: 1, Discontinuity: true, RandomAccess: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAdaptationField(tt.data)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Length != tt.want.Length {
				t.Errorf("Length = %d, want %d", got.Length, tt.want.Length)
			}
			if got.Discontinuity != tt.want.Discontinuity {
				t.Errorf("Discontinuity = %v, want %v", got.Discontinuity, tt.want.Discontinuity)
			}
			if got.RandomAccess != tt.want.RandomAccess {
				t.Errorf("RandomAccess = %v, want %v", got.RandomAccess, tt.want.RandomAccess)
			}
		})
	}
}

func TestParseAdaptationFieldPCR(t *testing.T) {
	data := []byte{
		0x07,                               // adaptation field length = 7
		0x10,                               // flags: PCR present
		0x00, 0x00, 0x00, 0x00, 0x7E, 0x00, // PCR bytes: base=0, ext=0
	}
	got, err := parseAdaptationField(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.PCR == nil {
		t.Fatal("PCR is nil, want non-nil")
	}
	if *got.PCR != 0 {
		t.Errorf("PCR = %d, want 0", *got.PCR)
	}
}

func TestParseAdaptationFieldTooShort(t *testing.T) {
	data := []byte{0x05, 0x00}
	_, err := parseAdaptationField(data)
	if err == nil {
		t.Fatal("expected error for truncated adaptation field")
	}
}

// TestParseAdaptationFieldRejectsPCRFlagWithInsufficientLength verifies
// that a set PCR flag combined with an af.Length too small to contain
// the 6-byte PCR field returns an error rather than silently dropping
// the PCR. A malformed packet should be surfaced, not papered over.
func TestParseAdaptationFieldRejectsPCRFlagWithInsufficientLength(t *testing.T) {
	// af.Length = 3 — enough for the flags byte plus 2 extras, but not
	// enough for the 6-byte PCR field (requires length >= 7).
	data := []byte{
		0x03,             // adaptation field length = 3
		0x10,             // flags: PCR_flag set
		0x00, 0x00, 0x00, // filler — no valid PCR here
	}
	_, err := parseAdaptationField(data)
	if err == nil {
		t.Fatal("expected error when PCR_flag is set with insufficient length")
	}
}
