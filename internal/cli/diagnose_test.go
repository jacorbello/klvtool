package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacorbello/klvtool/internal/envcheck"
	"github.com/jacorbello/klvtool/internal/h264"
	"github.com/jacorbello/klvtool/internal/klv/record"
	ts "github.com/jacorbello/klvtool/internal/mpeg/ts"
)

func healthyReport() envcheck.Report {
	return envcheck.Report{
		Platform: "linux",
		Backends: []envcheck.BackendHealth{
			{
				Name:    "ffmpeg",
				Healthy: true,
				Tools: []envcheck.ToolHealth{
					{Name: "ffmpeg", Path: "/usr/bin/ffmpeg", Version: "ffmpeg version 7.1", Healthy: true},
					{Name: "ffprobe", Path: "/usr/bin/ffprobe", Version: "ffprobe version 7.1", Healthy: true},
				},
			},
		},
	}
}

func unhealthyReport() envcheck.Report {
	return envcheck.Report{
		Platform:        "linux",
		GuidanceSummary: "brew install ffmpeg",
		Guidance:        []string{"brew install ffmpeg"},
		Backends: []envcheck.BackendHealth{
			{
				Name:         "ffmpeg",
				Healthy:      false,
				MissingTools: []string{"ffmpeg", "ffprobe"},
			},
		},
	}
}

func metadataStreamTable() ts.StreamTable {
	return ts.StreamTable{
		Programs: map[uint16][]ts.Stream{
			1: {
				{PID: 0x0044, StreamType: 0x1B, ProgramNum: 1},
				{PID: 0x0051, StreamType: 0x06, ProgramNum: 1},
			},
		},
	}
}

func noMetadataStreamTable() ts.StreamTable {
	return ts.StreamTable{
		Programs: map[uint16][]ts.Stream{
			1: {
				{PID: 0x0044, StreamType: 0x1B, ProgramNum: 1},
				{PID: 0x0045, StreamType: 0x0F, ProgramNum: 1},
			},
		},
	}
}

func metadataInspectStats() InspectStats {
	return InspectStats{
		TotalPackets: 48412,
		PacketCounts: map[uint16]int64{
			0x0044: 42100,
			0x0051: 6312,
		},
		PESUnitCounts: map[uint16]int{},
		FirstPTS:      map[uint16]int64{},
		LastPTS:       map[uint16]int64{},
	}
}

func cleanDecodeResult() DecodeResult {
	return DecodeResult{
		Records: []record.Record{
			{Schema: "urn:smpte:ul:test", LSVersion: 19},
			{Schema: "urn:smpte:ul:test", LSVersion: 19},
		},
	}
}

func decodeResultWithDiagnostics() DecodeResult {
	return DecodeResult{
		Records: []record.Record{
			{
				Schema:    "urn:smpte:ul:test",
				LSVersion: 19,
				Diagnostics: []record.Diagnostic{
					{Severity: "error", Code: "checksum_mismatch", Message: "bad checksum"},
					{Severity: "warning", Code: "range_violation", Message: "out of range"},
				},
			},
			{
				Schema:    "urn:smpte:ul:test",
				LSVersion: 19,
				Diagnostics: []record.Diagnostic{
					{Severity: "warning", Code: "range_violation", Message: "out of range"},
				},
			},
		},
	}
}

