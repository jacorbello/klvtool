package st0601

import (
	"bytes"
	"testing"

	"github.com/jacorbello/klvtool/internal/klv/specs"
)

func TestV19Metadata(t *testing.T) {
	sv := V19()
	if sv.URN() != "urn:misb:KLV:bin:0601.19" {
		t.Errorf("URN = %s", sv.URN())
	}
	if sv.VersionTag() != 65 {
		t.Errorf("VersionTag = %d, want 65", sv.VersionTag())
	}
	if sv.ExpectedVersion() != 19 {
		t.Errorf("ExpectedVersion = %d, want 19", sv.ExpectedVersion())
	}
	if !bytes.Equal(sv.UL(), UASDatalinkUL) {
		t.Errorf("UL = %x, want %x", sv.UL(), UASDatalinkUL)
	}
}

func TestV19MandatoryTags(t *testing.T) {
	sv := V19()
	for _, tag := range []int{1, 2, 65} {
		td, ok := sv.Tag(tag)
		if !ok {
			t.Errorf("Tag(%d) missing", tag)
			continue
		}
		if !td.Mandatory {
			t.Errorf("Tag(%d) not mandatory", tag)
		}
	}
}

func TestV19UnknownTag(t *testing.T) {
	sv := V19()
	_, ok := sv.Tag(9999)
	if ok {
		t.Errorf("Tag(9999): expected ok=false")
	}
}

var _ = specs.SpecVersion(V19()) // compile-time interface check

func TestV19CoreTagDecoding(t *testing.T) {
	sv := V19()
	tests := []struct {
		tag  int
		raw  []byte
		name string
	}{
		{5, []byte{0x71, 0xC2}, "Platform Heading Angle"},     // 0x71C2 = 29122 → ~159.9° of 360
		{10, []byte("REAPER"), "Platform Designation"},
		{13, []byte{0x04, 0x5D, 0x6D, 0x00}, "Sensor Latitude"},
		{15, []byte{0x40, 0x00}, "Sensor True Altitude"},
		{65, []byte{19}, "UAS Datalink LS Version Number"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td, ok := sv.Tag(tt.tag)
			if !ok {
				t.Fatalf("tag %d missing", tt.tag)
			}
			if td.Name != tt.name {
				t.Errorf("name = %s, want %s", td.Name, tt.name)
			}
			// Every phase-1 tag has either Format or Decode set.
			if td.Format == specs.FormatNone && td.Decode == nil {
				t.Errorf("tag %d has no decoder", tt.tag)
			}
		})
	}
}
