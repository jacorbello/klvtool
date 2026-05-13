package cli

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	ffmpegbackend "github.com/jacorbello/klvtool/internal/backends/ffmpeg"
	"github.com/jacorbello/klvtool/internal/cli/commanddef"
	"github.com/jacorbello/klvtool/internal/extract"
	"github.com/jacorbello/klvtool/internal/klv"
	"github.com/jacorbello/klvtool/internal/klv/record"
	"github.com/jacorbello/klvtool/internal/klv/specs/st0601"
	"github.com/jacorbello/klvtool/internal/model"
	"github.com/jacorbello/klvtool/internal/packetize"
)

// decodeFlags holds parsed --flag values for the decode subcommand.
type decodeFlags struct {
	inputPath        string
	format           string
	raw              bool
	strict           bool
	pid              int
	outPath          string
	schema           string
	view             string
	step             bool
	repairStrippedUL bool
}

func decodeFlagSet(v *decodeFlags) *flag.FlagSet {
	fs := flag.NewFlagSet("decode", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&v.inputPath, "input", "", "MPEG-TS input path")
	fs.StringVar(&v.format, "format", "ndjson", "output format: ndjson, text, or csv")
	fs.BoolVar(&v.raw, "raw", false, "include raw bytes per item (hex in text/csv, base64 in NDJSON)")
	fs.BoolVar(&v.strict, "strict", false, "exit non-zero if any error-severity diagnostic is emitted")
	fs.IntVar(&v.pid, "pid", 0, "limit decoding to a specific KLV stream PID (0 = all)")
	fs.StringVar(&v.outPath, "out", "", "write output to a file instead of stdout")
	fs.StringVar(&v.schema, "schema", "", "override auto-detection with a specific spec URN")
	fs.StringVar(&v.view, "view", string(viewAuto), "presentation mode: auto, pretty, or raw")
	fs.BoolVar(&v.step, "step", false, "step through decoded packets interactively")
	fs.BoolVar(&v.repairStrippedUL, "repair-stripped-ul", false, "re-attach the 5-byte SMPTE category prefix that some ffmpeg builds strip from KLV data streams")
	return fs
}

// mpegTSPIDMax is the highest valid MPEG-TS packet identifier (13-bit field).
const mpegTSPIDMax = 0x1FFF

// DecodeRequest carries the per-invocation options the Decode callback acts
// on. New options should be added as struct fields so existing tests don't
// need to re-thread positional arguments.
type DecodeRequest struct {
	Path             string
	PID              int
	Schema           string
	RepairStrippedUL bool
}

