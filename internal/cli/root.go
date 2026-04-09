package cli

import (
	"fmt"
	"io"
	"os"
)

const usageExitCode = 2

type RootCommand struct {
	Use string
	Out io.Writer
	Err io.Writer
}

func NewRootCommand() *RootCommand {
	return &RootCommand{
		Use: "klvtool",
		Out: os.Stdout,
		Err: os.Stderr,
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
	fmt.Fprintf(w, "Usage: %s [--help|-h]\n", c.Use)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Baseline CLI for the klvtool repository.")
}

func (c *RootCommand) writeUnsupportedArgs(args []string) {
	if c.Err == nil {
		return
	}
	c.writeUsage(c.Err)
	fmt.Fprintf(c.Err, "error: unsupported arguments: %v\n", args)
}
