package klv

import (
	"testing"
	"time"

	"github.com/jacorbello/klvtool/internal/klv/record"
	"github.com/jacorbello/klvtool/internal/klv/specs/st0601"
)

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
	body = append(body, 0x01)       // tag 1
	body = append(body, 0x02)       // length 2
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

	// Include tag 99 (not in the phase-1 minimal set) with arbitrary bytes.
	packet := buildPacket(t, map[int][]byte{
		2:  make([]byte, 8),
		65: {19},
		99: {0xde, 0xad, 0xbe, 0xef},
	})

	rec, err := Decode(reg, packet)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	var found bool
	for _, it := range rec.Items {
		if it.Tag == 99 {
			found = true
			if _, ok := it.Value.(record.BytesValue); !ok {
				t.Errorf("tag 99 value type = %T, want BytesValue", it.Value)
			}
			if it.Name != "Unknown Tag 99" {
				t.Errorf("tag 99 name = %s", it.Name)
			}
		}
	}
	if !found {
		t.Errorf("tag 99 not found in items")
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
	reg := NewRegistry() // empty
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
