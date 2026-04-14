package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	ffmpegbackend "github.com/jacorbello/klvtool/internal/backends/ffmpeg"
	"github.com/jacorbello/klvtool/internal/extract"
	"github.com/jacorbello/klvtool/internal/klv"
	"github.com/jacorbello/klvtool/internal/klv/record"
	"github.com/jacorbello/klvtool/internal/klv/specs/st0601"
	"github.com/jacorbello/klvtool/internal/model"
	"github.com/jacorbello/klvtool/internal/packetize"
)

// mpegTSPIDMax is the highest valid MPEG-TS packet identifier (13-bit field).
const mpegTSPIDMax = 0x1FFF


// DecodeCommand decodes MISB ST 0601 KLV from an MPEG-TS file into
// typed records.
type DecodeCommand struct {
	Out io.Writer
	Err io.Writer
	// Decode runs the decode pipeline. When schema is non-empty, the
	// implementation must restrict decoding to the SpecVersion registered
	// under that URN (bypassing UL-based auto-detection).
	Decode   func(path string, pid int, schema string) (DecodeResult, error)
	Registry func() *klv.Registry
	// openOut creates the output file for --out. Defaults to os.Create.
	// Exposed for testing close-error propagation.
	openOut func(path string) (io.WriteCloser, error)
}

// DecodeResult holds decoded records plus stream-level diagnostics that
// aren't attached to any specific decoded packet (e.g. packetize recovery
// events on a raw stream that produced zero KLV packets). Stream-level
// diagnostics are reported to stderr and counted toward --strict without
// polluting the decoded-record output.
type DecodeResult struct {
	Records           []record.Record
	StreamDiagnostics []record.Diagnostic
}

// NewDecodeCommand returns a DecodeCommand with default runtime dependencies.
func NewDecodeCommand() *DecodeCommand {
	c := &DecodeCommand{
		Out: os.Stdout,
		Err: os.Stderr,
		Registry: func() *klv.Registry {
			r := klv.NewRegistry()
			r.Register(st0601.V19())
			return r
		},
	}
	c.Decode = func(path string, pid int, schema string) (DecodeResult, error) {
		report := defaultDoctorDetect(context.Background(), "", currentEnvMap())
		desc := ffmpegDescriptor(report)
		if !desc.Healthy {
			return DecodeResult{}, model.MissingDependency(fmt.Errorf("ffmpeg backend is unavailable"))
		}

		extractor := extract.NewExtractor(ffmpegbackend.NewBackend())
		result, err := extractor.Run(context.Background(), extract.RunRequest{
			InputPath: path,
			Backend:   desc,
		})
		if err != nil {
			return DecodeResult{}, err
		}

		reg := c.Registry()
		// When --schema is set, restrict decoding to just the requested
		// SpecVersion by building a single-entry registry. This makes the
		// flag a genuine override rather than a no-op gate.
		if schema != "" {
			sv, ok := reg.Lookup(schema)
			if !ok {
				return DecodeResult{}, model.InvalidUsage(fmt.Errorf("schema %q not registered", schema))
			}
			reg = klv.NewRegistry()
			reg.Register(sv)
		}
		parser := packetize.NewParser()
		var res DecodeResult
		for _, raw := range result.Records {
			if pid != 0 && int(raw.PID) != pid {
				continue
			}
			stream, err := parser.Parse(packetize.Request{
				Mode:   packetize.ModeBestEffort,
				Record: raw,
			})
			if err != nil {
				return DecodeResult{}, err
			}
			// Lift packetize-layer diagnostics (recovery events, malformed
			// packet scans) into record.Diagnostic so --strict and the final
			// summary see them. Without this, best-effort recovery is silent.
			sourceDiags := liftPacketizeDiagnostics(stream.Diagnostics)
			if len(stream.Packets) == 0 {
				// No KLV packets decoded from this raw stream. Any
				// packetize diagnostics become stream-level diagnostics
				// so they aren't dropped. Do NOT emit a synthetic record
				// — the output should not claim a packet was decoded.
				res.StreamDiagnostics = append(res.StreamDiagnostics, sourceDiags...)
				continue
			}
			for i, pkt := range stream.Packets {
				// Preserve the exact wire BER length bytes — the checksum
				// covers them and may use a non-canonical encoding.
				var lengthBytes []byte
				if pkt.LengthStart >= 0 && pkt.ValueStart >= pkt.LengthStart && pkt.ValueStart <= len(raw.Payload) {
					lengthBytes = raw.Payload[pkt.LengthStart:pkt.ValueStart]
				}
				rec, err := klv.DecodeLocalSet(reg, pkt.Key, lengthBytes, pkt.Value)
				if err != nil {
					return DecodeResult{}, err
				}
				// Attach packetize diagnostics to the first decoded record
				// from this raw stream so they flow through the normal
				// per-packet reporting path.
				if i == 0 && len(sourceDiags) > 0 {
					rec.Diagnostics = append(sourceDiags, rec.Diagnostics...)
				}
				res.Records = append(res.Records, rec)
			}
		}
		return res, nil
	}
	return c
}

