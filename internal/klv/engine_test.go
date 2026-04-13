package klv

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/jacorbello/klvtool/internal/klv/record"
	"github.com/jacorbello/klvtool/internal/klv/specs"
	"github.com/jacorbello/klvtool/internal/klv/specs/st0601"
)

// TestDecodeErrorIndicatorRecordMarshals covers the end-to-end path the
// reviewer flagged: a real 0601 packet with a signed error-indicator
// sentinel (e.g. Platform Pitch Angle = 0x8000) must still marshal to
// NDJSON cleanly, emitting JSON null for the NaN value instead of
// crashing json.Marshal and exiting the whole decode run.
func TestDecodeErrorIndicatorRecordMarshals(t *testing.T) {
	reg := NewRegistry()
	reg.Register(st0601.V19())

	pts := uint64(time.Date(2023, 3, 2, 12, 34, 56, 789_000_000, time.UTC).UnixNano() / 1000)
	ptsBytes := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		ptsBytes[i] = byte(pts & 0xFF)
		pts >>= 8
	}

	packet := buildPacket(t, map[int][]byte{
		2:  ptsBytes,
		5:  {0x80, 0x00}, // Platform Heading Angle — uint16, not error indicator (skip)
		6:  {0x80, 0x00}, // Platform Pitch Angle — int16 error indicator
		65: {19},
	})

	rec, err := Decode(reg, packet)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if _, err := json.Marshal(rec); err != nil {
		t.Fatalf("json.Marshal of record with error indicator failed: %v", err)
	}
}

// buildPacket constructs a complete ST 0601 LS packet with the given items.
// Handles UL key prefix, BER length, BER-OID tags, and computes the checksum
// so tests don't have to.
func buildPacket(t *testing.T, items map[int][]byte) []byte {
	t.Helper()
	// Deterministic iteration: put Tag 2 first, then everything except Tag 1
	// in ascending order, then Tag 1 last.
	var body []byte
	appendItem := func(tag int, val []byte) {
		body = append(body, encodeBEROID(tag)...)
		body = append(body, encodeBERLength(len(val))...)
		body = append(body, val...)
	}
	if val, ok := items[2]; ok {
		appendItem(2, val)
	}
	for tag := 3; tag < 200; tag++ {
		if tag == 1 {
			continue
		}
		if val, ok := items[tag]; ok {
			appendItem(tag, val)
		}
	}
	// Placeholder for checksum (Tag 1 with 2-byte value). We need the full
	// bytes through "length of checksum item" to compute the checksum.
	body = append(body, 0x01) // tag 1
	body = append(body, 0x02) // length 2
	// The UL key + BER length + body (including tag 1, length 1) is what
	// gets summed. We now know the full length.
	valueLen := len(body) + 2 // +2 for the checksum value we're about to add
	ul := st0601.UASDatalinkUL
	header := append([]byte{}, ul...)
	header = append(header, encodeBERLength(valueLen)...)
	// Compute checksum over header + body (body already includes tag 1 + length).
	sumRange := append([]byte{}, header...)
	sumRange = append(sumRange, body...)
	sum := computeChecksum(sumRange)
	checksumBytes := []byte{byte(sum >> 8), byte(sum)}
	body = append(body, checksumBytes...)
	full := append(header, body...)
	return full
}

// encodeBEROID is the inverse of decodeBEROID, for test packet construction.
func encodeBEROID(tag int) []byte {
	if tag < 0x80 {
		return []byte{byte(tag)}
	}
	var out []byte
	for tag > 0 {
		b := byte(tag & 0x7F)
		if len(out) > 0 {
			b |= 0x80
		}
		out = append([]byte{b}, out...)
		tag >>= 7
	}
	// Set continuation on all but the last byte.
	for i := 0; i < len(out)-1; i++ {
		out[i] |= 0x80
	}
	return out
}

// encodeBERLength writes a BER short/long length.
func encodeBERLength(length int) []byte {
	if length < 0x80 {
		return []byte{byte(length)}
	}
	// Long form: find the minimum number of bytes needed.
	var bytes_ []byte
	for length > 0 {
		bytes_ = append([]byte{byte(length & 0xFF)}, bytes_...)
		length >>= 8
	}
	return append([]byte{0x80 | byte(len(bytes_))}, bytes_...)
}

