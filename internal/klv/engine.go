package klv

import (
	"encoding/binary"
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/jacorbello/klvtool/internal/klv/record"
	"github.com/jacorbello/klvtool/internal/klv/specs"
	"github.com/jacorbello/klvtool/internal/model"
)

const ulSize = 16

// Decode parses a full KLV packet: 16-byte UL + BER length + value bytes.
// Returns a Record with typed items, or a top-level error only on structural
// decode failures (truncation, malformed BER length, bad BER-OID). Per-tag
// issues, unknown specs, and validation failures are reported via the
// Record's Diagnostics field.
func Decode(reg *Registry, full []byte) (record.Record, error) {
	if len(full) < ulSize+1 {
		return record.Record{}, model.KLVDecode(fmt.Errorf("packet too short (%d bytes)", len(full)))
	}
	ul := full[:ulSize]
	length, n, err := decodeBERLength(full[ulSize:])
	if err != nil {
		return record.Record{}, model.KLVDecode(fmt.Errorf("ber length: %w", err))
	}
	valueStart := ulSize + n
	if valueStart+length > len(full) {
		return record.Record{}, model.KLVDecode(fmt.Errorf("value length %d exceeds packet (%d bytes remaining)", length, len(full)-valueStart))
	}
	value := full[valueStart : valueStart+length]
	return decodeLocalSetInternal(reg, ul, value, full[:valueStart+length], valueStart)
}

// DecodeLocalSet parses a pre-split packet. Caller supplies the UL key,
// the original wire BER length bytes, and the value bytes; used by CLI
// pathways that have already consumed the outer triplet (e.g. via
// packetize.Parse). The exact length bytes are required because the
// checksum covers the wire representation — a reconstructed minimal-form
// BER header would mis-validate any packet that used a valid non-canonical
// long-form length encoding.
func DecodeLocalSet(reg *Registry, ul []byte, lengthBytes []byte, value []byte) (record.Record, error) {
	valueStart := len(ul) + len(lengthBytes)
	full := make([]byte, 0, valueStart+len(value))
	full = append(full, ul...)
	full = append(full, lengthBytes...)
	full = append(full, value...)
	return decodeLocalSetInternal(reg, ul, value, full, valueStart)
}

