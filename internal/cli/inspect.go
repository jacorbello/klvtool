package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/jacorbello/klvtool/internal/cli/commanddef"
	"github.com/jacorbello/klvtool/internal/model"
	ts "github.com/jacorbello/klvtool/internal/mpeg/ts"
	"github.com/jacorbello/klvtool/internal/stream"
)

type inspectFlags struct {
	inputPath string
	view      string
	streamFlags
}

func inspectFlagSet(v *inspectFlags) *flag.FlagSet {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&v.inputPath, "input", "", "MPEG-TS input path or URL (file://, udp://, tcp://, http(s)://, rtsp://, srt://, or '-' for stdin)")
	fs.StringVar(&v.view, "view", string(viewAuto), "presentation mode: auto, pretty, or raw")
	registerStreamFlags(fs, &v.streamFlags, streamFlagsInspect)
	return fs
}

// InspectStats holds aggregated statistics from a transport stream scan.
type InspectStats struct {
	TotalPackets  int64
	PacketCounts  map[uint16]int64
	PESUnitCounts map[uint16]int
	FirstPTS      map[uint16]int64
	LastPTS       map[uint16]int64
	Diagnostics   []ts.Diagnostic
}

// InspectCommand reports transport stream diagnostics.
type InspectCommand struct {
	Out     io.Writer
	Err     io.Writer
	Inspect func(path string) (ts.StreamTable, InspectStats, error)
	// InspectStream is called for non-file inputs (UDP/RTSP/SRT/...).
	// It snapshots PMT and per-PID counts over the run window and is
	// driven by the shared lifecycle (--duration / --idle-timeout /
	// --max-packets / SIGINT). Defaults to defaultInspectStream.
	InspectStream func(ctx context.Context, src stream.Source) (ts.StreamTable, InspectStats, error)
	// OpenSource is overridable for tests; defaults to stream.Open.
	OpenSource streamSourceOpener

	isOutputTTY func(io.Writer) bool
}

func NewInspectCommand() *InspectCommand {
	return &InspectCommand{
		Out:           os.Stdout,
		Err:           os.Stderr,
		Inspect:       defaultInspect,
		InspectStream: defaultInspectStream,
	}
}

func (c *InspectCommand) Execute(args []string) int {
	if c == nil {
		return 1
	}
	if len(args) == 1 && isHelpArg(args[0]) {
		c.writeUsage(c.Out)
		return 0
	}

	var v inspectFlags
	fs := inspectFlagSet(&v)

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

	isStreamInput := stream.IsURL(inputPath)
	if !isStreamInput {
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
	}

	var (
		table ts.StreamTable
		stats InspectStats
	)
	if isStreamInput {
		open := c.OpenSource
		if open == nil {
			open = stream.Open
		}
		streamFn := c.InspectStream
		if streamFn == nil {
			streamFn = defaultInspectStream
		}
		ctx, _, finalize := stream.Spawn(context.Background(), v.streamFlags.stopOptions())
		src, err := open(ctx, inputPath, v.streamFlags.streamOptions())
		if err != nil {
			c.writeError(c.Err, err)
			_ = finalize()
			return exitCodeForError(err)
		}
		t, s, err := streamFn(ctx, src)
		_ = src.Close()
		summary := finalize()
		if c.Err != nil {
			fmt.Fprintln(c.Err, summary.String()) //nolint:errcheck
		}
		if err != nil {
			c.writeError(c.Err, err)
			return exitCodeForError(err)
		}
		table, stats = t, s
	} else {
		inspect := c.Inspect
		if inspect == nil {
			inspect = defaultInspect
		}
		t, s, err := inspect(inputPath)
		if err != nil {
			c.writeError(c.Err, err)
			return exitCodeForError(err)
		}
		table, stats = t, s
	}

	c.writeReport(table, stats, usePrettyView(viewMode, c.outputTTY(c.Out)))
	return 0
}