// DecodeCommand decodes MISB ST 0601 KLV from an MPEG-TS file into
// typed records.
type DecodeCommand struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
	// Decode runs the decode pipeline. When req.Schema is non-empty, the
	// implementation must restrict decoding to the SpecVersion registered
	// under that URN (bypassing UL-based auto-detection).
	Decode   func(req DecodeRequest) (DecodeResult, error)
	Registry func() *klv.Registry
	// openOut creates the output file for --out. Defaults to os.Create.
	// Exposed for testing close-error propagation.
	openOut     func(path string) (io.WriteCloser, error)
	isOutputTTY func(io.Writer) bool
	isInputTTY  func(io.Reader) bool
	makeRaw     func(io.Reader) (func() error, error)
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
		In:  os.Stdin,
		Out: os.Stdout,
		Err: os.Stderr,
		Registry: func() *klv.Registry {
			r := klv.NewRegistry()
			r.Register(st0601.V19())
			return r
		},
	}
	c.Decode = func(req DecodeRequest) (DecodeResult, error) {
		report := defaultDoctorDetect(context.Background(), "", currentEnvMap())
		desc := ffmpegDescriptor(report)
		if !desc.Healthy {
			return DecodeResult{}, model.MissingDependency(fmt.Errorf("ffmpeg backend is unavailable"))
		}

		extractor := extract.NewExtractor(ffmpegbackend.NewBackend())
		result, err := extractor.Run(context.Background(), extract.RunRequest{
			InputPath: req.Path,
			Backend:   desc,
		})
		if err != nil {
			return DecodeResult{}, err
		}

		reg := c.Registry()
		// When --schema is set, restrict decoding to just the requested
		// SpecVersion by building a single-entry registry. This makes the
		// flag a genuine override rather than a no-op gate.
		if req.Schema != "" {
			sv, ok := reg.Lookup(req.Schema)
			if !ok {
				return DecodeResult{}, model.InvalidUsage(fmt.Errorf("schema %q not registered", req.Schema))
			}
			reg = klv.NewRegistry()
			reg.Register(sv)
		}
		knownULs := reg.KnownULs()
		parser := packetize.NewParser()
		var res DecodeResult
		for _, raw := range result.Records {
			if req.PID != 0 && int(raw.PID) != req.PID {
				continue
			}
			stream, err := parser.Parse(packetize.Request{
				Mode:             packetize.ModeBestEffort,
				Record:           raw,
				KnownULs:         knownULs,
				RepairStrippedUL: req.RepairStrippedUL,
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
			// When the parser repaired a stripped-UL payload, packet
			// offsets refer to the rebuilt buffer in stream.Source.Payload
			// rather than the original raw.Payload — slice from the source
			// the parser actually used.
			parsedPayload := stream.Source.Payload
			for i, pkt := range stream.Packets {
				// Preserve the exact wire BER length bytes — the checksum
				// covers them and may use a non-canonical encoding.
				var lengthBytes []byte
				if pkt.LengthStart >= 0 && pkt.ValueStart >= pkt.LengthStart && pkt.ValueStart <= len(parsedPayload) {
					lengthBytes = parsedPayload[pkt.LengthStart:pkt.ValueStart]
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

	var v decodeFlags
	fs := decodeFlagSet(&v)

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			c.writeUsage(c.Out)
			return 0
		}
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(err))
		return usageExitCode
	}
	inputPath, format, raw, strict, pid, outPath, schema, view, step :=
		v.inputPath, v.format, v.raw, v.strict, v.pid, v.outPath, v.schema, v.view, v.step
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
	if format != "ndjson" && format != "text" && format != "csv" {
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(fmt.Errorf("invalid format %q (want ndjson|text|csv)", format)))
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
	viewMode, err := parseViewMode(view)
	if err != nil {
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(err))
		return usageExitCode
	}

	info, err := os.Stat(inputPath)
	if err != nil {
		var e error
		if os.IsNotExist(err) {
			e = model.TSRead(fmt.Errorf("input file does not exist: %s", inputPath))
		} else {
			e = model.TSRead(fmt.Errorf("failed to stat input file %q: %w", inputPath, err))
		}
		c.writeError(c.Err, e)
		return exitCodeForError(e)
	}
	if !info.Mode().IsRegular() {
		e := model.TSRead(fmt.Errorf("input path must be a regular file: %s", inputPath))
		c.writeError(c.Err, e)
		return exitCodeForError(e)
	}

	decode := c.Decode
	if decode == nil {
		decode = NewDecodeCommand().Decode
	}

	result, err := decode(DecodeRequest{
		Path:             inputPath,
		PID:              pid,
		Schema:           schema,
		RepairStrippedUL: v.repairStrippedUL,
	})
	if err != nil {
		c.writeError(c.Err, err)
		return exitCodeForError(err)
	}

	outputTTY := c.outputTTY(c.Out)
	prettyView := usePrettyView(viewMode, outputTTY)
	inputTTY := c.inputTTY(c.In)
	formatSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "format" {
			formatSet = true
		}
	})
	effectiveFormat := format
	if prettyView && !formatSet {
		effectiveFormat = "text"
	}
	if step {
		if !prettyView || !outputTTY || !inputTTY {
			c.writeUsage(c.Err)
			c.writeError(c.Err, model.InvalidUsage(errors.New("--step requires an interactive terminal")))
			return usageExitCode
		}
		effectiveFormat = "text"
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
	color := newColorizer(prettyView && supportsANSI())

	var csvW *csv.Writer
	if effectiveFormat == "csv" {
		csvW = csv.NewWriter(sink)
		if err := writeCSVHeader(csvW, raw); err != nil {
			c.writeError(c.Err, model.OutputWrite(err))
			exitCode = 1
			csvW = nil
		}
	}

	var errorCount int
	if step {
		if code, err := c.runStepMode(sink, result.Records, raw, color); err != nil {
			c.writeError(c.Err, model.OutputWrite(err))
			exitCode = code
		}
	} else {
	records:
		for i, rec := range result.Records {
			switch effectiveFormat {
			case "ndjson":
				if err := writeNDJSON(sink, i, rec, raw); err != nil {
					c.writeError(c.Err, model.OutputWrite(err))
					exitCode = 1
					break records
				}
			case "text":
				if err := writeTextView(sink, i, rec, raw, color); err != nil {
					c.writeError(c.Err, model.OutputWrite(err))
					exitCode = 1
					break records
				}
			case "csv":
				if csvW != nil {
					if err := writeCSVRecords(csvW, i, rec, raw); err != nil {
						c.writeError(c.Err, model.OutputWrite(err))
						exitCode = 1
						break records
					}
				}
			}
		}
	}
	for _, rec := range result.Records {
		for _, d := range rec.Diagnostics {
			if d.Severity == "error" {
				errorCount++
			}
		}
	}

	if effectiveFormat == "csv" && csvW != nil {
		csvW.Flush()
		if err := csvW.Error(); err != nil {
			if exitCode == 0 {
				c.writeError(c.Err, model.OutputWrite(err))
			}
			exitCode = 1
		}
	}

	// Stream-level diagnostics (e.g. packetize recovery events on raw
	// streams that produced zero decoded KLV packets) are reported to
	// stderr and counted toward --strict, but not emitted as packets.
	for _, d := range result.StreamDiagnostics {
		if c.Err != nil {
			fmt.Fprintf(c.Err, "[stream] %s %s: %s\n", colorSeverity(color, d.Severity), d.Code, d.Message) //nolint:errcheck
		}
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
	if c.Err != nil {
		errColor := newColorizer(c.outputTTY(c.Err) && supportsANSI())
		fmt.Fprintf(c.Err, "decoded %d packet(s), %d error diagnostic(s)\n", len(result.Records), errorCount) //nolint:errcheck
		if pid != 0 && len(result.Records) == 0 {
			fmt.Fprintln(c.Err, warningLine(errColor, "no KLV packets found on PID %d", pid)) //nolint:errcheck
		}
	}
	if strict && errorCount > 0 {
		return 1
	}
	if prettyView && effectiveFormat == "text" {
		writeHintFooters(c.Out, color, decodeHints(inputPath, pid))
	}
	return 0
}

func (c *DecodeCommand) writeUsage(w io.Writer) {
	commanddef.RenderHelp(decodeDef, decodeFlagSet(&decodeFlags{}), w)
}

func (c *DecodeCommand) writeError(w io.Writer, err error) {
	if w == nil || err == nil {
		return
	}
	fmt.Fprintln(w, errorLine(newColorizer(c.outputTTY(w) && supportsANSI()), err)) //nolint:errcheck
}

// ndjsonRecord is the serialization shape for one packet.
type ndjsonRecord struct {
	Schema      string              `json:"schema"`
	PacketIndex int                 `json:"packetIndex"`
	LSVersion   int                 `json:"lsVersion"`
	ValueLength int                 `json:"valueLength"`
	Checksum    record.ChecksumInfo `json:"checksum"`
	Items       []ndjsonItem        `json:"items"`
	Diagnostics []record.Diagnostic `json:"diagnostics"`
}

type ndjsonItem struct {
	Tag   int          `json:"tag"`
	Name  string       `json:"name"`
	Value record.Value `json:"value"`
	Units string       `json:"units"`
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
		ValueLength: rec.ValueLength,
		Checksum:    rec.Checksum,
		Items:       []ndjsonItem{},
		Diagnostics: diags,
	}
	for _, it := range rec.Items {
		ni := ndjsonItem{Tag: it.Tag, Name: it.Name, Value: it.Value, Units: it.Units}
		if includeRaw {
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

func writeCSVHeader(w *csv.Writer, includeRaw bool) error {
	header := []string{"packetIndex", "tag", "name", "value", "units"}
	if includeRaw {
		header = append(header, "raw")
	}
	return w.Write(header)
}

func writeCSVRecords(w *csv.Writer, index int, rec record.Record, includeRaw bool) error {
	for _, it := range rec.Items {
		row := []string{
			strconv.Itoa(index),
			strconv.Itoa(it.Tag),
			it.Name,
			formatCSVValue(it.Value),
			it.Units,
		}
		if includeRaw {
			row = append(row, formatRawHex(it.Raw))
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

func formatCSVValue(v record.Value) string {
	if v == nil {
		return "<nil>"
	}
	if s, ok := v.(record.StringValue); ok {
		return string(s)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "<error>"
	}
	if s, err := strconv.Unquote(string(b)); err == nil {
		return s
	}
	return string(b)
}

func formatRawHex(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return fmt.Sprintf("0x%x", b)
}

func writeTextView(w io.Writer, index int, rec record.Record, includeRaw bool, color colorizer) error {
	header := fmt.Sprintf("Packet %d   schema=%s  checksum=%s", index, rec.Schema, checksumLabel(rec))
	if _, err := fmt.Fprintln(w, color.bold(header)); err != nil {
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
		if _, err := fmt.Fprintf(w, "  ! [%s] %s: %s%s\n", colorSeverity(color, d.Severity), d.Code, d.Message, formatDiagnosticContext(d)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	return nil
}

func writeText(w io.Writer, index int, rec record.Record, includeRaw bool) error {
	return writeTextView(w, index, rec, includeRaw, newColorizer(false))
}

func colorSeverity(color colorizer, severity string) string {
	switch severity {
	case "error":
		return color.red(severity)
	case "warning":
		return color.yellow(severity)
	default:
		return severity
	}
}

func (c *DecodeCommand) outputTTY(w io.Writer) bool {
	if c != nil && c.isOutputTTY != nil {
		return c.isOutputTTY(w)
	}
	return isTTYWriter(w)
}

func (c *DecodeCommand) inputTTY(r io.Reader) bool {
	if c != nil && c.isInputTTY != nil {
		return c.isInputTTY(r)
	}
	return isTTYReader(r)
}

func (c *DecodeCommand) enableRawInput(r io.Reader) (func() error, error) {
	if c != nil && c.makeRaw != nil {
		return c.makeRaw(r)
	}
	return makeRawInput(r)
}

func (c *DecodeCommand) runStepMode(w io.Writer, records []record.Record, raw bool, color colorizer) (int, error) {
	if len(records) == 0 {
		return 0, nil
	}
	reader := bufio.NewReader(c.In)
	index := 0
	restore, err := c.enableRawInput(c.In)
	if err != nil {
		return 1, err
	}
	if restore != nil {
		defer func() { _ = restore() }()
	}
	if c.Err != nil {
		_, _ = fmt.Fprintln(c.Err, color.cyan("step mode: r=next, w=previous, d=next diagnostic, e=next error, q=quit"))
	}
	for {
		if err := writeTextView(w, index, records[index], raw, color); err != nil {
			return 1, err
		}
		if c.Err != nil {
			_, _ = fmt.Fprint(c.Err, color.cyan("step [r=next,w=prev,d=diag,e=error,q=quit]> "))
		}
		ch, err := reader.ReadByte()
		if errors.Is(err, io.EOF) {
			return 0, nil
		}
		if err != nil {
			return 1, err
		}
		switch ch {
		case 'q':
			return 0, nil
		case '\n', '\r':
			continue
		case 'r':
			if index < len(records)-1 {
				index++
			}
		case 'w':
			if index > 0 {
				index--
			}
		case 'd':
			next := nextMatchingRecord(records, index+1, func(rec record.Record) bool {
				return len(rec.Diagnostics) > 0
			})
			if next == -1 {
				return 0, nil
			}
			index = next
		case 'e':
			next := nextMatchingRecord(records, index+1, func(rec record.Record) bool {
				for _, d := range rec.Diagnostics {
					if d.Severity == "error" {
						return true
					}
				}
				return false
			})
			if next == -1 {
				return 0, nil
			}
			index = next
		}
	}
}

func nextMatchingRecord(records []record.Record, start int, match func(record.Record) bool) int {
	for i := start; i < len(records); i++ {
		if match(records[i]) {
			return i
		}
	}
	return -1
}

func decodeHints(inputPath string, pid int) []hintFooter {
	hints := []hintFooter{
		{
			Title: "Cross-check stream structure",
			Body:  fmt.Sprintf("klvtool inspect --input %s", inputPath),
		},
	}
	if pid != 0 {
		hints = append(hints, hintFooter{
			Title: "Capture raw payload checkpoints",
			Body:  fmt.Sprintf("klvtool extract --input %s --out ./klvtool-raw", inputPath),
		})
	}
	return hints
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

func formatDiagnosticContext(d record.Diagnostic) string {
	parts := make([]string, 0, 4)
	if d.Tag != nil {
		tag := fmt.Sprintf("tag=%d", *d.Tag)
		if d.TagName != "" {
			tag += " " + d.TagName
		}
		parts = append(parts, tag)
	} else if d.TagName != "" {
		parts = append(parts, "tag="+d.TagName)
	}
	if d.Actual != "" {
		parts = append(parts, "actual="+d.Actual)
	}
	if d.Expected != "" {
		parts = append(parts, "expected="+d.Expected)
	}
	if d.Raw != "" {
		parts = append(parts, "raw="+d.Raw)
	}
	if len(parts) == 0 {
		return ""
	}
	return " [" + strings.Join(parts, ", ") + "]"
}

// Definition returns the CommandDef driving --help and man-page generation.
func (c *DecodeCommand) Definition() commanddef.CommandDef { return decodeDef }

// decodeDef captures the documentation for `klvtool decode`.
//
// Keep flag descriptions in decodeFlagSet — they are pulled into the rendered
// output via flag.FlagSet.VisitAll. Anything below describes prose, examples,
// and the analyst-facing schema reference for downstream pipelines.
var decodeDef = commanddef.CommandDef{
	Name:       "klvtool-decode",
	Subcommand: "decode",
	Synopsis:   "Decode MISB ST 0601 KLV records from an MPEG-TS file.",
	UsageLine:  "klvtool decode --input <file.ts> [--format ndjson|text|csv] [--pid N] [--out <path>] [--strict] [--raw] [--step] [--view auto|pretty|raw] [--schema <urn>] [--repair-stripped-ul]",
	Description: "Decode MISB ST 0601 KLV metadata from an MPEG-TS file into typed records.\n" +
		"\n" +
		"Use this after `klvtool inspect` to validate a likely metadata PID or to review packets in a terminal-friendly view. The --raw flag includes raw bytes per item: hex (0x...) in text and csv formats, base64 in NDJSON. The --step flag enables one-handed packet navigation in pretty text view: r=next, w=previous, d=next diagnostic, e=next error, q=quit.",
	Examples: []commanddef.Example{
		{
			Comment: "Decode every metadata record as NDJSON for downstream pipelines",
			Command: "klvtool decode --input mission.ts --format ndjson --out frames.ndjson",
		},
		{
			Comment: "Decode just one PID after `klvtool inspect` identifies it",
			Command: "klvtool decode --input mission.ts --pid 257 --format text",
		},
		{
			Comment: "Step through packets with diagnostics highlighted",
			Command: "klvtool decode --input mission.ts --pid 257 --step",
		},
	},
	OutputFormat: &commanddef.OutputDoc{
		Format: "NDJSON, one JSON object per decoded KLV packet (default --format=ndjson). The --format=text and --format=csv variants are for human review and are not contractual.",
		Fields: []commanddef.FieldDef{
			{Name: "schema", Type: "string", Notes: "spec URN of the registered MISB Local Set used to decode this packet (e.g. \"urn:misb:KLV:bin:0601.19\")"},
			{Name: "packetIndex", Type: "integer", Notes: "0-based index of this packet within the decoded run"},
			{Name: "lsVersion", Type: "integer", Notes: "MISB ST 0601 Local Set version reported by tag 65, or 0 if absent"},
			{Name: "valueLength", Type: "integer", Units: "bytes", Notes: "BER-encoded value length of the outer Local Set"},
			{Name: "checksum", Type: "object", Notes: "{expected, computed, valid}; tag 1 must match the engine's recomputation"},
			{Name: "items", Type: "array", Notes: "decoded tags: {tag, name, value, units, raw?}; units is always present (empty string for tags without a unit)"},
			{Name: "items[].value", Type: "polymorphic", Notes: "type follows the spec: int/uint/float/string/bool/enum/time. ST 0601 \"error indicator\" sentinels marshal to JSON null."},
			{Name: "items[].raw", Type: "string", Notes: "present only with --raw; base64-encoded value bytes"},
			{Name: "diagnostics", Type: "array", Notes: "structural and per-tag findings: {severity, code, message, tag?, tagName?, actual?, expected?, raw?}"},
		},
		TimeSemantics: "ST 0601 tag 2 (Precision Time Stamp) decodes to a TimeValue and serializes as RFC 3339 with microsecond precision in UTC, e.g. \"2024-01-15T14:23:45.123456Z\". When correlating frames against a wall-clock event, prefer tag 2 over MPEG-TS PES PTS — PTS is transport-relative and resets on stream restarts.",
		Stability:     "Schema is stable within klvtool 1.x: new fields are additive; existing field names and types do not change in minor releases. Diagnostic codes may be added but existing codes will not be repurposed.",
	},
	MISBTagSummary: []commanddef.MISBTag{
		{Number: 1, Name: "Checksum", Notes: "16-bit BCC over the packet; klvtool recomputes and reports valid/mismatch/malformed/N/A"},
		{Number: 2, Name: "Precision Time Stamp", Notes: "wall-clock at sample time (UTC); preferred timestamp for cross-correlation"},
		{Number: 5, Name: "Platform Heading Angle", Units: "degrees", Notes: "0 = north, clockwise"},
		{Number: 6, Name: "Platform Pitch Angle", Units: "degrees"},
		{Number: 7, Name: "Platform Roll Angle", Units: "degrees"},
		{Number: 13, Name: "Sensor Latitude", Units: "degrees", Notes: "WGS-84"},
		{Number: 14, Name: "Sensor Longitude", Units: "degrees", Notes: "WGS-84"},
		{Number: 15, Name: "Sensor True Altitude", Units: "meters"},
		{Number: 23, Name: "Frame Center Latitude", Units: "degrees", Notes: "WGS-84; the geo-anchor most GIS workflows want"},
		{Number: 24, Name: "Frame Center Longitude", Units: "degrees", Notes: "WGS-84; pair with tag 23"},
		{Number: 25, Name: "Frame Center Elevation", Units: "meters"},
		{Number: 65, Name: "UAS LDS Version Number", Notes: "spec revision the producer claims to comply with"},
		{Number: 94, Name: "MISB Tag (deprecated key list)", Notes: "rarely emitted by modern producers; surfaced for completeness"},
	},
	ExitCodes: []commanddef.ExitCode{
		{Code: 0, Meaning: "success — decoded zero or more packets without errors"},
		{Code: 1, Meaning: "decode failure (TS read error, ffmpeg failure, output write failure, or --strict and any error-severity diagnostic fired)"},
		{Code: 2, Meaning: "invalid usage (missing or malformed flags, unknown schema URN, --step requested without an interactive terminal)"},
	},
	EnvVars: []commanddef.EnvVar{
		{Name: "NO_COLOR", Description: "disable ANSI color in pretty output, regardless of TTY detection"},
	},
	RequiredTools: []string{"ffmpeg", "ffprobe"},
	SeeAlso: []commanddef.SeeAlsoRef{
		{Name: "klvtool", Section: 1},
		{Name: "klvtool-inspect", Section: 1},
		{Name: "klvtool-extract", Section: 1},
		{Name: "klvtool-diagnose", Section: 1},
	},
	ExternalRefs: []commanddef.ExternalRef{
		{Title: "MISB ST 0601.19 — UAS Datalink Local Set", URL: "https://nsgreg.nga.mil/doc/view?i=5337"},
		{Title: "MISB ST 1402 — MPEG-2 Transport of Compressed Video Metadata", URL: "https://nsgreg.nga.mil/doc/view?i=5135"},
	},
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
