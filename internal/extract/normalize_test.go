package extract

import (
	"reflect"
	"testing"
)

func TestCanonicalizeRecordsSortsByPIDAndAssignsStableIDs(t *testing.T) {
	records := []PayloadRecord{
		{PID: 0x1bd, Payload: []byte("b")},
		{PID: 0x045, Payload: []byte("a")},
	}

	got := CanonicalizeRecords(records)

	if len(got) != 2 {
		t.Fatalf("expected 2 records, got %d", len(got))
	}
	if got[0].PID != 0x045 {
		t.Fatalf("expected first pid 0x045, got %#x", got[0].PID)
	}
	if got[0].RecordID != "klv-001" {
		t.Fatalf("expected first record id klv-001, got %q", got[0].RecordID)
	}
	if got[1].RecordID != "klv-002" {
		t.Fatalf("expected second record id klv-002, got %q", got[1].RecordID)
	}
}

func TestCanonicalizeRecordsNormalizesNilWarningsToEmpty(t *testing.T) {
	got := CanonicalizeRecords([]PayloadRecord{{PID: 1, Payload: []byte("a")}})
	if got[0].Warnings == nil {
		t.Fatal("expected warnings slice to be non-nil")
	}
	if len(got[0].Warnings) != 0 {
		t.Fatalf("expected empty warnings, got %v", got[0].Warnings)
	}
}

func TestCanonicalizeRecordsDeterministicAcrossEquivalentInputs(t *testing.T) {
	tsid := uint16Ptr(7)
	offset30 := int64Ptr(30)
	offset10 := int64Ptr(10)
	packetIndex := int64Ptr(2)
	continuityCounter := uint8Ptr(4)
	pts := int64Ptr(100)
	dts := int64Ptr(90)
	laterPTS := int64Ptr(110)

	recordNilTSID := PayloadRecord{
		PID:               0x045,
		Payload:           []byte("same"),
		PacketOffset:      offset30,
		PacketIndex:       packetIndex,
		ContinuityCounter: continuityCounter,
		PTS:               pts,
		DTS:               dts,
	}
	recordShortOffset := PayloadRecord{
		PID:               0x045,
		Payload:           []byte("same"),
		TransportStreamID: tsid,
		PacketOffset:      offset10,
		PacketIndex:       packetIndex,
		ContinuityCounter: continuityCounter,
		PTS:               pts,
		DTS:               dts,
	}
	recordHigherPTS := PayloadRecord{
		PID:               0x045,
		Payload:           []byte("same"),
		TransportStreamID: tsid,
		PacketOffset:      offset10,
		PacketIndex:       packetIndex,
		ContinuityCounter: continuityCounter,
		PTS:               laterPTS,
		DTS:               dts,
	}

	gotA := CanonicalizeRecords([]PayloadRecord{recordHigherPTS, recordNilTSID, recordShortOffset})
	gotB := CanonicalizeRecords([]PayloadRecord{recordShortOffset, recordHigherPTS, recordNilTSID})

	if !reflect.DeepEqual(gotA, gotB) {
		t.Fatalf("expected canonicalized outputs to match across equivalent inputs\nA: %#v\nB: %#v", gotA, gotB)
	}
	if len(gotA) != 3 {
		t.Fatalf("expected 3 records, got %d", len(gotA))
	}
	if gotA[0].TransportStreamID != nil {
		t.Fatal("expected nil TransportStreamID record first")
	}
	if gotA[0].RecordID != "klv-001" || gotA[1].RecordID != "klv-002" || gotA[2].RecordID != "klv-003" {
		t.Fatalf("expected stable record ids, got %#v", []string{gotA[0].RecordID, gotA[1].RecordID, gotA[2].RecordID})
	}
}

func uint16Ptr(v uint16) *uint16 {
	return &v
}

func int64Ptr(v int64) *int64 {
	return &v
}

func uint8Ptr(v uint8) *uint8 {
	return &v
}