func (c *InspectCommand) writeReport(table ts.StreamTable, stats InspectStats, pretty bool) {
	w := c.Out
	if w == nil {
		return
	}
	color := newColorizer(pretty && supportsANSI())

	_, _ = fmt.Fprintf(w, "%stotal packets: %d\n", labelPrefix(color, pretty, "Transport "), stats.TotalPackets)
	_, _ = fmt.Fprintln(w)

	programNums := make([]int, 0, len(table.Programs))
	for pn := range table.Programs {
		programNums = append(programNums, int(pn))
	}
	sort.Ints(programNums)

	for _, pn := range programNums {
		streams := table.Programs[uint16(pn)]
		_, _ = fmt.Fprintf(w, "%s%d:\n", labelPrefix(color, pretty, "program "), pn)

		for _, stream := range streams {
			typeName := streamTypeName(stream.StreamType)
			count := stats.PacketCounts[stream.PID]
			streamLabel := typeName
			if pretty && isLikelyMetadataStream(stream.StreamType) {
				streamLabel = "Likely metadata: " + typeName
			}
			_, _ = fmt.Fprintf(w, "  PID 0x%04X  type=0x%02X (%s)  packets=%d",
				stream.PID, stream.StreamType, streamLabel, count)

			if pesCount, ok := stats.PESUnitCounts[stream.PID]; ok && pesCount > 0 {
				_, _ = fmt.Fprintf(w, "  PES units=%d", pesCount)
			}
			if firstPTS, ok := stats.FirstPTS[stream.PID]; ok {
				_, _ = fmt.Fprintf(w, "  PTS=[%d (%s)", firstPTS, formatPTS(firstPTS))
				if lastPTS, ok2 := stats.LastPTS[stream.PID]; ok2 {
					_, _ = fmt.Fprintf(w, "..%d (%s)", lastPTS, formatPTS(lastPTS))
				}
				_, _ = fmt.Fprint(w, "]")
			}
			_, _ = fmt.Fprintln(w)
		}
	}

	if len(stats.Diagnostics) > 0 {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintf(w, "%s%d\n", labelPrefix(color, pretty, "diagnostics: "), len(stats.Diagnostics))
		for _, d := range stats.Diagnostics {
			_, _ = fmt.Fprintf(w, "  [%s] %s: %s\n", colorSeverity(color, d.Severity), d.Code, d.Message)
		}
	}
	if pretty {
		writeHintFooters(w, color, inspectHints(table))
	}
}

const (
	ptsClockHz          = 90000             // MPEG-TS PTS clock frequency
	ticksPerMillisecond = ptsClockHz / 1000 // ticks in one millisecond
)

// formatPTS converts 90kHz PTS ticks to HH:MM:SS.mmm format.
func formatPTS(ticks int64) string {
	totalSeconds := ticks / ptsClockHz
	millis := (ticks % ptsClockHz) / ticksPerMillisecond
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, millis)
}

func streamTypeName(st uint8) string {
	switch st {
	case 0x01:
		return "MPEG-1 Video"
	case 0x02:
		return "MPEG-2 Video"
	case 0x03:
		return "MPEG-1 Audio"
	case 0x04:
		return "MPEG-2 Audio"
	case 0x06:
		return "Private Data"
	case 0x0F:
		return "AAC Audio"
	case 0x15:
		return "Metadata PES"
	case 0x1B:
		return "H.264 Video"
	case 0x24:
		return "H.265 Video"
	default:
		if st >= 0xC0 {
			return "User Private"
		}
		return "Unknown"
	}
}

func (c *InspectCommand) writeUsage(w io.Writer) {
	commanddef.RenderHelp(inspectDef, inspectFlagSet(&inspectFlags{}), w)
}

// Definition returns the CommandDef driving --help and man-page generation.
func (c *InspectCommand) Definition() commanddef.CommandDef { return inspectDef }

