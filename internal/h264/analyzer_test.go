package h264

import (
	"strings"
	"testing"

	ts "github.com/jacorbello/klvtool/internal/mpeg/ts"
)

// mkPES builds a PES unit whose payload contains the given NAL units,
// each prefixed with a 4-byte Annex B start code. The body byte of each
// unit is `nalType | 0x00` (forbidden bit clear, nal_ref_idc = 0).
func mkPES(t *testing.T, pid uint16, pts int64, nalTypes ...uint8) *ts.PESUnit {
	t.Helper()
	var payload []byte
	for _, nt := range nalTypes {
		payload = append(payload, 0x00, 0x00, 0x00, 0x01, nt&0x1F, 0xAA)
	}
	p := pts
	return &ts.PESUnit{PID: pid, PTS: &p, Payload: payload}
}

func TestAnalyzerReportVerdict(t *testing.T) {
	tests := []struct {
		name    string
		pid     uint16
		units   []*ts.PESUnit
		wantV   Verdict
		wantIDR int
		wantSPS int
		wantPPS int
		wantNIR int // non-IDR slice count
	}{
		{
			name: "zero IDR → STALLS_IN_MSE",
			pid:  0x0100,
			units: []*ts.PESUnit{
				mkPES(t, 0x0100, 0, NALAUD, NALSPS, NALPPS, NALSlice),
				mkPES(t, 0x0100, 3600, NALAUD, NALSlice),
				mkPES(t, 0x0100, 7200, NALAUD, NALSlice),
			},
			wantV:   VerdictStallsInMSE,
			wantIDR: 0,
			wantSPS: 1,
			wantPPS: 1,
			wantNIR: 3,
		},
		{
			name: "IDR present, short stream → PLAYABLE",
			pid:  0x0100,
			units: []*ts.PESUnit{
				mkPES(t, 0x0100, 0, NALAUD, NALSPS, NALPPS, NALIDR),
				mkPES(t, 0x0100, 3000, NALAUD, NALSlice),
				mkPES(t, 0x0100, 6000, NALAUD, NALSlice),
			},
			wantV:   VerdictPlayable,
			wantIDR: 1,
			wantSPS: 1,
			wantPPS: 1,
			wantNIR: 2,
		},
		{
			name:    "no units at all → STALLS_IN_MSE",
			pid:     0x0100,
			units:   nil,
			wantV:   VerdictStallsInMSE,
			wantIDR: 0,
		},
		{
			name: "IDR present but missing SPS → STALLS_IN_MSE",
			pid:  0x0100,
			units: []*ts.PESUnit{
				mkPES(t, 0x0100, 0, NALPPS, NALIDR),
				mkPES(t, 0x0100, 3000, NALSlice),
			},
			wantV:   VerdictStallsInMSE,
			wantIDR: 1,
			wantSPS: 0,
			wantPPS: 1,
			wantNIR: 1,
		},
		{
			name: "IDR present but missing PPS → STALLS_IN_MSE",
			pid:  0x0100,
			units: []*ts.PESUnit{
				mkPES(t, 0x0100, 0, NALSPS, NALIDR),
			},
			wantV:   VerdictStallsInMSE,
			wantIDR: 1,
			wantSPS: 1,
			wantPPS: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewAnalyzer(tt.pid)
			for _, u := range tt.units {
				a.Feed(u)
			}
			rep := a.Report()

			if rep.Verdict != tt.wantV {
				t.Errorf("verdict = %q, want %q (reasons: %v)", rep.Verdict, tt.wantV, rep.Reasons)
			}
			if rep.IDRCount != tt.wantIDR {
				t.Errorf("IDRCount = %d, want %d", rep.IDRCount, tt.wantIDR)
			}
			if rep.SPSCount != tt.wantSPS {
				t.Errorf("SPSCount = %d, want %d", rep.SPSCount, tt.wantSPS)
			}
			if rep.PPSCount != tt.wantPPS {
				t.Errorf("PPSCount = %d, want %d", rep.PPSCount, tt.wantPPS)
			}
			if rep.NonIDRCount != tt.wantNIR {
				t.Errorf("NonIDRCount = %d, want %d", rep.NonIDRCount, tt.wantNIR)
			}
		})
	}
}

func TestAnalyzerReportFirstIDRPTS(t *testing.T) {
	a := NewAnalyzer(0x0100)
	a.Feed(mkPES(t, 0x0100, 1000, NALSPS, NALPPS, NALSlice))
	a.Feed(mkPES(t, 0x0100, 4000, NALIDR))
	a.Feed(mkPES(t, 0x0100, 7000, NALIDR))

	rep := a.Report()
	if rep.IDRCount != 2 {
		t.Fatalf("IDRCount = %d, want 2", rep.IDRCount)
	}
	if rep.FirstIDRPTS == nil || *rep.FirstIDRPTS != 4000 {
		t.Errorf("FirstIDRPTS = %v, want 4000", rep.FirstIDRPTS)
	}
}

func TestAnalyzerDetectsFrameDropsAsDegraded(t *testing.T) {
	a := NewAnalyzer(0x0100)
	// Establish a clean IDR + SPS/PPS so P0 passes.
	a.Feed(mkPES(t, 0x0100, 0, NALSPS, NALPPS, NALIDR))
	// 29.97 fps → ~3003 ticks between frames. Seed 20 "clean" frames.
	// Then inject 8 double-gaps (8/28 ≈ 29% multi-frame drops).
	pts := int64(3003)
	for i := 0; i < 20; i++ {
		a.Feed(mkPES(t, 0x0100, pts, NALSlice))
		pts += 3003
	}
	for i := 0; i < 8; i++ {
		a.Feed(mkPES(t, 0x0100, pts, NALSlice))
		pts += 6006 // double-slot gap → a dropped frame
	}

	rep := a.Report()
	if rep.Verdict != VerdictDegraded {
		t.Fatalf("verdict = %q, want %q (reasons: %v)", rep.Verdict, VerdictDegraded, rep.Reasons)
	}
	joined := strings.Join(rep.Reasons, " ")
	if !strings.Contains(joined, "frame") {
		t.Errorf("expected frame-drop reason, got %v", rep.Reasons)
	}
}

