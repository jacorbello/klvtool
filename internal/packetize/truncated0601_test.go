package packetize

import (
	"bytes"
	"strings"
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

func TestDetectTruncatedUASDatalinkIgnoresFullULAfterPreamble(t *testing.T) {
	val := []byte{1, 2, 3}
	payload := []byte{0xaa}
	payload = append(payload, st0601.UASDatalinkUL...)
	payload = append(payload, berWireLen(len(val))...)
	payload = append(payload, val...)

	if DetectTruncatedUASDatalink(payload) {
		t.Fatal("did not expect detection for full UL packet after preamble")
	}
	if got := findTruncatedUASDatalinkSuffix(payload, 0); got != -1 {
		t.Fatalf("expected no truncated suffix boundary, got %d", got)
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

func TestParseTruncatedUASDatalinkStreamAggregatesGapWarningCount(t *testing.T) {
	suf := st0601.TruncatedUASDatalinkKeySuffix
	packet := func(value byte) []byte {
		p := append([]byte{}, suf...)
		p = append(p, berWireLen(1)...)
		return append(p, value)
	}

	payload := packet(1)
	payload = append(payload, 0xaa)
	payload = append(payload, packet(2)...)
	payload = append(payload, 0xbb, 0xcc)
	payload = append(payload, packet(3)...)

	stream, err := ParseTruncatedUASDatalinkStream(Request{
		Mode: ModeBestEffort,
		Record: extract.RawPayloadRecord{
			Payload: payload,
		},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if stream.WarningCount != 1 {
		t.Fatalf("expected 1 aggregate warning, got %d", stream.WarningCount)
	}
	warnings := 0
	for _, diag := range stream.Diagnostics {
		if diag.Severity == "warning" {
			warnings++
			if diag.Code == "trunc0601_inter_record_gaps" && !strings.Contains(diag.Message, "2 inter-record gaps") {
				t.Fatalf("expected diagnostic to report 2 gaps, got %q", diag.Message)
			}
		}
	}
	if warnings != 1 {
		t.Fatalf("expected 1 warning diagnostic, got %d: %#v", warnings, stream.Diagnostics)
	}
}

func TestParseTruncatedUASDatalinkStreamRejectsOverflowingLengthWithoutPanic(t *testing.T) {
	suf := st0601.TruncatedUASDatalinkKeySuffix
	maxInt := int(^uint(0) >> 1)
	payload := append([]byte{}, suf...)
	payload = append(payload, berWireLen(maxInt)...)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("parse panicked: %v", r)
		}
	}()

	stream, err := ParseTruncatedUASDatalinkStream(Request{
		Mode: ModeBestEffort,
		Record: extract.RawPayloadRecord{
			Payload: payload,
		},
	})
	if err != nil {
		t.Fatalf("best-effort parse returned error: %v", err)
	}
	if stream.ErrorCount != 1 {
		t.Fatalf("expected 1 error, got %d", stream.ErrorCount)
	}
	if len(stream.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(stream.Diagnostics))
	}
	if stream.Diagnostics[0].Code != "packet_bounds_overflow" {
		t.Fatalf("expected packet_bounds_overflow, got %q", stream.Diagnostics[0].Code)
	}
	if !stream.Recovered {
		t.Fatal("expected recovered=true")
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
