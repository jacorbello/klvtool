package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/jacorbello/klvtool/internal/cli/commanddef"
	"github.com/jacorbello/klvtool/internal/envcheck"
	"github.com/jacorbello/klvtool/internal/h264"
	"github.com/jacorbello/klvtool/internal/model"
	ts "github.com/jacorbello/klvtool/internal/mpeg/ts"
)

type diagnoseFlags struct {
	inputPath string
	view      string
}

func diagnoseFlagSet(v *diagnoseFlags) *flag.FlagSet {
	fs := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&v.inputPath, "input", "", "MPEG-TS input path")
	fs.StringVar(&v.view, "view", string(viewAuto), "presentation mode: auto, pretty, or raw")
	return fs
}

// DiagnoseCommand runs the full diagnostic pipeline on an MPEG-TS file:
// health check, transport inspection, video bitstream analysis, and
// KLV decode. Video analysis is non-fatal — failures are reported and
// the pipeline continues into KLV decode.
type DiagnoseCommand struct {
	Out io.Writer
	Err io.Writer

	isOutputTTY  func(io.Writer) bool
	Detect       func(context.Context, string, map[string]string) envcheck.Report
	Inspect      func(path string) (ts.StreamTable, InspectStats, error)
	VideoAnalyze func(path string, table ts.StreamTable) ([]h264.VideoReport, error)
	Decode       func(path string, pid int, schema string) (DecodeResult, error)

	GOOS    string
	Env     map[string]string
	Version string
}

// NewDiagnoseCommand returns a DiagnoseCommand with default runtime dependencies.
func NewDiagnoseCommand() *DiagnoseCommand {
	cmd := &DiagnoseCommand{
		Out:          os.Stdout,
		Err:          os.Stderr,
		GOOS:         runtime.GOOS,
		Env:          currentEnvMap(),
		Detect:       defaultDoctorDetect,
		Inspect:      defaultInspect,
		VideoAnalyze: defaultVideoAnalyze,
	}
	// Decode is wired to the same pipeline as DecodeCommand.
	decodeCmd := NewDecodeCommand()
	cmd.Decode = decodeCmd.Decode
	return cmd
}

func (c *DiagnoseCommand) Execute(args []string) int {
	if c == nil {
		return 1
	}
	if len(args) == 1 && isHelpArg(args[0]) {
		c.writeUsage(c.Out)
		return 0
	}

	var v diagnoseFlags
	fs := diagnoseFlagSet(&v)

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
	inputPath, view := v.inputPath, v.view
	if strings.TrimSpace(inputPath) == "" {
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(fmt.Errorf("input path is required")))
		return usageExitCode
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
		e := model.TSRead(fmt.Errorf("input path is not a regular file: %s", inputPath))
		c.writeError(c.Err, e)
		return exitCodeForError(e)
	}

	pretty := usePrettyView(viewMode, c.outputTTY(c.Out))
	return c.run(inputPath, pretty)
}

type diagnoseStage string

const (
	stageHealth  diagnoseStage = "health check"
	stageInspect diagnoseStage = "inspect"
	stageDecode  diagnoseStage = "decode"
)

