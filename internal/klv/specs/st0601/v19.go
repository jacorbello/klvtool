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

	// ---- Enumerated tags ----
	tags[34] = specs.TagDefinition{
		Tag: 34, Name: "Icing Detected",
		Decode: decodeIcingDetected,
		Enum:   map[int64]string{0: "Detector Off", 1: "No Icing Detected", 2: "Icing Detected"},
	}
	tags[77] = specs.TagDefinition{
		Tag: 77, Name: "Operational Mode",
		Decode: decodeOperationalMode,
		Enum: map[int64]string{
			0: "Other", 1: "Operational", 2: "Training",
			3: "Exercise", 4: "Maintenance", 5: "Test",
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
