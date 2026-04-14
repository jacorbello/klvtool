package main

import (
	"os"

	"github.com/jacorbello/klvtool/internal/cli"
)

func main() {
	ignoreSIGPIPE()
	os.Exit(cli.Main())
}
