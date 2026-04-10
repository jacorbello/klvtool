package packetize

import (
	"testing"

	"github.com/jacorbello/klvtool/internal/extract"
)

func TestZeroValuePacketizedStreamUsesStableDefaults(t *testing.T) {
	var raw extract.RawPayloadRecord
	stream := PacketizedStream{Source: raw}

	if stream.Mode != "" {
		t.Fatalf("expected zero-value mode to be unset, got %q", stream.Mode)
	}
	if stream.Packets != nil {
		t.Fatalf("expected nil packets before normalization, got %#v", stream.Packets)
	}
	if stream.Diagnostics != nil {
		t.Fatalf("expected nil diagnostics before normalization, got %#v", stream.Diagnostics)
	}
}
