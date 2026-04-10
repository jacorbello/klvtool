package ts

import (
	"fmt"
	"io"

	"github.com/jacorbello/klvtool/internal/extract"
)

// EnrichRecords scans the .ts file and attaches per-PES metadata to existing
// RawPayloadRecords by matching on PID. Returns enriched copies — does not
// mutate the input.
func EnrichRecords(r io.ReadSeeker, records []extract.RawPayloadRecord) ([]extract.RawPayloadRecord, error) {
	if len(records) == 0 {
		return nil, nil
	}

	targetPIDs := make(map[uint16]bool)
	for _, rec := range records {
		targetPIDs[rec.PID] = true
	}

	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek to start: %w", err)
	}

	scanner := NewPacketScanner(r, ScanConfig{PayloadPIDs: targetPIDs})
	asm := NewPESAssembler()

	pesUnits := make(map[uint16][]PESUnit)
	for {
		pkt, err := scanner.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("scan: %w", err)
		}
		if unit := asm.Feed(pkt); unit != nil {
			pesUnits[unit.PID] = append(pesUnits[unit.PID], *unit)
		}
	}
	for _, unit := range asm.Flush() {
		pesUnits[unit.PID] = append(pesUnits[unit.PID], unit)
	}

	enriched := make([]extract.RawPayloadRecord, len(records))
	for i, rec := range records {
		enriched[i] = copyRecord(rec)

		units, ok := pesUnits[rec.PID]
		if !ok || len(units) == 0 {
			enriched[i].Warnings = append(enriched[i].Warnings,
				fmt.Sprintf("no PES units found for PID 0x%04X in transport stream", rec.PID))
			continue
		}

		first := units[0]
		enriched[i].PTS = first.PTS
		enriched[i].DTS = first.DTS
		offset := first.PacketStart
		index := first.PacketIndex
		enriched[i].PacketOffset = &offset
		enriched[i].PacketIndex = &index
	}

	return enriched, nil
}

func copyRecord(rec extract.RawPayloadRecord) extract.RawPayloadRecord {
	cp := rec
	if rec.Payload != nil {
		cp.Payload = make([]byte, len(rec.Payload))
		copy(cp.Payload, rec.Payload)
	}
	if rec.Warnings != nil {
		cp.Warnings = make([]string, len(rec.Warnings))
		copy(cp.Warnings, rec.Warnings)
	}
	return cp
}
