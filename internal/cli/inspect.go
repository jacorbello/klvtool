package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/jacorbello/klvtool/internal/model"
	ts "github.com/jacorbello/klvtool/internal/mpeg/ts"
)

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
}

func NewInspectCommand() *InspectCommand {
	return &InspectCommand{
		Out:     os.Stdout,
		Err:     os.Stderr,
		Inspect: defaultInspect,
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

	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var inputPath string
	fs.StringVar(&inputPath, "input", "", "path to the MPEG-TS input file")

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

	inspect := c.Inspect
	if inspect == nil {
		inspect = defaultInspect
	}

	table, stats, err := inspect(inputPath)
	if err != nil {
		c.writeError(c.Err, err)
		return exitCodeForError(err)
	}

	c.writeReport(table, stats)
	return 0
}

func (c *InspectCommand) writeReport(table ts.StreamTable, stats InspectStats) {
	w := c.Out
	if w == nil {
		return
	}

	_, _ = fmt.Fprintf(w, "total packets: %d\n", stats.TotalPackets)
	_, _ = fmt.Fprintln(w)

	programNums := make([]int, 0, len(table.Programs))
	for pn := range table.Programs {
		programNums = append(programNums, int(pn))
	}
	sort.Ints(programNums)

	for _, pn := range programNums {
		streams := table.Programs[uint16(pn)]
		_, _ = fmt.Fprintf(w, "program %d:\n", pn)

		for _, stream := range streams {
			typeName := streamTypeName(stream.StreamType)
			count := stats.PacketCounts[stream.PID]
			_, _ = fmt.Fprintf(w, "  PID 0x%04X  type=0x%02X (%s)  packets=%d",
				stream.PID, stream.StreamType, typeName, count)

			if pesCount, ok := stats.PESUnitCounts[stream.PID]; ok && pesCount > 0 {
				_, _ = fmt.Fprintf(w, "  PES units=%d", pesCount)
			}
			if firstPTS, ok := stats.FirstPTS[stream.PID]; ok {
				_, _ = fmt.Fprintf(w, "  PTS=[%d", firstPTS)
				if lastPTS, ok2 := stats.LastPTS[stream.PID]; ok2 {
					_, _ = fmt.Fprintf(w, "..%d", lastPTS)
				}
				_, _ = fmt.Fprint(w, "]")
			}
			_, _ = fmt.Fprintln(w)
		}
	}

	if len(stats.Diagnostics) > 0 {
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintf(w, "diagnostics: %d\n", len(stats.Diagnostics))
		for _, d := range stats.Diagnostics {
			_, _ = fmt.Fprintf(w, "  [%s] %s: %s\n", d.Severity, d.Code, d.Message)
		}
	}
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
	if w == nil {
		return
	}
	_, _ = fmt.Fprintln(w, "Usage: klvtool inspect --input <file.ts>")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Inspect an MPEG-TS file: stream inventory, packet counts, PES timing, continuity diagnostics.")
}

func (c *InspectCommand) writeError(w io.Writer, err error) {
	if w == nil || err == nil {
		return
	}
	_, _ = fmt.Fprintf(w, "error: %v\n", err)
}

func defaultInspect(path string) (ts.StreamTable, InspectStats, error) {
	file, err := os.Open(path)
	if err != nil {
		return ts.StreamTable{}, InspectStats{}, model.TSRead(fmt.Errorf("open %q: %w", path, err))
	}
	defer func() { _ = file.Close() }()

	table, err := ts.DiscoverStreams(file)
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

	stats.Diagnostics = append(stats.Diagnostics, scanner.Diagnostics()...)
	return table, stats, nil
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