func decodeLocalSetInternal(reg *Registry, ul []byte, value []byte, fullForChecksum []byte, valueStart int) (record.Record, error) {
	rec := record.Record{
		UL:          append([]byte{}, ul...),
		LSVersion:   -1,
		ValueLength: len(value),
		Items:       []record.Item{},
		Diagnostics: []record.Diagnostic{},
	}

	spec, err := reg.Resolve(ul, value)
	if err != nil {
		rec.Diagnostics = append(rec.Diagnostics, record.Diagnostic{
			Severity: "error",
			Code:     "unknown_spec",
			Message:  err.Error(),
		})
		return rec, nil
	}
	rec.Schema = spec.URN()

	// Walk the TLV stream.
	cursor := 0
	var checksumFromTag uint16
	var checksumValueStart int // byte offset in fullForChecksum where Tag 1's value begins; checksum range is fullForChecksum[:checksumValueStart]
	var sawChecksum bool
	for cursor < len(value) {
		tag, tagN, terr := decodeBEROID(value[cursor:])
		if terr != nil {
			return rec, model.KLVDecode(fmt.Errorf("ber-oid at offset %d: %w", cursor, terr))
		}
		cursor += tagN

		length, lenN, lerr := decodeBERLength(value[cursor:])
		if lerr != nil {
			return rec, model.KLVDecode(fmt.Errorf("ber length at offset %d: %w", cursor, lerr))
		}
		cursor += lenN
		if cursor+length > len(value) {
			return rec, model.KLVDecode(fmt.Errorf("tag %d value extends past packet", tag))
		}
		raw := value[cursor : cursor+length]
		cursor += length

		td, known := spec.Tag(tag)
		if !known {
			tagCopy := tag
			rec.Items = append(rec.Items, record.Item{
				Tag:   tag,
				Name:  fmt.Sprintf("Unknown Tag %d", tag),
				Value: record.BytesValue(append([]byte{}, raw...)),
				Raw:   append([]byte{}, raw...),
			})
			rec.Diagnostics = append(rec.Diagnostics, record.Diagnostic{
				Severity: "warning",
				Code:     "unknown_tag",
				Message:  fmt.Sprintf("tag %d not defined in spec %s", tag, spec.URN()),
				Tag:      &tagCopy,
			})
			continue
		}

		// Tag 65: capture LS version for later validation.
		if tag == spec.VersionTag() && len(raw) >= 1 {
			rec.LSVersion = int(raw[0])
		}
		// Tag 1: capture checksum value for comparison with computed.
		// cursor is now past the length bytes but before the value bytes have been
		// consumed (raw was sliced before cursor += length). Record the byte
		// offset within fullForChecksum at the start of Tag 1's value bytes.
		if tag == 1 && len(raw) == 2 {
			checksumFromTag = binary.BigEndian.Uint16(raw)
			sawChecksum = true
			checksumValueStart = valueStart + cursor - length // points to start of tag 1 value
		}

		val, derr := dispatchDecode(td, raw)
		item := record.Item{
			Tag:   tag,
			Name:  td.Name,
			Value: val,
			Units: td.Units,
			Raw:   append([]byte{}, raw...),
		}
		if derr != nil {
			tagCopy := tag
			diag := record.Diagnostic{
				Severity: "error",
				Code:     "tag_decode_error",
				Message:  derr.Error(),
				Tag:      &tagCopy,
				TagName:  td.Name,
				Raw:      formatDiagnosticRaw(raw),
			}
			if details, ok := derr.(diagnosticContextError); ok {
				diag.Actual = details.actual
				diag.Expected = details.expected
			}
			rec.Diagnostics = append(rec.Diagnostics, diag)
		}
		rec.Items = append(rec.Items, item)
	}

	// Compute checksum over fullForChecksum[:checksumValueStart] — everything
	// up to but not including Tag 1's value bytes. When we didn't see Tag 1
	// we can't validate.
	rec.Checksum.Expected = checksumFromTag
	if sawChecksum {
		rec.Checksum.Computed = computeChecksum(fullForChecksum[:checksumValueStart])
		rec.Checksum.Valid = rec.Checksum.Computed == rec.Checksum.Expected
	}

	rec.Diagnostics = append(rec.Diagnostics, Validate(spec, &rec)...)
	return rec, nil
}

// DecodeTag runs the tag's Decode function if set, otherwise dispatches by Format/Scale.
func DecodeTag(td specs.TagDefinition, raw []byte) (record.Value, error) {
	return dispatchDecode(td, raw)
}

// dispatchDecode runs the Decode function pointer if set, otherwise dispatches
// by Format.
func dispatchDecode(td specs.TagDefinition, raw []byte) (record.Value, error) {
	if td.Decode != nil {
		return td.Decode(raw)
	}
	switch td.Format {
	case specs.FormatUint8, specs.FormatUint16, specs.FormatUint32, specs.FormatUint64:
		u, err := decodeUint(raw, td.Format)
		if err != nil {
			return nil, err
		}
		if td.Scale != nil {
			return applyScaleUnsigned(u, td.Format, *td.Scale), nil
		}
		return record.UintValue(u), nil
	case specs.FormatInt8, specs.FormatInt16, specs.FormatInt32, specs.FormatInt64:
		s, err := decodeInt(raw, td.Format)
		if err != nil {
			return nil, err
		}
		if td.Scale != nil {
			return applyScaleSigned(s, td.Format, *td.Scale)
		}
		return record.IntValue(s), nil
	case specs.FormatIMAPB:
		if td.Scale == nil {
			return nil, fmt.Errorf("tag %d: FormatIMAPB requires Scale", td.Tag)
		}
		if td.Length > 0 && len(raw) != td.Length {
			return nil, fmt.Errorf("tag %d: expected %d bytes, got %d", td.Tag, td.Length, len(raw))
		}
		f, err := fromIMAPB(td.Scale.Min, td.Scale.Max, len(raw), raw)
		if err != nil {
			return nil, err
		}
		return record.FloatValue(f), nil
	case specs.FormatUTF8:
		if !utf8.Valid(raw) {
			return record.BytesValue(append([]byte{}, raw...)), fmt.Errorf("invalid utf-8")
		}
		return record.StringValue(string(raw)), nil
	case specs.FormatBytes:
		return record.BytesValue(append([]byte{}, raw...)), nil
	case specs.FormatNone:
		return nil, fmt.Errorf("tag %d: no decoder (Format=None and Decode=nil)", td.Tag)
	default:
		return nil, fmt.Errorf("tag %d: unsupported format %d", td.Tag, td.Format)
	}
}

