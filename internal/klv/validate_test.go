package klv

import (
	"testing"

	"github.com/jacorbello/klvtool/internal/klv/record"
	"github.com/jacorbello/klvtool/internal/klv/specs"
	"github.com/jacorbello/klvtool/internal/klv/specs/st0601"
)

func TestValidateHappyPath(t *testing.T) {
	rec := &record.Record{
		LSVersion: 19,
		Checksum:  record.ChecksumInfo{Valid: true},
		Items: []record.Item{
			{Tag: 2, Name: "Precision Time Stamp", Raw: make([]byte, 8)},
			{Tag: 65, Name: "UAS Datalink LS Version Number", Raw: []byte{19}},
			{Tag: 1, Name: "Checksum", Raw: []byte{0x00, 0x00}},
		},
	}
	diags := Validate(st0601.V19(), rec)
	for _, d := range diags {
		if d.Severity == "error" {
			t.Errorf("unexpected error diagnostic: %+v", d)
		}
	}
}

func TestValidateMissingMandatory(t *testing.T) {
	rec := &record.Record{
		LSVersion: 19,
		Checksum:  record.ChecksumInfo{Valid: true},
		Items: []record.Item{
			{Tag: 2, Name: "Precision Time Stamp", Raw: make([]byte, 8)},
			{Tag: 1, Name: "Checksum", Raw: []byte{0x00, 0x00}},
		},
		// Missing tag 65.
	}
	diags := Validate(st0601.V19(), rec)
	var found bool
	for _, d := range diags {
		if d.Code == "missing_mandatory_item" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected missing_mandatory_item diagnostic")
	}
}

func TestValidateOrderTag2NotFirst(t *testing.T) {
	rec := &record.Record{
		LSVersion: 19,
		Checksum:  record.ChecksumInfo{Valid: true},
		Items: []record.Item{
			{Tag: 65, Name: "UAS Datalink LS Version Number", Raw: []byte{19}},
			{Tag: 2, Name: "Precision Time Stamp", Raw: make([]byte, 8)},
			{Tag: 1, Name: "Checksum", Raw: []byte{0x00, 0x00}},
		},
	}
	diags := Validate(st0601.V19(), rec)
	var found bool
	for _, d := range diags {
		if d.Code == "tag_out_of_order" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected tag_out_of_order diagnostic")
	}
}

func TestValidateOrderTag1NotLast(t *testing.T) {
	rec := &record.Record{
		LSVersion: 19,
		Checksum:  record.ChecksumInfo{Valid: true},
		Items: []record.Item{
			{Tag: 2, Name: "Precision Time Stamp", Raw: make([]byte, 8)},
			{Tag: 1, Name: "Checksum", Raw: []byte{0x00, 0x00}},
			{Tag: 65, Name: "UAS Datalink LS Version Number", Raw: []byte{19}},
		},
	}
	diags := Validate(st0601.V19(), rec)
	var found bool
	for _, d := range diags {
		if d.Code == "tag_out_of_order" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected tag_out_of_order diagnostic")
	}
}

func TestValidateLengthMismatch(t *testing.T) {
	rec := &record.Record{
		LSVersion: 19,
		Checksum:  record.ChecksumInfo{Valid: true},
		Items: []record.Item{
			{Tag: 2, Name: "Precision Time Stamp", Raw: make([]byte, 4)}, // wrong
			{Tag: 65, Name: "UAS Datalink LS Version Number", Raw: []byte{19}},
			{Tag: 1, Name: "Checksum", Raw: []byte{0x00, 0x00}},
		},
	}
	diags := Validate(st0601.V19(), rec)
	var found bool
	for _, d := range diags {
		if d.Code == "tag_length_mismatch" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected tag_length_mismatch diagnostic")
	}
}

