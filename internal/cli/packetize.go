package cli

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jacorbello/klvtool/internal/extract"
	"github.com/jacorbello/klvtool/internal/model"
	"github.com/jacorbello/klvtool/internal/output"
	"github.com/jacorbello/klvtool/internal/packetize"
)

type packetParser interface {
	Parse(packetize.Request) (packetize.PacketizedStream, error)
}

type packetManifestWriter interface {
	WriteManifest(model.PacketManifest) error
}

// PacketizeCommand wires CLI flag parsing to raw checkpoint replay and packet output.
type PacketizeCommand struct {
	Out io.Writer
	Err io.Writer

	isOutputTTY func(io.Writer) bool

	ReadRawPayloads func(string) ([]extract.RawPayloadRecord, error)
	Parser          packetParser
	NewManifestOut  func(io.Writer) packetManifestWriter
	OpenManifest    func(string) (io.WriteCloser, error)
}

func NewPacketizeCommand() *PacketizeCommand {
	return &PacketizeCommand{
		Out:             os.Stdout,
		Err:             os.Stderr,
		ReadRawPayloads: output.ReadRawPayloadManifest,
		Parser:          packetize.NewParser(),
		NewManifestOut: func(w io.Writer) packetManifestWriter {
			return output.NewPacketManifestWriter(w)
		},
	}
}

func (c *PacketizeCommand) Execute(args []string) int {
	if c == nil {
		return 1
	}
	if len(args) == 1 && isHelpArg(args[0]) {
		c.writeUsage(c.Out)
		return 0
	}

	fs := flag.NewFlagSet("packetize", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var inputDir string
	var outDir string
	var mode string
	var view string

	fs.StringVar(&inputDir, "input", "", "directory containing raw payload checkpoint output")
	fs.StringVar(&outDir, "out", "", "directory for packet checkpoint output")
	fs.StringVar(&mode, "mode", string(packetize.ModeStrict), "parser mode: strict or best-effort")
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
	if strings.TrimSpace(inputDir) == "" {
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(fmt.Errorf("input directory is required")))
		return usageExitCode
	}
	if strings.TrimSpace(outDir) == "" {
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(fmt.Errorf("output directory is required")))
		return usageExitCode
	}
	sameDir, err := sameDirectory(inputDir, outDir)
	if err != nil {
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(err))
		return usageExitCode
	}
	if sameDir {
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(fmt.Errorf("input and output directories must be different")))
		return usageExitCode
	}

	info, err := os.Stat(inputDir)
	if err != nil {
		var e error
		if os.IsNotExist(err) {
			e = model.CheckpointRead(fmt.Errorf("input directory does not exist: %s", inputDir))
		} else {
			e = model.CheckpointRead(fmt.Errorf("cannot access input directory %q: %w", inputDir, err))
		}
		c.writeError(c.Err, e)
		return exitCodeForError(e)
	}
	if !info.IsDir() {
		e := model.CheckpointRead(fmt.Errorf("input path is not a directory: %s", inputDir))
		c.writeError(c.Err, e)
		return exitCodeForError(e)
	}

	packetMode := packetize.Mode(mode)
	if packetMode != packetize.ModeStrict && packetMode != packetize.ModeBestEffort {
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(fmt.Errorf("unsupported packetization mode %q", mode)))
		return usageExitCode
	}
	viewMode, err := parseViewMode(view)
	if err != nil {
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(err))
		return usageExitCode
	}
	prettyView := usePrettyView(viewMode, c.outputTTY(c.Out))
	color := newColorizer(prettyView && supportsANSI())

	if dirNonEmpty(outDir) && c.Err != nil {
		_, _ = fmt.Fprintln(c.Err, warningLine(color, "output directory already exists, files will be overwritten: %s", outDir))
	}

	records, err := c.rawPayloadReader()(inputDir)
	if err != nil {
		c.writeError(c.Err, err)
		return exitCodeForError(err)
	}

	streams := make([]packetize.PacketizedStream, 0, len(records))
	for _, record := range records {
		result, err := c.parser().Parse(packetize.Request{Mode: packetMode, Record: record})
		if err != nil {
			c.writeError(c.Err, err)
			return exitCodeForError(err)
		}
		streams = append(streams, result)
	}

	if err := c.writeOutputs(outDir, inputDir, streams); err != nil {
		c.writeError(c.Err, err)
		return exitCodeForError(err)
	}

	if c.Out != nil {
		_, _ = fmt.Fprintf(c.Out, "records: %d\n", len(streams))
		_, _ = fmt.Fprintf(c.Out, "manifest: %s\n", filepath.Join(outDir, "manifest.ndjson"))
		if prettyView {
			writeHintFooters(c.Out, color, []hintFooter{
				{
					Title: "Decode against the original transport stream after packet inspection",
					Body:  "klvtool decode --input sample.ts",
				},
			})
		}
	}
	return 0
}