func decodeUint(raw []byte, format specs.Format) (uint64, error) {
	expected := map[specs.Format]int{
		specs.FormatUint8:  1,
		specs.FormatUint16: 2,
		specs.FormatUint32: 4,
		specs.FormatUint64: 8,
	}[format]
	if len(raw) != expected {
		return 0, fmt.Errorf("expected %d bytes, got %d", expected, len(raw))
	}
	var v uint64
	for _, b := range raw {
		v = (v << 8) | uint64(b)
	}
	return v, nil
}

func decodeInt(raw []byte, format specs.Format) (int64, error) {
	expected := map[specs.Format]int{
		specs.FormatInt8:  1,
		specs.FormatInt16: 2,
		specs.FormatInt32: 4,
		specs.FormatInt64: 8,
	}[format]
	if len(raw) != expected {
		return 0, fmt.Errorf("expected %d bytes, got %d", expected, len(raw))
	}
	var v uint64
	for _, b := range raw {
		v = (v << 8) | uint64(b)
	}
	// Sign-extend.
	bits := uint(expected * 8)
	mask := uint64(1) << (bits - 1)
	if v&mask != 0 {
		v |= ^((uint64(1) << bits) - 1)
	}
	return int64(v), nil
}

// applyScaleUnsigned maps an unsigned integer onto [Min, Max] linearly.
// For an L-byte unsigned integer, encoded 0 → Min, encoded 2^(8L)-1 → Max.
func applyScaleUnsigned(v uint64, format specs.Format, scale specs.LinearScale) record.Value {
	bytes := map[specs.Format]int{
		specs.FormatUint8: 1, specs.FormatUint16: 2,
		specs.FormatUint32: 4, specs.FormatUint64: 8,
	}[format]
	maxInt := math.Pow(2, float64(8*bytes)) - 1
	f := scale.Min + float64(v)/maxInt*(scale.Max-scale.Min)
	return record.FloatValue(f)
}

// applyScaleSigned maps a signed integer onto [Min, Max] linearly.
// For an L-byte signed integer, encoded -(2^(8L-1)-1) → Min, encoded 2^(8L-1)-1 → Max.
// If ErrorIndicator is true, the encoded value -2^(8L-1) (0x80..0) returns a
// NaN FloatValue together with an error; current callers surface that as a
// tag_decode_error diagnostic via dispatchDecode.
func applyScaleSigned(v int64, format specs.Format, scale specs.LinearScale) (record.Value, error) {
	bytes := map[specs.Format]int{
		specs.FormatInt8: 1, specs.FormatInt16: 2,
		specs.FormatInt32: 4, specs.FormatInt64: 8,
	}[format]
	halfRange := math.Pow(2, float64(8*bytes-1)) - 1
	minEncoded := -halfRange - 1
	if scale.ErrorIndicator && float64(v) == minEncoded {
		return record.FloatValue(math.NaN()), diagnosticContextError{
			message:  "value out of range (error indicator)",
			actual:   fmt.Sprintf("%d", v),
			expected: fmt.Sprintf("physical range [%g, %g]; encoded minimum %d is reserved as error indicator", scale.Min, scale.Max, int64(minEncoded)),
		}
	}
	f := float64(v) / halfRange * ((scale.Max - scale.Min) / 2)
	// Center on (Min+Max)/2.
	f += (scale.Min + scale.Max) / 2
	return record.FloatValue(f), nil
}

type diagnosticContextError struct {
	message  string
	actual   string
	expected string
}

func (e diagnosticContextError) Error() string { return e.message }

func formatDiagnosticRaw(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	return fmt.Sprintf("0x%x", raw)
}
