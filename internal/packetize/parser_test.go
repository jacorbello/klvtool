package packetize

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/jacorbello/klvtool/internal/extract"
)

func TestZeroValuePacketizedStreamUsesStableDefaults(t *testing.T) {
	var raw extract.RawPayloadRecord
	stream := PacketizedStream{Source: raw}

	if stream.Mode != "" {
		t.Fatalf("expected zero-value mode to be unset, got %q", stream.Mode)
	}
	if stream.Packets != nil {
		t.Fatalf("expected nil packets before normalization, got %#v", stream.Packets)
	}
	if stream.Diagnostics != nil {
		t.Fatalf("expected nil diagnostics before normalization, got %#v", stream.Diagnostics)
	}
}

func TestPacketizeJSONContractUsesExplicitFieldNames(t *testing.T) {
	stream := PacketizedStream{
		Source: extract.RawPayloadRecord{
			RecordID: "klv-001",
			PID:      42,
		},
		Mode: ModeBestEffort,
		Packets: []Packet{
			{
				PacketIndex:        7,
				PacketStart:        100,
				KeyStart:           101,
				LengthStart:        104,
				ValueStart:         106,
				PacketEndExclusive: 110,
				Classification:     ClassificationUniversalSet,
			},
		},
		ParsedCount: 1,
	}

	data, err := json.Marshal(stream)
	if err != nil {
		t.Fatalf("marshal packetized stream: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal packetized stream: %v", err)
	}

	for _, key := range []string{"source", "mode", "parserVersion", "packets", "diagnostics", "parsedCount", "warningCount", "errorCount", "recovered"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("expected top-level key %q in %s", key, data)
		}
	}

	packets, ok := got["packets"].([]any)
	if !ok || len(packets) != 1 {
		t.Fatalf("expected one packet in %s, got %#v", data, got["packets"])
	}

	packet, ok := packets[0].(map[string]any)
	if !ok {
		t.Fatalf("expected packet object in %s, got %#v", data, packets[0])
	}

	for _, key := range []string{"packetIndex", "packetStart", "keyStart", "lengthStart", "valueStart", "packetEndExclusive", "key", "length", "value", "classification", "diagnostics"} {
		if _, ok := packet[key]; !ok {
			t.Fatalf("expected packet key %q in %s", key, data)
		}
	}
}

func TestParserStrictModeFailsOnTruncatedPacket(t *testing.T) {
	parser := NewParser()

	_, err := parser.Parse(Request{
		Mode: ModeStrict,
		Record: extract.RawPayloadRecord{
			RecordID: "klv-001",
			Payload:  append(bytes.Repeat([]byte{0x06}, 16), 0x82, 0x01),
		},
	})
	if err == nil {
		t.Fatal("expected strict parse error")
	}
}

func TestParserBestEffortReturnsDiagnosticsOnMalformedPacket(t *testing.T) {
	parser := NewParser()

	stream, err := parser.Parse(Request{
		Mode: ModeBestEffort,
		Record: extract.RawPayloadRecord{
			RecordID: "klv-001",
			Payload:  append(bytes.Repeat([]byte{0x06}, 16), 0x82, 0x01),
		},
	})
	if err != nil {
		t.Fatalf("best-effort parse returned error: %v", err)
	}
	if len(stream.Diagnostics) < 1 {
		t.Fatalf("expected at least 1 diagnostic, got %d", len(stream.Diagnostics))
	}
	if !stream.Recovered {
		t.Fatal("expected recovered=true")
	}
}