func TestValidateVersionMismatch(t *testing.T) {
	rec := &record.Record{
		LSVersion: 14, // wrong
		Checksum:  record.ChecksumInfo{Valid: true},
		Items: []record.Item{
			{Tag: 2, Name: "Precision Time Stamp", Raw: make([]byte, 8)},
			{Tag: 65, Name: "UAS Datalink LS Version Number", Raw: []byte{14}},
			{Tag: 1, Name: "Checksum", Raw: []byte{0x00, 0x00}},
		},
	}
	diags := Validate(st0601.V19(), rec)
	var found bool
	for _, d := range diags {
		if d.Code == "ls_version_mismatch" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ls_version_mismatch diagnostic")
	}
}

func TestValidateChecksumMismatch(t *testing.T) {
	rec := &record.Record{
		LSVersion: 19,
		Checksum:  record.ChecksumInfo{Valid: false, Expected: 1, Computed: 2},
		Items: []record.Item{
			{Tag: 2, Name: "Precision Time Stamp", Raw: make([]byte, 8)},
			{Tag: 65, Name: "UAS Datalink LS Version Number", Raw: []byte{19}},
			{Tag: 1, Name: "Checksum", Raw: []byte{0x00, 0x00}},
		},
	}
	diags := Validate(st0601.V19(), rec)
	var found bool
	for _, d := range diags {
		if d.Code == "checksum_mismatch" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected checksum_mismatch diagnostic")
	}
}

var _ = specs.FormatUint8 // keep import

// rangeEnumSpec is a minimal spec with two crafted tags used only by
// TestValidateRangeViolation and TestValidateEnumInvalid.
type rangeEnumSpec struct{}

func (rangeEnumSpec) URN() string          { return "urn:test:range-enum" }
func (rangeEnumSpec) UL() []byte           { return make([]byte, 16) }
func (rangeEnumSpec) VersionTag() int      { return 65 }
func (rangeEnumSpec) ExpectedVersion() int { return 1 }
func (rangeEnumSpec) Tag(n int) (specs.TagDefinition, bool) {
	tags := rangeEnumSpec{}.AllTags()
	for _, t := range tags {
		if t.Tag == n {
			return t, true
		}
	}
	return specs.TagDefinition{}, false
}
func (rangeEnumSpec) AllTags() []specs.TagDefinition {
	return []specs.TagDefinition{
		{
			Tag: 5, Name: "Bounded Float",
			Format: specs.FormatUint16, Length: 2,
			Scale: &specs.LinearScale{Min: 0, Max: 100},
		},
		{
			Tag: 6, Name: "Enum Code",
			Format: specs.FormatUint8, Length: 1,
			Enum: map[int64]string{0: "zero", 1: "one"},
		},
	}
}

func TestValidateRangeViolation(t *testing.T) {
	rec := &record.Record{
		LSVersion: 1,
		Checksum:  record.ChecksumInfo{Valid: true},
		Items: []record.Item{
			// Out of range: Scale is 0..100, value is 250.
			{Tag: 5, Name: "Bounded Float", Value: record.FloatValue(250.0), Raw: []byte{0x00, 0x00}},
		},
	}
	diags := Validate(rangeEnumSpec{}, rec)
	var found bool
	for _, d := range diags {
		if d.Code == "tag_range_violation" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected tag_range_violation diagnostic; got %+v", diags)
	}
}

func TestValidateEnumInvalid(t *testing.T) {
	rec := &record.Record{
		LSVersion: 1,
		Checksum:  record.ChecksumInfo{Valid: true},
		Items: []record.Item{
			// Enum has keys 0 and 1; 99 is not in the enum.
			{Tag: 6, Name: "Enum Code", Value: record.IntValue(99), Raw: []byte{99}},
		},
	}
	diags := Validate(rangeEnumSpec{}, rec)
	var found bool
	for _, d := range diags {
		if d.Code == "tag_enum_invalid" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected tag_enum_invalid diagnostic; got %+v", diags)
	}
}

