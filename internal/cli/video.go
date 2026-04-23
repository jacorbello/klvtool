package cli

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/jacorbello/klvtool/internal/h264"
	"github.com/jacorbello/klvtool/internal/model"
	ts "github.com/jacorbello/klvtool/internal/mpeg/ts"
)

// videoStreamTypes lists the PMT stream types the analyzer currently
// understands. H.265 (0x24) is recognized but not yet analyzed.
const (
	streamTypeH264 uint8 = 0x1B
	streamTypeH265 uint8 = 0x24
)

// isVideoStream reports whether a PMT stream type should be fed to the
// H.264 analyzer. Only H.264 in v1.
func isVideoStream(st uint8) bool {
	return st == streamTypeH264
}

// isKnownVideoStreamType reports whether a PMT stream type represents a
// video stream klvtool recognizes by name (used to emit a
// "not yet analyzed" notice for types like H.265 that we list but don't
// scan).
func isKnownVideoStreamType(st uint8) bool {
	return st == streamTypeH264 || st == streamTypeH265
}

// unsupportedVideoStreams returns the video streams present in the
// table whose NAL bitstream klvtool does not yet scan. Currently: H.265.
// Results are sorted by PID so render order is stable across runs.
func unsupportedVideoStreams(table ts.StreamTable) []ts.Stream {
	var out []ts.Stream
	for _, streams := range table.Programs {
		for _, s := range streams {
			if isKnownVideoStreamType(s.StreamType) && !isVideoStream(s.StreamType) {
				out = append(out, s)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PID < out[j].PID })
	return out
}

// videoPIDs returns the set of PIDs whose stream types the analyzer
// should scan. Deterministic iteration order is not required — the
// caller uses the result as a scanner filter.
func videoPIDs(table ts.StreamTable) map[uint16]uint8 {
	pids := make(map[uint16]uint8)
	for _, streams := range table.Programs {
		for _, s := range streams {
			if isVideoStream(s.StreamType) {
				pids[s.PID] = s.StreamType
			}
		}
	}
	return pids
}

// defaultVideoAnalyze is the production VideoAnalyze implementation.
// It opens path, re-scans the file with video PIDs selected, feeds
// each completed PES unit into a per-PID h264.Analyzer, and returns
// one VideoReport per video PID (empty slice if there are no video
// streams).
func defaultVideoAnalyze(path string, table ts.StreamTable) ([]h264.VideoReport, error) {
	pids := videoPIDs(table)
	if len(pids) == 0 {
		return nil, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, model.TSRead(fmt.Errorf("open %q: %w", path, err))
	}
	defer func() { _ = file.Close() }()

	payloadFilter := make(map[uint16]bool, len(pids))
	analyzers := make(map[uint16]*h264.Analyzer, len(pids))
	for pid, st := range pids {
		payloadFilter[pid] = true
		a := h264.NewAnalyzer(pid)
		a.SetStreamType(st)
		analyzers[pid] = a
	}

	scanner := ts.NewPacketScanner(file, ts.ScanConfig{PayloadPIDs: payloadFilter})
	asm := ts.NewPESAssembler()

	for {
		pkt, err := scanner.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if !payloadFilter[pkt.PID] {
			continue
		}
		if unit := asm.Feed(pkt); unit != nil {
			if a := analyzers[unit.PID]; a != nil {
				a.Feed(unit)
			}
		}
	}
	for _, unit := range asm.Flush() {
		u := unit
		if a := analyzers[u.PID]; a != nil {
			a.Feed(&u)
		}
	}

	reports := make([]h264.VideoReport, 0, len(analyzers))
	for _, a := range analyzers {
		reports = append(reports, a.Report())
	}
	sortReportsByPID(reports)
	return reports, nil
}

// sortReportsByPID gives deterministic output across runs.
func sortReportsByPID(reports []h264.VideoReport) {
	for i := 1; i < len(reports); i++ {
		for j := i; j > 0 && reports[j-1].PID > reports[j].PID; j-- {
			reports[j-1], reports[j] = reports[j], reports[j-1]
		}
	}
}