func (c *DiagnoseCommand) run(inputPath string, pretty bool) int {
	w := c.Out
	color := newColorizer(pretty && supportsANSI())

	detect := c.Detect
	if detect == nil {
		detect = defaultDoctorDetect
	}
	inspect := c.Inspect
	if inspect == nil {
		inspect = defaultInspect
	}
	decode := c.Decode
	if decode == nil {
		decodeCmd := NewDecodeCommand()
		decode = decodeCmd.Decode
	}

	// Stage 1: Health check
	stop := startSpinner(c.Err, color, pretty, "Checking backend health...")
	report := detect(context.Background(), c.goos(), c.env())
	stop()
	c.writeHealthSection(w, color, report)

	for _, b := range report.Backends {
		if !b.Healthy {
			c.writeStoppedAt(w, color, stageHealth)
			if pretty {
				writeHintFooters(w, color, []hintFooter{
					{Title: "Install ffmpeg and retry", Body: "klvtool doctor"},
				})
			}
			return 1
		}
	}

	// Stage 2: Inspect
	stop = startSpinner(c.Err, color, pretty, "Scanning transport stream...")
	table, stats, err := inspect(inputPath)
	stop()
	if err != nil {
		c.writeStoppedAt(w, color, stageInspect)
		_, _ = fmt.Fprintf(w, "  %s\n", err)
		if pretty {
			writeHintFooters(w, color, []hintFooter{
				{Title: "Inspect manually for details", Body: fmt.Sprintf("klvtool inspect --input %s", inputPath)},
			})
		}
		return 1
	}

	c.writeTransportSection(w, color, table, stats, pretty)

	// Stage 2.5: Video analysis (non-fatal — any failure is reported
	// and the pipeline continues into KLV decode).
	if videoAnalyze := c.VideoAnalyze; videoAnalyze != nil {
		stop = startSpinner(c.Err, color, pretty, "Analyzing video bitstream...")
		videoReports, vErr := videoAnalyze(inputPath, table)
		stop()
		if vErr != nil {
			_, _ = fmt.Fprintln(w)
			_, _ = fmt.Fprintf(w, "%s%s\n", labelPrefix(color, true, "Video "), color.yellow("analysis skipped: "+vErr.Error()))
		} else {
			c.writeVideoSection(w, color, videoReports, unsupportedVideoStreams(table), pretty)
		}
	}

	// Find candidate metadata PIDs.
	metaPIDs := candidateMetadataPIDs(table)
	if len(metaPIDs) == 0 {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "No likely metadata streams found.")
		if pretty {
			writeHintFooters(w, color, []hintFooter{
				{Title: "Try a full decode across all PIDs", Body: fmt.Sprintf("klvtool decode --input %s", inputPath)},
			})
		}
		return 0
	}

	// Stage 3: Decode each candidate PID.
	for _, pid := range metaPIDs {
		stop = startSpinner(c.Err, color, pretty, fmt.Sprintf("Decoding PID 0x%04X...", pid))
		result, err := decode(inputPath, int(pid), "")
		stop()
		if err != nil {
			_, _ = fmt.Fprintln(w)
			c.writeStoppedAt(w, color, stageDecode)
			_, _ = fmt.Fprintf(w, "  PID 0x%04X: %s\n", pid, err)
			if pretty {
				writeHintFooters(w, color, []hintFooter{
					{Title: "Decode manually for details", Body: fmt.Sprintf("klvtool decode --input %s --pid %d --format text", inputPath, pid)},
				})
			}
			return 1
		}

		c.writeDecodeSection(w, color, pid, result)
	}

	if pretty {
		pid := metaPIDs[0]
		writeHintFooters(w, color, diagnoseHints(inputPath, int(pid)))
	}
	return 0
}

func (c *DiagnoseCommand) writeHealthSection(w io.Writer, color colorizer, report envcheck.Report) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "%s\n", labelPrefix(color, true, "Backend"))
	for _, backend := range report.Backends {
		if backend.Healthy {
			versions := make([]string, 0, len(backend.Tools))
			for _, tool := range backend.Tools {
				v := parseToolVersion(backend.Name, tool.Version)
				if v != "" {
					versions = append(versions, tool.Name+" "+v)
				}
			}
			detail := ""
			if len(versions) > 0 {
				detail = " (" + strings.Join(versions, ", ") + ")"
			}
			_, _ = fmt.Fprintf(w, "  %s %s%s\n", backend.Name, color.green("\xe2\x9c\x93 available"), detail)
		} else if len(backend.MissingTools) > 0 {
			_, _ = fmt.Fprintf(w, "  %s %s\n", backend.Name, color.red("\xe2\x9c\x97 not installed"))
			_, _ = fmt.Fprintf(w, "  %s %s\n", color.red("missing:"), strings.Join(backend.MissingTools, ", "))
			for _, step := range report.Guidance {
				_, _ = fmt.Fprintf(w, "  install: %s\n", color.dim(step))
			}
		} else {
			_, _ = fmt.Fprintf(w, "  %s %s\n", backend.Name, color.red("\xe2\x9c\x97 unhealthy"))
		}
	}
}

