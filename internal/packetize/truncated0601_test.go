package packetize

import (
	"bytes"
	"testing"

	"github.com/jacorbello/klvtool/internal/extract"
	"github.com/jacorbello/klvtool/internal/klv/specs/st0601"
)

func berWireLen(n int) []byte {
	if n < 0x80 {
		return []byte{byte(n)}
	}
	var b []byte
	for x := n; x > 0; x >>= 8 {
		b = append([]byte{byte(x & 0xff)}, b...)
	}
	return append([]byte{0x80 | byte(len(b))}, b...)
}

func TestDetectTruncatedUASDatalink(t *testing.T) {
	suf := st0601.TruncatedUASDatalinkKeySuffix
	val := []byte{1, 2, 3, 4, 5}
	p := append(append([]byte{}, suf...), berWireLen(len(val))...)
	p = append(p, val...)

	if !DetectTruncatedUASDatalink(p) {
		t.Fatal("expected detection on truncated-key payload")
	}
	if DetectTruncatedUASDatalink(st0601.UASDatalinkUL) {
		t.Fatal("did not expect detection on bare UL without length")
	}
	full := append(append([]byte{}, st0601.UASDatalinkUL...), berWireLen(len(val))...)
	full = append(full, val...)
	if DetectTruncatedUASDatalink(full) {
		t.Fatal("did not expect detection when wire uses full 16-byte key")
	}
}

func TestParseTruncatedUASDatalinkStream(t *testing.T) {
	suf := st0601.TruncatedUASDatalinkKeySuffix
	v1 := []byte{1, 2, 3}
	v2 := []byte{4, 5, 6, 7}
	gap := []byte{0xaa, 0xbb}

	var payload []byte
	payload = append(payload, suf...)
	payload = append(payload, berWireLen(len(v1))...)
	payload = append(payload, v1...)
	payload = append(payload, gap...)
	payload = append(payload, suf...)
	payload = append(payload, berWireLen(len(v2))...)
	payload = append(payload, v2...)

	stream, err := ParseTruncatedUASDatalinkStream(Request{
		Mode: ModeBestEffort,
		Record: extract.RawPayloadRecord{
			Payload: payload,
		},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if stream.ParsedCount != 2 {
		t.Fatalf("expected 2 packets, got %d", stream.ParsedCount)
	}
	if !bytes.Equal(stream.Packets[0].Value, v1) {
		t.Fatalf("packet0 value: got %x want %x", stream.Packets[0].Value, v1)
	}
	if !bytes.Equal(stream.Packets[1].Value, v2) {
		t.Fatalf("packet1 value: got %x want %x", stream.Packets[1].Value, v2)
	}
	if !bytes.Equal(stream.Packets[0].Key, st0601.UASDatalinkUL) {
		t.Fatalf("expected synthetic full UL key")
	}
	if stream.WarningCount == 0 {
		t.Fatal("expected gap warning")
	}
}

func TestParserParseAutoTruncated(t *testing.T) {
	suf := st0601.TruncatedUASDatalinkKeySuffix
	val := []byte{9, 9, 9}
	payload := append(append([]byte{}, suf...), berWireLen(len(val))...)
	payload = append(payload, val...)

	p := NewParser()
	stream, err := p.ParseAuto(Request{
		Mode: ModeBestEffort,
		Record: extract.RawPayloadRecord{
			Payload: payload,
		},
	})
	if err != nil {
		t.Fatalf("ParseAuto: %v", err)
	}
	if stream.ParsedCount != 1 {
		t.Fatalf("expected 1 packet from ParseAuto, got %d", stream.ParsedCount)
	}
}