func (c *DecodeCommand) Execute(args []string) int {
	if c == nil {
		return 1
	}
	if len(args) == 1 && isHelpArg(args[0]) {
		c.writeUsage(c.Out)
		return 0
	}

	fs := flag.NewFlagSet("decode", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		inputPath string
		format    string
		raw       bool
		strict    bool
		pid       int
		outPath   string
		schema    string
	)
	fs.StringVar(&inputPath, "input", "", "path to the MPEG-TS input file")
	fs.StringVar(&format, "format", "ndjson", "output format: ndjson or text")
	fs.BoolVar(&raw, "raw", false, "include raw bytes and units per item")
	fs.BoolVar(&strict, "strict", false, "exit 1 if any error-severity diagnostic is emitted")
	fs.IntVar(&pid, "pid", 0, "limit to a specific KLV data stream PID (0 = all)")
	fs.StringVar(&outPath, "out", "", "write output to a file instead of stdout")
	fs.StringVar(&schema, "schema", "", "override auto-detection with a specific spec URN")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			c.writeUsage(c.Out)
			return 0
		}
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(err))
		return usageExitCode
	}
	if len(fs.Args()) > 0 {
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(fmt.Errorf("unsupported arguments: %v", fs.Args())))
		return usageExitCode
	}
	if strings.TrimSpace(inputPath) == "" {
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(fmt.Errorf("input path is required")))
		return usageExitCode
	}
	if format != "ndjson" && format != "text" {
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(fmt.Errorf("invalid format %q (want ndjson|text)", format)))
		return usageExitCode
	}
	if pid < 0 || pid > mpegTSPIDMax {
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(errors.New("--pid must be 0 (all) or 1-8191")))
		return usageExitCode
	}
	if strings.TrimSpace(schema) != "" {
		// Fail fast at the CLI layer — no point spinning up ffmpeg only
		// to discover the schema URN is unknown. Consult whichever
		// registry the command is configured with so this check scales
		// to future spec versions without touching decode.go.
		regFn := c.Registry
		if regFn == nil {
			regFn = NewDecodeCommand().Registry
		}
		if _, ok := regFn().Lookup(schema); !ok {
			c.writeUsage(c.Err)
			c.writeError(c.Err, model.InvalidUsage(fmt.Errorf("unknown schema %q", schema)))
			return usageExitCode
		}
	}

	decode := c.Decode
	if decode == nil {
		decode = NewDecodeCommand().Decode
	}

	result, err := decode(inputPath, pid, schema)
	if err != nil {
		c.writeError(c.Err, err)
		return exitCodeForError(err)
	}

	sink := c.Out
	var closer io.Closer
	if outPath != "" {
		open := c.openOut
		if open == nil {
			open = func(path string) (io.WriteCloser, error) {
				return os.Create(path)
			}
		}
		f, err := open(outPath)
		if err != nil {
			c.writeError(c.Err, model.OutputWrite(err))
			return exitCodeForError(err)
		}
		sink = f
		closer = f
	}

	exitCode := 0

	var errorCount int
	for i, rec := range result.Records {
		if format == "ndjson" {
			if err := writeNDJSON(sink, i, rec, raw); err != nil {
				c.writeError(c.Err, model.OutputWrite(err))
				exitCode = 1
				break
			}
		} else {
			if err := writeText(sink, i, rec, raw); err != nil {
				c.writeError(c.Err, model.OutputWrite(err))
				exitCode = 1
				break
			}
		}
		for _, d := range rec.Diagnostics {
			if d.Severity == "error" {
				errorCount++
			}
		}
	}

	// Stream-level diagnostics (e.g. packetize recovery events on raw
	// streams that produced zero decoded KLV packets) are reported to
	// stderr and counted toward --strict, but not emitted as packets.
	for _, d := range result.StreamDiagnostics {
		fmt.Fprintf(c.Err, "[stream] %s %s: %s\n", d.Severity, d.Code, d.Message) //nolint:errcheck
		if d.Severity == "error" {
			errorCount++
		}
	}

	if closer != nil {
		if err := closer.Close(); err != nil {
			c.writeError(c.Err, model.OutputWrite(err))
			return 1
		}
	}

	if exitCode != 0 {
		return exitCode
	}

	// errorCount includes structural decode errors (e.g. unknown_spec,
	// tag_decode_error), packetize-layer diagnostics, and validation
	// failures. The label reflects that.
	fmt.Fprintf(c.Err, "decoded %d packet(s), %d error diagnostic(s)\n", len(result.Records), errorCount) //nolint:errcheck
	if strict && errorCount > 0 {
		return 1
	}
	return 0
}

