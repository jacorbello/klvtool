package version

import "testing"

func TestStringDefaultIsNonEmpty(t *testing.T) {
	if got := String(); got == "" {
		t.Fatal("expected default version string to be non-empty")
	}
}
