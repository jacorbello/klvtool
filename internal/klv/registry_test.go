package klv

import (
	"testing"

	"github.com/jacorbello/klvtool/internal/klv/record"
	"github.com/jacorbello/klvtool/internal/klv/specs"
)

// fakeSpec is a minimal SpecVersion for registry tests.
type fakeSpec struct {
	urn        string
	ul         []byte
	versionTag int
	version    int
	tags       map[int]specs.TagDefinition
}

func (f *fakeSpec) URN() string          { return f.urn }
func (f *fakeSpec) UL() []byte           { return f.ul }
func (f *fakeSpec) VersionTag() int      { return f.versionTag }
func (f *fakeSpec) ExpectedVersion() int { return f.version }
func (f *fakeSpec) Tag(n int) (specs.TagDefinition, bool) {
	t, ok := f.tags[n]
	return t, ok
}
func (f *fakeSpec) AllTags() []specs.TagDefinition {
	out := make([]specs.TagDefinition, 0, len(f.tags))
	for _, t := range f.tags {
		out = append(out, t)
	}
	return out
}

func newFakeSpec(urn string) *fakeSpec {
	return &fakeSpec{
		urn:        urn,
		ul:         []byte{0x06, 0x0e, 0x2b, 0x34, 0x02, 0x0b, 0x01, 0x01, 0x0e, 0x01, 0x03, 0x01, 0x01, 0x00, 0x00, 0x00},
		versionTag: 65,
		version:    19,
		tags: map[int]specs.TagDefinition{
			65: {Tag: 65, Name: "Version", Format: specs.FormatUint8, Length: 1},
		},
	}
}

var _ = record.Record{} // keep import

func TestRegistryLookup(t *testing.T) {
	reg := NewRegistry()
	sv := newFakeSpec("urn:misb:KLV:bin:0601.19")
	reg.Register(sv)

	got, ok := reg.Lookup("urn:misb:KLV:bin:0601.19")
	if !ok {
		t.Fatalf("Lookup: expected ok=true")
	}
	if got.URN() != "urn:misb:KLV:bin:0601.19" {
		t.Errorf("URN = %s", got.URN())
	}

	_, ok = reg.Lookup("urn:misb:KLV:bin:0601.14")
	if ok {
		t.Errorf("Lookup unknown URN: expected ok=false")
	}
}

func TestRegistryResolveMatchingUL(t *testing.T) {
	reg := NewRegistry()
	sv := newFakeSpec("urn:misb:KLV:bin:0601.19")
	reg.Register(sv)

	// Value bytes don't matter when only one spec is registered for the UL.
	got, err := reg.Resolve(sv.UL(), []byte{0x41, 0x01, 0x13})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.URN() != sv.URN() {
		t.Errorf("got %s, want %s", got.URN(), sv.URN())
	}
}

func TestRegistryResolveUnknownUL(t *testing.T) {
	reg := NewRegistry()
	reg.Register(newFakeSpec("urn:misb:KLV:bin:0601.19"))

	unknownUL := make([]byte, 16)
	_, err := reg.Resolve(unknownUL, nil)
	if err == nil {
		t.Errorf("Resolve unknown UL: expected error")
	}
}

func TestRegistryResolveUnknownULEmpty(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Resolve(make([]byte, 16), nil)
	if err == nil {
		t.Errorf("Resolve on empty registry: expected error")
	}
}
