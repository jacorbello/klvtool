package cli

import (
	"bytes"
	"testing"

	ts "github.com/jacorbello/klvtool/internal/mpeg/ts"
)

func TestInspectPrintsStreamInventory(t *testing.T) {
	var out, errBuf bytes.Buffer

	table := ts.StreamTable{
		Programs: map[uint16][]ts.Stream{
			1: {
				{PID: 0x0100, StreamType: 0x1B, ProgramNum: 1},
				{PID: 0x0300, StreamType: 0x06, ProgramNum: 1},
			},
		},
	}
	stats := InspectStats{
		TotalPackets:  1000,
		PacketCounts:  map[uint16]int64{0x0100: 900, 0x0300: 50},
		PESUnitCounts: map[uint16]int{0x0300: 10},
	}

	cmd := &InspectCommand{
		Out: &out,
		Err: &errBuf,
		Inspect: func(path string) (ts.StreamTable, InspectStats, error) {
			return table, stats, nil
		},
	}

	code := cmd.Execute([]string{"--input", "test.ts"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}

	output := out.String()
	if !bytes.Contains([]byte(output), []byte("0x0300")) {
		t.Errorf("output should contain PID 0x0300, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("1000")) {
		t.Errorf("output should contain total packet count, got:\n%s", output)
	}
}

func TestInspectRequiresInputFlag(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := &InspectCommand{Out: &out, Err: &errBuf}
	code := cmd.Execute([]string{})
	if code != usageExitCode {
		t.Errorf("exit code = %d, want %d", code, usageExitCode)
	}
}

func TestInspectHelp(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := &InspectCommand{Out: &out, Err: &errBuf}
	code := cmd.Execute([]string{"--help"})
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if out.Len() == 0 {
		t.Error("expected usage output")
	}
}
