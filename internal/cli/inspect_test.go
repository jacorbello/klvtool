package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ts "github.com/jacorbello/klvtool/internal/mpeg/ts"
)

func TestInspectRejectsNonExistentInput(t *testing.T) {
	var stderr bytes.Buffer
	cmd := &InspectCommand{
		Out: nil,
		Err: &stderr,
		Inspect: func(path string) (ts.StreamTable, InspectStats, error) {
			t.Fatal("inspect should not be called for non-existent input")
			return ts.StreamTable{}, InspectStats{}, nil
		},
	}

	missing := filepath.Join(t.TempDir(), "missing.ts")
	got := cmd.Execute([]string{"--input", missing})
	if got != 1 {
		t.Fatalf("exit code = %d, want 1", got)
	}
	text := stderr.String()
	if !strings.Contains(text, "ts_read_failure") {
		t.Fatalf("expected ts_read_failure error code, got %q", text)
	}
	if !strings.Contains(text, missing) {
		t.Fatalf("expected file path in error, got %q", text)
	}
}

func TestInspectRejectsDirectory(t *testing.T) {
	var stderr bytes.Buffer
	cmd := &InspectCommand{
		Out: nil,
		Err: &stderr,
		Inspect: func(path string) (ts.StreamTable, InspectStats, error) {
			t.Fatal("inspect should not be called for directory input")
			return ts.StreamTable{}, InspectStats{}, nil
		},
	}

	got := cmd.Execute([]string{"--input", t.TempDir()})
	if got != 1 {
		t.Fatalf("exit code = %d, want 1", got)
	}
	text := stderr.String()
	if !strings.Contains(text, "ts_read_failure") {
		t.Fatalf("expected ts_read_failure error code, got %q", text)
	}
	if !strings.Contains(text, "not a regular file") {
		t.Fatalf("expected 'not a regular file' message, got %q", text)
	}
}

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

	input := filepath.Join(t.TempDir(), "test.ts")
	if err := os.WriteFile(input, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	code := cmd.Execute([]string{"--input", input})
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

func TestInspectHelpMixedWithFlags(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := &InspectCommand{Out: &out, Err: &errBuf}
	code := cmd.Execute([]string{"--help", "--input", "foo.ts"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Errorf("expected usage on stdout, got %q", out.String())
	}
	if errBuf.Len() != 0 {
		t.Errorf("expected empty stderr, got %q", errBuf.String())
	}
}

// TestInspectSurfacesPESDiagnosticsEndToEnd builds a synthetic .ts file
// containing a PAT, PMT, and a data stream with a deliberate continuity
// counter gap, runs defaultInspect against it, and verifies the continuity
// gap diagnostic makes it into the final report.
func TestInspectSurfacesPESDiagnosticsEndToEnd(t *testing.T) {
	pat := buildTSPacket(0x0000, 0, true, []byte{
		0x00, 0x00, 0xB0, 0x0D,
		0x00, 0x01, 0xC1, 0x00, 0x00,
		0x00, 0x01, 0xF0, 0x00,
		0x00, 0x00, 0x00, 0x00,
	})
	pmt := buildTSPacket(0x1000, 0, true, []byte{
		0x00, 0x02, 0xB0, 0x12,
		0x00, 0x01, 0xC1, 0x00, 0x00,
		0xE1, 0x00, 0xF0, 0x00,
		0x06, 0xE3, 0x00, 0xF0, 0x00,
		0x00, 0x00, 0x00, 0x00,
	})

	// First PES unit: CC=0, PUSI=true, PES header with no PTS.
	pes1 := buildTSPacket(0x0300, 0, true, []byte{
		0x00, 0x00, 0x01, 0xBD, 0x00, 0x05, 0x80, 0x00, 0x00,
		0xAA,
	})
	// Continuation: CC=2 (expected 1, so gap).
	pes2 := buildTSPacket(0x0300, 2, false, []byte{0xBB})
	// Trigger emit of the first unit with a new PUSI (CC=3 — continues from CC=2).
	pes3 := buildTSPacket(0x0300, 3, true, []byte{
		0x00, 0x00, 0x01, 0xBD, 0x00, 0x05, 0x80, 0x00, 0x00,
		0xCC,
	})

	var file bytes.Buffer
	file.Write(pat)
	file.Write(pmt)
	file.Write(pes1)
	file.Write(pes2)
	file.Write(pes3)

	dir := t.TempDir()
	path := filepath.Join(dir, "sample.ts")
	if err := os.WriteFile(path, file.Bytes(), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	_, stats, err := defaultInspect(path)
	if err != nil {
		t.Fatalf("defaultInspect: %v", err)
	}

	foundGap := false
	for _, d := range stats.Diagnostics {
		if d.Code == "continuity_gap" {
			foundGap = true
			break
		}
	}
	if !foundGap {
		t.Errorf("expected continuity_gap diagnostic, got %+v", stats.Diagnostics)
	}
}

// buildTSPacket constructs a synthetic 188-byte TS packet — a test-local
// duplicate of internal/mpeg/ts.buildPacket since that helper is unexported.
func buildTSPacket(pid uint16, cc uint8, pusi bool, payload []byte) []byte {
	pkt := make([]byte, 188)
	pkt[0] = 0x47
	pkt[1] = byte(pid>>8) & 0x1F
	if pusi {
		pkt[1] |= 0x40
	}
	pkt[2] = byte(pid)
	pkt[3] = 0x10 | (cc & 0x0F) // payload only, no adaptation
	copy(pkt[4:], payload)
	return pkt
}