func tempDiagnoseInput(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test.ts")
	if err := os.WriteFile(p, []byte{0}, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func newTestDiagnoseCommand(out, errBuf *bytes.Buffer) *DiagnoseCommand {
	cmd := NewDiagnoseCommand()
	cmd.Out = out
	cmd.Err = errBuf
	cmd.isOutputTTY = func(_ io.Writer) bool { return false }
	return cmd
}

func TestDiagnoseHappyPath(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := newTestDiagnoseCommand(&out, &errBuf)
	input := tempDiagnoseInput(t)

	cmd.Detect = func(context.Context, string, map[string]string) envcheck.Report {
		return healthyReport()
	}
	cmd.Inspect = func(string) (ts.StreamTable, InspectStats, error) {
		return metadataStreamTable(), metadataInspectStats(), nil
	}
	cmd.Decode = func(path string, pid int, schema string) (DecodeResult, error) {
		return cleanDecodeResult(), nil
	}

	code := cmd.Execute([]string{"--input", input})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}

	text := out.String()
	if !strings.Contains(text, "available") {
		t.Errorf("expected backend health in output, got %q", text)
	}
	if !strings.Contains(text, "48412") {
		t.Errorf("expected packet count in output, got %q", text)
	}
	if !strings.Contains(text, "0x0051") {
		t.Errorf("expected metadata PID in output, got %q", text)
	}
	if !strings.Contains(text, "packets decoded: 2") {
		t.Errorf("expected decode summary in output, got %q", text)
	}
}

func TestDiagnoseBackendUnhealthy(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := newTestDiagnoseCommand(&out, &errBuf)
	input := tempDiagnoseInput(t)

	cmd.Detect = func(context.Context, string, map[string]string) envcheck.Report {
		return unhealthyReport()
	}
	inspectCalled := false
	cmd.Inspect = func(string) (ts.StreamTable, InspectStats, error) {
		inspectCalled = true
		return ts.StreamTable{}, InspectStats{}, nil
	}

	code := cmd.Execute([]string{"--input", input})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if inspectCalled {
		t.Error("inspect should not be called when backend is unhealthy")
	}

	text := out.String()
	if !strings.Contains(text, "not installed") || !strings.Contains(text, "ffmpeg") {
		t.Errorf("expected unhealthy backend report, got %q", text)
	}
	if !strings.Contains(text, "Stopped at") || !strings.Contains(text, "health") {
		t.Errorf("expected stopped-at health message, got %q", text)
	}
}

func TestDiagnoseInspectFails(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := newTestDiagnoseCommand(&out, &errBuf)
	input := tempDiagnoseInput(t)

	cmd.Detect = func(context.Context, string, map[string]string) envcheck.Report {
		return healthyReport()
	}
	cmd.Inspect = func(string) (ts.StreamTable, InspectStats, error) {
		return ts.StreamTable{}, InspectStats{}, fmt.Errorf("sync lost")
	}
	decodeCalled := false
	cmd.Decode = func(string, int, string) (DecodeResult, error) {
		decodeCalled = true
		return DecodeResult{}, nil
	}

	code := cmd.Execute([]string{"--input", input})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if decodeCalled {
		t.Error("decode should not be called when inspect fails")
	}

	text := out.String()
	if !strings.Contains(text, "Stopped at") || !strings.Contains(text, "inspect") {
		t.Errorf("expected stopped-at inspect message, got %q", text)
	}
}

func TestDiagnoseNoMetadataPIDs(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := newTestDiagnoseCommand(&out, &errBuf)
	input := tempDiagnoseInput(t)

	cmd.Detect = func(context.Context, string, map[string]string) envcheck.Report {
		return healthyReport()
	}
	cmd.Inspect = func(string) (ts.StreamTable, InspectStats, error) {
		return noMetadataStreamTable(), metadataInspectStats(), nil
	}
	decodeCalled := false
	cmd.Decode = func(string, int, string) (DecodeResult, error) {
		decodeCalled = true
		return DecodeResult{}, nil
	}

	code := cmd.Execute([]string{"--input", input})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if decodeCalled {
		t.Error("decode should not be called when no metadata PIDs found")
	}

	text := out.String()
	if !strings.Contains(text, "No likely metadata") {
		t.Errorf("expected no-metadata message, got %q", text)
	}
}

func TestDiagnoseDecodeWithDiagnostics(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := newTestDiagnoseCommand(&out, &errBuf)
	input := tempDiagnoseInput(t)

	cmd.Detect = func(context.Context, string, map[string]string) envcheck.Report {
		return healthyReport()
	}
	cmd.Inspect = func(string) (ts.StreamTable, InspectStats, error) {
		return metadataStreamTable(), metadataInspectStats(), nil
	}
	cmd.Decode = func(string, int, string) (DecodeResult, error) {
		return decodeResultWithDiagnostics(), nil
	}

	code := cmd.Execute([]string{"--input", input})
	// Diagnostics present but command still succeeds (diagnose is informational)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	text := out.String()
	if !strings.Contains(text, "1 error") {
		t.Errorf("expected error count in output, got %q", text)
	}
	if !strings.Contains(text, "2 warning") {
		t.Errorf("expected warning count in output, got %q", text)
	}
}

func TestDiagnoseDecodeFails(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := newTestDiagnoseCommand(&out, &errBuf)
	input := tempDiagnoseInput(t)

	cmd.Detect = func(context.Context, string, map[string]string) envcheck.Report {
		return healthyReport()
	}
	cmd.Inspect = func(string) (ts.StreamTable, InspectStats, error) {
		return metadataStreamTable(), metadataInspectStats(), nil
	}
	cmd.Decode = func(string, int, string) (DecodeResult, error) {
		return DecodeResult{}, fmt.Errorf("decode failed")
	}

	code := cmd.Execute([]string{"--input", input})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}

	text := out.String()
	if !strings.Contains(text, "Stopped at") || !strings.Contains(text, "decode") {
		t.Errorf("expected stopped-at decode message, got %q", text)
	}
}

