package st0601

import (
	"encoding/binary"
	"fmt"
	"sort"
	"time"

	"github.com/jacorbello/klvtool/internal/klv/record"
	"github.com/jacorbello/klvtool/internal/klv/specs"
)

// V19 returns the ST 0601.19 SpecVersion.
func V19() specs.SpecVersion {
	return &v19{tags: v19Tags()}
}

type v19 struct {
	tags map[int]specs.TagDefinition
}

func (v *v19) URN() string                           { return "urn:misb:KLV:bin:0601.19" }
func (v *v19) UL() []byte                            { return UASDatalinkUL }
func (v *v19) VersionTag() int                       { return 65 }
func (v *v19) ExpectedVersion() int                  { return 19 }
func (v *v19) Tag(n int) (specs.TagDefinition, bool) { t, ok := v.tags[n]; return t, ok }

// AllTags returns tag definitions sorted by tag number so downstream
// consumers (e.g. validate.Validate) see deterministic iteration order.
func (v *v19) AllTags() []specs.TagDefinition {
	out := make([]specs.TagDefinition, 0, len(v.tags))
	for _, t := range v.tags {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Tag < out[j].Tag })
	return out
}

// v19Tags returns the phase-1 tag table for ST 0601.19 — mandatory items,
// declarative core subset, and custom-decode tags (timestamps, enums, nested local sets).
func v19Tags() map[int]specs.TagDefinition {
	tags := map[int]specs.TagDefinition{}

	// ---- Mandatory items ----
	tags[1] = specs.TagDefinition{
		Tag: 1, Name: "Checksum",
		Format: specs.FormatUint16, Length: 2, Mandatory: true,
	}
	tags[2] = specs.TagDefinition{
		Tag: 2, Name: "Precision Time Stamp",
		Format: specs.FormatUint64, Length: 8, Mandatory: true,
		Decode: decodeMISPTimestamp,
	}
	tags[65] = specs.TagDefinition{
		Tag: 65, Name: "UAS Datalink LS Version Number",
		Format: specs.FormatUint8, Length: 1, Mandatory: true,
	}

	// ---- Platform identity strings ----
	tags[3] = specs.TagDefinition{Tag: 3, Name: "Mission ID", Format: specs.FormatUTF8}
	tags[4] = specs.TagDefinition{Tag: 4, Name: "Platform Tail Number", Format: specs.FormatUTF8}
	tags[10] = specs.TagDefinition{Tag: 10, Name: "Platform Designation", Format: specs.FormatUTF8}
	tags[11] = specs.TagDefinition{Tag: 11, Name: "Image Source Sensor", Format: specs.FormatUTF8}
	tags[12] = specs.TagDefinition{Tag: 12, Name: "Image Coordinate System", Format: specs.FormatUTF8}

	// ---- Platform attitude (short form, int16/uint16) ----
	tags[5] = specs.TagDefinition{
		Tag: 5, Name: "Platform Heading Angle", Units: "°",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: 0, Max: 360},
	}
	tags[6] = specs.TagDefinition{
		Tag: 6, Name: "Platform Pitch Angle", Units: "°",
		Format: specs.FormatInt16, Length: 2,
		Scale: &specs.LinearScale{Min: -20, Max: 20, ErrorIndicator: true},
	}
	tags[7] = specs.TagDefinition{
		Tag: 7, Name: "Platform Roll Angle", Units: "°",
		Format: specs.FormatInt16, Length: 2,
		Scale: &specs.LinearScale{Min: -50, Max: 50, ErrorIndicator: true},
	}

	// ---- Platform attitude (full form, int32) ----
	tags[90] = specs.TagDefinition{
		Tag: 90, Name: "Platform Pitch Angle (Full)", Units: "°",
		Format: specs.FormatInt32, Length: 4,
		Scale: &specs.LinearScale{Min: -90, Max: 90, ErrorIndicator: true},
	}
	tags[91] = specs.TagDefinition{
		Tag: 91, Name: "Platform Roll Angle (Full)", Units: "°",
		Format: specs.FormatInt32, Length: 4,
		Scale: &specs.LinearScale{Min: -90, Max: 90, ErrorIndicator: true},
	}
	tags[92] = specs.TagDefinition{
		Tag: 92, Name: "Platform Angle of Attack (Full)", Units: "°",
		Format: specs.FormatInt32, Length: 4,
		Scale: &specs.LinearScale{Min: -90, Max: 90, ErrorIndicator: true},
	}

	// ---- Sensor position ----
	tags[13] = specs.TagDefinition{
		Tag: 13, Name: "Sensor Latitude", Units: "°",
		Format: specs.FormatInt32, Length: 4,
		Scale: &specs.LinearScale{Min: -90, Max: 90, ErrorIndicator: true},
	}
	// NOTE: All coordinate tags (13/14, 23/24, 40/41, 82-89) use FormatInt32
	// per ST 0601.19 §8.13–§8.14, §8.23–§8.24, §8.40–§8.41, §8.82–§8.89.
	tags[14] = specs.TagDefinition{
		Tag: 14, Name: "Sensor Longitude", Units: "°",
		Format: specs.FormatInt32, Length: 4,
		Scale: &specs.LinearScale{Min: -180, Max: 180, ErrorIndicator: true},
	}
	tags[15] = specs.TagDefinition{
		Tag: 15, Name: "Sensor True Altitude", Units: "m",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: -900, Max: 19000},
	}
	tags[75] = specs.TagDefinition{
		Tag: 75, Name: "Sensor Ellipsoid Height", Units: "m",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: -900, Max: 19000},
	}

	// ---- Sensor angles ----
	tags[16] = specs.TagDefinition{
		Tag: 16, Name: "Sensor Horizontal Field of View", Units: "°",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: 0, Max: 180},
	}
	tags[17] = specs.TagDefinition{
		Tag: 17, Name: "Sensor Vertical Field of View", Units: "°",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: 0, Max: 180},
	}
	tags[18] = specs.TagDefinition{
		Tag: 18, Name: "Sensor Relative Azimuth Angle", Units: "°",
		Format: specs.FormatUint32, Length: 4,
		Scale: &specs.LinearScale{Min: 0, Max: 360},
	}
	tags[19] = specs.TagDefinition{
		Tag: 19, Name: "Sensor Relative Elevation Angle", Units: "°",
		Format: specs.FormatInt32, Length: 4,
		Scale: &specs.LinearScale{Min: -180, Max: 180, ErrorIndicator: true},
	}
	tags[20] = specs.TagDefinition{
		Tag: 20, Name: "Sensor Relative Roll Angle", Units: "°",
		Format: specs.FormatUint32, Length: 4,
		Scale: &specs.LinearScale{Min: 0, Max: 360},
	}

	// ---- Frame center ----
	tags[23] = specs.TagDefinition{
		Tag: 23, Name: "Frame Center Latitude", Units: "°",
		Format: specs.FormatInt32, Length: 4,
		Scale: &specs.LinearScale{Min: -90, Max: 90, ErrorIndicator: true},
	}
	tags[24] = specs.TagDefinition{
		Tag: 24, Name: "Frame Center Longitude", Units: "°",
		Format: specs.FormatInt32, Length: 4,
		Scale: &specs.LinearScale{Min: -180, Max: 180, ErrorIndicator: true},
	}
	tags[25] = specs.TagDefinition{
		Tag: 25, Name: "Frame Center Elevation", Units: "m",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: -900, Max: 19000},
	}

	// ---- Frame corners (Full, int32) ----
	cornerTag := func(tag int, name string, lon bool) {
		scale := specs.LinearScale{Min: -90, Max: 90, ErrorIndicator: true}
		if lon {
			scale = specs.LinearScale{Min: -180, Max: 180, ErrorIndicator: true}
		}
		tags[tag] = specs.TagDefinition{
			Tag: tag, Name: name, Units: "°",
			Format: specs.FormatInt32, Length: 4,
			Scale: &scale,
		}
	}
	cornerTag(82, "Corner Latitude Point 1 (Full)", false)
	cornerTag(83, "Corner Longitude Point 1 (Full)", true)
	cornerTag(84, "Corner Latitude Point 2 (Full)", false)
	cornerTag(85, "Corner Longitude Point 2 (Full)", true)
	cornerTag(86, "Corner Latitude Point 3 (Full)", false)
	cornerTag(87, "Corner Longitude Point 3 (Full)", true)
	cornerTag(88, "Corner Latitude Point 4 (Full)", false)
	cornerTag(89, "Corner Longitude Point 4 (Full)", true)

	// ---- Range / geometry ----
	tags[21] = specs.TagDefinition{
		Tag: 21, Name: "Slant Range", Units: "m",
		Format: specs.FormatUint32, Length: 4,
		Scale: &specs.LinearScale{Min: 0, Max: 5000000},
	}
	tags[22] = specs.TagDefinition{
		Tag: 22, Name: "Target Width", Units: "m",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: 0, Max: 10000},
	}
	tags[57] = specs.TagDefinition{
		Tag: 57, Name: "Ground Range", Units: "m",
		Format: specs.FormatUint32, Length: 4,
		Scale: &specs.LinearScale{Min: 0, Max: 5000000},
	}

	// ---- Wind ----
	tags[35] = specs.TagDefinition{
		Tag: 35, Name: "Wind Direction", Units: "°",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: 0, Max: 360},
	}
	tags[36] = specs.TagDefinition{
		Tag: 36, Name: "Wind Speed", Units: "m/s",
		Format: specs.FormatUint8, Length: 1,
		Scale: &specs.LinearScale{Min: 0, Max: 100},
	}

	// ---- Pressure ----
	tags[37] = specs.TagDefinition{
		Tag: 37, Name: "Static Pressure", Units: "mbar",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: 0, Max: 5000},
	}
	tags[49] = specs.TagDefinition{
		Tag: 49, Name: "Differential Pressure", Units: "mbar",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: 0, Max: 5000},
	}
	tags[53] = specs.TagDefinition{
		Tag: 53, Name: "Airfield Barometric Pressure", Units: "mbar",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: 0, Max: 5000},
	}

	// ---- Target tracking ----
	tags[43] = specs.TagDefinition{
		Tag: 43, Name: "Target Track Gate Width", Units: "pixels",
		Format: specs.FormatUint8, Length: 1,
		Scale: &specs.LinearScale{Min: 0, Max: 510},
	}
	tags[44] = specs.TagDefinition{
		Tag: 44, Name: "Target Track Gate Height", Units: "pixels",
		Format: specs.FormatUint8, Length: 1,
		Scale: &specs.LinearScale{Min: 0, Max: 510},
	}
	tags[45] = specs.TagDefinition{
		Tag: 45, Name: "Target Error Estimate - CE90", Units: "m",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: 0, Max: 4095},
	}
	tags[46] = specs.TagDefinition{
		Tag: 46, Name: "Target Error Estimate - LE90", Units: "m",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: 0, Max: 4095},
	}

	// ---- Airfield / altitude ----
	tags[54] = specs.TagDefinition{
		Tag: 54, Name: "Airfield Elevation", Units: "m",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: -900, Max: 19000},
	}

	// ---- Platform ----
	tags[58] = specs.TagDefinition{
		Tag: 58, Name: "Platform Fuel Remaining", Units: "kg",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: 0, Max: 10000},
	}
	tags[64] = specs.TagDefinition{
		Tag: 64, Name: "Platform Magnetic Heading", Units: "°",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: 0, Max: 360},
	}

	// ---- Alternate platform ----
	tags[69] = specs.TagDefinition{
		Tag: 69, Name: "Alternate Platform Altitude", Units: "m",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: -900, Max: 19000},
	}
	tags[71] = specs.TagDefinition{
		Tag: 71, Name: "Alternate Platform Heading", Units: "°",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: 0, Max: 360},
	}
	tags[76] = specs.TagDefinition{
		Tag: 76, Name: "Alternate Platform Ellipsoid Height", Units: "m",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: -900, Max: 19000},
	}

	// ---- Frame center ----
	tags[78] = specs.TagDefinition{
		Tag: 78, Name: "Frame Center Height Above Ellipsoid", Units: "m",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: -900, Max: 19000},
	}

	// ---- Environmental ----
	tags[38] = specs.TagDefinition{
		Tag: 38, Name: "Density Altitude", Units: "m",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: -900, Max: 19000},
	}
	tags[39] = specs.TagDefinition{
		Tag: 39, Name: "Outside Air Temperature", Units: "°C",
		Format: specs.FormatInt8, Length: 1,
	}
	tags[55] = specs.TagDefinition{
		Tag: 55, Name: "Relative Humidity", Units: "%",
		Format: specs.FormatUint8, Length: 1,
		Scale: &specs.LinearScale{Min: 0, Max: 100},
	}

	// ---- Target location ----
	tags[40] = specs.TagDefinition{
		Tag: 40, Name: "Target Location Latitude", Units: "°",
		Format: specs.FormatInt32, Length: 4,
		Scale: &specs.LinearScale{Min: -90, Max: 90, ErrorIndicator: true},
	}
	tags[41] = specs.TagDefinition{
		Tag: 41, Name: "Target Location Longitude", Units: "°",
		Format: specs.FormatInt32, Length: 4,
		Scale: &specs.LinearScale{Min: -180, Max: 180, ErrorIndicator: true},
	}
	tags[42] = specs.TagDefinition{
		Tag: 42, Name: "Target Location Elevation", Units: "m",
		Format: specs.FormatUint16, Length: 2,
		Scale: &specs.LinearScale{Min: -900, Max: 19000},
	}

	// ---- Timestamps ----
	tags[72] = specs.TagDefinition{
		Tag: 72, Name: "Event Start Time",
		Format: specs.FormatUint64, Length: 8,
		Decode: decodeMISPTimestamp,
	}
	tags[131] = specs.TagDefinition{
		Tag: 131, Name: "Take-off Time",
		Decode: decodeVariableMISPTimestamp,
	}
	tags[136] = specs.TagDefinition{
		Tag: 136, Name: "Leap Seconds", Units: "s",
		Decode: decodeVariableInt,
	}
	tags[137] = specs.TagDefinition{
		Tag: 137, Name: "Correction Offset", Units: "μs",
		Decode: decodeVariableInt,
	}

	// ---- Platform airspeed ----
	tags[8] = specs.TagDefinition{
		Tag: 8, Name: "Platform True Airspeed", Units: "m/s",
		Format: specs.FormatUint8, Length: 1,
	}
	tags[9] = specs.TagDefinition{
		Tag: 9, Name: "Platform Indicated Airspeed", Units: "m/s",
		Format: specs.FormatUint8, Length: 1,
	}

	// ---- Offset corners (int16, ±0.075°) ----
	offsetCorner := func(tag int, name string) {
		tags[tag] = specs.TagDefinition{
			Tag: tag, Name: name, Units: "°",
			Format: specs.FormatInt16, Length: 2,
			Scale: &specs.LinearScale{Min: -0.075, Max: 0.075, ErrorIndicator: true},
		}
	}
	offsetCorner(26, "Offset Corner Latitude Point 1")
	offsetCorner(27, "Offset Corner Longitude Point 1")
	offsetCorner(28, "Offset Corner Latitude Point 2")
	offsetCorner(29, "Offset Corner Longitude Point 2")
	offsetCorner(30, "Offset Corner Latitude Point 3")
	offsetCorner(31, "Offset Corner Longitude Point 3")
	offsetCorner(32, "Offset Corner Latitude Point 4")
	offsetCorner(33, "Offset Corner Longitude Point 4")

	// ---- Flags and discrete values ----
	tags[47] = specs.TagDefinition{
		Tag: 47, Name: "Generic Flag Data",
		Format: specs.FormatUint8, Length: 1,
	}

	// ---- Platform attitude (short form, additional) ----
	tags[50] = specs.TagDefinition{
		Tag: 50, Name: "Platform Angle of Attack", Units: "°",
		Format: specs.FormatInt16, Length: 2,
		Scale: &specs.LinearScale{Min: -20, Max: 20, ErrorIndicator: true},
	}
	tags[51] = specs.TagDefinition{
		Tag: 51, Name: "Platform Vertical Speed", Units: "m/s",
		Format: specs.FormatInt16, Length: 2,
		Scale: &specs.LinearScale{Min: -180, Max: 180, ErrorIndicator: true},
	}
	tags[52] = specs.TagDefinition{
		Tag: 52, Name: "Platform Sideslip Angle", Units: "°",
		Format: specs.FormatInt16, Length: 2,
		Scale: &specs.LinearScale{Min: -20, Max: 20, ErrorIndicator: true},
	}

	// ---- Platform ground speed ----
	tags[56] = specs.TagDefinition{
		Tag: 56, Name: "Platform Ground Speed", Units: "m/s",
		Format: specs.FormatUint8, Length: 1,
	}

	// ---- Platform identity strings (additional) ----
	tags[59] = specs.TagDefinition{Tag: 59, Name: "Platform Call Sign", Format: specs.FormatUTF8}

	// ---- Weapons ----
	tags[60] = specs.TagDefinition{
		Tag: 60, Name: "Weapon Load",
		Format: specs.FormatUint16, Length: 2,
	}
	tags[61] = specs.TagDefinition{
		Tag: 61, Name: "Weapon Fired",
		Format: specs.FormatUint8, Length: 1,
	}
	tags[62] = specs.TagDefinition{
		Tag: 62, Name: "Laser PRF Code",
		Format: specs.FormatUint16, Length: 2,
	}

	// ---- Deprecated ----
	tags[66] = specs.TagDefinition{Tag: 66, Name: "Deprecated", Format: specs.FormatBytes}

	// ---- Alternate platform (additional) ----
	tags[67] = specs.TagDefinition{
		Tag: 67, Name: "Alternate Platform Latitude", Units: "°",
		Format: specs.FormatInt32, Length: 4,
		Scale: &specs.LinearScale{Min: -90, Max: 90, ErrorIndicator: true},
	}
	tags[68] = specs.TagDefinition{
		Tag: 68, Name: "Alternate Platform Longitude", Units: "°",
		Format: specs.FormatInt32, Length: 4,
		Scale: &specs.LinearScale{Min: -180, Max: 180, ErrorIndicator: true},
	}
	tags[70] = specs.TagDefinition{Tag: 70, Name: "Alternate Platform Name", Format: specs.FormatUTF8}

	// ---- Sensor velocity ----
	tags[79] = specs.TagDefinition{
		Tag: 79, Name: "Sensor North Velocity", Units: "m/s",
		Format: specs.FormatInt16, Length: 2,
		Scale: &specs.LinearScale{Min: -327, Max: 327, ErrorIndicator: true},
	}
	tags[80] = specs.TagDefinition{
		Tag: 80, Name: "Sensor East Velocity", Units: "m/s",
		Format: specs.FormatInt16, Length: 2,
		Scale: &specs.LinearScale{Min: -327, Max: 327, ErrorIndicator: true},
	}

	// ---- Platform sideslip (full) ----
	tags[93] = specs.TagDefinition{
		Tag: 93, Name: "Platform Sideslip Angle (Full)", Units: "°",
		Format: specs.FormatInt32, Length: 4,
		Scale: &specs.LinearScale{Min: -180, Max: 180, ErrorIndicator: true},
	}

	// ---- Raw bytes ----
	tags[94] = specs.TagDefinition{Tag: 94, Name: "MIIS Core Identifier", Format: specs.FormatBytes}

	// ---- Additional strings ----
	tags[106] = specs.TagDefinition{Tag: 106, Name: "Stream Designator", Format: specs.FormatUTF8}
	tags[107] = specs.TagDefinition{Tag: 107, Name: "Operational Base", Format: specs.FormatUTF8}
	tags[108] = specs.TagDefinition{Tag: 108, Name: "Broadcast Source", Format: specs.FormatUTF8}

	// ---- Navigation ----
	tags[123] = specs.TagDefinition{
		Tag: 123, Name: "Number of NAVSATs in View",
		Format: specs.FormatUint8, Length: 1,
	}
	tags[124] = specs.TagDefinition{
		Tag: 124, Name: "Positioning Method Source",
		Format: specs.FormatUint8, Length: 1,
	}

	// ---- Additional strings (high tags) ----
	tags[129] = specs.TagDefinition{Tag: 129, Name: "Target ID", Format: specs.FormatUTF8}
	tags[135] = specs.TagDefinition{Tag: 135, Name: "Communications Method", Format: specs.FormatUTF8}

	// ---- IMAPB (variable-length floating-point) tags ----
	tags[96] = specs.TagDefinition{
		Tag: 96, Name: "Target Width Extended", Units: "m",
		Format: specs.FormatIMAPB,
		Scale:  &specs.LinearScale{Min: 0, Max: 1500000},
	}
	tags[103] = specs.TagDefinition{
		Tag: 103, Name: "Density Altitude Extended", Units: "m",
		Format: specs.FormatIMAPB,
		Scale:  &specs.LinearScale{Min: -900, Max: 40000},
	}
	tags[104] = specs.TagDefinition{
		Tag: 104, Name: "Sensor Ellipsoid Height Extended", Units: "m",
		Format: specs.FormatIMAPB,
		Scale:  &specs.LinearScale{Min: -900, Max: 40000},
	}
	tags[105] = specs.TagDefinition{
		Tag: 105, Name: "Alternate Platform Ellipsoid Height Extended", Units: "m",
		Format: specs.FormatIMAPB,
		Scale:  &specs.LinearScale{Min: -900, Max: 40000},
	}
	tags[109] = specs.TagDefinition{
		Tag: 109, Name: "Range To Recovery Location", Units: "km",
		Format: specs.FormatIMAPB,
		Scale:  &specs.LinearScale{Min: 0, Max: 21000},
	}
	tags[112] = specs.TagDefinition{
		Tag: 112, Name: "Platform Course Angle", Units: "°",
		Format: specs.FormatIMAPB,
		Scale:  &specs.LinearScale{Min: 0, Max: 360},
	}
	tags[113] = specs.TagDefinition{
		Tag: 113, Name: "Altitude AGL", Units: "m",
		Format: specs.FormatIMAPB,
		Scale:  &specs.LinearScale{Min: -900, Max: 40000},
	}
	tags[114] = specs.TagDefinition{
		Tag: 114, Name: "Radar Altimeter", Units: "m",
		Format: specs.FormatIMAPB,
		Scale:  &specs.LinearScale{Min: -900, Max: 40000},
	}
	tags[117] = specs.TagDefinition{
		Tag: 117, Name: "Sensor Azimuth Rate", Units: "°/s",
		Format: specs.FormatIMAPB,
		Scale:  &specs.LinearScale{Min: -1000, Max: 1000},
	}
	tags[118] = specs.TagDefinition{
		Tag: 118, Name: "Sensor Elevation Rate", Units: "°/s",
		Format: specs.FormatIMAPB,
		Scale:  &specs.LinearScale{Min: -1000, Max: 1000},
	}
	tags[119] = specs.TagDefinition{
		Tag: 119, Name: "Sensor Roll Rate", Units: "°/s",
		Format: specs.FormatIMAPB,
		Scale:  &specs.LinearScale{Min: -1000, Max: 1000},
	}
	tags[120] = specs.TagDefinition{
		Tag: 120, Name: "On-board MI Storage Percent Full", Units: "%",
		Format: specs.FormatIMAPB,
		Scale:  &specs.LinearScale{Min: 0, Max: 100},
	}
	tags[132] = specs.TagDefinition{
		Tag: 132, Name: "Transmission Frequency", Units: "MHz",
		Format: specs.FormatIMAPB,
		Scale:  &specs.LinearScale{Min: 1, Max: 99999},
	}
	tags[134] = specs.TagDefinition{
		Tag: 134, Name: "Zoom Percentage", Units: "%",
		Format: specs.FormatIMAPB,
		Scale:  &specs.LinearScale{Min: 0, Max: 100},
	}

	// ---- Variable-length unsigned integer ----
	tags[110] = specs.TagDefinition{
		Tag: 110, Name: "Time Airborne", Units: "s",
		Decode: decodeVariableUint,
	}
	tags[111] = specs.TagDefinition{
		Tag: 111, Name: "Propulsion Unit Speed", Units: "RPM",
		Decode: decodeVariableUint,
	}
	tags[133] = specs.TagDefinition{
		Tag: 133, Name: "On-board MI Storage Capacity", Units: "GB",
		Decode: decodeVariableUint,
	}

	// ---- Enumerated tags ----
	tags[34] = specs.TagDefinition{
		Tag: 34, Name: "Icing Detected",
		Decode: decodeIcingDetected,
		Enum:   map[int64]string{0: "Detector Off", 1: "No Icing Detected", 2: "Icing Detected"},
	}
	tags[63] = specs.TagDefinition{
		Tag: 63, Name: "Sensor Field of View Name",
		Decode: decodeSensorFOVName,
		Enum: map[int64]string{
			0: "Ultranarrow", 1: "Narrow", 2: "Medium", 3: "Wide", 4: "Ultrawide",
			5: "Narrow Medium", 6: "2x Ultranarrow", 7: "4x Ultranarrow", 8: "Continuous Zoom",
		},
	}
	tags[77] = specs.TagDefinition{
		Tag: 77, Name: "Operational Mode",
		Decode: decodeOperationalMode,
		Enum: map[int64]string{
			0: "Other", 1: "Operational", 2: "Training",
			3: "Exercise", 4: "Maintenance", 5: "Test",
		},
	}
	tags[125] = specs.TagDefinition{
		Tag: 125, Name: "Platform Status",
		Decode: decodePlatformStatus,
		Enum: map[int64]string{
			0: "Active", 1: "Pre-flight", 2: "Pre-flight-taxiing", 3: "Run-up",
			4: "Take-off", 5: "Ingress", 6: "Manual operation", 7: "Automated-orbit",
			8: "Transitioning", 9: "Egress", 10: "Landing", 11: "Landed-taxiing",
			12: "Landed-Parked",
		},
	}
	tags[126] = specs.TagDefinition{
		Tag: 126, Name: "Sensor Control Mode",
		Decode: decodeSensorControlMode,
		Enum: map[int64]string{
			0: "Off", 1: "Home Position", 2: "Uncontrolled", 3: "Manual Control",
			4: "Calibrating", 5: "Auto - Holding Position", 6: "Auto - Tracking",
		},
	}

	// ---- Nested Local Sets (opaque passthrough) ----
	nested := func(tag int, name, hint string) {
		tags[tag] = specs.TagDefinition{
			Tag: tag, Name: name,
			Decode: func(raw []byte) (record.Value, error) {
				return record.NestedValue{SpecHint: hint, Raw: append([]byte{}, raw...)}, nil
			},
		}
	}
	nested(48, "Security Local Set", "MISB ST 0102")
	nested(73, "RVT Local Set", "MISB ST 0806")
	nested(74, "VMTI Local Set", "MISB ST 0903")
	nested(95, "SAR Motion Imagery Local Set", "MISB ST 1206")
	nested(97, "Range Image Local Set", "MISB ST 1002")
	nested(98, "Geo-Registration Local Set", "MISB ST 1601")
	nested(99, "Composite Imaging Local Set", "MISB ST 1602")
	nested(100, "Segment Local Set", "MISB ST 1607")
	nested(101, "Amend Local Set", "MISB ST 1607")

	return tags
}

