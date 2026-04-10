package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPacketManifestMarshalJSONUsesStableEmptySlices(t *testing.T) {
	manifest := PacketManifest{
		SchemaVersion: "1",
		SourcePath:    "/tmp/raw",
		Records: []PacketCheckpoint{
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
	for _, want := range []string{`"records":[`, `"diagnostics":[]`} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in %s", want, got)
		}
	}
}