// buildLongFormPacket assembles a full ST 0601 LS packet whose outer BER
// length uses long-form encoding (0x81 LL) even though the value fits in
// short form. This bypasses buildPacket's encodeBERLength helper, which
// always picks the shortest form. Returns the full packet plus the parts
// (UL, longFormLen header, body-with-checksum) that DecodeLocalSet expects,
// so callers can exercise either the top-level Decode entry point or the
// lower-level DecodeLocalSet split.
//
// The items arg is identical in shape to buildPacket's: tag 2 is emitted
// first, tags 3-199 in ascending order, and tag 1 (checksum) is appended
// last with its value computed over the wire header+body.
func buildLongFormPacket(t *testing.T, items map[int][]byte) (packet, ul, longFormLen, body []byte) {
	t.Helper()
	appendItem := func(dst []byte, tag int, val []byte) []byte {
		dst = append(dst, encodeBEROID(tag)...)
		dst = append(dst, encodeBERLength(len(val))...)
		return append(dst, val...)
	}
	body = make([]byte, 0, 64)
	if val, ok := items[2]; ok {
		body = appendItem(body, 2, val)
	}
	for tag := 3; tag < 200; tag++ {
		if val, ok := items[tag]; ok {
			body = appendItem(body, tag, val)
		}
	}
	body = append(body, 0x01, 0x02) // Tag 1 (checksum), length 2 placeholder

	valueLen := len(body) + 2 // +2 for checksum value bytes to be appended
	if valueLen >= 0x80 {
		t.Fatalf("buildLongFormPacket: value len must be < 0x80 for long-form test; got %d", valueLen)
	}
	// Force long-form: 0x81 <1-byte length>. encodeBERLength picks short form.
	longFormLen = []byte{0x81, byte(valueLen)}
	ul = st0601.UASDatalinkUL

	sumRange := make([]byte, 0, len(ul)+len(longFormLen)+len(body))
	sumRange = append(sumRange, ul...)
	sumRange = append(sumRange, longFormLen...)
	sumRange = append(sumRange, body...)
	sum := computeChecksum(sumRange)
	body = append(body, byte(sum>>8), byte(sum))

	packet = make([]byte, 0, len(ul)+len(longFormLen)+len(body))
	packet = append(packet, ul...)
	packet = append(packet, longFormLen...)
	packet = append(packet, body...)
	return packet, ul, longFormLen, body
}

// TestDecodeLocalSetPreservesNonCanonicalBERLength verifies the checksum
// validates when the wire used a valid but non-canonical (long-form) BER
// length encoding. The CLI path splits the outer triplet via packetize
// before calling DecodeLocalSet, so DecodeLocalSet must receive the exact
// original length bytes rather than reconstruct a minimal header.
func TestDecodeLocalSetPreservesNonCanonicalBERLength(t *testing.T) {
	reg := NewRegistry()
	reg.Register(st0601.V19())

	_, ul, longFormLen, body := buildLongFormPacket(t, map[int][]byte{
		2:  make([]byte, 8),
		65: {19},
	})

	// Caller (e.g. CLI via packetize) supplies the exact wire length bytes.
	rec, err := DecodeLocalSet(reg, ul, longFormLen, body)
	if err != nil {
		t.Fatalf("DecodeLocalSet error: %v", err)
	}
	if !rec.Checksum.Valid {
		t.Errorf("checksum invalid: expected=%04x computed=%04x (non-canonical BER length must roundtrip)",
			rec.Checksum.Expected, rec.Checksum.Computed)
	}
}

