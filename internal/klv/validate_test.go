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