func (c *DiagnoseCommand) writeTransportSection(w io.Writer, color colorizer, table ts.StreamTable, stats InspectStats, pretty bool) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "%stotal packets: %d\n", labelPrefix(color, pretty, "Transport "), stats.TotalPackets)

	programNums := make([]int, 0, len(table.Programs))
	for pn := range table.Programs {
		programNums = append(programNums, int(pn))
	}
	sort.Ints(programNums)

	for _, pn := range programNums {
		streams := table.Programs[uint16(pn)]
		_, _ = fmt.Fprintf(w, "  program %d:\n", pn)
		for _, stream := range streams {
			typeName := streamTypeName(stream.StreamType)
			count := stats.PacketCounts[stream.PID]
			streamLabel := typeName
			if pretty && isLikelyMetadataStream(stream.StreamType) {
				streamLabel = "Likely metadata: " + typeName
			}
			_, _ = fmt.Fprintf(w, "    PID 0x%04X  %s  packets=%d\n", stream.PID, streamLabel, count)
		}
	}
}

func (c *DiagnoseCommand) writeVideoSection(w io.Writer, color colorizer, reports []h264.VideoReport, unsupported []ts.Stream, pretty bool) {
	if w == nil {
		return
	}
	if len(reports) == 0 && len(unsupported) == 0 {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintf(w, "%sno video streams found\n", labelPrefix(color, true, "Video "))
		return
	}
	for _, rep := range reports {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintf(w, "%sPID 0x%04X  %s  %s\n",
			labelPrefix(color, true, "Video "),
			rep.PID,
			streamTypeName(rep.StreamType),
			verdictLabel(color, rep.Verdict),
		)
		_, _ = fmt.Fprintf(w, "  IDR frames:   %d\n", rep.IDRCount)
		_, _ = fmt.Fprintf(w, "  SPS:          %d\n", rep.SPSCount)
		_, _ = fmt.Fprintf(w, "  PPS:          %d\n", rep.PPSCount)
		_, _ = fmt.Fprintf(w, "  Non-IDR:      %d\n", rep.NonIDRCount)
		if rep.DeltaMode > 0 {
			_, _ = fmt.Fprintf(w, "  PTS gaps:     single=%d  double=%d  larger=%d  (mode Δ=%d ticks)\n",
				rep.SingleGapCount, rep.DoubleGapCount, rep.LargerGapCount, rep.DeltaMode)
		}
		for _, reason := range rep.Reasons {
			_, _ = fmt.Fprintf(w, "  %s %s\n", color.yellow("!"), reason)
		}
		if pretty && rep.FixHint != "" {
			_, _ = fmt.Fprintln(w)
			_, _ = fmt.Fprintf(w, "  Hint: %s\n", rep.FixHint)
		}
	}
	for _, s := range unsupported {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintf(w, "%sPID 0x%04X  %s  %s\n",
			labelPrefix(color, true, "Video "),
			s.PID,
			streamTypeName(s.StreamType),
			color.yellow("not yet analyzed"),
		)
		_, _ = fmt.Fprintf(w, "  klvtool currently analyzes H.264 only; this stream type is recognized but skipped.\n")
	}
}

func verdictLabel(color colorizer, v h264.Verdict) string {
	switch v {
	case h264.VerdictPlayable:
		return color.green(string(v))
	case h264.VerdictStallsInMSE:
		return color.red(string(v))
	case h264.VerdictDegraded:
		return color.yellow(string(v))
	}
	return string(v)
}

func (c *DiagnoseCommand) writeDecodeSection(w io.Writer, color colorizer, pid uint16, result DecodeResult) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "%sPID 0x%04X\n", labelPrefix(color, true, "Decode "), pid)

	if len(result.Records) > 0 {
		schema := result.Records[0].Schema
		if schema != "" {
			_, _ = fmt.Fprintf(w, "  schema: %s\n", schema)
		}
	}
	_, _ = fmt.Fprintf(w, "  packets decoded: %d\n", len(result.Records))

	var errors, warnings int
	for _, rec := range result.Records {
		for _, d := range rec.Diagnostics {
			switch d.Severity {
			case "error":
				errors++
			case "warning":
				warnings++
			}
		}
	}
	for _, d := range result.StreamDiagnostics {
		switch d.Severity {
		case "error":
			errors++
		case "warning":
			warnings++
		}
	}

	_, _ = fmt.Fprintf(w, "  diagnostics: %s, %s\n",
		countLabel(color, errors, "error"),
		countLabel(color, warnings, "warning"))
}

