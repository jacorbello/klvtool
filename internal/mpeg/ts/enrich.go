package ts

import (
	"fmt"
	"io"

	"github.com/jacorbello/klvtool/internal/extract"
)

// EnrichRecords scans the .ts file and attaches per-PES metadata to existing
// RawPayloadRecords by matching on PID. Returns enriched copies — does not
// mutate the input.
//
// EnrichRecords assumes each input record corresponds to a unique PID. If
// two records share a PID, EnrichRecords returns an error — this assumption
// matches the current ffmpeg backend contract (one record per data stream).
func EnrichRecords(r io.ReadSeeker, records []extract.RawPayloadRecord) ([]extract.RawPayloadRecord, error) {
	if len(records) == 0 {
		return nil, nil
	}

	targetPIDs := make(map[uint16]bool)
	for _, rec := range records {
		if targetPIDs[rec.PID] {
			return nil, fmt.Errorf("duplicate PID 0x%04X in input records: EnrichRecords requires unique PIDs", rec.PID)
		}
		targetPIDs[rec.PID] = true
	}

	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek to start: %w", err)
	}

	scanner := NewPacketScanner(r, ScanConfig{PayloadPIDs: targetPIDs})
	asm := NewPESAssembler()

	// Record only the first PES unit's metadata per PID rather than
	// buffering every unit (with its full payload) across the whole
	// stream. On multi-GB captures this keeps memory bounded and lets
	// us exit the scan as soon as every target PID has a first unit.
	type enrichMetadata struct {
		pts         *int64
		dts         *int64
		packetStart int64
		packetIndex int64
		cc          *uint8
	}
	firstMeta := make(map[uint16]enrichMetadata, len(targetPIDs))

	record := func(unit *PESUnit) {
		if !targetPIDs[unit.PID] {
			return
		}
		if _, seen := firstMeta[unit.PID]; seen {
			return
		}
		m := enrichMetadata{
			pts:         unit.PTS,
			dts:         unit.DTS,
			packetStart: unit.PacketStart,
			packetIndex: unit.PacketIndex,
		}
		if unit.ContinuityCounter != nil {
			cc := *unit.ContinuityCounter
			m.cc = &cc
		}
		firstMeta[unit.PID] = m
	}

scan:
	for {
		pkt, err := scanner.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("scan: %w", err)
		}
		if unit := asm.Feed(pkt); unit != nil {
			record(unit)
			if len(firstMeta) == len(targetPIDs) {
				break scan
			}
		}
	}
	// Only flush the assembler if we haven't already satisfied every
	// target PID — avoids unnecessary work when the scan exited early.
	if len(firstMeta) < len(targetPIDs) {
		for _, unit := range asm.Flush() {
			u := unit
			record(&u)
		}
	}

	enriched := make([]extract.RawPayloadRecord, len(records))
	for i, rec := range records {
		enriched[i] = copyRecord(rec)

		meta, ok := firstMeta[rec.PID]
		if !ok {
			enriched[i].Warnings = append(enriched[i].Warnings,
				fmt.Sprintf("no PES units found for PID 0x%04X in transport stream", rec.PID))
			continue
		}

		enriched[i].PTS = meta.pts
		enriched[i].DTS = meta.dts
		offset := meta.packetStart
		index := meta.packetIndex
		enriched[i].PacketOffset = &offset
		enriched[i].PacketIndex = &index
		if meta.cc != nil {
			cc := *meta.cc
			enriched[i].ContinuityCounter = &cc
		}
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
