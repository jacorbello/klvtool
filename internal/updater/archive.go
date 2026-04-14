package updater

import "fmt"

// ArchiveFileName returns the GoReleaser-published archive name for the given GOOS/GOARCH.
func ArchiveFileName(goos, goarch string) string {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("klvtool_%s_%s%s", goos, goarch, ext)
}

// BinaryFileName returns the executable name inside release archives for goos.
func BinaryFileName(goos string) string {
	if goos == "windows" {
		return "klvtool.exe"
	}
	return "klvtool"
}
