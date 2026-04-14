package klv

import (
	"math"
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
			if d.TagName != "UAS Datalink LS Version Number" {
				t.Errorf("TagName = %q, want %q", d.TagName, "UAS Datalink LS Version Number")
			}
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
			if d.TagName != "Precision Time Stamp" {
				t.Errorf("TagName = %q, want %q", d.TagName, "Precision Time Stamp")
			}
			if d.Actual != "position 2" {
				t.Errorf("Actual = %q, want %q", d.Actual, "position 2")
			}
			if d.Expected != "position 1 (first item)" {
				t.Errorf("Expected = %q, want %q", d.Expected, "position 1 (first item)")
			}
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
			if d.TagName != "Precision Time Stamp" {
				t.Errorf("TagName = %q, want %q", d.TagName, "Precision Time Stamp")
			}
			if d.Actual != "4 bytes" {
				t.Errorf("Actual = %q, want %q", d.Actual, "4 bytes")
			}
			if d.Expected != "8 bytes" {
				t.Errorf("Expected = %q, want %q", d.Expected, "8 bytes")
			}
			if d.Raw != "0x00000000" {
				t.Errorf("Raw = %q, want %q", d.Raw, "0x00000000")
			}
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
			if d.Severity != "note" {
				t.Errorf("ls_version_mismatch severity = %q, want %q", d.Severity, "note")
			}
			if d.TagName != "UAS Datalink LS Version Number" {
				t.Errorf("TagName = %q, want %q", d.TagName, "UAS Datalink LS Version Number")
			}
			if d.Actual != "14" {
				t.Errorf("Actual = %q, want %q", d.Actual, "14")
			}
			if d.Expected != "19" {
				t.Errorf("Expected = %q, want %q", d.Expected, "19")
			}
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
			if d.TagName != "Checksum" {
				t.Errorf("TagName = %q, want %q", d.TagName, "Checksum")
			}
			if d.Actual != "0x0002" {
				t.Errorf("Actual = %q, want %q", d.Actual, "0x0002")
			}
			if d.Expected != "0x0001" {
				t.Errorf("Expected = %q, want %q", d.Expected, "0x0001")
			}
			if d.Raw != "0x0000" {
				t.Errorf("Raw = %q, want %q", d.Raw, "0x0000")
			}
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
			if d.TagName != "Bounded Float" {
				t.Errorf("TagName = %q, want %q", d.TagName, "Bounded Float")
			}
			if d.Actual != "250" {
				t.Errorf("Actual = %q, want %q", d.Actual, "250")
			}
			if d.Expected != "[0, 100]" {
				t.Errorf("Expected = %q, want %q", d.Expected, "[0, 100]")
			}
			if d.Raw != "0x0000" {
				t.Errorf("Raw = %q, want %q", d.Raw, "0x0000")
			}
		}
	}
	if !found {
		t.Errorf("expected tag_range_violation diagnostic; got %+v", diags)
	}
}

// Regression guard: NaN comparisons are always false, so the range check
// already short-circuits today. This test exists to catch a future edit that
// drops the explicit math.IsNaN/IsInf guard and replaces it with something
// NaN-sensitive.
func TestValidateNaNFloatSkipsRangeViolation(t *testing.T) {
	nan := math.NaN()
	rec := &record.Record{
		LSVersion: 1,
		Checksum:  record.ChecksumInfo{Valid: true},
		Items: []record.Item{
			{Tag: 5, Name: "Bounded Float", Value: record.FloatValue(nan), Raw: []byte{0x00, 0x00}},
		},
	}
	diags := Validate(rangeEnumSpec{}, rec)
	for _, d := range diags {
		if d.Code == "tag_range_violation" {
			t.Errorf("NaN FloatValue must not produce tag_range_violation; got %+v", d)
		}
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
			if d.TagName != "Enum Code" {
				t.Errorf("TagName = %q, want %q", d.TagName, "Enum Code")
			}
			if d.Actual != "99" {
				t.Errorf("Actual = %q, want %q", d.Actual, "99")
			}
			if d.Expected != "{0:zero, 1:one}" {
				t.Errorf("Expected = %q, want %q", d.Expected, "{0:zero, 1:one}")
			}
			if d.Raw != "0x63" {
				t.Errorf("Raw = %q, want %q", d.Raw, "0x63")
			}
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

// TestValidateDuplicateTag verifies that a packet containing the same tag
// more than once emits exactly one duplicate_tag error diagnostic per
// duplicated tag (not one per duplicate occurrence).
func TestValidateDuplicateTag(t *testing.T) {
	rec := &record.Record{
		LSVersion: 1,
		Checksum:  record.ChecksumInfo{Valid: true},
		Items: []record.Item{
			// Tag 6 appears three times; expect exactly one duplicate_tag diag.
			{Tag: 6, Name: "Enum Code", Value: record.IntValue(0), Raw: []byte{0}},
			{Tag: 6, Name: "Enum Code", Value: record.IntValue(1), Raw: []byte{1}},
			{Tag: 6, Name: "Enum Code", Value: record.IntValue(0), Raw: []byte{0}},
		},
	}
	diags := Validate(rangeEnumSpec{}, rec)
	var count int
	for _, d := range diags {
		if d.Code == "duplicate_tag" {
			count++
			if d.Severity != "error" {
				t.Errorf("expected severity=error, got %q", d.Severity)
			}
			if d.Tag == nil || *d.Tag != 6 {
				t.Errorf("expected Tag pointer to 6, got %+v", d.Tag)
			}
			if d.Message == "" {
				t.Errorf("expected non-empty message identifying duplicated tag")
			}
			if d.TagName != "Enum Code" {
				t.Errorf("TagName = %q, want %q", d.TagName, "Enum Code")
			}
			if d.Actual != "3 occurrences" {
				t.Errorf("Actual = %q, want %q", d.Actual, "3 occurrences")
			}
			if d.Expected != "at most once" {
				t.Errorf("Expected = %q, want %q", d.Expected, "at most once")
			}
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 duplicate_tag diagnostic, got %d: %+v", count, diags)
	}
}

// TestValidateNoDuplicateTagWhenUnique verifies that a well-formed record
// with all unique tags emits no duplicate_tag diagnostics.
func TestValidateNoDuplicateTagWhenUnique(t *testing.T) {
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
		if d.Code == "duplicate_tag" {
			t.Errorf("unexpected duplicate_tag diagnostic on unique-tag record: %+v", d)
		}
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
