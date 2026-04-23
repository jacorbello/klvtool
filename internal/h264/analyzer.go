package h264

import (
	"fmt"
	"sort"

	ts "github.com/jacorbello/klvtool/internal/mpeg/ts"
)

// Analyzer aggregates H.264 NAL-level statistics for a single PID.
// It is fed completed PES units via Feed, then queried with Report.
// The analyzer is stateful; create one per PID per file.
type Analyzer struct {
	pid         uint16
	streamType  uint8
	idrCount    int
	spsCount    int
	ppsCount    int
	nonIDRCount int
	audCount    int
	seiCount    int
	pesUnits    int
	firstIDRPTS *int64
	lastIDRPTS  *int64
	firstPTS    *int64
	lastPTS     *int64
	ptsSamples  []int64 // PTS of every PES unit that had one — used for gap analysis.
}

// NewAnalyzer returns a fresh Analyzer for the given PID.
func NewAnalyzer(pid uint16) *Analyzer {
	return &Analyzer{pid: pid}
}

// SetStreamType records the PMT-declared stream type (e.g. 0x1B for
// H.264). This is carried through to the final report so the CLI can
// label the PID correctly. Optional.
func (a *Analyzer) SetStreamType(st uint8) {
	a.streamType = st
}

// Feed consumes one PES unit. The unit's payload is scanned for NAL
// units; counts are incremented and the first/last PTS of IDR-bearing
// units are recorded.
func (a *Analyzer) Feed(unit *ts.PESUnit) {
	if a == nil || unit == nil {
		return
	}
	a.pesUnits++
	if unit.PTS != nil {
		pts := *unit.PTS
		if a.firstPTS == nil {
			a.firstPTS = &pts
		}
		a.lastPTS = &pts
		a.ptsSamples = append(a.ptsSamples, pts)
	}

	var sawIDR bool
	IterateNALUnits(unit.Payload, func(nalType uint8, _ []byte) {
		switch nalType {
		case NALSlice:
			a.nonIDRCount++
		case NALIDR:
			a.idrCount++
			sawIDR = true
		case NALSEI:
			a.seiCount++
		case NALSPS:
			a.spsCount++
		case NALPPS:
			a.ppsCount++
		case NALAUD:
			a.audCount++
		}
	})

	if sawIDR && unit.PTS != nil {
		pts := *unit.PTS
		if a.firstIDRPTS == nil {
			a.firstIDRPTS = &pts
		}
		a.lastIDRPTS = &pts
	}
}

// Report returns an aggregated VideoReport with the computed verdict
// and supporting evidence.
func (a *Analyzer) Report() VideoReport {
	rep := VideoReport{
		PID:         a.pid,
		StreamType:  a.streamType,
		IDRCount:    a.idrCount,
		SPSCount:    a.spsCount,
		PPSCount:    a.ppsCount,
		NonIDRCount: a.nonIDRCount,
		AUDCount:    a.audCount,
		SEICount:    a.seiCount,
		PESUnits:    a.pesUnits,
		FirstIDRPTS: a.firstIDRPTS,
		LastIDRPTS:  a.lastIDRPTS,
		FirstPTS:    a.firstPTS,
		LastPTS:     a.lastPTS,
	}

	a.fillGapStats(&rep)

	var p0, degraded []string

	if a.idrCount == 0 {
		p0 = append(p0, fmt.Sprintf("no IDR frames found (scanned %d PES units) — MSE requires an IDR at stream start", a.pesUnits))
	}
	if a.spsCount == 0 {
		p0 = append(p0, "no SPS (sequence parameter set) NAL units — decoder cannot initialize")
	}
	if a.ppsCount == 0 {
		p0 = append(p0, "no PPS (picture parameter set) NAL units — decoder cannot initialize")
	}

	// P1: cadence checks, only when IDRs exist.
	if a.idrCount > 0 && a.firstIDRPTS != nil && a.firstPTS != nil {
		delayTicks := *a.firstIDRPTS - *a.firstPTS
		if delayTicks > 2*ptsClockHz {
			degraded = append(degraded, fmt.Sprintf("first IDR is %.2fs into the stream; HLS segmenters may stall", float64(delayTicks)/float64(ptsClockHz)))
		}
	}

	// P2: source-side frame drops inferred from PTS gaps.
	if rep.DeltaMode > 0 {
		total := rep.SingleGapCount + rep.DoubleGapCount + rep.LargerGapCount
		dropped := rep.DoubleGapCount + rep.LargerGapCount
		if total > 0 && float64(dropped)/float64(total) > 0.05 {
			degraded = append(degraded, fmt.Sprintf(
				"%d of %d PES PTS deltas are multi-frame gaps (~%d dropped frames); likely source-side loss",
				dropped, total, rep.DroppedFrameEst,
			))
		}
	}

	switch {
	case len(p0) > 0:
		rep.Verdict = VerdictStallsInMSE
		rep.Reasons = p0
		rep.FixHint = libx264FixHint
	case len(degraded) > 0:
		rep.Verdict = VerdictDegraded
		rep.Reasons = degraded
	default:
		rep.Verdict = VerdictPlayable
	}

	return rep
}

const ptsClockHz = 90000 // MPEG-TS 90kHz PTS clock.

// fillGapStats classifies inter-PES PTS deltas into single / double /
// larger gaps relative to the mode of the distribution. Results are
// written into rep. PTS samples are sorted first so B-frame reordering
// does not produce fake "gaps."
func (a *Analyzer) fillGapStats(rep *VideoReport) {
	if len(a.ptsSamples) < 3 {
		return
	}
	sorted := make([]int64, len(a.ptsSamples))
	copy(sorted, a.ptsSamples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	deltas := make([]int64, 0, len(sorted)-1)
	for i := 1; i < len(sorted); i++ {
		d := sorted[i] - sorted[i-1]
		if d > 0 {
			deltas = append(deltas, d)
		}
	}
	if len(deltas) == 0 {
		return
	}

	// Compute mode by finding the most common delta (rounded to the
	// nearest 30 ticks to tolerate jitter). 30 ticks ≈ 0.33 ms.
	bucket := make(map[int64]int, len(deltas))
	const bucketWidth int64 = 30
	var modeBucket int64
	var modeCount int
	for _, d := range deltas {
		b := (d + bucketWidth/2) / bucketWidth * bucketWidth
		bucket[b]++
		if bucket[b] > modeCount {
			modeCount = bucket[b]
			modeBucket = b
		}
	}
	if modeBucket <= 0 {
		return
	}

	rep.DeltaMode = modeBucket
	var dropped int
	for _, d := range deltas {
		ratio := float64(d) / float64(modeBucket)
		switch {
		case ratio < 1.25:
			rep.SingleGapCount++
		case ratio <= 2.5:
			rep.DoubleGapCount++
			dropped++
		default:
			rep.LargerGapCount++
			extra := int(ratio) - 1
			if extra < 1 {
				extra = 1
			}
			dropped += extra
		}
	}
	rep.DroppedFrameEst = dropped
}
