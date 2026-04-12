package klv

import (
	"fmt"

	"github.com/jacorbello/klvtool/internal/klv/record"
	"github.com/jacorbello/klvtool/internal/klv/specs"
)

// Validate runs structural + per-tag checks against a decoded Record.
// Returns diagnostics in the order they are discovered; never mutates rec.
func Validate(spec specs.SpecVersion, rec *record.Record) []record.Diagnostic {
	var diags []record.Diagnostic

	// 1. Mandatory items present.
	present := make(map[int]bool, len(rec.Items))
	for _, it := range rec.Items {
		present[it.Tag] = true
	}
	for _, td := range spec.AllTags() {
		if td.Mandatory && !present[td.Tag] {
			tag := td.Tag
			diags = append(diags, record.Diagnostic{
				Severity: "error",
				Code:     "missing_mandatory_item",
				Message:  fmt.Sprintf("mandatory tag %d (%s) missing", td.Tag, td.Name),
				Tag:      &tag,
			})
		}
	}

	// 2. Ordering: tag 2 first, tag 1 last (per ST 0601 §6.1 requirement 13-23).
	// Only enforce when the tag is actually present — missing tags are
	// already reported as missing_mandatory_item above, and emitting
	// "tag 2 must be first" for a packet that lacks tag 2 entirely is noise.
	if len(rec.Items) > 0 {
		if present[2] && rec.Items[0].Tag != 2 {
			two := 2
			diags = append(diags, record.Diagnostic{
				Severity: "error",
				Code:     "tag_out_of_order",
				Message:  "Precision Time Stamp (tag 2) must be the first item",
				Tag:      &two,
			})
		}
		if present[1] && rec.Items[len(rec.Items)-1].Tag != 1 {
			one := 1
			diags = append(diags, record.Diagnostic{
				Severity: "error",
				Code:     "tag_out_of_order",
				Message:  "Checksum (tag 1) must be the last item",
				Tag:      &one,
			})
		}
	}

	// 3. LS Version consistency.
	if rec.LSVersion >= 0 && rec.LSVersion != spec.ExpectedVersion() {
		vt := spec.VersionTag()
		diags = append(diags, record.Diagnostic{
			Severity: "error",
			Code:     "ls_version_mismatch",
			Message:  fmt.Sprintf("LS version %d does not match spec %d", rec.LSVersion, spec.ExpectedVersion()),
			Tag:      &vt,
		})
	}

	// 4. Per-item length and range.
	for i, it := range rec.Items {
		td, ok := spec.Tag(it.Tag)
		if !ok {
			continue
		}
		if td.Length > 0 && len(rec.Items[i].Raw) != td.Length {
			tag := it.Tag
			diags = append(diags, record.Diagnostic{
				Severity: "error",
				Code:     "tag_length_mismatch",
				Message:  fmt.Sprintf("tag %d: expected %d bytes, got %d", it.Tag, td.Length, len(rec.Items[i].Raw)),
				Tag:      &tag,
			})
		}
		// Range checks apply to FloatValue after a Scale was applied.
		if fv, ok := it.Value.(record.FloatValue); ok && td.Scale != nil {
			f := float64(fv)
			if f < td.Scale.Min || f > td.Scale.Max {
				tag := it.Tag
				diags = append(diags, record.Diagnostic{
					Severity: "warning",
					Code:     "tag_range_violation",
					Message:  fmt.Sprintf("tag %d: value %v outside [%v, %v]", it.Tag, f, td.Scale.Min, td.Scale.Max),
					Tag:      &tag,
				})
			}
		}
		// Enum validation.
		// Note: tags that decode to EnumValue (via custom Decode) already validate
		// their codes during decode. Only IntValue tags with an Enum map are checked here.
		if td.Enum != nil {
			if iv, ok := it.Value.(record.IntValue); ok {
				if _, exists := td.Enum[int64(iv)]; !exists {
					tag := it.Tag
					diags = append(diags, record.Diagnostic{
						Severity: "error",
						Code:     "tag_enum_invalid",
						Message:  fmt.Sprintf("tag %d: value %d not in enum", it.Tag, int64(iv)),
						Tag:      &tag,
					})
				}
			}
		}
	}

	// 5. Checksum. Only emit when tag 1 is present with the correct 2-byte
	// length (i.e. the engine actually computed a checksum) and the
	// computed value disagrees with the wire. When tag 1 is missing or
	// malformed the Record's ChecksumInfo is zero-valued (Valid = false)
	// but we must not report a mismatch — missing_mandatory_item and
	// tag_length_mismatch above already cover those cases.
	if present[1] && !rec.Checksum.Valid {
		var tag1Raw []byte
		for _, it := range rec.Items {
			if it.Tag == 1 {
				tag1Raw = it.Raw
				break
			}
		}
		if len(tag1Raw) == 2 {
			one := 1
			diags = append(diags, record.Diagnostic{
				Severity: "error",
				Code:     "checksum_mismatch",
				Message:  fmt.Sprintf("checksum expected=0x%04X computed=0x%04X", rec.Checksum.Expected, rec.Checksum.Computed),
				Tag:      &one,
			})
		}
	}

	return diags
}