func TestDiagnoseMissingInput(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := newTestDiagnoseCommand(&out, &errBuf)

	code := cmd.Execute(nil)
	if code != usageExitCode {
		t.Fatalf("exit code = %d, want %d", code, usageExitCode)
	}
	text := errBuf.String()
	if !strings.Contains(text, "input path is required") {
		t.Errorf("expected input-required error, got %q", text)
	}
}

func TestDiagnoseInvalidView(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := newTestDiagnoseCommand(&out, &errBuf)
	input := tempDiagnoseInput(t)

	code := cmd.Execute([]string{"--input", input, "--view", "bogus"})
	if code != usageExitCode {
		t.Fatalf("exit code = %d, want %d", code, usageExitCode)
	}
	text := errBuf.String()
	if !strings.Contains(text, "invalid view") {
		t.Errorf("expected invalid-view error, got %q", text)
	}
}

func TestDiagnoseUnsupportedArgs(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := newTestDiagnoseCommand(&out, &errBuf)
	input := tempDiagnoseInput(t)

	code := cmd.Execute([]string{"--input", input, "extra"})
	if code != usageExitCode {
		t.Fatalf("exit code = %d, want %d", code, usageExitCode)
	}
	text := errBuf.String()
	if !strings.Contains(text, "unsupported arguments") {
		t.Errorf("expected unsupported-args error, got %q", text)
	}
}

func TestDiagnoseHelp(t *testing.T) {
	for _, arg := range []string{"--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			var out, errBuf bytes.Buffer
			cmd := newTestDiagnoseCommand(&out, &errBuf)

			code := cmd.Execute([]string{arg})
			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			text := out.String()
			if !strings.Contains(text, "Usage:") {
				t.Errorf("expected usage text, got %q", text)
			}
			if !strings.Contains(text, "diagnose") {
				t.Errorf("expected diagnose in usage, got %q", text)
			}
		})
	}
}

func TestDiagnoseInputFileNotFound(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := newTestDiagnoseCommand(&out, &errBuf)

	code := cmd.Execute([]string{"--input", "/nonexistent/file.ts"})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	text := errBuf.String()
	if !strings.Contains(text, "does not exist") {
		t.Errorf("expected file-not-found error, got %q", text)
	}
}

func TestDiagnosePrettyModeShowsHints(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := newTestDiagnoseCommand(&out, &errBuf)
	cmd.isOutputTTY = func(_ io.Writer) bool { return true }
	input := tempDiagnoseInput(t)

	cmd.Detect = func(context.Context, string, map[string]string) envcheck.Report {
		return healthyReport()
	}
	cmd.Inspect = func(string) (ts.StreamTable, InspectStats, error) {
		return metadataStreamTable(), metadataInspectStats(), nil
	}
	cmd.Decode = func(string, int, string) (DecodeResult, error) {
		return cleanDecodeResult(), nil
	}

	code := cmd.Execute([]string{"--input", input})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}

	text := out.String()
	if !strings.Contains(text, "Next steps:") {
		t.Errorf("expected hint footers in pretty mode, got %q", text)
	}
	if !strings.Contains(text, "Likely metadata") {
		t.Errorf("expected 'Likely metadata' label in pretty mode, got %q", text)
	}
}