func (c *PacketizeCommand) writeOutputs(outDir, sourcePath string, streams []packetize.PacketizedStream) error {
	var manifest model.PacketManifest

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return model.OutputWrite(fmt.Errorf("create packet output directory %q: %w", outDir, err))
	}

	packetDir := filepath.Join(outDir, "packets")
	if err := os.MkdirAll(packetDir, 0o755); err != nil {
		return model.OutputWrite(fmt.Errorf("create packet checkpoint directory %q: %w", packetDir, err))
	}

	manifest.SchemaVersion = model.PacketSchemaVersion
	manifest.SourcePath = sourcePath
	manifest.Records = make([]model.PacketManifestEntry, 0, len(streams))

	for _, stream := range streams {
		packetPath, err := writePacketCheckpoint(packetDir, stream)
		if err != nil {
			return err
		}
		manifest.Records = append(manifest.Records, model.PacketManifestEntry{
			RecordID:      stream.Source.RecordID,
			Mode:          string(stream.Mode),
			ParserVersion: stream.ParserVersion,
			ParsedCount:   stream.ParsedCount,
			WarningCount:  stream.WarningCount,
			ErrorCount:    stream.ErrorCount,
			Recovered:     stream.Recovered,
			PacketPath:    filepath.ToSlash(filepath.Join("packets", filepath.Base(packetPath))),
			Diagnostics:   toPacketDiagnostics(stream.Diagnostics),
		})
	}

	manifestPath := filepath.Join(outDir, "manifest.ndjson")
	open := c.openManifest()
	file, err := open(manifestPath)
	if err != nil {
		return model.OutputWrite(fmt.Errorf("create packet manifest file %q: %w", manifestPath, err))
	}

	writer := c.packetManifestWriter(file)
	if err := writer.WriteManifest(manifest); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return model.OutputWrite(err)
	}
	return nil
}

func (c *PacketizeCommand) writeUsage(w io.Writer) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprintln(w, "Usage: klvtool packetize --input <raw-checkpoint-dir> --out <dir> [--mode strict|best-effort]")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Replay raw extraction checkpoints, parse KLV packets, and write packet checkpoint output.")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Use this after extract when you need KLV framing diagnostics or packet checkpoint artifacts.")
}

func (c *PacketizeCommand) writeError(w io.Writer, err error) {
	if w == nil || err == nil {
		return
	}
	_, _ = fmt.Fprintln(w, errorLine(newColorizer(c.outputTTY(w) && supportsANSI()), err))
}

func (c *PacketizeCommand) outputTTY(w io.Writer) bool {
	if c != nil && c.isOutputTTY != nil {
		return c.isOutputTTY(w)
	}
	return isTTYWriter(w)
}

func (c *PacketizeCommand) rawPayloadReader() func(string) ([]extract.RawPayloadRecord, error) {
	if c != nil && c.ReadRawPayloads != nil {
		return c.ReadRawPayloads
	}
	return output.ReadRawPayloadManifest
}

func (c *PacketizeCommand) parser() packetParser {
	if c != nil && c.Parser != nil {
		return c.Parser
	}
	return packetize.NewParser()
}