func TestAnalyzerCleanStreamIsPlayableNotDegraded(t *testing.T) {
	a := NewAnalyzer(0x0100)
	a.Feed(mkPES(t, 0x0100, 0, NALSPS, NALPPS, NALIDR))
	pts := int64(3003)
	for i := 0; i < 30; i++ {
		a.Feed(mkPES(t, 0x0100, pts, NALSlice))
		pts += 3003
	}
	rep := a.Report()
	if rep.Verdict != VerdictPlayable {
		t.Errorf("verdict = %q, want PLAYABLE (reasons: %v)", rep.Verdict, rep.Reasons)
	}
}

func TestAnalyzerBFrameReorderingNotFlaggedAsDrops(t *testing.T) {
	// Simulate decode-order PTS from an H.264 stream with B-frames:
	// encode order  I P B B P B B P B B ...
	// PTS in decode order arrives non-monotonically. The analyzer must
	// sort PTS samples before computing deltas so reordering does not
	// look like frame drops.
	a := NewAnalyzer(0x0100)
	a.Feed(mkPES(t, 0x0100, 0, NALSPS, NALPPS, NALIDR))
	const step int64 = 3003
	// 30 presentations, emitted in an IPBBPBB... pattern.
	order := []int{3, 1, 2, 6, 4, 5, 9, 7, 8, 12, 10, 11, 15, 13, 14, 18, 16, 17, 21, 19, 20, 24, 22, 23, 27, 25, 26, 30, 28, 29}
	for _, frameIdx := range order {
		a.Feed(mkPES(t, 0x0100, int64(frameIdx)*step, NALSlice))
	}

	rep := a.Report()
	if rep.Verdict != VerdictPlayable {
		t.Fatalf("B-frame reorder flagged as %q (reasons: %v)", rep.Verdict, rep.Reasons)
	}
	if rep.DoubleGapCount != 0 || rep.LargerGapCount != 0 {
		t.Errorf("sorted deltas should have no multi-frame gaps, got double=%d larger=%d",
			rep.DoubleGapCount, rep.LargerGapCount)
	}
}

func TestAnalyzerDuplicatePTSDoesNotInflateDrops(t *testing.T) {
	// Two PES units sharing the same PTS produce a zero delta that must
	// be dropped, not classified as a gap.
	a := NewAnalyzer(0x0100)
	a.Feed(mkPES(t, 0x0100, 0, NALSPS, NALPPS, NALIDR))
	pts := int64(3003)
	for i := 0; i < 20; i++ {
		a.Feed(mkPES(t, 0x0100, pts, NALSlice))
		a.Feed(mkPES(t, 0x0100, pts, NALSlice)) // duplicate
		pts += 3003
	}

	rep := a.Report()
	if rep.Verdict != VerdictPlayable {
		t.Errorf("duplicate PTS flagged as %q (reasons: %v)", rep.Verdict, rep.Reasons)
	}
	if rep.DoubleGapCount != 0 || rep.LargerGapCount != 0 {
		t.Errorf("zero-delta samples should be filtered, got double=%d larger=%d",
			rep.DoubleGapCount, rep.LargerGapCount)
	}
}

func TestAnalyzerHandlesPTSWraparound(t *testing.T) {
	// A 26.5h capture will cross the 33-bit PTS wrap boundary.
	// Samples on either side of the wrap must not be misclassified as
	// one huge multi-frame gap.
	const pts33Max = int64(1) << 33
	const step = int64(3003)

	a := NewAnalyzer(0x0100)
	a.Feed(mkPES(t, 0x0100, pts33Max-30*step, NALSPS, NALPPS, NALIDR))
	// 20 samples leading up to the wrap.
	for i := int64(29); i > 0; i-- {
		a.Feed(mkPES(t, 0x0100, pts33Max-i*step, NALSlice))
	}
	// 20 samples after the wrap (low PTS values).
	for i := int64(0); i < 20; i++ {
		a.Feed(mkPES(t, 0x0100, i*step, NALSlice))
	}

	rep := a.Report()
	if rep.Verdict != VerdictPlayable {
		t.Fatalf("wrap flagged as %q (reasons: %v, mode=%d, single=%d double=%d larger=%d)",
			rep.Verdict, rep.Reasons, rep.DeltaMode, rep.SingleGapCount, rep.DoubleGapCount, rep.LargerGapCount)
	}
	if rep.LargerGapCount != 0 {
		t.Errorf("wrap should not produce a larger-gap classification, got %d", rep.LargerGapCount)
	}
}

func TestAnalyzerFixHintOnlyForStalls(t *testing.T) {
	// Playable: no hint.
	a := NewAnalyzer(0x0100)
	a.Feed(mkPES(t, 0x0100, 0, NALSPS, NALPPS, NALIDR))
	rep := a.Report()
	if rep.FixHint != "" {
		t.Errorf("playable FixHint should be empty, got %q", rep.FixHint)
	}

	// Stalls: hint must mention libx264.
	a = NewAnalyzer(0x0100)
	a.Feed(mkPES(t, 0x0100, 0, NALSPS, NALPPS, NALSlice))
	rep = a.Report()
	if rep.FixHint == "" {
		t.Errorf("stalls FixHint should be populated")
	}
}
