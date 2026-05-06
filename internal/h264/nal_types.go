// Package h264 provides a minimal H.264 NAL unit scanner and stream
// analyzer used by klvtool's video playability diagnostics. It reads
// only what is needed to decide whether an MSE player can begin
// playback (IDR/SPS/PPS presence, IDR cadence, frame-drop timing).
package h264

// NAL unit types from ITU-T H.264 Table 7-1.
const (
	NALSlice uint8 = 1 // Coded slice of a non-IDR picture
	NALIDR   uint8 = 5 // Coded slice of an IDR picture
	NALSEI   uint8 = 6 // Supplemental enhancement information
	NALSPS   uint8 = 7 // Sequence parameter set
	NALPPS   uint8 = 8 // Picture parameter set
	NALAUD   uint8 = 9 // Access unit delimiter
)

// NALTypeName returns a short human-readable name for a NAL unit type.
func NALTypeName(t uint8) string {
	switch t {
	case NALSlice:
		return "non-IDR slice"
	case 2:
		return "slice data partition A"
	case 3:
		return "slice data partition B"
	case 4:
		return "slice data partition C"
	case NALIDR:
		return "IDR slice"
	case NALSEI:
		return "SEI"
	case NALSPS:
		return "SPS"
	case NALPPS:
		return "PPS"
	case NALAUD:
		return "AUD"
	}
	return "unknown"
}