// decodeMISPTimestamp converts an 8-byte big-endian uint64 of MISP
// microseconds-since-epoch into a TimeValue. The MISP epoch is
// 1970-01-01T00:00:00Z and excludes leap seconds.
func decodeMISPTimestamp(raw []byte) (record.Value, error) {
	if len(raw) != 8 {
		return nil, fmt.Errorf("precision time stamp: expected 8 bytes, got %d", len(raw))
	}
	micros := binary.BigEndian.Uint64(raw)
	secs := int64(micros / 1_000_000)
	nanos := int64((micros % 1_000_000) * 1_000)
	return record.TimeValue(time.Unix(secs, nanos).UTC()), nil
}

// decodeVariableMISPTimestamp reads a 1..8 byte big-endian unsigned integer
// representing MISP microseconds and returns a TimeValue. Used for tag 131.
func decodeVariableMISPTimestamp(raw []byte) (record.Value, error) {
	if len(raw) == 0 || len(raw) > 8 {
		return nil, fmt.Errorf("variable timestamp: expected 1..8 bytes, got %d", len(raw))
	}
	var micros uint64
	for _, b := range raw {
		micros = (micros << 8) | uint64(b)
	}
	secs := int64(micros / 1_000_000)
	nanos := int64((micros % 1_000_000) * 1_000)
	return record.TimeValue(time.Unix(secs, nanos).UTC()), nil
}

