package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/jacorbello/klvtool/internal/backends/ffmpeg"
	"github.com/jacorbello/klvtool/internal/backends/gstreamer"
	"github.com/jacorbello/klvtool/internal/envcheck"
)

type DoctorCommand struct {
	Out io.Writer
	Err io.Writer

	GOOS       string
	Env        map[string]string
	Version    string
	IsTerminal func() bool
	Detect     func(context.Context, string, map[string]string) envcheck.Report
}

func NewDoctorCommand() *DoctorCommand {
	return &DoctorCommand{
		Out:        os.Stdout,
		Err:        os.Stderr,
		GOOS:       runtime.GOOS,
		Env:        currentEnvMap(),
		Detect:     defaultDoctorDetect,
		IsTerminal: defaultIsTerminal,
	}
}

func (c *DoctorCommand) Execute(args []string) int {
	if c == nil {
		return 1
	}
	if len(args) == 1 && isHelpArg(args[0]) {
		c.writeUsage(c.Out)
		return 0
	}
	if len(args) > 0 {
		c.writeUsage(c.Err)
		if c.Err != nil {
			_, _ = fmt.Fprintf(c.Err, "error: unsupported arguments: %v\n", args)
		}
		return usageExitCode
	}

	report := c.detect()
	c.writeReport(c.Out, report)
	return 0
}

func (c *DoctorCommand) detect() envcheck.Report {
	if c.Detect != nil {
		return c.Detect(context.Background(), c.goos(), c.env())
	}
	return defaultDoctorDetect(context.Background(), c.goos(), c.env())
}

func (c *DoctorCommand) goos() string {
	if c != nil && c.GOOS != "" {
		return c.GOOS
	}
	return runtime.GOOS
}

func (c *DoctorCommand) env() map[string]string {
	if c != nil && c.Env != nil {
		return c.Env
	}
	return currentEnvMap()
}

func (c *DoctorCommand) writeUsage(w io.Writer) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprintln(w, "Usage: klvtool doctor [--help|-h]")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Check backend availability, detected versions, required modules, and install guidance.")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Required tools:")
	_, _ = fmt.Fprintln(w, "  ffmpeg:     ffmpeg, ffprobe")
	_, _ = fmt.Fprintln(w, "  gstreamer:  gst-launch-1.0, gst-inspect-1.0, gst-discoverer-1.0, tsdemux module")
}

func defaultIsTerminal() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func (c *DoctorCommand) colorEnabled() bool {
	if _, ok := c.env()["NO_COLOR"]; ok {
		return false
	}
	if c.IsTerminal != nil {
		return c.IsTerminal()
	}
	return false
}

func parseToolVersion(backendName, rawVersion string) string {
	if rawVersion == "" {
		return ""
	}
	switch backendName {
	case "ffmpeg":
		return ffmpeg.ParseVersion(rawVersion)
	case "gstreamer":
		return gstreamer.ParseVersion(rawVersion)
	default:
		return rawVersion
	}
}

func (c *DoctorCommand) writeReport(w io.Writer, report envcheck.Report) {
	if w == nil {
		return
	}

	clr := newColorizer(c.colorEnabled())
	v := c.Version
	if v == "" {
		v = "dev"
	}

	_, _ = fmt.Fprintf(w, "klvtool: %s\n", v)
	_, _ = fmt.Fprintln(w, "backend preference: auto")
	_, _ = fmt.Fprintf(w, "platform: %s\n", report.Platform)
	if report.GuidanceSummary != "" {
		_, _ = fmt.Fprintf(w, "install guidance: %s\n", report.GuidanceSummary)
	}
	_, _ = fmt.Fprintln(w)

	for i, backend := range report.Backends {
		if backend.Healthy {
			_, _ = fmt.Fprintf(w, "%s %s\n", clr.green(backend.Name), clr.green("\xe2\x9c\x93 available"))
			for _, tool := range backend.Tools {
				ver := parseToolVersion(backend.Name, tool.Version)
				_, _ = fmt.Fprintf(w, "  %-10s%-6s%s\n", tool.Name, ver, clr.dim(tool.Path))
			}
		} else {
			_, _ = fmt.Fprintf(w, "%s %s\n", clr.red(backend.Name), clr.red("\xe2\x9c\x97 not installed"))
			missing := append([]string(nil), backend.MissingTools...)
			missing = append(missing, backend.MissingModules...)
			if len(missing) > 0 {
				_, _ = fmt.Fprintf(w, "  %s %s\n", clr.red("missing:"), strings.Join(missing, ", "))
			}
			for _, step := range report.Guidance {
				_, _ = fmt.Fprintf(w, "  install: %s\n", clr.dim(step))
			}
		}

		if i < len(report.Backends)-1 {
			_, _ = fmt.Fprintln(w)
		}
	}
}

func defaultDoctorDetect(ctx context.Context, goos string, env map[string]string) envcheck.Report {
	return envcheck.Detect(ctx, goos, env, nil, nil)
}

func currentEnvMap() map[string]string {
	env := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		env[key] = value
	}
	return env
}
