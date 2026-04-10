package packetize

import (
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
