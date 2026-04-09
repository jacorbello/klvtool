package cli

import "testing"

func TestNewRootCommand(t *testing.T) {
	cmd := NewRootCommand()
	if cmd == nil {
		t.Fatal("expected root command")
	}
	if cmd.Use != "klvtool" {
		t.Fatalf("expected command use klvtool, got %q", cmd.Use)
	}
}