// TestCoordinateTagsMatchSpecExamples validates that coordinate tags decode
// to the software values published in ST 0601.19 Section 8 within the
// allowed tolerance for the spec-defined encoded resolution.
func TestCoordinateTagsMatchSpecExamples(t *testing.T) {
	sv := st0601.V19()
	tests := []struct {
		tag       int
		name      string
		raw       []byte
		want      float64
		tolerance float64
	}{
		{13, "Sensor Latitude", []byte{0x55, 0x95, 0xB6, 0x6D}, 60.176822966978335, 42e-9},
		{14, "Sensor Longitude", []byte{0x5B, 0x53, 0x60, 0xC4}, 128.42675904204452, 84e-9},
		{23, "Frame Center Latitude", []byte{0xF1, 0x01, 0xA2, 0x29}, -10.542388633146132, 42e-9},
		{24, "Frame Center Longitude", []byte{0x14, 0xBC, 0x08, 0x2B}, 29.157890122923014, 84e-9},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td, ok := sv.Tag(tt.tag)
			if !ok {
				t.Fatalf("tag %d missing", tt.tag)
			}
			v, err := dispatchDecode(td, tt.raw)
			if err != nil {
				t.Fatalf("dispatchDecode: %v", err)
			}
			fv, ok := v.(record.FloatValue)
			if !ok {
				t.Fatalf("value type = %T, want FloatValue", v)
			}
			if diff := math.Abs(float64(fv) - tt.want); diff > tt.tolerance {
				t.Errorf("decoded = %.15f, want %.15f (diff %e, tolerance %e)",
					float64(fv), tt.want, diff, tt.tolerance)
			}
		})
	}
}

func TestDecodeLocalSetMandatoryItems(t *testing.T) {
	reg := NewRegistry()
	reg.Register(st0601.V19())

	// Precision Time Stamp: 2023-03-02T12:34:56.789Z as MISP microseconds.
	pts := uint64(time.Date(2023, 3, 2, 12, 34, 56, 789_000_000, time.UTC).UnixNano() / 1000)
	ptsBytes := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		ptsBytes[i] = byte(pts & 0xFF)
		pts >>= 8
	}

	packet := buildPacket(t, map[int][]byte{
		2:  ptsBytes,
		65: {19},
	})

	rec, err := Decode(reg, packet)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if rec.Schema != "urn:misb:KLV:bin:0601.19" {
		t.Errorf("Schema = %s", rec.Schema)
	}
	if rec.LSVersion != 19 {
		t.Errorf("LSVersion = %d, want 19", rec.LSVersion)
	}
	if !rec.Checksum.Valid {
		t.Errorf("Checksum invalid: expected=%04x computed=%04x",
			rec.Checksum.Expected, rec.Checksum.Computed)
	}
	// Expect three items: tag 2, tag 65, tag 1.
	if len(rec.Items) != 3 {
		t.Fatalf("Items count = %d, want 3", len(rec.Items))
	}
	if rec.Items[0].Tag != 2 {
		t.Errorf("first item tag = %d, want 2", rec.Items[0].Tag)
	}
	if _, ok := rec.Items[0].Value.(record.TimeValue); !ok {
		t.Errorf("tag 2 value type = %T, want TimeValue", rec.Items[0].Value)
	}
	if rec.Items[len(rec.Items)-1].Tag != 1 {
		t.Errorf("last item tag = %d, want 1", rec.Items[len(rec.Items)-1].Tag)
	}
}

