package klv

import (
	"encoding/binary"
	"fmt"
	"math"
)

// toIMAPB encodes a floating-point value v in the inclusive range [a, b]
// as an L-byte unsigned big-endian integer per MISB ST 1201 §8.
// This is a simplified IMAPB variant suitable for the linear-mapping tags
// present in MISB ST 0601 (most tags).
func toIMAPB(a, b float64, length int, v float64) ([]byte, error) {
	if length <= 0 || length > 8 {
		return nil, fmt.Errorf("imapb: length %d out of range", length)
	}
	if b <= a {
		return nil, fmt.Errorf("imapb: invalid range [%v, %v]", a, b)
	}
	maxInt := math.Pow(2, float64(8*length)) - 1
	// Clamp to range.
	if v < a {
		v = a
	}
	if v > b {
		v = b
	}
	// Linear map: encoded = (v - a) / (b - a) * maxInt
	encoded := uint64(math.Round((v - a) / (b - a) * maxInt))
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, encoded)
	return buf[8-length:], nil
}

// fromIMAPB decodes an L-byte unsigned big-endian integer into a
// floating-point value in [a, b] per MISB ST 1201 §8.
func fromIMAPB(a, b float64, length int, raw []byte) (float64, error) {
	if length <= 0 || length > 8 {
		return 0, fmt.Errorf("imapb: length %d out of range", length)
	}
	if len(raw) != length {
		return 0, fmt.Errorf("imapb: expected %d bytes, got %d", length, len(raw))
	}
	if b <= a {
		return 0, fmt.Errorf("imapb: invalid range [%v, %v]", a, b)
	}
	// Read big-endian unsigned integer of `length` bytes.
	var encoded uint64
	for _, byte_ := range raw {
		encoded = (encoded << 8) | uint64(byte_)
	}
	maxInt := math.Pow(2, float64(8*length)) - 1
	return a + float64(encoded)/maxInt*(b-a), nil
}