func (c *DecodeCommand) writeUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: klvtool decode --input <file.ts> [--format ndjson|text] [--raw] [--strict] [--pid N] [--out path] [--schema urn]") //nolint:errcheck
}

func (c *DecodeCommand) writeError(w io.Writer, err error) {
	fmt.Fprintln(w, "error:", err) //nolint:errcheck
}

// ndjsonRecord is the serialization shape for one packet.
type ndjsonRecord struct {
	Schema      string              `json:"schema"`
	PacketIndex int                 `json:"packetIndex"`
	LSVersion   int                 `json:"lsVersion"`
	TotalLength int                 `json:"totalLength"`
	Checksum    record.ChecksumInfo `json:"checksum"`
	Items       []ndjsonItem        `json:"items"`
	Diagnostics []record.Diagnostic `json:"diagnostics"`
}

type ndjsonItem struct {
	Tag   int          `json:"tag"`
	Name  string       `json:"name"`
	Value record.Value `json:"value"`
	Units string       `json:"units,omitempty"`
	Raw   string       `json:"raw,omitempty"`
}

func writeNDJSON(w io.Writer, index int, rec record.Record, includeRaw bool) error {
	// Initialize slices explicitly so empty collections serialize as [] not
	// null — Layer 1 convention for stable consumer-side iteration.
	diags := rec.Diagnostics
	if diags == nil {
		diags = []record.Diagnostic{}
	}
	nr := ndjsonRecord{
		Schema:      rec.Schema,
		PacketIndex: index,
		LSVersion:   rec.LSVersion,
		TotalLength: rec.TotalLength,
		Checksum:    rec.Checksum,
		Items:       []ndjsonItem{},
		Diagnostics: diags,
	}
	for _, it := range rec.Items {
		ni := ndjsonItem{Tag: it.Tag, Name: it.Name, Value: it.Value}
		if includeRaw {
			ni.Units = it.Units
			ni.Raw = encodeBase64(it.Raw)
		}
		nr.Items = append(nr.Items, ni)
	}
	b, err := json.Marshal(nr)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

func writeText(w io.Writer, index int, rec record.Record, includeRaw bool) error {
	if _, err := fmt.Fprintf(w, "Packet %d   schema=%s  checksum=%s\n", index, rec.Schema, checksumLabel(rec)); err != nil {
		return err
	}
	for _, it := range rec.Items {
		units := ""
		if includeRaw {
			units = it.Units
		}
		if _, err := fmt.Fprintf(w, "  [%d]\t%-40s\t%s\n", it.Tag, it.Name, formatValue(it.Value, units)); err != nil {
			return err
		}
		if includeRaw && len(it.Raw) > 0 {
			if _, err := fmt.Fprintf(w, "       \traw=0x%x\n", it.Raw); err != nil {
				return err
			}
		}
	}
	for _, d := range rec.Diagnostics {
		if _, err := fmt.Fprintf(w, "  ! [%s] %s: %s\n", d.Severity, d.Code, d.Message); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	return nil
}

// checksumLabel distinguishes the four states operator output needs:
// OK (engine computed and matched), MISMATCH (computed and disagreed),
// MALFORMED (tag 1 present but wrong length — engine couldn't compute),
// and N/A (tag 1 absent). Validate emits the corresponding structural
// diagnostics, so this label is purely for the human-readable header.
func checksumLabel(rec record.Record) string {
	for _, it := range rec.Items {
		if it.Tag == 1 {
			if len(it.Raw) != 2 {
				return "MALFORMED"
			}
			if rec.Checksum.Valid {
				return "OK"
			}
			return "MISMATCH"
		}
	}
	return "N/A"
}

func formatValue(v record.Value, units string) string {
	if v == nil {
		return "<nil>"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "<error>"
	}
	s := strings.Trim(string(b), `"`)
	if units != "" {
		return s + units
	}
	return s
}

func encodeBase64(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

// liftPacketizeDiagnostics converts packetize.Diagnostic entries into
// record.Diagnostic entries so CLI reporting (counters, --strict, NDJSON
// output) treats them the same as KLV-layer diagnostics.
func liftPacketizeDiagnostics(in []packetize.Diagnostic) []record.Diagnostic {
	if len(in) == 0 {
		return nil
	}
	out := make([]record.Diagnostic, 0, len(in))
	for _, d := range in {
		out = append(out, record.Diagnostic{
			Severity: d.Severity,
			Code:     "packetize_" + d.Code,
			Message:  d.Message,
		})
	}
	return out
}
