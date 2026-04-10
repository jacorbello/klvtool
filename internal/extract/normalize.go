package extract

import (
	"bytes"
	"fmt"
	"sort"
)

// CanonicalizeRecords returns a stable, normalized copy of the extracted records.
func CanonicalizeRecords(records []PayloadRecord) []PayloadRecord {
	out := append([]PayloadRecord(nil), records...)
	sort.SliceStable(out, func(i, j int) bool {
		return lessPayloadRecord(out[i], out[j])
	})
	for i := range out {
		out[i].RecordID = canonicalRecordID(i)
		if out[i].Warnings == nil {
			out[i].Warnings = []string{}
		}
	}
	return out
}

func canonicalRecordID(index int) string {
	return fmt.Sprintf("klv-%03d", index+1)
}

func lessPayloadRecord(a, b PayloadRecord) bool {
	if a.PID != b.PID {
		return a.PID < b.PID
	}
	if len(a.Payload) != len(b.Payload) {
		return len(a.Payload) < len(b.Payload)
	}
	if cmp := bytes.Compare(a.Payload, b.Payload); cmp != 0 {
		return cmp < 0
	}
	if cmp := compareOptionalUint16(a.TransportStreamID, b.TransportStreamID); cmp != 0 {
		return cmp < 0
	}
	if cmp := compareOptionalInt64(a.PacketOffset, b.PacketOffset); cmp != 0 {
		return cmp < 0
	}
	if cmp := compareOptionalInt64(a.PacketIndex, b.PacketIndex); cmp != 0 {
		return cmp < 0
	}
	if cmp := compareOptionalUint8(a.ContinuityCounter, b.ContinuityCounter); cmp != 0 {
		return cmp < 0
	}
	if cmp := compareOptionalInt64(a.PTS, b.PTS); cmp != 0 {
		return cmp < 0
	}
	if cmp := compareOptionalInt64(a.DTS, b.DTS); cmp != 0 {
		return cmp < 0
	}
	return false
}

func compareOptionalUint16(a, b *uint16) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return -1
	case b == nil:
		return 1
	case *a < *b:
		return -1
	case *a > *b:
		return 1
	default:
		return 0
	}
}

func compareOptionalInt64(a, b *int64) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return -1
	case b == nil:
		return 1
	case *a < *b:
		return -1
	case *a > *b:
		return 1
	default:
		return 0
	}
}

func compareOptionalUint8(a, b *uint8) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return -1
	case b == nil:
		return 1
	case *a < *b:
		return -1
	case *a > *b:
		return 1
	default:
		return 0
	}
}