func TestDecodeUnknownTagPassthrough(t *testing.T) {
	reg := NewRegistry()
	reg.Register(st0601.V19())

	// Include tag 50 (not defined in phase-1) with arbitrary bytes.
	packet := buildPacket(t, map[int][]byte{
		2:  make([]byte, 8),
		65: {19},
		50: {0xde, 0xad, 0xbe, 0xef},
	})

	rec, err := Decode(reg, packet)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	var found bool
	for _, it := range rec.Items {
		if it.Tag == 50 {
			found = true
			if _, ok := it.Value.(record.BytesValue); !ok {
				t.Errorf("tag 50 value type = %T, want BytesValue", it.Value)
			}
			if it.Name != "Unknown Tag 50" {
				t.Errorf("tag 50 name = %s", it.Name)
			}
		}
	}
	if !found {
		t.Errorf("tag 50 not found in items")
	}
	// Expect an unknown_tag warning diagnostic.
	var hasWarning bool
	for _, d := range rec.Diagnostics {
		if d.Code == "unknown_tag" {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Errorf("expected unknown_tag diagnostic")
	}
}

func TestDecodeUnknownSpec(t *testing.T) {
	reg := NewRegistry()         // empty
	packet := make([]byte, 16+1) // just a UL of zeros and an empty length byte
	rec, err := Decode(reg, packet)
	if err != nil {
		t.Fatalf("Decode should not return top-level error for unknown spec, got %v", err)
	}
	var found bool
	for _, d := range rec.Diagnostics {
		if d.Code == "unknown_spec" && d.Severity == "error" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unknown_spec error diagnostic")
	}
}

func TestDecodeTruncatedPacket(t *testing.T) {
	reg := NewRegistry()
	reg.Register(st0601.V19())
	// UL key + length byte says 10 but only 2 bytes follow.
	truncated := append([]byte{}, st0601.UASDatalinkUL...)
	truncated = append(truncated, 0x0A, 0x01, 0x02)
	_, err := Decode(reg, truncated)
	if err == nil {
		t.Errorf("expected error for truncated packet")
	}
}

func TestDispatchDecodeIMAPB(t *testing.T) {
	td := specs.TagDefinition{
		Tag: 100, Name: "Test IMAPB", Format: specs.FormatIMAPB, Length: 2,
		Scale: &specs.LinearScale{Min: 0, Max: 360},
	}
	// Encoded 0xFFFF over 0..360 should map to ~360.
	v, err := dispatchDecode(td, []byte{0xFF, 0xFF})
	if err != nil {
		t.Fatalf("dispatchDecode IMAPB: %v", err)
	}
	fv, ok := v.(record.FloatValue)
	if !ok {
		t.Fatalf("got %T, want FloatValue", v)
	}
	if float64(fv) < 359.99 || float64(fv) > 360.001 {
		t.Errorf("IMAPB 0xFFFF over 0..360 = %v, want ~360", float64(fv))
	}
	// Missing Scale is an error.
	bad := specs.TagDefinition{Tag: 101, Name: "Bad", Format: specs.FormatIMAPB, Length: 2}
	if _, err := dispatchDecode(bad, []byte{0x00, 0x00}); err == nil {
		t.Errorf("expected error when Scale is nil")
	}
}

func TestDispatchDecodeAppliesScale(t *testing.T) {
	// Unsigned 0..360 over uint16 — encoded 0xFFFF ≈ 360°
	unsignedTD := specs.TagDefinition{
		Tag: 5, Name: "Test Heading", Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: 0, Max: 360},
	}
	v, err := dispatchDecode(unsignedTD, []byte{0xFF, 0xFF})
	if err != nil {
		t.Fatalf("unsigned scale: %v", err)
	}
	fv, ok := v.(record.FloatValue)
	if !ok {
		t.Fatalf("unsigned scale: got %T, want FloatValue", v)
	}
	if float64(fv) < 359.99 || float64(fv) > 360.001 {
		t.Errorf("unsigned scale: got %v, want ~360", float64(fv))
	}

	// Signed -20..20 over int16 — encoded 0 → 0°
	signedTD := specs.TagDefinition{
		Tag: 6, Name: "Test Pitch", Format: specs.FormatInt16, Length: 2,
		Scale: &specs.LinearScale{Min: -20, Max: 20, ErrorIndicator: true},
	}
	v, err = dispatchDecode(signedTD, []byte{0x00, 0x00})
	if err != nil {
		t.Fatalf("signed scale: %v", err)
	}
	fv, ok = v.(record.FloatValue)
	if !ok {
		t.Fatalf("signed scale: got %T, want FloatValue", v)
	}
	if float64(fv) < -0.001 || float64(fv) > 0.001 {
		t.Errorf("signed scale: got %v, want ~0", float64(fv))
	}

	// Signed error indicator: -2^15 → NaN + error
	v, err = dispatchDecode(signedTD, []byte{0x80, 0x00})
	if err == nil {
		t.Errorf("signed error indicator: expected error")
	}
	fv, ok = v.(record.FloatValue)
	if !ok {
		t.Fatalf("error indicator: got %T, want FloatValue", v)
	}
	if !math.IsNaN(float64(fv)) {
		t.Errorf("error indicator: got %v, want NaN", float64(fv))
	}
}