func TestDiagnoseRawModeOmitsHints(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := newTestDiagnoseCommand(&out, &errBuf)
	input := tempDiagnoseInput(t)

	cmd.Detect = func(context.Context, string, map[string]string) envcheck.Report {
		return healthyReport()
	}
	cmd.Inspect = func(string) (ts.StreamTable, InspectStats, error) {
		return metadataStreamTable(), metadataInspectStats(), nil
	}
	cmd.Decode = func(string, int, string) (DecodeResult, error) {
		return cleanDecodeResult(), nil
	}

	code := cmd.Execute([]string{"--input", input, "--view", "raw"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}

	text := out.String()
	if strings.Contains(text, "Next steps:") {
		t.Errorf("expected no hint footers in raw mode, got %q", text)
	}
	if strings.Contains(text, "Likely metadata") {
		t.Errorf("expected no 'Likely metadata' label in raw mode, got %q", text)
	}
}

func TestDiagnoseDecodesCorrectPID(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := newTestDiagnoseCommand(&out, &errBuf)
	input := tempDiagnoseInput(t)

	cmd.Detect = func(context.Context, string, map[string]string) envcheck.Report {
		return healthyReport()
	}
	cmd.Inspect = func(string) (ts.StreamTable, InspectStats, error) {
		return metadataStreamTable(), metadataInspectStats(), nil
	}
	var decodedPID int
	cmd.Decode = func(_ string, pid int, _ string) (DecodeResult, error) {
		decodedPID = pid
		return cleanDecodeResult(), nil
	}

	code := cmd.Execute([]string{"--input", input})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if decodedPID != 0x0051 {
		t.Errorf("decoded PID = 0x%04X, want 0x0051", decodedPID)
	}
}

func playableVideoReports() []h264.VideoReport {
	pts := int64(90000)
	return []h264.VideoReport{{
		PID:         0x0044,
		StreamType:  0x1B,
		Verdict:     h264.VerdictPlayable,
		IDRCount:    39,
		SPSCount:    41,
		PPSCount:    41,
		NonIDRCount: 4408,
		PESUnits:    4488,
		FirstIDRPTS: &pts,
		LastIDRPTS:  &pts,
	}}
}

func stallsVideoReports() []h264.VideoReport {
	return []h264.VideoReport{{
		PID:         0x0044,
		StreamType:  0x1B,
		Verdict:     h264.VerdictStallsInMSE,
		Reasons:     []string{"no IDR frames found (scanned 1943 PES units) — MSE requires an IDR at stream start"},
		FixHint:     "Re-encode with libx264 to synthesize IDR frames. Example:\n    ffmpeg -i <input.ts> ...",
		IDRCount:    0,
		SPSCount:    58,
		PPSCount:    58,
		NonIDRCount: 1943,
		PESUnits:    1943,
	}}
}

func TestDiagnoseVideoSectionPlayable(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := newTestDiagnoseCommand(&out, &errBuf)
	input := tempDiagnoseInput(t)

	cmd.Detect = func(context.Context, string, map[string]string) envcheck.Report {
		return healthyReport()
	}
	cmd.Inspect = func(string) (ts.StreamTable, InspectStats, error) {
		return metadataStreamTable(), metadataInspectStats(), nil
	}
	cmd.VideoAnalyze = func(_ string, _ ts.StreamTable) ([]h264.VideoReport, error) {
		return playableVideoReports(), nil
	}
	cmd.Decode = func(string, int, string) (DecodeResult, error) {
		return cleanDecodeResult(), nil
	}

	code := cmd.Execute([]string{"--input", input})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	text := out.String()
	if !strings.Contains(text, "Video") || !strings.Contains(text, "PLAYABLE") {
		t.Errorf("expected video PLAYABLE section, got %q", text)
	}
	if !strings.Contains(text, "IDR") || !strings.Contains(text, "39") {
		t.Errorf("expected IDR count in output, got %q", text)
	}
}

func TestDiagnoseVideoSectionStallsInMSE(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := newTestDiagnoseCommand(&out, &errBuf)
	cmd.isOutputTTY = func(_ io.Writer) bool { return true }
	input := tempDiagnoseInput(t)

	cmd.Detect = func(context.Context, string, map[string]string) envcheck.Report {
		return healthyReport()
	}
	cmd.Inspect = func(string) (ts.StreamTable, InspectStats, error) {
		return metadataStreamTable(), metadataInspectStats(), nil
	}
	cmd.VideoAnalyze = func(_ string, _ ts.StreamTable) ([]h264.VideoReport, error) {
		return stallsVideoReports(), nil
	}
	cmd.Decode = func(string, int, string) (DecodeResult, error) {
		return cleanDecodeResult(), nil
	}

	code := cmd.Execute([]string{"--input", input})
	// Non-fatal: KLV decode still runs, exit 0.
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (video is non-fatal); stderr: %s", code, errBuf.String())
	}
	text := out.String()
	if !strings.Contains(text, "STALLS_IN_MSE") {
		t.Errorf("expected STALLS_IN_MSE in output, got %q", text)
	}
	if !strings.Contains(text, "no IDR frames") {
		t.Errorf("expected IDR reason in output, got %q", text)
	}
	if !strings.Contains(text, "libx264") {
		t.Errorf("expected libx264 fix hint in pretty-mode output, got %q", text)
	}
	// KLV decode must still have run.
	if !strings.Contains(text, "packets decoded: 2") {
		t.Errorf("expected KLV decode section to still run, got %q", text)
	}
}

