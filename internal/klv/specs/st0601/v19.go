package st0601

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/jacorbello/klvtool/internal/klv/record"
	"github.com/jacorbello/klvtool/internal/klv/specs"
)

// V19 returns the ST 0601.19 SpecVersion.
func V19() specs.SpecVersion {
	return &v19{tags: v19Tags()}
}

type v19 struct {
	tags map[int]specs.TagDefinition
}

func (v *v19) URN() string                            { return "urn:misb:KLV:bin:0601.19" }
func (v *v19) UL() []byte                             { return UASDatalinkUL }
func (v *v19) VersionTag() int                        { return 65 }
func (v *v19) ExpectedVersion() int                   { return 19 }
func (v *v19) Tag(n int) (specs.TagDefinition, bool)  { t, ok := v.tags[n]; return t, ok }
func (v *v19) AllTags() []specs.TagDefinition {
	out := make([]specs.TagDefinition, 0, len(v.tags))
	for _, t := range v.tags {
		out = append(out, t)
	}
	return out
}

// v19Tags builds the tag table. Phase 1 ships mandatory tags plus the core
// subset in tasks 12/13. This function starts with just the mandatory items
// and is extended in later tasks.
func v19Tags() map[int]specs.TagDefinition {
	tags := map[int]specs.TagDefinition{}

	// Tag 1: Checksum. Captured specially by the runner; a TagDefinition
	// still exists so validation sees it as Mandatory.
	tags[1] = specs.TagDefinition{
		Tag: 1, Name: "Checksum", Units: "",
		Format: specs.FormatUint16, Length: 2, Mandatory: true,
	}

	// Tag 2: Precision Time Stamp. Custom Decode converts MISP microseconds
	// since epoch (1970-01-01 00:00:00, no leap seconds) into a TimeValue.
	tags[2] = specs.TagDefinition{
		Tag: 2, Name: "Precision Time Stamp", Units: "μs",
		Format: specs.FormatUint64, Length: 8, Mandatory: true,
		Decode: decodeMISPTimestamp,
	}

	// Tag 65: UAS Datalink LS Version Number.
	tags[65] = specs.TagDefinition{
		Tag: 65, Name: "UAS Datalink LS Version Number", Units: "",
		Format: specs.FormatUint8, Length: 1, Mandatory: true,
	}

	return tags
}

// decodeMISPTimestamp converts an 8-byte big-endian uint64 of MISP
// microseconds-since-epoch into a TimeValue. The MISP epoch is
// 1970-01-01T00:00:00Z and excludes leap seconds.
func decodeMISPTimestamp(raw []byte) (record.Value, error) {
	if len(raw) != 8 {
		return nil, fmt.Errorf("precision time stamp: expected 8 bytes, got %d", len(raw))
	}
	micros := binary.BigEndian.Uint64(raw)
	secs := int64(micros / 1_000_000)
	nanos := int64((micros % 1_000_000) * 1_000)
	return record.TimeValue(time.Unix(secs, nanos).UTC()), nil
}
