package ffmpeg

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/jacorbello/klvtool/internal/extract"
	"github.com/jacorbello/klvtool/internal/model"
)

var versionPattern = regexp.MustCompile(`(?m)^ff(?:probe|mpeg) version (\S+)`)

// Command describes an external command invocation without executing it.
type Command struct {
	Path string
	Args []string
}

// Runner executes an external command and returns its combined output.
type Runner func(ctx context.Context, path string, args ...string) ([]byte, error)

// Backend adapts ffmpeg/ffprobe into the normalized extraction interface.
type Backend struct {
	Run Runner
}

func NewBackend() *Backend {
	return &Backend{Run: defaultRunner}
}

func (b *Backend) Descriptor() extract.BackendDescriptor {
	return extract.BackendDescriptor{
		Name:    extract.BackendFFmpeg,
		Healthy: true,
		Tools:   []string{"ffmpeg", "ffprobe"},
	}
}

func (b *Backend) Version(ctx context.Context) (string, error) {
	output, err := b.runner()(ctx, "ffmpeg", "-version")
	if err != nil {
		return "", model.BackendExecution(fmt.Errorf("run ffmpeg -version: %w", err))
	}
	return ParseVersion(string(output)), nil
}

func (b *Backend) Extract(ctx context.Context, inputPath string) ([]extract.PayloadRecord, error) {
	if strings.TrimSpace(inputPath) == "" {
		return nil, model.InvalidUsage(fmt.Errorf("input path is required"))
	}

	probeOutput, err := b.runner()(ctx, BuildProbeCommand(inputPath).Path, BuildProbeCommand(inputPath).Args...)
	if err != nil {
		return nil, model.BackendExecution(fmt.Errorf("probe input with ffprobe: %w", err))
	}

	streams, err := parseProbeStreams(probeOutput)
	if err != nil {
		return nil, err
	}
	if len(streams) == 0 {
		return []extract.PayloadRecord{}, nil
	}

	tmpDir, err := os.MkdirTemp("", "klvtool-ffmpeg-*")
	if err != nil {
		return nil, model.OutputWrite(fmt.Errorf("create ffmpeg temp dir: %w", err))
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	records := make([]extract.PayloadRecord, 0, len(streams))
	for i, stream := range streams {
		outPath := filepath.Join(tmpDir, fmt.Sprintf("klv-%03d.bin", i+1))
		cmd := BuildExtractCommand(inputPath, outPath, stream.Index)
		if _, err := b.runner()(ctx, cmd.Path, cmd.Args...); err != nil {
			return nil, model.BackendExecution(fmt.Errorf("extract data stream %d with ffmpeg: %w", stream.Index, err))
		}

		payload, err := os.ReadFile(outPath)
		if err != nil {
			return nil, model.OutputWrite(fmt.Errorf("read extracted payload %q: %w", outPath, err))
		}

		record := extract.PayloadRecord{
			PID:     stream.PID,
			Payload: payload,
		}
		if stream.Warning != "" {
			record.Warnings = append(record.Warnings, normalizeWarning(stream.Warning))
		}
		records = append(records, record)
	}

	return records, nil
}

func (b *Backend) runner() Runner {
	if b != nil && b.Run != nil {
		return b.Run
	}
	return defaultRunner
}

// ParseVersion normalizes an ffmpeg or ffprobe version banner.
func ParseVersion(output string) string {
	matches := versionPattern.FindStringSubmatch(strings.TrimSpace(output))
	if len(matches) < 2 {
		return strings.TrimSpace(output)
	}
	return matches[1]
}

// BuildProbeCommand returns the ffprobe command used to discover data streams.
func BuildProbeCommand(inputPath string) Command {
	return Command{
		Path: "ffprobe",
		Args: []string{
			"-v", "error",
			"-select_streams", "d",
			"-show_entries", "stream=index,id",
			"-of", "json",
			inputPath,
		},
	}
}

// BuildExtractCommand returns the ffmpeg command used to extract one data stream.
func BuildExtractCommand(inputPath, outputPath string, streamIndex int) Command {
	return Command{
		Path: "ffmpeg",
		Args: []string{
			"-hide_banner",
			"-nostdin",
			"-loglevel", "error",
			"-y",
			"-i", inputPath,
			"-map", fmt.Sprintf("0:%d", streamIndex),
			"-c", "copy",
			"-f", "data",
			outputPath,
		},
	}
}

type probeResponse struct {
	Streams []probeStream `json:"streams"`
}

type probeStream struct {
	Index int    `json:"index"`
	ID    string `json:"id"`
}

type normalizedStream struct {
	Index   int
	PID     uint16
	Warning string
}

func parseProbeStreams(data []byte) ([]normalizedStream, error) {
	var response probeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, model.BackendParse(fmt.Errorf("parse ffprobe json: %w", err))
	}

	streams := make([]normalizedStream, 0, len(response.Streams))
	for _, stream := range response.Streams {
		pid, warning := parsePID(stream.ID)
		streams = append(streams, normalizedStream{
			Index:   stream.Index,
			PID:     pid,
			Warning: warning,
		})
	}
	return streams, nil
}

func parsePID(raw string) (uint16, string) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, "pid unavailable from backend metadata"
	}

	base := 10
	if strings.HasPrefix(strings.ToLower(value), "0x") {
		base = 16
		value = value[2:]
	}

	parsed, err := strconv.ParseUint(value, base, 16)
	if err != nil {
		return 0, fmt.Sprintf("pid unavailable from backend metadata: %q", raw)
	}
	return uint16(parsed), ""
}

func normalizeWarning(warning string) string {
	if strings.TrimSpace(warning) == "" {
		return ""
	}
	return "pid unavailable from backend metadata"
}

func defaultRunner(ctx context.Context, path string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return output, nil
}
