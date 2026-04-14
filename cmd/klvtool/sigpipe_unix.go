//go:build unix

package main

import (
	"os/signal"
	"syscall"
)

func ignoreSIGPIPE() {
	signal.Ignore(syscall.SIGPIPE)
}