// decodeVariableInt reads a 1..8 byte big-endian signed integer (sign-extended
// from the high bit of the first byte). Used for tags 136 (Leap Seconds) and
// 137 (Correction Offset).
func decodeVariableInt(raw []byte) (record.Value, error) {
	if len(raw) == 0 || len(raw) > 8 {
		return nil, fmt.Errorf("variable int: expected 1..8 bytes, got %d", len(raw))
	}
	var v uint64
	for _, b := range raw {
		v = (v << 8) | uint64(b)
	}
	// Sign-extend from the top bit of the first byte.
	bits := uint(len(raw) * 8)
	mask := uint64(1) << (bits - 1)
	if v&mask != 0 {
		v |= ^((uint64(1) << bits) - 1)
	}
	return record.IntValue(int64(v)), nil
}

func decodeIcingDetected(raw []byte) (record.Value, error) {
	if len(raw) != 1 {
		return nil, fmt.Errorf("icing detected: expected 1 byte, got %d", len(raw))
	}
	labels := map[byte]string{
		0: "Detector Off",
		1: "No Icing Detected",
		2: "Icing Detected",
	}
	label, ok := labels[raw[0]]
	if !ok {
		return nil, fmt.Errorf("icing detected: invalid code %d", raw[0])
	}
	return record.EnumValue{Code: int64(raw[0]), Label: label}, nil
}

