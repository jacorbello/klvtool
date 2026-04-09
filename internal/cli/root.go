package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/jacorbello/klvtool/internal/version"
)

const usageExitCode = 2

type RootCommand struct {
	Use     string
	Version string
	Out     io.Writer
	Err     io.Writer
	Doctor  *DoctorCommand
}

func NewRootCommand() *RootCommand {
	return &RootCommand{
		Use:     "klvtool",
		Version: version.String(),
		Out:     os.Stdout,
		Err:     os.Stderr,
		Doctor:  NewDoctorCommand(),
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
	if len(args) > 0 && args[0] == "doctor" {
		return c.doctorCommand().Execute(args[1:])
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
	fmt.Fprintf(w, "Usage: %s [command] [--help|-h]\n", c.Use)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Version: %s\n", c.Version)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Baseline CLI for the klvtool repository.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  doctor   Check backend availability and environment health.")
}

func (c *RootCommand) writeUnsupportedArgs(args []string) {
	if c.Err == nil {
		return
	}
	c.writeUsage(c.Err)
	fmt.Fprintf(c.Err, "error: unsupported arguments: %v\n", args)
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
	if c.Out != nil {
		doctor.Out = c.Out
	}
	if c.Err != nil {
		doctor.Err = c.Err
	}
	return doctor
}
