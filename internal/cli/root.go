package cli

import "os"

type RootCommand struct {
	Use string
}

func NewRootCommand() *RootCommand {
	return &RootCommand{
		Use: "klvtool",
	}
}

func (c *RootCommand) Execute(args []string) int {
	if c == nil {
		return 1
	}
	if len(args) > 0 {
		return 2
	}
	return 0
}

func Main() int {
	return NewRootCommand().Execute(os.Args[1:])
}
