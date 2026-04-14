package updater

import "testing"

func TestArchiveFileName(t *testing.T) {
	tests := []struct {
		goos   string
		goarch string
		want   string
	}{
		{"linux", "amd64", "klvtool_linux_amd64.tar.gz"},
		{"linux", "arm64", "klvtool_linux_arm64.tar.gz"},
		{"darwin", "amd64", "klvtool_darwin_amd64.tar.gz"},
		{"darwin", "arm64", "klvtool_darwin_arm64.tar.gz"},
		{"windows", "amd64", "klvtool_windows_amd64.zip"},
		{"windows", "arm64", "klvtool_windows_arm64.zip"},
	}
	for _, tt := range tests {
		t.Run(tt.goos+"_"+tt.goarch, func(t *testing.T) {
			if got := ArchiveFileName(tt.goos, tt.goarch); got != tt.want {
				t.Fatalf("ArchiveFileName(%q, %q) = %q, want %q", tt.goos, tt.goarch, got, tt.want)
			}
		})
	}
}

func TestBinaryFileName(t *testing.T) {
	if got := BinaryFileName("linux"); got != "klvtool" {
		t.Fatalf("got %q, want klvtool", got)
	}
	if got := BinaryFileName("darwin"); got != "klvtool" {
		t.Fatalf("got %q, want klvtool", got)
	}
	if got := BinaryFileName("windows"); got != "klvtool.exe" {
		t.Fatalf("got %q, want klvtool.exe", got)
	}
}
