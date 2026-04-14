package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	ffmpegbackend "github.com/jacorbello/klvtool/internal/backends/ffmpeg"
	"github.com/jacorbello/klvtool/internal/envcheck"
	"github.com/jacorbello/klvtool/internal/extract"
	"github.com/jacorbello/klvtool/internal/model"
	"github.com/jacorbello/klvtool/internal/output"
)

type extractRunner interface {
	Run(context.Context, extract.RunRequest) (extract.RunResult, error)
}

type payloadWriterFunc func(dir, recordID string, payload []byte) (output.PayloadResult, error)

type manifestWriter interface {
	WriteManifest(model.Manifest) error
}

// ExtractCommand wires CLI flag parsing to the extraction orchestration layer.
type ExtractCommand struct {
	Out io.Writer
	Err io.Writer

	GOOS string
	Env  map[string]string

	Detect         func(context.Context, string, map[string]string) envcheck.Report
	Extractor      extractRunner
	WritePayload   payloadWriterFunc
	NewManifestOut func(io.Writer) manifestWriter
}

func NewExtractCommand() *ExtractCommand {
	return &ExtractCommand{
		Out:            os.Stdout,
		Err:            os.Stderr,
		GOOS:           runtime.GOOS,
		Env:            currentEnvMap(),
		Detect:         defaultDoctorDetect,
		Extractor:      extract.NewExtractor(ffmpegbackend.NewBackend()),
		WritePayload:   output.WritePayload,
		NewManifestOut: func(w io.Writer) manifestWriter { return output.NewManifestWriter(w) },
	}
}

func (c *ExtractCommand) Execute(args []string) int {
	if c == nil {
		return 1
	}
	if len(args) == 1 && isHelpArg(args[0]) {
		c.writeUsage(c.Out)
		return 0
	}

	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var inputPath string
	var outDir string

	fs.StringVar(&inputPath, "input", "", "path to the MPEG-TS input file")
	fs.StringVar(&outDir, "out", "", "directory for extracted payloads and manifest")

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
	if strings.TrimSpace(outDir) == "" {
		c.writeUsage(c.Err)
		c.writeError(c.Err, model.InvalidUsage(fmt.Errorf("output directory is required")))
		return usageExitCode
	}

	if _, err := os.Stat(filepath.Join(outDir, "manifest.ndjson")); err == nil && c.Err != nil {
		_, _ = fmt.Fprintf(c.Err, "warning: output directory already exists, files will be overwritten: %s\n", outDir)
	}

	report := c.detect()
	desc := ffmpegDescriptor(report)
	if !desc.Healthy {
		err := model.MissingDependency(fmt.Errorf("ffmpeg backend is unavailable"))
		c.writeError(c.Err, err)
		return exitCodeForError(err)
	}
	result, err := c.extractor().Run(context.Background(), extract.RunRequest{
		InputPath: inputPath,
		Backend:   desc,
	})
	if err != nil {
		c.writeError(c.Err, err)
		return exitCodeForError(err)
	}

	manifest, err := c.writeOutputs(outDir, inputPath, result)
	if err != nil {
		c.writeError(c.Err, err)
		return exitCodeForError(err)
	}

	if c.Out != nil {
		_, _ = fmt.Fprintf(c.Out, "backend: %s (%s)\n", manifest.BackendName, manifest.BackendVersion)
		_, _ = fmt.Fprintf(c.Out, "records: %d\n", len(manifest.Records))
		_, _ = fmt.Fprintf(c.Out, "manifest: %s\n", filepath.Join(outDir, "manifest.ndjson"))
	}
	return 0
}

