package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jacorbello/klvtool/internal/klv"
	"github.com/jacorbello/klvtool/internal/klv/record"
	"github.com/jacorbello/klvtool/internal/klv/specs/st0601"
	"github.com/jacorbello/klvtool/internal/packetize"
)

// fakeDecodePayloads returns a single synthetic decoded record for tests that
// need to exercise the writers without going through ffmpeg.
func fakeDecodePayloads(_ string, _ int, _ string) ([]record.Record, error) {
	rec := record.Record{
		Schema:      "urn:misb:KLV:bin:0601.19",
		LSVersion:   19,
		TotalLength: 12,
		Checksum:    record.ChecksumInfo{Expected: 0x1111, Computed: 0x1111, Valid: true},
		Items: []record.Item{
			{Tag: 2, Name: "Precision Time Stamp", Value: record.StringValue("2023-03-02T12:34:56.789Z")},
			{Tag: 5, Name: "Platform Heading Angle", Value: record.FloatValue(159.97), Units: "°"},
			{Tag: 65, Name: "UAS Datalink LS Version Number", Value: record.UintValue(19)},
			{Tag: 1, Name: "Checksum", Value: record.UintValue(0x1111)},
		},
	}
	return []record.Record{rec}, nil
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
	code := cmd.Execute([]string{"--input", "fake.ts"})
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
	code := cmd.Execute([]string{"--input", "fake.ts", "--format", "text"})
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

func TestDecodeCommandMissingInput(t *testing.T) {
	cmd := &DecodeCommand{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	code := cmd.Execute([]string{})
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (usage)", code)
	}
}

func TestDecodeCommandRejectsUnknownSchema(t *testing.T) {
	cmd := &DecodeCommand{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	code := cmd.Execute([]string{"--input", "fake.ts", "--schema", "urn:misb:KLV:bin:0601.14"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (usage)", code)
	}
}

func TestDecodeCommandRejectsStrayPositionalArgs(t *testing.T) {
	cmd := &DecodeCommand{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
	code := cmd.Execute([]string{"--input", "fake.ts", "junk"})
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (usage)", code)
	}
}

// fakeDecodeWithRaw returns a record with Raw bytes populated so --raw
// behavior can be verified.
func fakeDecodeWithRaw(_ string, _ int, _ string) ([]record.Record, error) {
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
	return []record.Record{rec}, nil
}

func TestDecodeCommandRawTextIncludesRawBytes(t *testing.T) {
	out := &bytes.Buffer{}
	cmd := &DecodeCommand{
		Out:    out,
		Err:    &bytes.Buffer{},
		Decode: fakeDecodeWithRaw,
	}
	code := cmd.Execute([]string{"--input", "fake.ts", "--format", "text", "--raw"})
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
		Out: &bytes.Buffer{},
		Err: &bytes.Buffer{},
		Decode: func(_ string, _ int, schema string) ([]record.Record, error) {
			gotSchema = schema
			return nil, nil
		},
	}
	code := cmd.Execute([]string{"--input", "fake.ts", "--schema", "urn:misb:KLV:bin:0601.19"})
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if gotSchema != "urn:misb:KLV:bin:0601.19" {
		t.Errorf("schema = %q, want urn:misb:KLV:bin:0601.19", gotSchema)
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
	code := cmd.Execute([]string{"--input", "fake.ts"})
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
	code := cmd.Execute([]string{"--input", "fake.ts", "--format", "text"})
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if strings.Contains(out.String(), "raw=") {
		t.Errorf("unexpected raw= in text output without --raw flag")
	}
}
