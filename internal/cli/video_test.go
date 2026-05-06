package cli

import (
	"testing"

	ts "github.com/jacorbello/klvtool/internal/mpeg/ts"
)

func TestUnsupportedVideoStreamsSortedByPID(t *testing.T) {
	// Unsorted map order; H.265 (0x24) is recognized-but-unsupported.
	table := ts.StreamTable{Programs: map[uint16][]ts.Stream{
		1: {
			{PID: 0x0300, StreamType: 0x24, ProgramNum: 1},
			{PID: 0x0100, StreamType: 0x24, ProgramNum: 1},
			{PID: 0x0200, StreamType: 0x24, ProgramNum: 1},
		},
	}}

	// Go's map iteration is randomized, so ordering bugs tend to
	// surface non-deterministically. Run several iterations to reduce
	// the chance of a false pass if the sort is ever removed.
	for i := 0; i < 20; i++ {
		got := unsupportedVideoStreams(table)
		if len(got) != 3 {
			t.Fatalf("iter %d: got %d streams, want 3", i, len(got))
		}
		if got[0].PID != 0x0100 || got[1].PID != 0x0200 || got[2].PID != 0x0300 {
			t.Fatalf("iter %d: PIDs not sorted ascending: %v", i, []uint16{got[0].PID, got[1].PID, got[2].PID})
		}
	}
}
