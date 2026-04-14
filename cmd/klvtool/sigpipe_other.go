//go:build !unix && !windows

package main

func ignoreSIGPIPE() {}
