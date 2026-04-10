package gstreamer

import (
	"context"
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

var (
	versionPattern       = regexp.MustCompile(`(?m)^gst-launch-1\.0 version (\S+)`)
	discoverStreamLine   = regexp.MustCompile(`^\s*([[:alpha:]][[:alnum:]_-]*_[0-9A-Fa-f]+):`)
	discoverPIDLine      = regexp.MustCompile(`(?i)\bpid\b[^0-9a-fx]*((?:0x)?[0-9a-fA-F]+)\b`)
	discoverOutputMarker = regexp.MustCompile(`(?m)(^Analyzing\b|^Done discovering\b|^Topology:\b|^Properties:\b)`)
)

// Command describes an external command invocation without executing it.
type Command struct {
	Path string
	Args []string
}

// Runner executes an external command and returns its combined output.
type Runner func(ctx context.Context, path string, args ...string) ([]byte, error)

// Backend adapts GStreamer tooling into the normalized extraction interface.
type Backend struct {
	Run Runner
}

func NewBackend() *Backend {
	return &Backend{Run: defaultRunner}
}

func (b *Backend) Descriptor() extract.BackendDescriptor {
	return extract.BackendDescriptor{
		Name:    extract.BackendGStreamer,
		Healthy: true,
		Tools:   []string{"gst-launch-1.0", "gst-inspect-1.0", "gst-discoverer-1.0"},
	}
}

func (b *Backend) Version(ctx context.Context) (string, error) {
	output, err := b.runner()(ctx, "gst-launch-1.0", "--version")
	if err != nil {
		return "", model.BackendExecution(fmt.Errorf("run gst-launch-1.0 --version: %w", err))
	}
	return ParseVersion(string(output)), nil
}

func (b *Backend) Extract(ctx context.Context, inputPath string) ([]extract.PayloadRecord, error) {
	if strings.TrimSpace(inputPath) == "" {
		return nil, model.InvalidUsage(fmt.Errorf("input path is required"))
	}

	discoverCmd := BuildDiscoverCommand(inputPath)
	discoverOutput, err := b.runner()(ctx, discoverCmd.Path, discoverCmd.Args...)
	if err != nil {
		return nil, model.BackendExecution(fmt.Errorf("discover input with gst-discoverer-1.0: %w", err))
	}

	streams, err := parseDiscoverStreams(discoverOutput)
	if err != nil {
		return nil, err
	}
	if len(streams) == 0 {
		return []extract.PayloadRecord{}, nil
	}

	tmpDir, err := os.MkdirTemp("", "klvtool-gstreamer-*")
	if err != nil {
		return nil, model.OutputWrite(fmt.Errorf("create gstreamer temp dir: %w", err))
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	records := make([]extract.PayloadRecord, 0, len(streams))
	for i, stream := range streams {
		outPath := filepath.Join(tmpDir, fmt.Sprintf("klv-%03d.bin", i+1))
		cmd := BuildExtractCommand(inputPath, outPath, stream.StreamID)
		if _, err := b.runner()(ctx, cmd.Path, cmd.Args...); err != nil {
			return nil, model.BackendExecution(fmt.Errorf("extract data stream %q with gst-launch-1.0: %w", stream.StreamID, err))
		}

		payload, err := os.ReadFile(outPath)
		if err != nil {
			return nil, model.OutputWrite(fmt.Errorf("read extracted payload %q: %w", outPath, err))
		}

		record := extract.PayloadRecord{
			PID:     stream.PID,
			Payload: payload,
		}
		if warning := normalizeWarning(stream.Warning); warning != "" {
			record.Warnings = append(record.Warnings, warning)
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

// ParseVersion normalizes the gst-launch banner into a version token.
func ParseVersion(output string) string {
	matches := versionPattern.FindStringSubmatch(strings.TrimSpace(output))
	if len(matches) < 2 {
		return strings.TrimSpace(output)
	}
	return matches[1]
}

// BuildDiscoverCommand returns the gst-discoverer command used to inspect streams.
func BuildDiscoverCommand(inputPath string) Command {
	return Command{
		Path: "gst-discoverer-1.0",
		Args: []string{
			"-v",
			inputPath,
		},
	}
}

// BuildExtractCommand returns the gst-launch invocation shape.
func BuildExtractCommand(inputPath, outputPath, streamID string) Command {
	return Command{
		Path: "gst-launch-1.0",
		Args: []string{
			"-q",
			"filesrc", "location=" + inputPath,
			"!",
			"tsdemux", "name=demux",
			"demux." + streamID,
			"!",
			"queue",
			"!",
			"filesink", "location=" + outputPath,
		},
	}
}

type streamInfo struct {
	StreamID string
	PID      uint16
	Warning  string
}

func parseDiscoverStreams(data []byte) ([]streamInfo, error) {
	text := strings.TrimSpace(string(data))
	if text == "" {
		return []streamInfo{}, nil
	}

	lines := strings.Split(text, "\n")
	streams := make([]streamInfo, 0)
	var current *streamInfo

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		if streamID := parseStreamID(line); streamID != "" {
			if current != nil {
				streams = append(streams, *current)
			}
			pid, warning := parsePIDFromStreamID(streamID)
			current = &streamInfo{
				StreamID: streamID,
				PID:      pid,
				Warning:  warning,
			}
			continue
		}

		if current == nil {
			continue
		}

		pid, warning, ok := parsePIDFromLine(line)
		if !ok {
			continue
		}
		current.PID = pid
		current.Warning = warning
	}

	if current != nil {
		streams = append(streams, *current)
	}

	if len(streams) == 0 && !discoverOutputMarker.MatchString(text) {
		return nil, model.BackendParse(fmt.Errorf("parse gst-discoverer output: unexpected format"))
	}

	return streams, nil
}

func parseStreamID(line string) string {
	matches := discoverStreamLine.FindStringSubmatch(line)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func parsePIDFromLine(line string) (uint16, string, bool) {
	matches := discoverPIDLine.FindStringSubmatch(line)
	if len(matches) < 2 {
		return 0, "", false
	}
	pid, warning := parsePID(matches[1])
	return pid, normalizeWarning(warning), true
}

func parsePIDFromStreamID(streamID string) (uint16, string) {
	idx := strings.LastIndex(streamID, "_")
	if idx < 0 || idx == len(streamID)-1 {
		return 0, "pid unavailable from backend metadata"
	}
	pid, warning := parsePID("0x" + streamID[idx+1:])
	return pid, normalizeWarning(warning)
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
	return warning
}

func defaultRunner(ctx context.Context, path string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return output, nil
}
