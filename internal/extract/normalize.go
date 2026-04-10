package extract

import (
	"fmt"
	"sort"
)

// CanonicalizeRecords returns a stable, normalized copy of the extracted records.
func CanonicalizeRecords(records []PayloadRecord) []PayloadRecord {
	out := append([]PayloadRecord(nil), records...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].PID != out[j].PID {
			return out[i].PID < out[j].PID
		}
		return string(out[i].Payload) < string(out[j].Payload)
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
