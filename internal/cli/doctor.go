package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/jacorbello/klvtool/internal/envcheck"
)

type DoctorCommand struct {
	Out io.Writer
	Err io.Writer

	GOOS   string
	Env    map[string]string
	Detect func(context.Context, string, map[string]string) envcheck.Report
}

func NewDoctorCommand() *DoctorCommand {
	return &DoctorCommand{
		Out:    os.Stdout,
		Err:    os.Stderr,
		GOOS:   runtime.GOOS,
		Env:    currentEnvMap(),
		Detect: defaultDoctorDetect,
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

func (c *DoctorCommand) writeReport(w io.Writer, report envcheck.Report) {
	if w == nil {
		return
	}

	_, _ = fmt.Fprintln(w, "backend resolution preference: auto")
	_, _ = fmt.Fprintf(w, "platform: %s\n", report.Platform)
	if report.GuidanceSummary != "" {
		_, _ = fmt.Fprintf(w, "install guidance: %s\n", report.GuidanceSummary)
	}
	_, _ = fmt.Fprintln(w)

	for i, backend := range report.Backends {
		status := "unavailable"
		if backend.Healthy {
			status = "available"
		}
		_, _ = fmt.Fprintf(w, "%s: %s\n", backend.Name, status)

		for _, tool := range backend.Tools {
			_, _ = fmt.Fprintf(w, "  %s\n", tool.Name)
			if tool.Path != "" {
				_, _ = fmt.Fprintf(w, "    path: %s\n", tool.Path)
			}
			if tool.Version != "" {
				_, _ = fmt.Fprintf(w, "    version: %s\n", tool.Version)
			}
			if tool.Error != "" {
				_, _ = fmt.Fprintf(w, "    error: %s\n", tool.Error)
			}
		}

		for _, module := range backend.Modules {
			status := "unavailable"
			if module.Healthy {
				status = "available"
			}
			_, _ = fmt.Fprintf(w, "  module: %s (%s)\n", module.Name, status)
			if module.Error != "" {
				_, _ = fmt.Fprintf(w, "    error: %s\n", module.Error)
			}
		}

		if !backend.Healthy && (len(backend.MissingTools) > 0 || len(backend.MissingModules) > 0) {
			missing := append([]string(nil), backend.MissingTools...)
			for _, module := range backend.MissingModules {
				missing = append(missing, "module:"+module)
			}
			_, _ = fmt.Fprintf(w, "  missing: %s\n", strings.Join(missing, ", "))
			for _, step := range report.Guidance {
				_, _ = fmt.Fprintf(w, "  install: %s\n", step)
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