func countLabel(color colorizer, n int, singular string) string {
	label := fmt.Sprintf("%d %s", n, singular)
	if n != 1 {
		label += "s"
	}
	if n > 0 && singular == "error" {
		return color.red(label)
	}
	if n > 0 && singular == "warning" {
		return color.yellow(label)
	}
	return label
}

func (c *DiagnoseCommand) writeStoppedAt(w io.Writer, color colorizer, stage diagnoseStage) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "Stopped at: %s\n", color.red(string(stage)))
}

func candidateMetadataPIDs(table ts.StreamTable) []uint16 {
	programNums := make([]int, 0, len(table.Programs))
	for pn := range table.Programs {
		programNums = append(programNums, int(pn))
	}
	sort.Ints(programNums)

	var pids []uint16
	for _, pn := range programNums {
		for _, s := range table.Programs[uint16(pn)] {
			if isLikelyMetadataStream(s.StreamType) {
				pids = append(pids, s.PID)
			}
		}
	}
	return pids
}

func diagnoseHints(inputPath string, pid int) []hintFooter {
	return []hintFooter{
		{
			Title: "Decode with full output",
			Body:  fmt.Sprintf("klvtool decode --input %s --pid %d --format text", inputPath, pid),
		},
		{
			Title: "Step through packets interactively",
			Body:  fmt.Sprintf("klvtool decode --input %s --pid %d --step", inputPath, pid),
		},
		{
			Title: "Capture raw payload checkpoints",
			Body:  fmt.Sprintf("klvtool extract --input %s --out ./klvtool-raw", inputPath),
		},
	}
}

func (c *DiagnoseCommand) outputTTY(w io.Writer) bool {
	if c != nil && c.isOutputTTY != nil {
		return c.isOutputTTY(w)
	}
	return isTTYWriter(w)
}

func (c *DiagnoseCommand) goos() string {
	if c != nil && c.GOOS != "" {
		return c.GOOS
	}
	return runtime.GOOS
}

func (c *DiagnoseCommand) env() map[string]string {
	if c != nil && c.Env != nil {
		return c.Env
	}
	return currentEnvMap()
}

func (c *DiagnoseCommand) writeUsage(w io.Writer) {
	commanddef.RenderHelp(diagnoseDef, diagnoseFlagSet(&diagnoseFlags{}), w)
}

// Definition returns the CommandDef driving --help and man-page generation.
func (c *DiagnoseCommand) Definition() commanddef.CommandDef { return diagnoseDef }

