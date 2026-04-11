package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
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

// DecodeCommand decodes MISB ST 0601 KLV from an MPEG-TS file into
// typed records.
type DecodeCommand struct {
	Out      io.Writer
	Err      io.Writer
	Decode   func(path string, pid int) ([]record.Record, error)
	Registry func() *klv.Registry
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
	c.Decode = func(path string, pid int) ([]record.Record, error) {
		report := defaultDoctorDetect(context.Background(), "", currentEnvMap())
		desc := ffmpegDescriptor(report)
		if !desc.Healthy {
			return nil, model.MissingDependency(fmt.Errorf("ffmpeg backend is unavailable"))
		}

		extractor := extract.NewExtractor(ffmpegbackend.NewBackend())
		result, err := extractor.Run(context.Background(), extract.RunRequest{
			InputPath: path,
			Backend:   desc,
		})
		if err != nil {
			return nil, err
		}

		reg := c.Registry()
		parser := packetize.NewParser()
		var out []record.Record
		for _, raw := range result.Records {
			if pid != 0 && int(raw.PID) != pid {
				continue
			}
			stream, err := parser.Parse(packetize.Request{
				Mode:   packetize.ModeBestEffort,
				Record: raw,
			})
			if err != nil {
				return nil, err
			}
			// Lift packetize-layer diagnostics (recovery events, malformed
			// packet scans) into record.Diagnostic so --strict and the final
			// summary see them. Without this, best-effort recovery is silent.
			sourceDiags := liftPacketizeDiagnostics(stream.Diagnostics)
			if len(stream.Packets) == 0 && len(sourceDiags) > 0 {
				// No KLV packets decoded from this raw stream but packetize
				// recovered from or flagged problems. Emit a placeholder
				// Record so the diagnostics aren't dropped.
				out = append(out, record.Record{
					Schema:      "",
					Diagnostics: sourceDiags,
				})
				continue
			}
			for i, pkt := range stream.Packets {
				rec, err := klv.DecodeLocalSet(reg, pkt.Key, pkt.Value)
				if err != nil {
					return nil, err
				}
				// Attach packetize diagnostics to the first decoded record
				// from this raw stream so they flow through the normal
				// reporting path.
				if i == 0 && len(sourceDiags) > 0 {
					rec.Diagnostics = append(sourceDiags, rec.Diagnostics...)
				}
				out = append(out, rec)
			}
		}
		return out, nil
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
	if strings.TrimSpace(schema) != "" && schema != "urn:misb:KLV:bin:0601.19" {
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(fmt.Errorf("unsupported schema %q (phase 1 only registers urn:misb:KLV:bin:0601.19)", schema)))
		return usageExitCode
	}

	decode := c.Decode
	if decode == nil {
		decode = NewDecodeCommand().Decode
	}

	records, err := decode(inputPath, pid)
	if err != nil {
		c.writeError(c.Err, err)
		return exitCodeForError(err)
	}

	sink := c.Out
	if outPath != "" {
		f, err := os.Create(outPath)
		if err != nil {
			c.writeError(c.Err, model.OutputWrite(err))
			return exitCodeForError(err)
		}
		defer f.Close() //nolint:errcheck
		sink = f
	}

	var errorCount int
	for i, rec := range records {
		if format == "ndjson" {
			if err := writeNDJSON(sink, i, rec, raw); err != nil {
				c.writeError(c.Err, model.OutputWrite(err))
				return 1
			}
		} else {
			if err := writeText(sink, i, rec, raw); err != nil {
				c.writeError(c.Err, model.OutputWrite(err))
				return 1
			}
		}
		for _, d := range rec.Diagnostics {
			if d.Severity == "error" {
				errorCount++
			}
		}
	}

	fmt.Fprintf(c.Err, "decoded %d packet(s), %d validation error(s)\n", len(records), errorCount) //nolint:errcheck
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
	nr := ndjsonRecord{
		Schema:      rec.Schema,
		PacketIndex: index,
		LSVersion:   rec.LSVersion,
		TotalLength: rec.TotalLength,
		Checksum:    rec.Checksum,
		Diagnostics: rec.Diagnostics,
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
	check := "OK"
	if !rec.Checksum.Valid {
		check = "MISMATCH"
	}
	fmt.Fprintf(w, "Packet %d   schema=%s  checksum=%s\n", index, rec.Schema, check) //nolint:errcheck
	for _, it := range rec.Items {
		fmt.Fprintf(w, "  [%d]\t%-40s\t%s\n", it.Tag, it.Name, formatValue(it.Value, it.Units)) //nolint:errcheck
		if includeRaw && len(it.Raw) > 0 {
			fmt.Fprintf(w, "       \traw=0x%x\n", it.Raw) //nolint:errcheck
		}
	}
	for _, d := range rec.Diagnostics {
		fmt.Fprintf(w, "  ! [%s] %s: %s\n", d.Severity, d.Code, d.Message) //nolint:errcheck
	}
	fmt.Fprintln(w) //nolint:errcheck
	return nil
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