func (c *ExtractCommand) writeOutputs(outDir, inputPath string, result extract.RunResult) (model.Manifest, error) {
	var manifest model.Manifest

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return manifest, model.OutputWrite(fmt.Errorf("create output directory %q: %w", outDir, err))
	}

	payloadDir := filepath.Join(outDir, "payloads")
	manifest.SchemaVersion = "1"
	manifest.SourceInputPath = inputPath
	manifest.BackendName = result.Backend.Name
	manifest.BackendVersion = result.BackendVersion
	manifest.Records = make([]model.Record, 0, len(result.Records))

	writePayload := c.payloadWriter()
	for _, record := range result.Records {
		payloadResult, err := writePayload(payloadDir, record.RecordID, record.Payload)
		if err != nil {
			return model.Manifest{}, err
		}

		relPath, err := filepath.Rel(outDir, payloadResult.Path)
		if err != nil {
			return model.Manifest{}, model.OutputWrite(fmt.Errorf("compute relative payload path for %q: %w", payloadResult.Path, err))
		}

		manifest.Records = append(manifest.Records, model.Record{
			RecordID:          record.RecordID,
			PID:               record.PID,
			TransportStreamID: record.TransportStreamID,
			PacketOffset:      record.PacketOffset,
			PacketIndex:       record.PacketIndex,
			ContinuityCounter: record.ContinuityCounter,
			PTS:               record.PTS,
			DTS:               record.DTS,
			PayloadPath:       filepath.ToSlash(relPath),
			PayloadSize:       payloadResult.Size,
			PayloadHash:       payloadResult.Hash,
			Warnings:          append([]string(nil), record.Warnings...),
		})
	}

	manifestPath := filepath.Join(outDir, "manifest.ndjson")
	file, err := os.Create(manifestPath)
	if err != nil {
		return model.Manifest{}, model.OutputWrite(fmt.Errorf("create manifest file %q: %w", manifestPath, err))
	}
	defer func() {
		_ = file.Close()
	}()

	writer := c.manifestWriter(file)
	if err := writer.WriteManifest(manifest); err != nil {
		return model.Manifest{}, err
	}
	return manifest, nil
}

func (c *ExtractCommand) detect() envcheck.Report {
	if c != nil && c.Detect != nil {
		return c.Detect(context.Background(), c.goos(), c.env())
	}
	return defaultDoctorDetect(context.Background(), c.goos(), c.env())
}

func (c *ExtractCommand) goos() string {
	if c != nil && c.GOOS != "" {
		return c.GOOS
	}
	return runtime.GOOS
}

func (c *ExtractCommand) env() map[string]string {
	if c != nil && c.Env != nil {
		return c.Env
	}
	return currentEnvMap()
}

func (c *ExtractCommand) extractor() extractRunner {
	if c != nil && c.Extractor != nil {
		return c.Extractor
	}
	return extract.NewExtractor(ffmpegbackend.NewBackend())
}

func (c *ExtractCommand) payloadWriter() payloadWriterFunc {
	if c != nil && c.WritePayload != nil {
		return c.WritePayload
	}
	return output.WritePayload
}

func (c *ExtractCommand) manifestWriter(w io.Writer) manifestWriter {
	if c != nil && c.NewManifestOut != nil {
		return c.NewManifestOut(w)
	}
	return output.NewManifestWriter(w)
}

func (c *ExtractCommand) writeUsage(w io.Writer) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprintln(w, "Usage: klvtool extract --input <file.ts> --out <dir>")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Validate backend availability, extract KLV/data payloads, and write manifest output.")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Required tools:")
	_, _ = fmt.Fprintln(w, "  ffmpeg:  ffmpeg, ffprobe")
}

func (c *ExtractCommand) writeError(w io.Writer, err error) {
	if w == nil || err == nil {
		return
	}
	_, _ = fmt.Fprintf(w, "error: %v\n", err)
}

func ffmpegDescriptor(report envcheck.Report) extract.BackendDescriptor {
	for _, backend := range report.Backends {
		if strings.ToLower(backend.Name) == "ffmpeg" {
			tools := make([]string, 0, len(backend.Tools))
			for _, tool := range backend.Tools {
				tools = append(tools, tool.Name)
			}
			return extract.BackendDescriptor{
				Name:    "ffmpeg",
				Healthy: backend.Healthy,
				Tools:   tools,
			}
		}
	}
	return extract.BackendDescriptor{Name: "ffmpeg"}
}

func exitCodeForError(err error) int {
	var typed *model.Error
	if errorsAs(err, &typed) {
		switch typed.Code {
		case model.CodeInvalidUsage:
			return usageExitCode
		default:
			return 1
		}
	}
	return 1
}

func errorsAs(err error, target **model.Error) bool {
	if err == nil {
		return false
	}
	if typed, ok := err.(*model.Error); ok {
		*target = typed
		return true
	}
	type unwrapper interface {
		Unwrap() error
	}
	u, ok := err.(unwrapper)
	if !ok {
		return false
	}
	return errorsAs(u.Unwrap(), target)
}