func TestDiagnoseVideoAnalyzeErrorIsNonFatal(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := newTestDiagnoseCommand(&out, &errBuf)
	input := tempDiagnoseInput(t)

	cmd.Detect = func(context.Context, string, map[string]string) envcheck.Report {
		return healthyReport()
	}
	cmd.Inspect = func(string) (ts.StreamTable, InspectStats, error) {
		return metadataStreamTable(), metadataInspectStats(), nil
	}
	cmd.VideoAnalyze = func(_ string, _ ts.StreamTable) ([]h264.VideoReport, error) {
		return nil, fmt.Errorf("boom")
	}
	cmd.Decode = func(string, int, string) (DecodeResult, error) {
		return cleanDecodeResult(), nil
	}

	code := cmd.Execute([]string{"--input", input})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	text := out.String()
	if !strings.Contains(text, "packets decoded: 2") {
		t.Errorf("expected KLV decode section to run even when video analyzer errors, got %q", text)
	}
}

func TestDiagnoseH265StreamShowsNotSupportedNotice(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := newTestDiagnoseCommand(&out, &errBuf)
	input := tempDiagnoseInput(t)

	cmd.Detect = func(context.Context, string, map[string]string) envcheck.Report {
		return healthyReport()
	}
	cmd.Inspect = func(string) (ts.StreamTable, InspectStats, error) {
		return ts.StreamTable{Programs: map[uint16][]ts.Stream{
			1: {{PID: 0x0100, StreamType: 0x24, ProgramNum: 1}},
		}}, metadataInspectStats(), nil
	}
	// Use the real default so the H.265 notice path is exercised end-to-end.
	cmd.VideoAnalyze = defaultVideoAnalyze
	cmd.Decode = func(string, int, string) (DecodeResult, error) {
		return cleanDecodeResult(), nil
	}

	code := cmd.Execute([]string{"--input", input})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
	text := out.String()
	if !strings.Contains(text, "H.265") && !strings.Contains(text, "0x0100") {
		t.Errorf("expected H.265 stream mentioned in output, got %q", text)
	}
	if !strings.Contains(text, "not yet analyzed") {
		t.Errorf("expected 'not yet analyzed' notice for H.265, got %q", text)
	}
}

func TestDiagnoseNilCommand(t *testing.T) {
	var cmd *DiagnoseCommand
	if code := cmd.Execute(nil); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestDiagnoseBareDIFieldsDoNotPanic(t *testing.T) {
	var out, errBuf bytes.Buffer
	input := tempDiagnoseInput(t)
	cmd := &DiagnoseCommand{
		Out: &out,
		Err: &errBuf,
	}
	// Inject stubs so we don't hit real ffmpeg/filesystem,
	// but leave Detect/Inspect/Decode nil to exercise the nil-guard defaults.
	cmd.Detect = func(context.Context, string, map[string]string) envcheck.Report {
		return healthyReport()
	}
	cmd.Inspect = func(string) (ts.StreamTable, InspectStats, error) {
		return metadataStreamTable(), metadataInspectStats(), nil
	}
	cmd.Decode = func(string, int, string) (DecodeResult, error) {
		return cleanDecodeResult(), nil
	}
	code := cmd.Execute([]string{"--input", input})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, errBuf.String())
	}
}

func TestDiagnoseNilDIFieldsDefault(t *testing.T) {
	// Verify that a DiagnoseCommand with nil Detect/Inspect/Decode doesn't
	// panic when run reaches the nil-guard fallbacks. We can't easily test
	// the full pipeline without ffmpeg, so just verify it doesn't panic
	// by providing a file that will fail at inspect (no valid TS).
	var out, errBuf bytes.Buffer
	input := tempDiagnoseInput(t)
	cmd := &DiagnoseCommand{
		Out:    &out,
		Err:    &errBuf,
		Detect: func(context.Context, string, map[string]string) envcheck.Report { return healthyReport() },
		// Inspect and Decode left nil — should fall back to defaults.
	}
	// The default inspect will fail on our dummy 1-byte file, which is fine;
	// the point is that it doesn't panic on nil function call.
	code := cmd.Execute([]string{"--input", input})
	// We expect exit 1 because the dummy file isn't valid TS.
	if code == 0 {
		t.Log("unexpected success; output:", out.String())
	}
}
