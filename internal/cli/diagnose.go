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

	"github.com/jacorbello/klvtool/internal/envcheck"
	"github.com/jacorbello/klvtool/internal/model"
	ts "github.com/jacorbello/klvtool/internal/mpeg/ts"
)

// DiagnoseCommand runs the full diagnostic pipeline on an MPEG-TS file:
// health check, transport inspection, and KLV decode.
type DiagnoseCommand struct {
	Out io.Writer
	Err io.Writer

	isOutputTTY func(io.Writer) bool
	Detect      func(context.Context, string, map[string]string) envcheck.Report
	Inspect     func(path string) (ts.StreamTable, InspectStats, error)
	Decode      func(path string, pid int, schema string) (DecodeResult, error)

	GOOS    string
	Env     map[string]string
	Version string
}

// NewDiagnoseCommand returns a DiagnoseCommand with default runtime dependencies.
func NewDiagnoseCommand() *DiagnoseCommand {
	cmd := &DiagnoseCommand{
		Out:     os.Stdout,
		Err:     os.Stderr,
		GOOS:    runtime.GOOS,
		Env:     currentEnvMap(),
		Detect:  defaultDoctorDetect,
		Inspect: defaultInspect,
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

	fs := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var inputPath, view string
	fs.StringVar(&inputPath, "input", "", "path to the MPEG-TS input file")
	fs.StringVar(&view, "view", string(viewAuto), "presentation mode: auto, pretty, or raw")

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

	// Stage 1: Health check
	report := c.Detect(context.Background(), c.goos(), c.env())
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
	table, stats, err := c.Inspect(inputPath)
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
		result, err := c.Decode(inputPath, int(pid), "")
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
			if isLikelyMetadataStream(stream.StreamType) {
				streamLabel = "Likely metadata: " + typeName
			}
			_, _ = fmt.Fprintf(w, "    PID 0x%04X  %s  packets=%d\n", stream.PID, streamLabel, count)
		}
	}
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
	if w == nil {
		return
	}
	_, _ = fmt.Fprintln(w, "Usage: klvtool diagnose --input <file.ts>")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Run the full diagnostic pipeline: health check, transport inspection, and KLV decode.")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Equivalent to running doctor, inspect, and decode in sequence, with a consolidated report.")
}

func (c *DiagnoseCommand) writeError(w io.Writer, err error) {
	if w == nil || err == nil {
		return
	}
	_, _ = fmt.Fprintln(w, errorLine(newColorizer(c.outputTTY(w) && supportsANSI()), err))
}
