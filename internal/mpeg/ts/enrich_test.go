package ts

import (
	"bytes"
	"testing"

	"github.com/jacorbello/klvtool/internal/extract"
)

func TestEnrichRecordsPopulatesMetadata(t *testing.T) {
	var file bytes.Buffer

	patSection := []byte{
		0x00, 0x00, 0xB0, 0x0D,
		0x00, 0x01, 0xC1, 0x00, 0x00,
		0x00, 0x01, 0xF0, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	patPkt := buildPacket(0x0000, 0, true, patSection)
	file.Write(patPkt)

	pmtSection := []byte{
		0x00, 0x02, 0xB0, 0x12,
		0x00, 0x01, 0xC1, 0x00, 0x00,
		0xE1, 0x00, 0xF0, 0x00,
		0x06, 0xE3, 0x00, 0xF0, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	pmtPkt := buildPacket(0x1000, 0, true, pmtSection)
	file.Write(pmtPkt)

	pesHeader := []byte{
		0x00, 0x00, 0x01, 0xBD,
		0x00, 0x08,
		0x80, 0x80, 0x05,
		0x21, 0x00, 0x01, 0x00, 0x01,
		0xDE, 0xAD,
	}
	dataPkt := buildPacket(0x0300, 0, true, pesHeader)
	file.Write(dataPkt)

	records := []extract.RawPayloadRecord{
		{RecordID: "klv-001", PID: 0x0300, Payload: []byte{0xDE, 0xAD}},
	}

	r := bytes.NewReader(file.Bytes())
	enriched, err := EnrichRecords(r, records)
	if err != nil {
		t.Fatalf("EnrichRecords: %v", err)
	}
	if len(enriched) != 1 {
		t.Fatalf("enriched count = %d, want 1", len(enriched))
	}

	rec := enriched[0]
	if rec.PTS == nil || *rec.PTS != 0 {
		t.Errorf("PTS = %v, want 0", rec.PTS)
	}
	if rec.PacketOffset == nil || *rec.PacketOffset != 376 {
		t.Errorf("PacketOffset = %v, want 376", rec.PacketOffset)
	}
	if rec.PacketIndex == nil || *rec.PacketIndex != 2 {
		t.Errorf("PacketIndex = %v, want 2", rec.PacketIndex)
	}
	if rec.ContinuityCounter == nil || *rec.ContinuityCounter != 0 {
		t.Errorf("ContinuityCounter = %v, want 0", rec.ContinuityCounter)
	}
}

func TestEnrichRecordsPIDMismatchAddsWarning(t *testing.T) {
	var file bytes.Buffer
	patSection := []byte{
		0x00, 0x00, 0xB0, 0x0D,
		0x00, 0x01, 0xC1, 0x00, 0x00,
		0x00, 0x01, 0xF0, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	patPkt := buildPacket(0x0000, 0, true, patSection)
	file.Write(patPkt)

	pmtSection := []byte{
		0x00, 0x02, 0xB0, 0x12,
		0x00, 0x01, 0xC1, 0x00, 0x00,
		0xE1, 0x00, 0xF0, 0x00,
		0x06, 0xE3, 0x00, 0xF0, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	pmtPkt := buildPacket(0x1000, 0, true, pmtSection)
	file.Write(pmtPkt)

	records := []extract.RawPayloadRecord{
		{RecordID: "klv-001", PID: 0x0999, Payload: []byte{0xFF}},
	}

	r := bytes.NewReader(file.Bytes())
	enriched, err := EnrichRecords(r, records)
	if err != nil {
		t.Fatalf("EnrichRecords: %v", err)
	}
	rec := enriched[0]
	if rec.PTS != nil {
		t.Error("PTS should be nil for missing PID")
	}
	if len(rec.Warnings) == 0 {
		t.Error("expected warning for PID mismatch")
	}
}

func TestEnrichRecordsDoesNotMutateInput(t *testing.T) {
	var file bytes.Buffer
	patPkt := buildPacket(0x0000, 0, true, []byte{
		0x00, 0x00, 0xB0, 0x0D, 0x00, 0x01, 0xC1, 0x00, 0x00,
		0x00, 0x01, 0xF0, 0x00, 0x00, 0x00, 0x00, 0x00,
	})
	file.Write(patPkt)
	pmtPkt := buildPacket(0x1000, 0, true, []byte{
		0x00, 0x02, 0xB0, 0x12, 0x00, 0x01, 0xC1, 0x00, 0x00,
		0xE1, 0x00, 0xF0, 0x00, 0x06, 0xE3, 0x00, 0xF0, 0x00,
		0x00, 0x00, 0x00, 0x00,
	})
	file.Write(pmtPkt)

	original := []extract.RawPayloadRecord{
		{RecordID: "klv-001", PID: 0x0300, Payload: []byte{0xAA}},
	}

	r := bytes.NewReader(file.Bytes())
	if _, err := EnrichRecords(r, original); err != nil {
		t.Fatalf("EnrichRecords: %v", err)
	}

	if original[0].PTS != nil {
		t.Error("original record was mutated")
	}
}
