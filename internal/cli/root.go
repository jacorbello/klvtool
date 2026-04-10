package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/jacorbello/klvtool/internal/version"
)

const usageExitCode = 2

type RootCommand struct {
	Use        string
	Version    string
	Out        io.Writer
	Err        io.Writer
	Doctor     *DoctorCommand
	Extract    *ExtractCommand
	VersionCmd *VersionCommand
}

func NewRootCommand() *RootCommand {
	return &RootCommand{
		Use:        "klvtool",
		Version:    version.String(),
		Out:        os.Stdout,
		Err:        os.Stderr,
		Doctor:     NewDoctorCommand(),
		Extract:    NewExtractCommand(),
		VersionCmd: NewVersionCommand(),
	}
}

func (c *RootCommand) Execute(args []string) int {
	if c == nil {
		return 1
	}
	if len(args) == 0 {
		c.writeUsage(c.Err)
		return usageExitCode
	}
	if len(args) == 1 && isHelpArg(args[0]) {
		c.writeUsage(c.Out)
		return 0
	}
	if len(args) > 0 && args[0] == "version" {
		return c.versionCommand().Execute(args[1:])
	}
	if len(args) > 0 && args[0] == "doctor" {
		return c.doctorCommand().Execute(args[1:])
	}
	if len(args) > 0 && args[0] == "extract" {
		return c.extractCommand().Execute(args[1:])
	}
	c.writeUnsupportedArgs(args)
	return usageExitCode
}

func Main() int {
	return NewRootCommand().Execute(os.Args[1:])
}

func isHelpArg(arg string) bool {
	return arg == "-h" || arg == "--help"
}

func (c *RootCommand) writeUsage(w io.Writer) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprintf(w, "Usage: %s [command] [--help|-h]\n", c.Use)
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "Version: %s\n", c.Version)
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Baseline CLI for the klvtool repository.")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Commands:")
	_, _ = fmt.Fprintln(w, "  version  Print version information.")
	_, _ = fmt.Fprintln(w, "  doctor   Check backend availability and environment health.")
	_, _ = fmt.Fprintln(w, "  extract  Extract payloads and write manifest output.")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Required tools:")
	_, _ = fmt.Fprintln(w, "  ffmpeg:     ffmpeg, ffprobe")
	_, _ = fmt.Fprintln(w, "  gstreamer:  gst-launch-1.0, gst-inspect-1.0, gst-discoverer-1.0, tsdemux module")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Install:")
	_, _ = fmt.Fprintln(w, "  go install github.com/jacorbello/klvtool/cmd/klvtool@latest")
}

func (c *RootCommand) writeUnsupportedArgs(args []string) {
	if c.Err == nil {
		return
	}
	c.writeUsage(c.Err)
	_, _ = fmt.Fprintf(c.Err, "error: unsupported arguments: %v\n", args)
}

func (c *RootCommand) doctorCommand() *DoctorCommand {
	if c == nil {
		return NewDoctorCommand()
	}
	doctor := c.Doctor
	if doctor == nil {
		doctor = NewDoctorCommand()
		c.Doctor = doctor
	}
	doctor.Out = c.Out
	doctor.Err = c.Err
	return doctor
}

func (c *RootCommand) extractCommand() *ExtractCommand {
	if c == nil {
		return NewExtractCommand()
	}
	extractCmd := c.Extract
	if extractCmd == nil {
		extractCmd = NewExtractCommand()
		c.Extract = extractCmd
	}
	extractCmd.Out = c.Out
	extractCmd.Err = c.Err
	return extractCmd
}

func (c *RootCommand) versionCommand() *VersionCommand {
	if c == nil {
		return NewVersionCommand()
	}
	v := c.VersionCmd
	if v == nil {
		v = NewVersionCommand()
		c.VersionCmd = v
	}
	v.Out = c.Out
	v.Err = c.Err
	v.Version = c.Version
	return v
}