func decodeOperationalMode(raw []byte) (record.Value, error) {
	if len(raw) != 1 {
		return nil, fmt.Errorf("operational mode: expected 1 byte, got %d", len(raw))
	}
	labels := map[byte]string{
		0: "Other", 1: "Operational", 2: "Training",
		3: "Exercise", 4: "Maintenance", 5: "Test",
	}
	label, ok := labels[raw[0]]
	if !ok {
		return nil, fmt.Errorf("operational mode: invalid code %d", raw[0])
	}
	return record.EnumValue{Code: int64(raw[0]), Label: label}, nil
}

func decodeVariableUint(raw []byte) (record.Value, error) {
	if len(raw) == 0 || len(raw) > 8 {
		return nil, fmt.Errorf("variable uint: expected 1..8 bytes, got %d", len(raw))
	}
	var v uint64
	for _, b := range raw {
		v = (v << 8) | uint64(b)
	}
	return record.UintValue(v), nil
}

func decodeSensorFOVName(raw []byte) (record.Value, error) {
	if len(raw) != 1 {
		return nil, fmt.Errorf("sensor field of view name: expected 1 byte, got %d", len(raw))
	}
	labels := map[byte]string{
		0: "Ultranarrow", 1: "Narrow", 2: "Medium", 3: "Wide", 4: "Ultrawide",
		5: "Narrow Medium", 6: "2x Ultranarrow", 7: "4x Ultranarrow", 8: "Continuous Zoom",
	}
	label, ok := labels[raw[0]]
	if !ok {
		return nil, fmt.Errorf("sensor field of view name: invalid code %d", raw[0])
	}
	return record.EnumValue{Code: int64(raw[0]), Label: label}, nil
}