func (c *PacketizeCommand) openManifest() func(string) (io.WriteCloser, error) {
	if c != nil && c.OpenManifest != nil {
		return c.OpenManifest
	}
	return func(path string) (io.WriteCloser, error) {
		return os.Create(path)
	}
}

func (c *PacketizeCommand) packetManifestWriter(w io.Writer) packetManifestWriter {
	if c != nil && c.NewManifestOut != nil {
		return c.NewManifestOut(w)
	}
	return output.NewPacketManifestWriter(w)
}

func writePacketCheckpoint(dir string, stream packetize.PacketizedStream) (string, error) {
	filename, err := packetCheckpointFilename(stream.Source.RecordID)
	if err != nil {
		return "", err
	}

	path := filepath.Join(dir, filename)
	data, err := json.Marshal(toPacketCheckpoint(stream))
	if err != nil {
		return "", model.OutputWrite(fmt.Errorf("marshal packet checkpoint for %q: %w", stream.Source.RecordID, err))
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", model.OutputWrite(fmt.Errorf("write packet checkpoint %q: %w", path, err))
	}
	return path, nil
}

func packetCheckpointFilename(recordID string) (string, error) {
	var b strings.Builder
	for _, r := range recordID {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}

	name := strings.Trim(b.String(), "._-")
	if name == "" {
		return "", model.OutputWrite(fmt.Errorf("record id is required"))
	}
	return name + ".json", nil
}

func toPacketDiagnostics(diags []packetize.Diagnostic) []model.PacketDiagnostic {
	if len(diags) == 0 {
		return []model.PacketDiagnostic{}
	}
	out := make([]model.PacketDiagnostic, 0, len(diags))
	for _, diag := range diags {
		out = append(out, model.PacketDiagnostic{
			Severity:    diag.Severity,
			Code:        diag.Code,
			Message:     diag.Message,
			Stage:       diag.Stage,
			PacketIndex: diag.PacketIndex,
			ByteOffset:  diag.ByteOffset,
		})
	}
	return out
}

func toPacketCheckpoint(stream packetize.PacketizedStream) model.PacketCheckpoint {
	checkpoint := model.PacketCheckpoint{
		SchemaVersion: model.PacketSchemaVersion,
		RecordID:      stream.Source.RecordID,
		Mode:          string(stream.Mode),
		ParserVersion: stream.ParserVersion,
		ParsedCount:   stream.ParsedCount,
		WarningCount:  stream.WarningCount,
		ErrorCount:    stream.ErrorCount,
		Recovered:     stream.Recovered,
		Packets:       make([]model.PacketRecord, 0, len(stream.Packets)),
		Diagnostics:   toPacketDiagnostics(stream.Diagnostics),
	}
	for _, packet := range stream.Packets {
		checkpoint.Packets = append(checkpoint.Packets, model.PacketRecord{
			PacketIndex:        packet.PacketIndex,
			PacketStart:        packet.PacketStart,
			KeyStart:           packet.KeyStart,
			LengthStart:        packet.LengthStart,
			ValueStart:         packet.ValueStart,
			PacketEndInclusive: packet.PacketEndExclusive - 1,
			RawKeyHex:          hex.EncodeToString(packet.Key),
			Length:             packet.Length,
			RawValueHex:        hex.EncodeToString(packet.Value),
			Classification:     string(packet.Classification),
			Diagnostics:        toPacketDiagnostics(packet.Diagnostics),
		})
	}
	return checkpoint
}

func sameDirectory(inputDir, outDir string) (bool, error) {
	inputPath, err := canonicalDirPath(inputDir)
	if err != nil {
		return false, fmt.Errorf("resolve input directory %q: %w", inputDir, err)
	}
	outPath, err := canonicalDirPath(outDir)
	if err != nil {
		return false, fmt.Errorf("resolve output directory %q: %w", outDir, err)
	}
	return inputPath == outPath, nil
}

func canonicalDirPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	cleanPath := filepath.Clean(absPath)
	if resolved, err := filepath.EvalSymlinks(cleanPath); err == nil {
		return resolved, nil
	}
	return cleanPath, nil
}