var inspectDef = commanddef.CommandDef{
	Name:       "klvtool-inspect",
	Subcommand: "inspect",
	Synopsis:   "Inspect an MPEG-TS stream inventory, packet counts, and continuity diagnostics.",
	UsageLine:  "klvtool inspect --input <path-or-url> [--view auto|pretty|raw] [--duration <dur>] [--idle-timeout <dur>] [--max-packets <N>] [--record <path>] [--header \"K: V\"] [--iface <name>]",
	Description: "Inspect an MPEG-TS file or live stream and report its program / stream inventory, per-PID packet counts, PES PTS bounds, and transport-layer continuity diagnostics.\n" +
		"\n" +
		"This is the entry point when triaging an unknown file: it surfaces likely metadata PIDs (KLV / data streams) so a follow-up `klvtool decode --pid <PID>` can target the right stream without scanning everything.\n" +
		"\n" +
		"--input also accepts URLs (udp://, tcp://, http(s)://, rtsp://, srt://, or '-' for stdin). For live sources, inspect snapshots the PMT and per-PID counts over the run window — bound the window with --duration or stop with Ctrl-C.",
	Examples: []commanddef.Example{
		{
			Comment: "List streams and find likely KLV metadata PIDs",
			Command: "klvtool inspect --input mission.ts",
		},
		{
			Comment: "Snapshot a live UDP multicast feed for 10 seconds",
			Command: "klvtool inspect --input \"udp://239.0.0.1:5000?iface=eth0\" --duration 10s",
		},
	},
	ExitCodes: []commanddef.ExitCode{
		{Code: 0, Meaning: "success"},
		{Code: 1, Meaning: "transport read failure (file unreadable, malformed, or truncated beyond recovery)"},
		{Code: 2, Meaning: "invalid usage"},
	},
	EnvVars: []commanddef.EnvVar{
		{Name: "NO_COLOR", Description: "disable ANSI color in pretty output"},
	},
	RequiredTools: []string{"ffmpeg", "ffprobe"},
	SeeAlso: []commanddef.SeeAlsoRef{
		{Name: "klvtool", Section: 1},
		{Name: "klvtool-decode", Section: 1},
		{Name: "klvtool-diagnose", Section: 1},
	},
	ExternalRefs: []commanddef.ExternalRef{
		{Title: "ITU-T H.222.0 / ISO/IEC 13818-1 — MPEG-TS systems layer", URL: "https://www.itu.int/rec/T-REC-H.222.0"},
	},
}

func (c *InspectCommand) writeError(w io.Writer, err error) {
	if w == nil || err == nil {
		return
	}
	_, _ = fmt.Fprintln(w, errorLine(newColorizer(c.outputTTY(w) && supportsANSI()), err))
}

func (c *InspectCommand) outputTTY(w io.Writer) bool {
	if c != nil && c.isOutputTTY != nil {
		return c.isOutputTTY(w)
	}
	return isTTYWriter(w)
}

func labelPrefix(color colorizer, pretty bool, label string) string {
	if !pretty {
		return label
	}
	return color.bold(label)
}

func isLikelyMetadataStream(streamType uint8) bool {
	return streamType == 0x06 || streamType == 0x15
}

func inspectHints(table ts.StreamTable) []hintFooter {
	// Iterate programs in sorted order for deterministic output.
	programNums := make([]int, 0, len(table.Programs))
	for pn := range table.Programs {
		programNums = append(programNums, int(pn))
	}
	sort.Ints(programNums)

	for _, pn := range programNums {
		streams := table.Programs[uint16(pn)]
		for _, stream := range streams {
			if isLikelyMetadataStream(stream.StreamType) {
				return []hintFooter{
					{
						Title: "Decode the likely metadata stream",
						Body:  fmt.Sprintf("klvtool decode --input <file.ts> --pid %d", stream.PID),
					},
					{
						Title: "Capture raw payload checkpoints",
						Body:  "klvtool extract --input <file.ts> --out ./klvtool-raw",
					},
				}
			}
		}
	}
	return []hintFooter{
		{
			Title: "Try a full decode if metadata streams were not obvious",
			Body:  "klvtool decode --input <file.ts>",
		},
	}
}