var diagnoseDef = commanddef.CommandDef{
	Name:       "klvtool-diagnose",
	Subcommand: "diagnose",
	Synopsis:   "Run the full diagnostic pipeline: health check, transport inspect, video analysis, and KLV decode.",
	UsageLine:  "klvtool diagnose --input <file.ts> [--view auto|pretty|raw]",
	Description: "Run a one-shot diagnostic pipeline against a single MPEG-TS file: backend health check → transport inspection → H.264 video bitstream analysis (non-fatal) → KLV decode of every likely metadata PID.\n" +
		"\n" +
		"This is the right entry point for triaging a fresh mission file. The output is a single consolidated report ending in either a clean run or a `Stopped at: <stage>` line that names the failing stage. Video analysis reports a playability verdict (PLAYABLE / STALLS_IN_MSE / DEGRADED) for any H.264 stream — useful for catching missing IDR frames that prevent hls.js / MSE playback.",
	Examples: []commanddef.Example{
		{
			Comment: "Triage a fresh mission file end to end",
			Command: "klvtool diagnose --input mission.ts",
		},
	},
	Troubleshooting: []commanddef.TroubleshootingEntry{
		{
			Symptom:     "`Stopped at: health check`",
			LikelyCause: "ffmpeg or ffprobe not found on PATH; the extraction backend cannot start.",
			Action:      "Install ffmpeg and re-run. Confirm with `klvtool doctor`. The doctor output prints platform-specific install steps.",
		},
		{
			Symptom:     "`Stopped at: inspect`",
			LikelyCause: "MPEG-TS read or parse failure: the file is unreadable, severely truncated, or not actually a TS stream.",
			Action:      "Re-pull the file from the source if possible. Otherwise run `klvtool inspect --input <file>` directly to see the underlying error and check whether the file has a valid TS sync byte and program map.",
		},
		{
			Symptom:     "`Stopped at: decode`",
			LikelyCause: "The metadata stream is structurally invalid for the auto-detected MISB spec, or transport corruption is severe enough to break KLV framing on every recovery attempt.",
			Action:      "Run `klvtool decode --input <file> --pid <PID> --format text` to see per-packet diagnostics. If decode fails everywhere, run `klvtool extract` then `klvtool packetize --mode best-effort` to surface byte offsets where the wire format diverges, and escalate to platform / sensor engineering with that context.",
		},
		{
			Symptom:     "`No likely metadata streams found.`",
			LikelyCause: "The transport is healthy but the program map advertises no KLV / data PIDs. Either the sensor was not configured to emit metadata for this capture, or the metadata is muxed under a stream type klvtool does not yet recognize.",
			Action:      "Confirm the platform was emitting KLV during this mission (sensor config, mission profile). Run `klvtool inspect --input <file>` to see the full PID inventory and consider running `klvtool decode --input <file>` (no --pid) to attempt decode across every PID.",
		},
		{
			Symptom:     "Decode succeeds but reports many error diagnostics",
			LikelyCause: "Likely a non-compliant or pre-1.x sensor: structural validation against ST 0601 fails on individual tags but framing is intact.",
			Action:      "Inspect the diagnostic codes (`tag_decode_error`, `unknown_tag`, `validation_failed`). If a vendor-specific spec is in use, try `--schema <urn>` to override auto-detection. If results are still poor, capture an offending packet's bytes via `--raw` and escalate with the diagnostic context.",
		},
		{
			Symptom:     "Video verdict `STALLS_IN_MSE`",
			LikelyCause: "The H.264 stream is missing IDR frames or SPS/PPS — common with some encoder configurations. The metadata path is unaffected; only browser-based MSE playback (hls.js, video.js) stalls.",
			Action:      "If you only need metadata, ignore the verdict — KLV decode is independent. For playback, re-encode with libx264 forcing an IDR every keyframe; the diagnose output prints a ready-to-use `ffmpeg ... libx264 -g 60 -keyint_min 60 -sc_threshold 0` command in the FixHint when applicable.",
		},
		{
			Symptom:     "Video verdict `DEGRADED`",
			LikelyCause: "IDRs present but PES PTS deltas indicate dropped frames in transit.",
			Action:      "Treat as a transport-quality signal. If lossy delivery is the norm for this feed, the file is still usable for analysis; if not, re-pull or investigate the upstream link.",
		},
	},
	ExitCodes: []commanddef.ExitCode{
		{Code: 0, Meaning: "pipeline completed (with or without warnings)"},
		{Code: 1, Meaning: "a stage failed (health check, inspect, or decode); see `Stopped at: <stage>` for which"},
		{Code: 2, Meaning: "invalid usage"},
	},
	EnvVars: []commanddef.EnvVar{
		{Name: "NO_COLOR", Description: "disable ANSI color in pretty output"},
	},
	RequiredTools: []string{"ffmpeg", "ffprobe"},
	SeeAlso: []commanddef.SeeAlsoRef{
		{Name: "klvtool", Section: 1},
		{Name: "klvtool-doctor", Section: 1},
		{Name: "klvtool-inspect", Section: 1},
		{Name: "klvtool-decode", Section: 1},
		{Name: "klvtool-extract", Section: 1},
		{Name: "klvtool-packetize", Section: 1},
	},
	ExternalRefs: []commanddef.ExternalRef{
		{Title: "MISB ST 0601.19 — UAS Datalink Local Set", URL: "https://nsgreg.nga.mil/doc/view?i=5337"},
	},
}

func (c *DiagnoseCommand) writeError(w io.Writer, err error) {
	if w == nil || err == nil {
		return
	}
	_, _ = fmt.Fprintln(w, errorLine(newColorizer(c.outputTTY(w) && supportsANSI()), err))
}
