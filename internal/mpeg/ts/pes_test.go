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
