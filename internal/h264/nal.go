package h264

// IterateNALUnits walks an H.264 Annex B byte stream and calls visit
// once per NAL unit found. nalType is the nal_unit_type value from the
// unit's header byte (low 5 bits). body is the unit payload after the
// header byte, up to (but not including) the next start code. The slice
// aliases the input — callers must copy if they need to retain it.
//
// Handles both 3-byte (0x000001) and 4-byte (0x00000001) start codes.
// Emulation prevention bytes are not removed: the analyzer only needs
// nal_unit_type from byte 0, which is unaffected.
func IterateNALUnits(data []byte, visit func(nalType uint8, body []byte)) {
	if len(data) < 3 || visit == nil {
		return
	}

	i := findStartCode(data, 0)
	for i >= 0 {
		headerStart := i + startCodeLen(data, i)
		if headerStart >= len(data) {
			return
		}
		next := findStartCode(data, headerStart+1)
		var body []byte
		bodyStart := headerStart + 1
		switch {
		case next < 0:
			if bodyStart <= len(data) {
				body = data[bodyStart:]
			}
		case bodyStart <= next:
			body = data[bodyStart:next]
		}
		visit(data[headerStart]&0x1F, body)
		i = next
	}
}

// findStartCode returns the index of the next Annex B start code at or
// after start. Returns -1 if no start code is found. A start code is
// either 0x000001 (3 bytes) or 0x00000001 (4 bytes); when both are
// possible at the same position, the earliest 0x00 byte is returned so
// startCodeLen can distinguish them.
func findStartCode(data []byte, start int) int {
	for i := start; i+2 < len(data); i++ {
		if data[i] != 0x00 || data[i+1] != 0x00 {
			continue
		}
		if data[i+2] == 0x01 {
			return i
		}
		if i+3 < len(data) && data[i+2] == 0x00 && data[i+3] == 0x01 {
			return i
		}
	}
	return -1
}

// startCodeLen returns the byte length of the Annex B start code at
// position i in data (either 3 or 4). The caller must have already
// verified via findStartCode that a start code exists at i.
func startCodeLen(data []byte, i int) int {
	if i+3 < len(data) && data[i+2] == 0x00 && data[i+3] == 0x01 {
		return 4
	}
	return 3
}
