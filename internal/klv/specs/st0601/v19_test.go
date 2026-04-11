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
