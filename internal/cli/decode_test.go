package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jacorbello/klvtool/internal/klv"
	"github.com/jacorbello/klvtool/internal/klv/record"
	"github.com/jacorbello/klvtool/internal/klv/specs/st0601"
)

// fakeDecodePayloads returns a single synthetic decoded record for tests that
// need to exercise the writers without going through ffmpeg.
func fakeDecodePayloads(_ string, _ int) ([]record.Record, error) {
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
