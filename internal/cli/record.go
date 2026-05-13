package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jacorbello/klvtool/internal/cli/commanddef"
	"github.com/jacorbello/klvtool/internal/model"
	"github.com/jacorbello/klvtool/internal/stream"
)

// recordFlags holds parsed --flag values for the record subcommand.
type recordFlags struct {
	inputPath string
	outPath   string
	streamFlags
}

func recordFlagSet(v *recordFlags) *flag.FlagSet {
	fs := flag.NewFlagSet("record", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&v.inputPath, "input", "", "input source path or URL (file://, udp://, tcp://, http(s)://, rtsp://, srt://, or '-' for stdin)")
	fs.StringVar(&v.outPath, "out", "", "destination file for captured bytes (required)")
	registerStreamFlags(fs, &v.streamFlags, streamFlagsRecord)
	return fs
}

// RecordCommand captures inbound source bytes to a file with no decoding.
// It's the simplest streaming command: a single io.Copy with the shared
// lifecycle gating duration, idle-timeout, and max-bytes.
type RecordCommand struct {
	Out io.Writer
	Err io.Writer
	// openSource is overridable for tests; defaults to stream.Open.
	openSource streamSourceOpener
}

// NewRecordCommand returns a RecordCommand with default runtime
// dependencies.
func NewRecordCommand() *RecordCommand {
	return &RecordCommand{
		Out: os.Stdout,
		Err: os.Stderr,
	}
}

// Definition returns the CommandDef driving --help and man-page generation.
func (c *RecordCommand) Definition() commanddef.CommandDef { return recordDef }

func (c *RecordCommand) writeUsage(w io.Writer) {
	commanddef.RenderHelp(recordDef, recordFlagSet(&recordFlags{}), w)
}

func (c *RecordCommand) writeError(w io.Writer, err error) {
	if w == nil || err == nil {
		return
	}
	fmt.Fprintln(w, errorLine(newColorizer(isTTYWriter(w) && supportsANSI()), err)) //nolint:errcheck
}

// Execute parses args and captures bytes from --input into --out until a
// stop condition fires (SIGINT, --duration, --idle-timeout, --max-bytes,
// or EOF).
func (c *RecordCommand) Execute(args []string) int {
	if c == nil {
		return 1
	}
	if len(args) == 1 && isHelpArg(args[0]) {
		c.writeUsage(c.Out)
		return 0
	}

	var v recordFlags
	fs := recordFlagSet(&v)
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
	if strings.TrimSpace(v.inputPath) == "" {
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(errors.New("--input is required")))
		return usageExitCode
	}
	if strings.TrimSpace(v.outPath) == "" {
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(errors.New("--out is required")))
		return usageExitCode
	}

	open := c.openSource
	if open == nil {
		open = stream.Open
	}

	ctx, counters, finalize := stream.Spawn(context.Background(), v.stopOptions())
	defer func() {
		summary := finalize()
		if c.Err != nil {
			fmt.Fprintln(c.Err, summary.String()) //nolint:errcheck
		}
	}()

	src, err := open(ctx, v.inputPath, v.streamOptions())
	if err != nil {
		c.writeError(c.Err, err)
		return exitCodeForError(err)
	}
	defer func() { _ = src.Close() }()

	flags := os.O_CREATE | os.O_WRONLY | os.O_EXCL
	if v.recordOverwrite {
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}
	dst, err := os.OpenFile(v.outPath, flags, 0o644)
	if err != nil {
		c.writeError(c.Err, model.OutputWrite(fmt.Errorf("open --out %q: %w", v.outPath, err)))
		return 1
	}
	defer func() { _ = dst.Close() }()

	// recordWriter wraps the destination so byte writes update both the
	// lifecycle's byte counter (for --max-bytes) AND the idle watchdog
	// (without separately calling MarkActivity per read).
	rw := &recordWriter{dst: dst, counters: counters}
	if _, copyErr := io.Copy(rw, src); copyErr != nil {
		// Cancellation closes the source mid-Read, which surfaces as an
		// io error. When the context has already been cancelled by the
		// lifecycle (signal, duration, idle, max-bytes), treat that as
		// a clean stop rather than a failure.
		if ctx.Err() == nil {
			c.writeError(c.Err, model.StreamInterrupted(copyErr))
			return exitCodeForError(model.StreamInterrupted(copyErr))
		}
	}
	if err := dst.Close(); err != nil {
		c.writeError(c.Err, model.OutputWrite(err))
		return 1
	}
	return 0
}

// recordWriter is a thin io.Writer that updates the lifecycle counters
// on every successful write. Wrapping the destination (rather than
// teeing on the source side) means a slow disk doesn't disable
// --max-bytes early.
type recordWriter struct {
	dst      io.Writer
	counters *stream.Counters
}

func (rw *recordWriter) Write(p []byte) (int, error) {
	n, err := rw.dst.Write(p)
	if n > 0 && rw.counters != nil {
		rw.counters.AddBytes(int64(n))
		// AddBytes also marks activity, so idle-timeout sees any
		// useful work (not just heartbeat reads with zero useful
		// bytes).
	}
	return n, err
}

// recordDef captures the documentation for `klvtool record`.
var recordDef = commanddef.CommandDef{
	Name:       "klvtool-record",
	Subcommand: "record",
	Synopsis:   "Capture inbound source bytes to a file with no decoding.",
	UsageLine:  "klvtool record --input <path-or-url> --out <path> [--duration <dur>] [--idle-timeout <dur>] [--max-bytes <N>] [--record-overwrite] [--header \"K: V\"] [--iface <name>]",
	Description: "Capture bytes from a path or URL source to a regular file with no MPEG-TS demuxing, PSI parsing, or KLV decoding involved.\n" +
		"\n" +
		"`record` is the streaming counterpart to letting another tool dump a file: it understands udp://, tcp://, http(s)://, rtsp://, srt://, and '-' (stdin) the same way the other streaming-aware commands do, and it accepts the shared `--duration`, `--idle-timeout`, and `--max-bytes` stop conditions. The captured file can be replayed later with the file-based commands.\n" +
		"\n" +
		"Use `--input file.ts` to copy a regular file (useful for normalization after sync-byte recovery).",
	Examples: []commanddef.Example{
		{
			Comment: "Capture 30 seconds of a multicast UDP feed",
			Command: "klvtool record --input \"udp://239.0.0.1:5000?iface=eth0\" --out cap.ts --duration 30s",
		},
		{
			Comment: "Capture from a Bearer-token authenticated HTTPS source",
			Command: "klvtool record --input https://relay/stream --out cap.ts --header \"Authorization: Bearer $TOKEN\" --duration 1m",
		},
		{
			Comment: "Pipe ffmpeg's MPEG-TS output through klvtool for byte capture",
			Command: "ffmpeg -i source.flv -c copy -f mpegts - | klvtool record --input - --out cap.ts --max-bytes 104857600",
		},
	},
	ExitCodes: []commanddef.ExitCode{
		{Code: 0, Meaning: "clean stop — captured bytes through duration / idle / max-bytes / SIGINT / EOF"},
		{Code: 1, Meaning: "source open failure, destination write failure, or stream interrupted mid-run"},
		{Code: 2, Meaning: "invalid usage (missing flags, malformed URL)"},
	},
	EnvVars: []commanddef.EnvVar{
		{Name: "NO_COLOR", Description: "disable ANSI color in any error output"},
	},
	SeeAlso: []commanddef.SeeAlsoRef{
		{Name: "klvtool", Section: 1},
		{Name: "klvtool-decode", Section: 1},
		{Name: "klvtool-inspect", Section: 1},
	},
}