func TestParserBestEffortRecoversPastMalformedPacket(t *testing.T) {
	parser := NewParser()

	// Build a payload with garbage bytes followed by a valid KLV packet.
	// The first 17 bytes are a truncated/bad packet (16-byte non-UL key + short BER).
	badPacket := append(bytes.Repeat([]byte{0xFF}, 16), 0x82, 0x01)

	// Valid packet: SMPTE universal key + 3 bytes of value.
	validPacket := append([]byte{0x06, 0x0e, 0x2b, 0x34}, bytes.Repeat([]byte{0x00}, 12)...)
	validPacket = append(validPacket, 0x03, 0xaa, 0xbb, 0xcc)

	payload := append(badPacket, validPacket...)

	stream, err := parser.Parse(Request{
		Mode: ModeBestEffort,
		Record: extract.RawPayloadRecord{
			RecordID: "klv-002",
			Payload:  payload,
		},
	})
	if err != nil {
		t.Fatalf("best-effort parse returned error: %v", err)
	}
	if !stream.Recovered {
		t.Fatal("expected recovered=true")
	}
	if stream.ParsedCount != 1 {
		t.Fatalf("expected 1 parsed packet after recovery, got %d", stream.ParsedCount)
	}
	if len(stream.Packets) != 1 {
		t.Fatalf("expected 1 packet, got %d", len(stream.Packets))
	}
	if stream.Packets[0].Classification != ClassificationUniversalSet {
		t.Fatalf("expected universal set classification, got %q", stream.Packets[0].Classification)
	}

	hasRecoverySkip := false
	for _, d := range stream.Diagnostics {
		if d.Code == "recovery_skip" {
			hasRecoverySkip = true
		}
	}
	if !hasRecoverySkip {
		t.Fatal("expected recovery_skip diagnostic")
	}
}

func TestParserParsesValidPacket(t *testing.T) {
	parser := NewParser()

	payload := append([]byte{0x06, 0x0e, 0x2b, 0x34}, bytes.Repeat([]byte{0x00}, 12)...)
	payload = append(payload, 0x03, 0xaa, 0xbb, 0xcc)

	stream, err := parser.Parse(Request{
		Record: extract.RawPayloadRecord{
			RecordID: "klv-001",
			Payload:  payload,
		},
	})
	if err != nil {
		t.Fatalf("parse returned error: %v", err)
	}
	if len(stream.Packets) != 1 {
		t.Fatalf("expected 1 packet, got %d", len(stream.Packets))
	}

	packet := stream.Packets[0]
	if packet.PacketEndExclusive != len(payload) {
		t.Fatalf("expected packet end %d, got %d", len(payload), packet.PacketEndExclusive)
	}
	if packet.Classification != ClassificationUniversalSet {
		t.Fatalf("expected universal set classification, got %q", packet.Classification)
	}
	if got := packet.Length; got != 3 {
		t.Fatalf("expected length 3, got %d", got)
	}
}

func TestParserRejectsInvalidMode(t *testing.T) {
	parser := NewParser()

	_, err := parser.Parse(Request{
		Mode: "best-effort-ish",
		Record: extract.RawPayloadRecord{
			RecordID: "klv-001",
		},
	})
	if err == nil {
		t.Fatal("expected invalid mode error")
	}
}

// fullST0601UL is the 16-byte MISB ST 0601 UAS Datalink UL. Duplicated here
// to keep the packetize tests free of an internal/klv import.
var fullST0601UL = []byte{
	0x06, 0x0e, 0x2b, 0x34, 0x02, 0x0b, 0x01, 0x01,
	0x0e, 0x01, 0x03, 0x01, 0x01, 0x00, 0x00, 0x00,
}

// makeStrippedKLV builds a payload of n consecutive KLV packets with the
// first 5 bytes of the UL removed (the ffmpeg `-c copy -f data` strip pattern).
// Each packet has a 3-byte payload `0xAA 0xBB 0xCC`.
func makeStrippedKLV(n int) []byte {
	stub := fullST0601UL[5:]
	var out []byte
	for i := 0; i < n; i++ {
		out = append(out, stub...)
		out = append(out, 0x03, 0xaa, 0xbb, 0xcc)
	}
	return out
}

// makeFullKLV builds n consecutive valid KLV packets with the full 16-byte UL.
func makeFullKLV(n int) []byte {
	var out []byte
	for i := 0; i < n; i++ {
		out = append(out, fullST0601UL...)
		out = append(out, 0x03, 0xaa, 0xbb, 0xcc)
	}
	return out
}

func diagCount(diags []Diagnostic, code string) int {
	n := 0
	for _, d := range diags {
		if d.Code == code {
			n++
		}
	}
	return n
}

func TestParserDetectsStrippedULPrefix(t *testing.T) {
	parser := NewParser()
	stream, err := parser.Parse(Request{
		Mode: ModeBestEffort,
		Record: extract.RawPayloadRecord{
			RecordID: "klv-001",
			Payload:  makeStrippedKLV(3),
		},
		KnownULs: [][]byte{fullST0601UL},
	})
	if err != nil {
		t.Fatalf("parse returned error: %v", err)
	}
	if got := diagCount(stream.Diagnostics, DiagnosticStrippedULPrefix); got != 1 {
		t.Fatalf("expected exactly 1 %s diagnostic, got %d (all: %+v)", DiagnosticStrippedULPrefix, got, stream.Diagnostics)
	}
	if got := diagCount(stream.Diagnostics, DiagnosticStrippedULRepaired); got != 0 {
		t.Fatalf("expected 0 %s diagnostics when repair is off, got %d", DiagnosticStrippedULRepaired, got)
	}
}

