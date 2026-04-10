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
	if len(stream.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(stream.Diagnostics))
	}
	if !stream.Recovered {
		t.Fatal("expected recovered=true")
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
