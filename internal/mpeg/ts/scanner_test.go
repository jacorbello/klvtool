package ts

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jacorbello/klvtool/internal/model"
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

// errReader returns (0, errInjected) on every Read call, simulating an
// I/O failure from the underlying stream. bufio.Reader.Peek surfaces this
// error once its buffer can't be filled.
type errReader struct{}

var errInjected = errors.New("injected I/O error")

func (errReader) Read(p []byte) (int, error) { return 0, errInjected }

// TestScannerPeekIOErrorIsTSRead verifies that a non-EOF read failure
// from the underlying reader is reported as a TSRead model error, not
// swallowed as io.EOF or misclassified as a TSParse.
// failAfterReader returns `data` on the first Read, then errInjected on
// every subsequent Read. This lets tests position a reader such that
// bufio.Reader.Peek can succeed with buffered content but fail when it
// tries to refill — exactly the state needed to exercise recoverSync's
// error path.
type failAfterReader struct {
	data []byte
	pos  int
}

func (r *failAfterReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, errInjected
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// TestRecoverSyncPropagatesPeekIOError verifies that a non-EOF read
// failure inside recoverSync is surfaced as a TSRead model error with
// the underlying cause preserved, not misclassified as a TSSync EOF.
func TestRecoverSyncPropagatesPeekIOError(t *testing.T) {
	// 200 non-sync bytes — enough to make the initial Peek(188) succeed
	// (so readAlignedPacket falls through to recoverSync) but small
	// enough that the bufio.Reader refill hits errInjected before
	// recoverSync can find 189 bytes of contiguous data.
	garbage := make([]byte, 200)
	for i := range garbage {
		garbage[i] = 0xAA
	}
	s := NewPacketScanner(&failAfterReader{data: garbage}, ScanConfig{})

	_, err := s.Next()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var typed *model.Error
	if !errors.As(err, &typed) {
		t.Fatalf("error is not *model.Error: %v", err)
	}
	if typed.Code != model.CodeTSRead {
		t.Errorf("Code = %q, want %q (non-EOF Peek error should be TSRead)", typed.Code, model.CodeTSRead)
	}
	if !errors.Is(err, errInjected) {
		t.Errorf("underlying error not preserved in chain: %v", err)
	}
}

func TestScannerPeekIOErrorIsTSRead(t *testing.T) {
	s := NewPacketScanner(errReader{}, ScanConfig{})
	_, err := s.Next()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, io.EOF) {
		t.Fatalf("non-EOF read error was swallowed as io.EOF: %v", err)
	}
	var typed *model.Error
	if !errors.As(err, &typed) {
		t.Fatalf("error is not *model.Error: %v", err)
	}
	if typed.Code != model.CodeTSRead {
		t.Errorf("Code = %q, want %q", typed.Code, model.CodeTSRead)
	}
	if !errors.Is(err, errInjected) {
		t.Errorf("underlying error not preserved in chain: %v", err)
	}
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

func TestScannerTrailingBytesErrorIsTSParse(t *testing.T) {
	data := append(buildPacket(0x100, 0, false, nil), 0x00, 0x00, 0x00)
	s := NewPacketScanner(bytes.NewReader(data), ScanConfig{})

	if _, err := s.Next(); err != nil {
		t.Fatalf("first packet: unexpected error: %v", err)
	}

	_, err := s.Next()
	if err == nil {
		t.Fatal("expected error for trailing bytes")
	}
	var mErr *model.Error
	if !errors.As(err, &mErr) {
		t.Fatalf("expected *model.Error, got %T: %v", err, err)
	}
	if mErr.Code != model.CodeTSParse {
		t.Errorf("Code = %q, want %q", mErr.Code, model.CodeTSParse)
	}
}

func TestScannerPIDFilteringReadsTargetPayloadsOnly(t *testing.T) {
	target := buildPacket(0x100, 0, false, []byte{0xAA})
	other := buildPacket(0x200, 0, false, []byte{0xBB})
	r := bytes.NewReader(append(target, other...))

	cfg := ScanConfig{PayloadPIDs: map[uint16]bool{0x100: true}}
	s := NewPacketScanner(r, cfg)

	pkt1, err := s.Next()
	if err != nil {
		t.Fatalf("packet 1: %v", err)
	}
	if pkt1.Payload == nil {
		t.Error("target PID payload is nil, want non-nil")
	}

	pkt2, err := s.Next()
	if err != nil {
		t.Fatalf("packet 2: %v", err)
	}
	if pkt2.Payload != nil {
		t.Error("non-target PID payload is non-nil, want nil")
	}
}

func TestScannerNilPayloadPIDsReadsAll(t *testing.T) {
	p1 := buildPacket(0x100, 0, false, []byte{0xAA})
	p2 := buildPacket(0x200, 0, false, []byte{0xBB})
	r := bytes.NewReader(append(p1, p2...))

	s := NewPacketScanner(r, ScanConfig{PayloadPIDs: nil})

	for i, want := range []uint16{0x100, 0x200} {
		pkt, err := s.Next()
		if err != nil {
			t.Fatalf("packet %d: %v", i, err)
		}
		if pkt.PID != want {
			t.Errorf("packet %d PID = 0x%X, want 0x%X", i, pkt.PID, want)
		}
		if pkt.Payload == nil {
			t.Errorf("packet %d payload is nil, want non-nil", i)
		}
	}
}

func TestScannerEmptyPayloadPIDsReadsNone(t *testing.T) {
	p1 := buildPacket(0x100, 0, false, []byte{0xAA})
	r := bytes.NewReader(p1)

	s := NewPacketScanner(r, ScanConfig{PayloadPIDs: map[uint16]bool{}})

	pkt, err := s.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkt.Payload != nil {
		t.Error("payload is non-nil with empty PayloadPIDs, want nil")
	}
}

// buildPacketWithAdaptation creates a 188-byte TS packet with an adaptation
// field followed by payload. adaptationControl is set to 0x03 (both).
func buildPacketWithAdaptation(pid uint16, cc uint8, af []byte, payload []byte) []byte {
	pkt := make([]byte, PacketSize)
	pkt[0] = SyncByte
	pkt[1] = byte(pid>>8) & 0x1F
	pkt[2] = byte(pid & 0xFF)
	pkt[3] = 0x30 | (cc & 0x0F) // adaptation_control=11 (both)
	copy(pkt[4:], af)
	copy(pkt[4+len(af):], payload)
	return pkt
}

func TestScannerParsesAdaptationField(t *testing.T) {
	af := []byte{0x01, 0x80} // length=1, discontinuity=true
	payload := []byte{0xDE, 0xAD}
	pkt := buildPacketWithAdaptation(0x100, 7, af, payload)
	r := bytes.NewReader(pkt)

	s := NewPacketScanner(r, ScanConfig{})
	got, err := s.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Adaptation == nil {
		t.Fatal("Adaptation is nil, want non-nil")
	}
	if !got.Adaptation.Discontinuity {
		t.Error("Discontinuity = false, want true")
	}
	if got.ContinuityCounter != 7 {
		t.Errorf("ContinuityCounter = %d, want 7", got.ContinuityCounter)
	}
	if got.Payload == nil {
		t.Fatal("Payload is nil, want non-nil")
	}
}

func TestScannerRejectsSpuriousSyncByteInPayload(t *testing.T) {
	// Garbage payload: 188 bytes where a 0x47 appears early but the byte
	// at +188 from that 0x47 is NOT 0x47. A naive scanner would lock on
	// the spurious sync byte; a correct one must skip it and find the
	// real packet that follows.
	garbage := make([]byte, 188)
	// Prefix a few non-sync bytes.
	garbage[0] = 0x00
	garbage[1] = 0x11
	garbage[2] = 0x22
	// Spurious sync at index 3.
	garbage[3] = SyncByte
	// Fill rest with non-sync bytes.
	for i := 4; i < 188; i++ {
		garbage[i] = 0xFF
	}

	// Real packet immediately follows. Its +188 byte must be 0x47 too,
	// so tack another valid packet on as the "lookahead verifier".
	real1 := buildPacket(0x100, 0, false, []byte{0xAA})
	real2 := buildPacket(0x200, 1, false, []byte{0xBB})

	var buf bytes.Buffer
	buf.Write(garbage)
	buf.Write(real1)
	buf.Write(real2)

	s := NewPacketScanner(&buf, ScanConfig{})

	got1, err := s.Next()
	if err != nil {
		t.Fatalf("packet 1: %v", err)
	}
	if got1.PID != 0x100 {
		t.Errorf("packet 1 PID = 0x%X, want 0x100 (scanner locked onto spurious 0x47)", got1.PID)
	}

	got2, err := s.Next()
	if err != nil {
		t.Fatalf("packet 2: %v", err)
	}
	if got2.PID != 0x200 {
		t.Errorf("packet 2 PID = 0x%X, want 0x200", got2.PID)
	}
}

func TestScannerRecoversSyncAfterGarbage(t *testing.T) {
	p1 := buildPacket(0x100, 0, false, []byte{0xAA})
	p2 := buildPacket(0x200, 1, false, []byte{0xBB})
	var buf bytes.Buffer
	buf.Write(p1)
	buf.Write([]byte{0x00, 0x11, 0x22}) // 3 garbage bytes
	buf.Write(p2)

	s := NewPacketScanner(&buf, ScanConfig{})

	got1, err := s.Next()
	if err != nil {
		t.Fatalf("packet 1: %v", err)
	}
	if got1.PID != 0x100 {
		t.Errorf("packet 1 PID = 0x%X, want 0x100", got1.PID)
	}

	got2, err := s.Next()
	if err != nil {
		t.Fatalf("packet 2: %v", err)
	}
	if got2.PID != 0x200 {
		t.Errorf("packet 2 PID = 0x%X, want 0x200", got2.PID)
	}

	diags := s.Diagnostics()
	found := false
	for _, d := range diags {
		if d.Code == "sync_recovery" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected sync_recovery diagnostic, got %+v", diags)
	}
}

func TestSyncFailureMessageMentionsInvalidFile(t *testing.T) {
	// Feed non-TS data (no 0x47 sync bytes anywhere); must be >= 188 bytes
	// so the first peek succeeds and recoverSync is reached.
	data := bytes.Repeat([]byte{0xFF}, 200)
	scanner := NewPacketScanner(bytes.NewReader(data), ScanConfig{})
	_, err := scanner.Next()
	if err == nil {
		t.Fatal("expected error for non-TS data")
	}
	msg := err.Error()
	if !strings.Contains(msg, "not a valid MPEG-TS file") {
		t.Errorf("expected friendly message containing 'not a valid MPEG-TS file'; got: %s", msg)
	}
}