func defaultInspect(path string) (ts.StreamTable, InspectStats, error) {
	file, err := os.Open(path)
	if err != nil {
		return ts.StreamTable{}, InspectStats{}, model.TSRead(fmt.Errorf("open %q: %w", path, err))
	}
	defer func() { _ = file.Close() }()

	table, discoveryDiags, err := ts.DiscoverStreams(file)
	if err != nil {
		return ts.StreamTable{}, InspectStats{}, err
	}

	dataPIDs := make(map[uint16]bool)
	for _, streams := range table.Programs {
		for _, s := range streams {
			if s.StreamType == 0x06 || s.StreamType == 0x15 || s.StreamType >= 0xC0 {
				dataPIDs[s.PID] = true
			}
		}
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return ts.StreamTable{}, InspectStats{}, model.TSRead(fmt.Errorf("seek: %w", err))
	}

	scanner := ts.NewPacketScanner(file, ts.ScanConfig{PayloadPIDs: dataPIDs})
	asm := ts.NewPESAssembler()

	stats := InspectStats{
		PacketCounts:  make(map[uint16]int64),
		PESUnitCounts: make(map[uint16]int),
		FirstPTS:      make(map[uint16]int64),
		LastPTS:       make(map[uint16]int64),
	}

	for {
		pkt, err := scanner.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return ts.StreamTable{}, InspectStats{}, err
		}
		stats.TotalPackets++
		stats.PacketCounts[pkt.PID]++
		if unit := asm.Feed(pkt); unit != nil {
			recordPESStats(&stats, unit)
		}
	}

	for _, unit := range asm.Flush() {
		u := unit
		recordPESStats(&stats, &u)
	}

	stats.Diagnostics = append(stats.Diagnostics, discoveryDiags...)
	stats.Diagnostics = append(stats.Diagnostics, scanner.Diagnostics()...)
	return table, stats, nil
}

// defaultInspectStream runs a single-pass MPEG-TS inspection over a
// non-seekable Source. It populates the same StreamTable + InspectStats
// shape as defaultInspect so the report renderer doesn't need to know
// which path it came from. The PMT/PAT snapshot reflects whatever
// arrived during the run window — when used against a long-running
// stream, the table converges quickly (PSI tables are typically
// announced every ~500 ms) and remains current as PMT updates arrive.
func defaultInspectStream(ctx context.Context, src stream.Source) (ts.StreamTable, InspectStats, error) {
	stats := InspectStats{
		PacketCounts:  make(map[uint16]int64),
		PESUnitCounts: make(map[uint16]int),
		FirstPTS:      make(map[uint16]int64),
		LastPTS:       make(map[uint16]int64),
	}
	demux := ts.NewLiveDemux(src)
	err := demux.Run(ctx, false,
		func(pkt ts.Packet) {
			stats.TotalPackets++
			stats.PacketCounts[pkt.PID]++
		},
		func(unit ts.PESUnit) {
			u := unit
			recordPESStats(&stats, &u)
		},
		func(d ts.Diagnostic) {
			stats.Diagnostics = append(stats.Diagnostics, d)
		},
	)
	if err != nil {
		return ts.StreamTable{}, InspectStats{}, err
	}
	return demux.Table(), stats, nil
}

func recordPESStats(stats *InspectStats, unit *ts.PESUnit) {
	stats.PESUnitCounts[unit.PID]++
	if unit.PTS != nil {
		if _, seen := stats.FirstPTS[unit.PID]; !seen {
			stats.FirstPTS[unit.PID] = *unit.PTS
		}
		stats.LastPTS[unit.PID] = *unit.PTS
	}
	stats.Diagnostics = append(stats.Diagnostics, unit.Diagnostics...)
}
