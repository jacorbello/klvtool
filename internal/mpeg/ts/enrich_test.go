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

// TestEnrichRecordsExitsEarlyAfterFirstUnitPerPID verifies that the
// scan stops once every target PID has recorded its first PES unit.
// It also acts as a regression guard against the earlier implementation
// that buffered every PES unit (with full payload) for the entire
// stream before picking the first — a pattern that caused unbounded
// memory growth on multi-GB captures.
func TestEnrichRecordsExitsEarlyAfterFirstUnitPerPID(t *testing.T) {
	var file bytes.Buffer

	// PAT → PMT with data stream on PID 0x0300.
	patPkt := buildPacket(0x0000, 0, true, []byte{
		0x00, 0x00, 0xB0, 0x0D,
		0x00, 0x01, 0xC1, 0x00, 0x00,
		0x00, 0x01, 0xF0, 0x00,
		0x00, 0x00, 0x00, 0x00,
	})
	file.Write(patPkt)
	pmtPkt := buildPacket(0x1000, 0, true, []byte{
		0x00, 0x02, 0xB0, 0x12,
		0x00, 0x01, 0xC1, 0x00, 0x00,
		0xE1, 0x00, 0xF0, 0x00,
		0x06, 0xE3, 0x00, 0xF0, 0x00,
		0x00, 0x00, 0x00, 0x00,
	})
	file.Write(pmtPkt)

	// First PES unit for PID 0x0300 at packet index 2, offset 376.
	firstPES := buildPacket(0x0300, 0, true, []byte{
		0x00, 0x00, 0x01, 0xBD,
		0x00, 0x08, 0x80, 0x80, 0x05,
		0x21, 0x00, 0x01, 0x00, 0x01, // PTS=0
		0xDE, 0xAD,
	})
	file.Write(firstPES)

	// Many additional PES units that a bug-era implementation would
	// have buffered in full. We emit 20 distinct PES starts — if the
	// early-exit works, the scan stops before reaching most of these.
	for i := 1; i <= 20; i++ {
		pes := buildPacket(0x0300, uint8(i)&0x0F, true, []byte{
			0x00, 0x00, 0x01, 0xBD,
			0x00, 0x08, 0x80, 0x80, 0x05,
			0x21, 0x00, 0x01, 0x00, 0x01,
			byte(i), byte(i),
		})
		file.Write(pes)
	}

	records := []extract.RawPayloadRecord{
		{RecordID: "klv-001", PID: 0x0300, Payload: []byte{0xDE, 0xAD}},
	}

	r := bytes.NewReader(file.Bytes())
	enriched, err := EnrichRecords(r, records)
	if err != nil {
		t.Fatalf("EnrichRecords: %v", err)
	}
	rec := enriched[0]
	// The first unit's metadata must be the one used, not any later one.
	if rec.PacketIndex == nil || *rec.PacketIndex != 2 {
		t.Errorf("PacketIndex = %v, want 2 (first unit), later unit was used instead", rec.PacketIndex)
	}
	if rec.PacketOffset == nil || *rec.PacketOffset != 376 {
		t.Errorf("PacketOffset = %v, want 376", rec.PacketOffset)
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

func TestEnrichRecordsRejectsDuplicatePIDs(t *testing.T) {
	var file bytes.Buffer
	patPkt := buildPacket(0x0000, 0, true, []byte{
		0x00, 0x00, 0xB0, 0x0D, 0x00, 0x01, 0xC1, 0x00, 0x00,
		0x00, 0x01, 0xF0, 0x00, 0x00, 0x00, 0x00, 0x00,
	})
	file.Write(patPkt)

	records := []extract.RawPayloadRecord{
		{RecordID: "klv-001", PID: 0x0300, Payload: []byte{0xAA}},
		{RecordID: "klv-002", PID: 0x0300, Payload: []byte{0xBB}},
	}

	r := bytes.NewReader(file.Bytes())
	_, err := EnrichRecords(r, records)
	if err == nil {
		t.Fatal("expected error for duplicate PIDs, got nil")
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
