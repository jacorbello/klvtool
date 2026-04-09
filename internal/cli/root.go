package cli

import "os"

type RootCommand struct {
	Use string
}

func NewRootCommand() *RootCommand {
	return &RootCommand{
		Use: "vidtool",
	}
}

func (c *RootCommand) Execute(_ []string) int {
	if c == nil {
		return 1
	}
	return 0
}

func Main() int {
	return NewRootCommand().Execute(os.Args[1:])
}
