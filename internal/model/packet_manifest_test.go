package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPacketManifestMarshalJSONUsesStableEmptySlices(t *testing.T) {
	manifest := PacketManifest{
		SchemaVersion: PacketSchemaVersion,
		SourcePath:    "/tmp/raw",
		Records: []PacketManifestEntry{
			{
				RecordID:   "klv-001",
				Mode:       "strict",
				PacketPath: "packets/klv-001.json",
			},
		},
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	got := string(data)
	if !strings.Contains(got, `"schemaVersion":"2"`) {
		t.Fatalf("expected packet manifest schema version 2 in %s", got)
	}
	for _, want := range []string{`"records":[`, `"diagnostics":[]`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in %s", want, got)
		}
	}
}

func TestPacketManifestMarshalJSONNormalizesNilRecords(t *testing.T) {
	data, err := json.Marshal(PacketManifest{})
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	got := string(data)
	if !strings.Contains(got, `"records":[]`) {
		t.Fatalf("expected empty records array, got %s", got)
	}
}

func TestPacketCheckpointMarshalJSONUsesStablePacketAndDiagnosticArrays(t *testing.T) {
	data, err := json.Marshal(PacketCheckpoint{
		SchemaVersion: PacketSchemaVersion,
		RecordID:      "klv-001",
		Mode:          "best-effort",
		Packets: []PacketRecord{
			{
				PacketIndex:    0,
				PacketStart:    0,
				KeyStart:       0,
				LengthStart:    16,
				ValueStart:     17,
				PacketEnd:      19,
				RawKeyHex:      "060e2b34",
				Length:         3,
				RawValueHex:    "aabbcc",
				Classification: "universal_set",
			},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	got := string(data)
	for _, want := range []string{`"schemaVersion":"2"`, `"packets":[`, `"packetEnd":19`, `"rawKeyHex":"060e2b34"`, `"rawValueHex":"aabbcc"`, `"diagnostics":[]`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in %s", want, got)
		}
	}
	for _, want := range []string{`"packetEndExclusive"`, `"key":`, `"value":`} {
		if strings.Contains(got, want) {
			t.Fatalf("did not expect legacy field %q in %s", want, got)
		}
	}
}
