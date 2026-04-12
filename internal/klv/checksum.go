package klv

// computeChecksum implements the bcc_16 algorithm from MISB ST 0601 §6.6.
// Each byte is added to a running 16-bit sum, shifted left by 8*((i+1) mod 2)
// so even-index bytes land in the high byte and odd-index bytes in the low.
// The caller supplies the exact byte range to checksum (UL key through the
// length byte of the checksum item per §6.6).
func computeChecksum(buf []byte) uint16 {
	var bcc uint16
	for i, b := range buf {
		bcc += uint16(b) << (8 * uint((i+1)%2))
	}
	return bcc
}
