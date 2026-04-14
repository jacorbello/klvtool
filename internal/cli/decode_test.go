package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacorbello/klvtool/internal/klv"
	"github.com/jacorbello/klvtool/internal/klv/record"
	"github.com/jacorbello/klvtool/internal/klv/specs/st0601"
	"github.com/jacorbello/klvtool/internal/packetize"
)

// tempInputFile creates an empty file in t.TempDir and returns its path.
// Tests that pass through os.Stat validation need a real file on disk.
func tempInputFile(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "input.ts")
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDecodeRejectsNonExistentInput(t *testing.T) {
	var stderr bytes.Buffer
	cmd := &DecodeCommand{
		Out: nil,
		Err: &stderr,
		Decode: func(path string, pid int, schema string) (DecodeResult, error) {
			t.Fatal("decode should not be called for non-existent input")
			return DecodeResult{}, nil
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

func TestDecodeRejectsDirectory(t *testing.T) {
	var stderr bytes.Buffer
	cmd := &DecodeCommand{
		Out: nil,
		Err: &stderr,
		Decode: func(path string, pid int, schema string) (DecodeResult, error) {
			t.Fatal("decode should not be called for directory input")
			return DecodeResult{}, nil
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
	if !strings.Contains(text, "regular file") {
		t.Fatalf("expected 'regular file' message, got %q", text)
	}
}

func TestDecodeHelpMixedWithFlags(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := &DecodeCommand{Out: &out, Err: &errBuf}
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

// TestWriteNDJSONEmptyCollectionsSerializeAsArrays pins the Layer 1 convention:
// empty items and diagnostics must marshal as [] not null so consumers can
// rely on array iteration without null-checking.
func TestWriteNDJSONEmptyCollectionsSerializeAsArrays(t *testing.T) {
	var buf bytes.Buffer
	rec := record.Record{
		Schema:      "urn:misb:KLV:bin:0601.19",
		LSVersion:   19,
		ValueLength: 0,
		Items:       nil,
		Diagnostics: nil,
	}
	if err := writeNDJSON(&buf, 0, rec, false); err != nil {
		t.Fatalf("writeNDJSON: %v", err)
	}
	line := buf.String()
	if !strings.Contains(line, `"items":[]`) {
		t.Errorf("expected empty items as []: %s", line)
	}
	if !strings.Contains(line, `"diagnostics":[]`) {
		t.Errorf("expected empty diagnostics as []: %s", line)
	}
}

func TestWriteNDJSONIncludesStructuredDiagnosticContext(t *testing.T) {
	var buf bytes.Buffer
	rec := record.Record{
		Schema:      "urn:misb:KLV:bin:0601.19",
		LSVersion:   19,
		ValueLength: 0,
		Diagnostics: []record.Diagnostic{{
			Severity: "error",
			Code:     "tag_decode_error",
			Message:  "value out of range (error indicator)",
			Tag:      intPtr(47),
			TagName:  "Error Indicator",
			Actual:   "255",
			Expected: "0..3",
			Raw:      "0xff",
		}},
	}
	if err := writeNDJSON(&buf, 0, rec, false); err != nil {
		t.Fatalf("writeNDJSON: %v", err)
	}
	line := buf.String()
	for _, want := range []string{
		`"tagName":"Error Indicator"`,
		`"actual":"255"`,
		`"expected":"0..3"`,
		`"raw":"0xff"`,
	} {
		if !strings.Contains(line, want) {
			t.Errorf("expected %s in %s", want, line)
		}
	}
}

// testRegistry is the registry used by tests that need --schema validation.
func testRegistry() *klv.Registry {
	r := klv.NewRegistry()
	r.Register(st0601.V19())
	return r
}

// fakeDecodePayloads returns a single synthetic decoded record for tests that
// need to exercise the writers without going through ffmpeg.
func fakeDecodePayloads(_ string, _ int, _ string) (DecodeResult, error) {
	rec := record.Record{
		Schema:      "urn:misb:KLV:bin:0601.19",
		LSVersion:   19,
		ValueLength: 12,
		Checksum:    record.ChecksumInfo{Expected: 0x1111, Computed: 0x1111, Valid: true},
		Items: []record.Item{
			{Tag: 2, Name: "Precision Time Stamp", Value: record.StringValue("2023-03-02T12:34:56.789Z")},
			{Tag: 5, Name: "Platform Heading Angle", Value: record.FloatValue(159.97), Units: "°"},
			{Tag: 65, Name: "UAS Datalink LS Version Number", Value: record.UintValue(19)},
			{Tag: 1, Name: "Checksum", Value: record.UintValue(0x1111)},
		},
	}
	return DecodeResult{Records: []record.Record{rec}}, nil
}

func TestDecodeCommandNDJSONOutput(t *testing.T) {
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd := &DecodeCommand{
		Out:    out,
		Err:    errBuf,
		Decode: fakeDecodePayloads,
		Registry: func() *klv.Registry {
			r := klv.NewRegistry()
			r.Register(st0601.V19())
			return r
		},
	}
	code := cmd.Execute([]string{"--input", tempInputFile(t)})
	if code != 0 {
		t.Fatalf("exit code = %d, stderr=%s", code, errBuf.String())
	}
	line := strings.TrimSpace(out.String())
	if !strings.HasPrefix(line, "{") {
		t.Errorf("expected NDJSON, got: %s", line)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatalf("invalid NDJSON: %v\n%s", err, line)
	}
	if parsed["schema"] != "urn:misb:KLV:bin:0601.19" {
		t.Errorf("schema mismatch: %v", parsed["schema"])
	}
}

func TestTextOutputOmitsUnitsWithoutRawFlag(t *testing.T) {
	var buf bytes.Buffer
	rec := record.Record{
		Schema:    "urn:misb:KLV:bin:0601.19",
		LSVersion: 19,
		Checksum:  record.ChecksumInfo{Valid: true},
		Items: []record.Item{
			{Tag: 5, Name: "Platform Heading Angle", Value: record.FloatValue(159.97), Units: "°"},
		},
	}
	if err := writeText(&buf, 0, rec, false); err != nil {
		t.Fatalf("writeText: %v", err)
	}
	if strings.Contains(buf.String(), "°") {
		t.Errorf("text output without --raw should not include units; got: %s", buf.String())
	}

	buf.Reset()
	if err := writeText(&buf, 0, rec, true); err != nil {
		t.Fatalf("writeText: %v", err)
	}
	if !strings.Contains(buf.String(), "°") {
		t.Errorf("text output with --raw should include units; got: %s", buf.String())
	}
}

func TestWriteTextIncludesStructuredDiagnosticContext(t *testing.T) {
	var buf bytes.Buffer
	rec := record.Record{
		Schema:   "urn:misb:KLV:bin:0601.19",
		Checksum: record.ChecksumInfo{Valid: true},
		Diagnostics: []record.Diagnostic{{
			Severity: "error",
			Code:     "tag_decode_error",
			Message:  "value out of range (error indicator)",
			Tag:      intPtr(47),
			TagName:  "Error Indicator",
			Actual:   "255",
			Expected: "0..3",
			Raw:      "0xff",
		}},
	}
	if err := writeText(&buf, 0, rec, false); err != nil {
		t.Fatalf("writeText: %v", err)
	}
	got := buf.String()
	for _, want := range []string{
		"tag=47 Error Indicator",
		"actual=255",
		"expected=0..3",
		"raw=0xff",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output: %s", want, got)
		}
	}
}

func TestWriteTextIncludesGenericStructuredDiagnosticContext(t *testing.T) {
	var buf bytes.Buffer
	rec := record.Record{
		Schema:   "urn:misb:KLV:bin:0601.19",
		Checksum: record.ChecksumInfo{Valid: true},
		Diagnostics: []record.Diagnostic{{
			Severity: "warning",
			Code:     "tag_range_violation",
			Message:  "tag 5: value 250 outside [0, 100]",
			Tag:      intPtr(5),
			TagName:  "Bounded Float",
			Actual:   "250",
			Expected: "[0, 100]",
			Raw:      "0x0000",
		}},
	}
	if err := writeText(&buf, 0, rec, false); err != nil {
		t.Fatalf("writeText: %v", err)
	}
	got := buf.String()
	for _, want := range []string{
		"tag=5 Bounded Float",
		"actual=250",
		"expected=[0, 100]",
		"raw=0x0000",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output: %s", want, got)
		}
	}
}

func TestDecodeCommandTextOutput(t *testing.T) {
	out := &bytes.Buffer{}
	cmd := &DecodeCommand{
		Out:    out,
		Err:    &bytes.Buffer{},
		Decode: fakeDecodePayloads,
		Registry: func() *klv.Registry {
			r := klv.NewRegistry()
			r.Register(st0601.V19())
			return r
		},
	}
	code := cmd.Execute([]string{"--input", tempInputFile(t), "--format", "text"})
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got := out.String()
	if !strings.Contains(got, "Packet 0") {
		t.Errorf("missing Packet header: %s", got)
	}
	if !strings.Contains(got, "Platform Heading Angle") {
		t.Errorf("missing item: %s", got)
	}
}

// TestWriteTextChecksumLabels verifies the text-mode Packet header reflects
// the actual checksum state: OK when the engine computed a valid checksum,
// MISMATCH only when tag 1 was present and comparison failed, N/A when tag
// 1 was absent, and MALFORMED when tag 1 was present but the wrong length.
func TestWriteTextChecksumLabels(t *testing.T) {
	mkItem := func(tag int, raw []byte) record.Item {
		return record.Item{Tag: tag, Name: "x", Raw: raw}
	}
	cases := []struct {
		name  string
		rec   record.Record
		label string
	}{
		{
			name: "valid",
			rec: record.Record{
				Schema:   "urn:misb:KLV:bin:0601.19",
				Checksum: record.ChecksumInfo{Valid: true},
				Items:    []record.Item{mkItem(1, []byte{0x12, 0x34})},
			},
			label: "OK",
		},
		{
			name: "mismatch",
			rec: record.Record{
				Schema:   "urn:misb:KLV:bin:0601.19",
				Checksum: record.ChecksumInfo{Valid: false, Expected: 0x1111, Computed: 0x2222},
				Items:    []record.Item{mkItem(1, []byte{0x11, 0x11})},
			},
			label: "MISMATCH",
		},
		{
			name: "missing_tag1",
			rec: record.Record{
				Schema:   "urn:misb:KLV:bin:0601.19",
				Checksum: record.ChecksumInfo{Valid: false}, // never computed
				Items:    []record.Item{mkItem(2, make([]byte, 8))},
			},
			label: "N/A",
		},
		{
			name: "malformed_tag1",
			rec: record.Record{
				Schema:   "urn:misb:KLV:bin:0601.19",
				Checksum: record.ChecksumInfo{Valid: false}, // never computed
				Items:    []record.Item{mkItem(1, []byte{0x00})},
			},
			label: "MALFORMED",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := writeText(&buf, 0, tc.rec, false); err != nil {
				t.Fatalf("writeText error: %v", err)
			}
			want := "checksum=" + tc.label
			if !strings.Contains(buf.String(), want) {
				t.Errorf("expected %q in header, got: %s", want, buf.String())
			}
		})
	}
}

func TestDecodeCommandMissingInput(t *testing.T) {
	cmd := &DecodeCommand{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	code := cmd.Execute([]string{})
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (usage)", code)
	}
}

func TestDecodeCommandRejectsUnknownSchema(t *testing.T) {
	cmd := &DecodeCommand{
		Out:      &bytes.Buffer{},
		Err:      &bytes.Buffer{},
		Registry: testRegistry,
	}
	code := cmd.Execute([]string{"--input", tempInputFile(t), "--schema", "urn:misb:KLV:bin:0601.14"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (usage)", code)
	}
}

// TestDecodeCommandAcceptsRegisteredSchema verifies --schema passes the
// Execute-layer validation when the URN is present in the command's
// Registry. Uses a fake Decode so ffmpeg isn't invoked.
func TestDecodeCommandAcceptsRegisteredSchema(t *testing.T) {
	var gotSchema string
	cmd := &DecodeCommand{
		Out:      &bytes.Buffer{},
		Err:      &bytes.Buffer{},
		Registry: testRegistry,
		Decode: func(_ string, _ int, schema string) (DecodeResult, error) {
			gotSchema = schema
			return DecodeResult{}, nil
		},
	}
	code := cmd.Execute([]string{"--input", tempInputFile(t), "--schema", "urn:misb:KLV:bin:0601.19"})
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if gotSchema != "urn:misb:KLV:bin:0601.19" {
		t.Errorf("schema = %q", gotSchema)
	}
}

func TestDecodeCommandRejectsStrayPositionalArgs(t *testing.T) {
	cmd := &DecodeCommand{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	code := cmd.Execute([]string{"--input", tempInputFile(t), "junk"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (usage)", code)
	}
}

// fakeDecodeWithRaw returns a record with Raw bytes populated so --raw
// behavior can be verified.
func fakeDecodeWithRaw(_ string, _ int, _ string) (DecodeResult, error) {
	rec := record.Record{
		Schema:    "urn:misb:KLV:bin:0601.19",
		LSVersion: 19,
		Checksum:  record.ChecksumInfo{Valid: true},
		Items: []record.Item{
			{
				Tag: 5, Name: "Platform Heading Angle",
				Value: record.FloatValue(159.97), Units: "°",
				Raw: []byte{0x71, 0xC2},
			},
		},
	}
	return DecodeResult{Records: []record.Record{rec}}, nil
}

func TestDecodeCommandRawTextIncludesRawBytes(t *testing.T) {
	out := &bytes.Buffer{}
	cmd := &DecodeCommand{
		Out:    out,
		Err:    &bytes.Buffer{},
		Decode: fakeDecodeWithRaw,
	}
	code := cmd.Execute([]string{"--input", tempInputFile(t), "--format", "text", "--raw"})
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	got := out.String()
	if !strings.Contains(got, "raw=0x71c2") {
		t.Errorf("expected raw=0x71c2 in text output; got:\n%s", got)
	}
}

// TestDecodeCommandSchemaPassedToDecode verifies that the --schema flag
// reaches the Decode closure so the closure can honor it as an override.
func TestDecodeCommandSchemaPassedToDecode(t *testing.T) {
	var gotSchema string
	cmd := &DecodeCommand{
		Out:      &bytes.Buffer{},
		Err:      &bytes.Buffer{},
		Registry: testRegistry,
		Decode: func(_ string, _ int, schema string) (DecodeResult, error) {
			gotSchema = schema
			return DecodeResult{}, nil
		},
	}
	code := cmd.Execute([]string{"--input", tempInputFile(t), "--schema", "urn:misb:KLV:bin:0601.19"})
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if gotSchema != "urn:misb:KLV:bin:0601.19" {
		t.Errorf("schema = %q, want urn:misb:KLV:bin:0601.19", gotSchema)
	}
}

// TestDecodeCommandStreamDiagnosticsReported verifies that stream-level
// diagnostics (no packets decoded, but packetize flagged something) are
// reported to stderr and do NOT produce a phantom Packet 0 in stdout.
func TestDecodeCommandStreamDiagnosticsReported(t *testing.T) {
	out := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd := &DecodeCommand{
		Out:      out,
		Err:      errBuf,
		Registry: testRegistry,
		Decode: func(_ string, _ int, _ string) (DecodeResult, error) {
			return DecodeResult{
				Records: nil,
				StreamDiagnostics: []record.Diagnostic{
					{Severity: "error", Code: "packetize_invalid_ber_length", Message: "length overflow"},
				},
			}, nil
		},
	}
	code := cmd.Execute([]string{"--input", tempInputFile(t)})
	if code != 0 {
		t.Fatalf("exit code = %d; stderr=%s", code, errBuf.String())
	}
	// stdout must be empty — no phantom packet.
	if strings.TrimSpace(out.String()) != "" {
		t.Errorf("expected empty stdout for zero-packet result; got: %q", out.String())
	}
	// stderr must mention the diagnostic.
	if !strings.Contains(errBuf.String(), "packetize_invalid_ber_length") {
		t.Errorf("expected stream diagnostic on stderr; got: %q", errBuf.String())
	}
	// Summary must say 0 packet(s).
	if !strings.Contains(errBuf.String(), "decoded 0 packet(s)") {
		t.Errorf("expected 'decoded 0 packet(s)' summary; got: %q", errBuf.String())
	}
}

// TestDecodeCommandStreamErrorStrictFails verifies that --strict causes
// exit 1 when a stream-level error diagnostic is present, even with zero
// decoded packets.
func TestDecodeCommandStreamErrorStrictFails(t *testing.T) {
	cmd := &DecodeCommand{
		Out:      &bytes.Buffer{},
		Err:      &bytes.Buffer{},
		Registry: testRegistry,
		Decode: func(_ string, _ int, _ string) (DecodeResult, error) {
			return DecodeResult{
				StreamDiagnostics: []record.Diagnostic{
					{Severity: "error", Code: "packetize_invalid_ber_length", Message: "length overflow"},
				},
			}, nil
		},
	}
	code := cmd.Execute([]string{"--input", tempInputFile(t), "--strict"})
	if code != 1 {
		t.Errorf("exit code = %d, want 1 (strict)", code)
	}
}

// errWriter fails every Write call. Used to verify writeText propagates
// output errors instead of silently ignoring them.
type errWriter struct{}

func (errWriter) Write(_ []byte) (int, error) { return 0, errors.New("disk full") }

func TestDecodeCommandTextWriteErrorSurfaced(t *testing.T) {
	errBuf := &bytes.Buffer{}
	cmd := &DecodeCommand{
		Out:    errWriter{},
		Err:    errBuf,
		Decode: fakeDecodePayloads,
	}
	code := cmd.Execute([]string{"--input", tempInputFile(t), "--format", "text"})
	if code == 0 {
		t.Fatalf("expected non-zero exit on write error; got 0")
	}
	if !strings.Contains(errBuf.String(), "output_write_failure") && !strings.Contains(errBuf.String(), "disk full") {
		t.Errorf("expected write error surfaced on stderr; got: %s", errBuf.String())
	}
}

// TestDecodeCommandErrorDiagnosticSummary verifies the summary wording
// mentions "error diagnostic(s)" rather than "validation error(s)" — the
// counter includes decode and packetize errors, not just validation.
func TestDecodeCommandErrorDiagnosticSummary(t *testing.T) {
	errBuf := &bytes.Buffer{}
	cmd := &DecodeCommand{
		Out:    &bytes.Buffer{},
		Err:    errBuf,
		Decode: fakeDecodePayloads,
	}
	code := cmd.Execute([]string{"--input", tempInputFile(t)})
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(errBuf.String(), "error diagnostic(s)") {
		t.Errorf("expected 'error diagnostic(s)' in summary; got: %s", errBuf.String())
	}
}

func TestLiftPacketizeDiagnostics(t *testing.T) {
	in := []packetize.Diagnostic{
		{Severity: "warning", Code: "recovery_skip", Message: "skipped 4 bytes"},
		{Severity: "error", Code: "invalid_ber_length", Message: "length overflow"},
	}
	got := liftPacketizeDiagnostics(in)
	if len(got) != 2 {
		t.Fatalf("got %d diagnostics, want 2", len(got))
	}
	if got[0].Code != "packetize_recovery_skip" || got[0].Severity != "warning" {
		t.Errorf("diag[0] = %+v", got[0])
	}
	if got[1].Code != "packetize_invalid_ber_length" || got[1].Severity != "error" {
		t.Errorf("diag[1] = %+v", got[1])
	}
}

func TestDecodeCommandRawTextAbsentWithoutFlag(t *testing.T) {
	out := &bytes.Buffer{}
	cmd := &DecodeCommand{
		Out:    out,
		Err:    &bytes.Buffer{},
		Decode: fakeDecodeWithRaw,
	}
	code := cmd.Execute([]string{"--input", tempInputFile(t), "--format", "text"})
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if strings.Contains(out.String(), "raw=") {
		t.Errorf("unexpected raw= in text output without --raw flag")
	}
}

// errCloser wraps a bytes.Buffer but returns an error on Close. Used to
// verify that file close errors are captured and reported.
type errCloser struct {
	bytes.Buffer
	closeErr error
}

func intPtr(v int) *int { return &v }

func (e *errCloser) Close() error { return e.closeErr }

// failWriter fails on Write and tracks whether Close was called.
type failWriter struct {
	writeErr error
	closed   bool
}

func (f *failWriter) Write([]byte) (int, error) { return 0, f.writeErr }
func (f *failWriter) Close() error              { f.closed = true; return nil }

// TestDecodeCommandFileCloseErrorSurfaced verifies that when --out is used
// and the file's Close returns an error, the command reports it on stderr
// and exits non-zero.
func TestDecodeCommandFileCloseErrorSurfaced(t *testing.T) {
	dir := t.TempDir()
	outPath := dir + "/out.ndjson"

	errBuf := &bytes.Buffer{}
	cmd := &DecodeCommand{
		Out:    &bytes.Buffer{},
		Err:    errBuf,
		Decode: fakeDecodePayloads,
		Registry: func() *klv.Registry {
			return testRegistry()
		},
		openOut: func(_ string) (io.WriteCloser, error) {
			return &errCloser{closeErr: errors.New("disk full on close")}, nil
		},
	}
	code := cmd.Execute([]string{"--input", tempInputFile(t), "--out", outPath})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%s", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "disk full on close") {
		t.Errorf("expected close error on stderr; got: %s", errBuf.String())
	}
}

// TestDecodeCommandFileCloseSuccessUnaffected verifies that normal --out
// operation (successful close) still works correctly.
func TestDecodeCommandFileCloseSuccessUnaffected(t *testing.T) {
	dir := t.TempDir()
	outPath := dir + "/out.ndjson"

	errBuf := &bytes.Buffer{}
	cmd := &DecodeCommand{
		Out:    &bytes.Buffer{},
		Err:    errBuf,
		Decode: fakeDecodePayloads,
		Registry: func() *klv.Registry {
			return testRegistry()
		},
	}
	code := cmd.Execute([]string{"--input", tempInputFile(t), "--out", outPath})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, errBuf.String())
	}
}

// TestDecodeCommandWriteErrorClosesFile verifies that when --out is used and
// a write error occurs, the output file is still closed.
func TestDecodeCommandWriteErrorClosesFile(t *testing.T) {
	fw := &failWriter{writeErr: errors.New("broken pipe")}

	errBuf := &bytes.Buffer{}
	cmd := &DecodeCommand{
		Out:    &bytes.Buffer{},
		Err:    errBuf,
		Decode: fakeDecodePayloads,
		Registry: func() *klv.Registry {
			return testRegistry()
		},
		openOut: func(_ string) (io.WriteCloser, error) {
			return fw, nil
		},
	}
	code := cmd.Execute([]string{"--input", tempInputFile(t), "--out", "/tmp/out.ndjson"})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !fw.closed {
		t.Fatal("expected output file to be closed after write error")
	}
}

func TestDecodePIDValidation(t *testing.T) {
	tests := []struct {
		name     string
		pid      string
		wantCode int
		wantErr  bool
	}{
		{"pid 0 is valid (all PIDs)", "0", 0, false},
		{"pid 1 is valid", "1", 0, false},
		{"pid 8191 is valid", "8191", 0, false},
		{"pid 8192 is rejected", "8192", usageExitCode, true},
		{"pid -1 is rejected", "-1", usageExitCode, true},
		{"pid 99999 is rejected", "99999", usageExitCode, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			errBuf := &bytes.Buffer{}
			cmd := &DecodeCommand{
				Out:    out,
				Err:    errBuf,
				Decode: fakeDecodePayloads,
				Registry: func() *klv.Registry {
					return testRegistry()
				},
			}
			code := cmd.Execute([]string{"--input", tempInputFile(t), "--pid", tt.pid})
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d; stderr=%s", code, tt.wantCode, errBuf.String())
			}
			if tt.wantErr {
				stderr := errBuf.String()
				if !strings.Contains(stderr, "--pid") {
					t.Errorf("expected stderr to mention --pid, got: %s", stderr)
				}
				if out.Len() > 0 {
					t.Errorf("expected no stdout output, got: %s", out.String())
				}
			}
		})
	}
}

func TestNDJSONAlwaysIncludesUnits(t *testing.T) {
	rec := record.Record{
		Schema:    "urn:misb:KLV:bin:0601.19",
		LSVersion: 19,
		Checksum:  record.ChecksumInfo{Valid: true},
		Items: []record.Item{
			{Tag: 5, Name: "Platform Heading Angle", Value: record.FloatValue(159.97), Units: "°"},
		},
	}
	var buf bytes.Buffer
	if err := writeNDJSON(&buf, 0, rec, false); err != nil {
		t.Fatalf("writeNDJSON: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	items := parsed["items"].([]any)
	item := items[0].(map[string]any)
	units, ok := item["units"]
	if !ok || units != "°" {
		t.Errorf("expected units=° in NDJSON without --raw; got %v (present=%v)", units, ok)
	}
}

func TestDecodeOutOpenFailure(t *testing.T) {
	errBuf := &bytes.Buffer{}
	cmd := &DecodeCommand{
		Out:    &bytes.Buffer{},
		Err:    errBuf,
		Decode: fakeDecodePayloads,
		Registry: func() *klv.Registry {
			return testRegistry()
		},
		openOut: func(_ string) (io.WriteCloser, error) {
			return nil, errors.New("permission denied")
		},
	}
	code := cmd.Execute([]string{"--input", tempInputFile(t), "--out", "/tmp/out.ndjson"})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "permission denied") {
		t.Errorf("expected error on stderr; got: %s", errBuf.String())
	}
}

func TestDecodeWarnsPIDNoMatch(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cmd := &DecodeCommand{
		Out: &stdout,
		Err: &stderr,
		Decode: func(path string, pid int, schema string) (DecodeResult, error) {
			return DecodeResult{Records: nil}, nil
		},
	}
	code := cmd.Execute([]string{"--input", tempInputFile(t), "--pid", "999"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stderr.String(), "no KLV packets found on PID 999") {
		t.Errorf("expected PID no-match warning; got stderr=%q", stderr.String())
	}
}

func TestDecodeHelpDocumentsRawEncoding(t *testing.T) {
	var buf bytes.Buffer
	cmd := &DecodeCommand{Out: &buf, Err: &bytes.Buffer{}}
	code := cmd.Execute([]string{"--help"})
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(buf.String(), "hex") || !strings.Contains(buf.String(), "base64") {
		t.Errorf("--help should document raw encoding formats; got: %s", buf.String())
	}
}
