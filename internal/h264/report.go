package h264

// Verdict is klvtool's one-line playability summary for a video PID.
type Verdict string

const (
	VerdictPlayable    Verdict = "PLAYABLE"
	VerdictStallsInMSE Verdict = "STALLS_IN_MSE"
	VerdictDegraded    Verdict = "DEGRADED"
)

// VideoReport is the aggregated NAL-level diagnostic for a single
// video PID. It is the primary output of Analyzer.Report and the
// input to the CLI's text rendering.
type VideoReport struct {
	PID         uint16
	StreamType  uint8
	Verdict     Verdict
	Reasons     []string
	FixHint     string
	IDRCount    int
	SPSCount    int
	PPSCount    int
	NonIDRCount int
	AUDCount    int
	SEICount    int
	PESUnits    int
	FirstIDRPTS *int64
	LastIDRPTS  *int64
	FirstPTS    *int64
	LastPTS     *int64

	// Frame-drop analysis (derived from PES PTS deltas).
	// Deltas outside the documented bands are intentionally unclassified:
	// sub-mode values (ratio < 0.75) are jitter or near-duplicate samples,
	// and ambiguous middle values (1.25 < ratio < 1.5) are neither clearly
	// on-nominal nor clearly a dropped frame.
	DeltaMode       int64 // most common inter-PES PTS delta, in 90kHz ticks; 0 if unknown
	SingleGapCount  int   // deltas in [0.75, 1.25] × DeltaMode (on-nominal spacing)
	DoubleGapCount  int   // deltas in [1.5, 2.5] × DeltaMode (approximately one frame missing)
	LargerGapCount  int   // deltas > 2.5 × DeltaMode (multi-frame gap)
	DroppedFrameEst int   // estimated dropped frames: double × 1 + sum of (int(ratio) - 1) for larger gaps
}

const libx264FixHint = `Re-encode with libx264 to synthesize IDR frames. Example:
    ffmpeg -i <input.ts> -map 0:v:0 -map 0:d? \
      -c:v libx264 -g 60 -keyint_min 60 -sc_threshold 0 \
      -c:d copy -f mpegts <output.ts>`
