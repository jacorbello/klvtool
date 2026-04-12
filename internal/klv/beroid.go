package klv

import "fmt"

// maxBEROID caps the decoded BER-OID tag value. MISB ST 0601 assigns tags in
// a small numeric range (currently < 2^14); 2^28 is several orders of
// magnitude beyond any real tag and still comfortably inside a 32-bit int, so
// the decoder can reject nonsense encodings without tripping over platform
// int width. Without this cap, a malicious 5-byte encoding can overflow
// 32-bit int (5 * 7 = 35 bits of value) and silently alias a real tag.
const maxBEROID = 1 << 28

// decodeBEROID decodes a BER-OID encoded tag number from the start of input.
// Returns the decoded tag, the number of bytes consumed, and an error if
// the encoding is truncated, malformed, or exceeds maxBEROID.
//
// BER-OID encoding uses the high bit of each byte as a continuation flag:
// bytes with the high bit set (0x80) indicate more bytes follow; the final
// byte has the high bit clear. The remaining 7 bits of each byte form the
// value, concatenated MSB-first.
func decodeBEROID(input []byte) (int, int, error) {
	if len(input) == 0 {
		return 0, 0, fmt.Errorf("ber-oid: empty input")
	}
	tag := 0
	for i, b := range input {
		tag = (tag << 7) | int(b&0x7F)
		if tag > maxBEROID {
			return 0, 0, fmt.Errorf("ber-oid: value exceeds cap")
		}
		if b&0x80 == 0 {
			return tag, i + 1, nil
		}
		if i == 8 {
			// Belt-and-braces: 9 bytes would overflow int64 for the 7-bit groups.
			return 0, 0, fmt.Errorf("ber-oid: encoding too long")
		}
	}
	return 0, 0, fmt.Errorf("ber-oid: truncated continuation")
}