func TestParserRepairStrippedULRecoversAllPackets(t *testing.T) {
	parser := NewParser()
	stream, err := parser.Parse(Request{
		Mode: ModeBestEffort,
		Record: extract.RawPayloadRecord{
			RecordID: "klv-001",
			Payload:  makeStrippedKLV(5),
		},
		KnownULs:         [][]byte{fullST0601UL},
		RepairStrippedUL: true,
	})
	if err != nil {
		t.Fatalf("parse returned error: %v", err)
	}
	if stream.ParsedCount != 5 {
		t.Fatalf("expected 5 parsed packets after repair, got %d (diags: %+v)", stream.ParsedCount, stream.Diagnostics)
	}
	for i, p := range stream.Packets {
		if p.Classification != ClassificationUniversalSet {
			t.Fatalf("packet %d: expected universal_set, got %q", i, p.Classification)
		}
		if !bytes.Equal(p.Key, fullST0601UL) {
			t.Fatalf("packet %d: expected key %x, got %x", i, fullST0601UL, p.Key)
		}
	}
	if got := diagCount(stream.Diagnostics, DiagnosticStrippedULRepaired); got != 1 {
		t.Fatalf("expected exactly 1 %s diagnostic, got %d", DiagnosticStrippedULRepaired, got)
	}
}

func TestParserDoesNotFalsePositiveOnFullULStream(t *testing.T) {
	parser := NewParser()
	stream, err := parser.Parse(Request{
		Mode: ModeBestEffort,
		Record: extract.RawPayloadRecord{
			RecordID: "klv-001",
			Payload:  makeFullKLV(3),
		},
		KnownULs: [][]byte{fullST0601UL},
	})
	if err != nil {
		t.Fatalf("parse returned error: %v", err)
	}
	if stream.ParsedCount != 3 {
		t.Fatalf("expected 3 parsed packets, got %d", stream.ParsedCount)
	}
	if got := diagCount(stream.Diagnostics, DiagnosticStrippedULPrefix); got != 0 {
		t.Fatalf("expected 0 stripped-UL diagnostics on a clean stream, got %d", got)
	}
}

func TestParserAbstainsFromDetectionWhenAnyFullULIsPresent(t *testing.T) {
	// Defensive: if even one full UL is present, the payload may already be
	// valid (just with a leading prefix byte we'd misread). Abstain from
	// the detection probe to avoid corrupting genuinely-good streams.
	parser := NewParser()
	mixed := append(makeStrippedKLV(2), makeFullKLV(1)...)
	stream, err := parser.Parse(Request{
		Mode: ModeBestEffort,
		Record: extract.RawPayloadRecord{
			RecordID: "klv-001",
			Payload:  mixed,
		},
		KnownULs:         [][]byte{fullST0601UL},
		RepairStrippedUL: true,
	})
	if err != nil {
		t.Fatalf("parse returned error: %v", err)
	}
	if got := diagCount(stream.Diagnostics, DiagnosticStrippedULPrefix); got != 0 {
		t.Fatalf("expected 0 stripped-UL diagnostics on a mixed stream, got %d", got)
	}
	if got := diagCount(stream.Diagnostics, DiagnosticStrippedULRepaired); got != 0 {
		t.Fatalf("expected no repair to fire when full ULs are present, got %d", got)
	}
}

func TestParserRejectsOverflowingBERLengthWithoutPanic(t *testing.T) {
	parser := NewParser()
	payload := append(bytes.Repeat([]byte{0x06}, 16), []byte{0x88, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}...)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("parse panicked: %v", r)
		}
	}()

	stream, err := parser.Parse(Request{
		Mode: ModeBestEffort,
		Record: extract.RawPayloadRecord{
			RecordID: "klv-001",
			Payload:  payload,
		},
	})
	if err != nil {
		t.Fatalf("best-effort parse returned error: %v", err)
	}
	if len(stream.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(stream.Diagnostics))
	}
	if !stream.Recovered {
		t.Fatal("expected recovered=true")
	}
}
