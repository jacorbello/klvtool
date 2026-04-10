package extract

import "testing"

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

func TestCanonicalizeRecordsDeterministicallyOrdersEqualPIDs(t *testing.T) {
	records := []PayloadRecord{
		{PID: 0x045, Payload: []byte("b")},
		{PID: 0x045, Payload: []byte("a")},
	}

	got := CanonicalizeRecords(records)

	if len(got) != 2 {
		t.Fatalf("expected 2 records, got %d", len(got))
	}
	if string(got[0].Payload) != "a" {
		t.Fatalf("expected first payload a, got %q", got[0].Payload)
	}
	if got[0].RecordID != "klv-001" {
		t.Fatalf("expected first record id klv-001, got %q", got[0].RecordID)
	}
	if string(got[1].Payload) != "b" {
		t.Fatalf("expected second payload b, got %q", got[1].Payload)
	}
	if got[1].RecordID != "klv-002" {
		t.Fatalf("expected second record id klv-002, got %q", got[1].RecordID)
	}
}
