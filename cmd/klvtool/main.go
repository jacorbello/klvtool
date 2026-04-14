package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/jacorbello/klvtool/internal/cli"
)

func main() {
	signal.Ignore(syscall.SIGPIPE)
	os.Exit(cli.Main())
}