// TestValidateMissingTag2SkipsOrderingDiag verifies that a packet missing
// tag 2 entirely gets a missing_mandatory_item diagnostic but NOT a
// "tag 2 must be first" ordering diagnostic — the two are redundant and
// the ordering check is noise when the tag is absent.
func TestValidateMissingTag2SkipsOrderingDiag(t *testing.T) {
	rec := &record.Record{
		LSVersion: 19,
		Checksum:  record.ChecksumInfo{Valid: true},
		Items: []record.Item{
			// No tag 2 at all; tag 65 first, tag 1 last.
			{Tag: 65, Name: "UAS Datalink LS Version Number", Raw: []byte{19}},
			{Tag: 1, Name: "Checksum", Raw: []byte{0x00, 0x00}},
		},
	}
	diags := Validate(st0601.V19(), rec)
	var hasOrder, hasMissing bool
	for _, d := range diags {
		if d.Code == "tag_out_of_order" {
			hasOrder = true
		}
		if d.Code == "missing_mandatory_item" && d.Tag != nil && *d.Tag == 2 {
			hasMissing = true
		}
	}
	if hasOrder {
		t.Errorf("unexpected tag_out_of_order diagnostic when tag 2 is missing: %+v", diags)
	}
	if !hasMissing {
		t.Errorf("expected missing_mandatory_item for tag 2; got %+v", diags)
	}
}

// TestValidateMissingTag1SkipsChecksumDiag verifies that a packet missing
// tag 1 does not produce a checksum_mismatch diagnostic (the checksum was
// never computed, so the default Checksum.Valid == false is not a real
// mismatch). Missing tag 1 is reported via missing_mandatory_item.
func TestValidateMissingTag1SkipsChecksumDiag(t *testing.T) {
	rec := &record.Record{
		LSVersion: 19,
		Checksum:  record.ChecksumInfo{Valid: false}, // default zero value
		Items: []record.Item{
			{Tag: 2, Name: "Precision Time Stamp", Raw: make([]byte, 8)},
			{Tag: 65, Name: "UAS Datalink LS Version Number", Raw: []byte{19}},
			// No tag 1.
		},
	}
	diags := Validate(st0601.V19(), rec)
	var hasChecksum, hasMissing bool
	for _, d := range diags {
		if d.Code == "checksum_mismatch" {
			hasChecksum = true
		}
		if d.Code == "missing_mandatory_item" && d.Tag != nil && *d.Tag == 1 {
			hasMissing = true
		}
	}
	if hasChecksum {
		t.Errorf("unexpected checksum_mismatch diagnostic when tag 1 is missing: %+v", diags)
	}
	if !hasMissing {
		t.Errorf("expected missing_mandatory_item for tag 1; got %+v", diags)
	}
}

// TestValidateTag1WrongLengthSkipsChecksumDiag verifies that when tag 1 is
// present but has the wrong length, the checksum was never computed by the
// engine, so Validate must not emit a misleading checksum_mismatch. The
// length problem is reported via tag_length_mismatch instead.
func TestValidateTag1WrongLengthSkipsChecksumDiag(t *testing.T) {
	rec := &record.Record{
		LSVersion: 19,
		Checksum:  record.ChecksumInfo{Valid: false}, // never computed
		Items: []record.Item{
			{Tag: 2, Name: "Precision Time Stamp", Raw: make([]byte, 8)},
			{Tag: 65, Name: "UAS Datalink LS Version Number", Raw: []byte{19}},
			{Tag: 1, Name: "Checksum", Raw: []byte{0x00}}, // 1 byte, wrong length
		},
	}
	diags := Validate(st0601.V19(), rec)
	var hasChecksum, hasLength bool
	for _, d := range diags {
		if d.Code == "checksum_mismatch" {
			hasChecksum = true
		}
		if d.Code == "tag_length_mismatch" && d.Tag != nil && *d.Tag == 1 {
			hasLength = true
		}
	}
	if hasChecksum {
		t.Errorf("unexpected checksum_mismatch when tag 1 has wrong length: %+v", diags)
	}
	if !hasLength {
		t.Errorf("expected tag_length_mismatch for tag 1; got %+v", diags)
	}
}