func decodePlatformStatus(raw []byte) (record.Value, error) {
	if len(raw) != 1 {
		return nil, fmt.Errorf("platform status: expected 1 byte, got %d", len(raw))
	}
	labels := map[byte]string{
		0: "Active", 1: "Pre-flight", 2: "Pre-flight-taxiing", 3: "Run-up",
		4: "Take-off", 5: "Ingress", 6: "Manual operation", 7: "Automated-orbit",
		8: "Transitioning", 9: "Egress", 10: "Landing", 11: "Landed-taxiing",
		12: "Landed-Parked",
	}
	label, ok := labels[raw[0]]
	if !ok {
		return nil, fmt.Errorf("platform status: invalid code %d", raw[0])
	}
	return record.EnumValue{Code: int64(raw[0]), Label: label}, nil
}

func decodeSensorControlMode(raw []byte) (record.Value, error) {
	if len(raw) != 1 {
		return nil, fmt.Errorf("sensor control mode: expected 1 byte, got %d", len(raw))
	}
	labels := map[byte]string{
		0: "Off", 1: "Home Position", 2: "Uncontrolled", 3: "Manual Control",
		4: "Calibrating", 5: "Auto - Holding Position", 6: "Auto - Tracking",
	}
	label, ok := labels[raw[0]]
	if !ok {
		return nil, fmt.Errorf("sensor control mode: invalid code %d", raw[0])
	}
	return record.EnumValue{Code: int64(raw[0]), Label: label}, nil
}
