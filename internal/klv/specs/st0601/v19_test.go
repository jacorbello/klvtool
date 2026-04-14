package st0601

import (
	"bytes"
	"math"
	"testing"
	"time"

	"github.com/jacorbello/klvtool/internal/klv"
	"github.com/jacorbello/klvtool/internal/klv/record"
	"github.com/jacorbello/klvtool/internal/klv/specs"
)

func TestV19Metadata(t *testing.T) {
	sv := V19()
	if sv.URN() != "urn:misb:KLV:bin:0601.19" {
		t.Errorf("URN = %s", sv.URN())
	}
	if sv.VersionTag() != 65 {
		t.Errorf("VersionTag = %d, want 65", sv.VersionTag())
	}
	if sv.ExpectedVersion() != 19 {
		t.Errorf("ExpectedVersion = %d, want 19", sv.ExpectedVersion())
	}
	if !bytes.Equal(sv.UL(), UASDatalinkUL) {
		t.Errorf("UL = %x, want %x", sv.UL(), UASDatalinkUL)
	}
}

func TestV19MandatoryTags(t *testing.T) {
	sv := V19()
	for _, tag := range []int{1, 2, 65} {
		td, ok := sv.Tag(tag)
		if !ok {
			t.Errorf("Tag(%d) missing", tag)
			continue
		}
		if !td.Mandatory {
			t.Errorf("Tag(%d) not mandatory", tag)
		}
	}
}

func TestV19UnknownTag(t *testing.T) {
	sv := V19()
	_, ok := sv.Tag(9999)
	if ok {
		t.Errorf("Tag(9999): expected ok=false")
	}
}

// TestV19AllTagsSortedByTag verifies that AllTags returns tag definitions
// in deterministic ascending order so diagnostic iteration is stable.
func TestV19AllTagsSortedByTag(t *testing.T) {
	tags := V19().AllTags()
	for i := 1; i < len(tags); i++ {
		if tags[i-1].Tag >= tags[i].Tag {
			t.Errorf("AllTags unsorted at index %d: %d then %d", i, tags[i-1].Tag, tags[i].Tag)
		}
	}
}

var _ = specs.SpecVersion(V19()) // compile-time interface check

func TestV19CoreTagDecoding(t *testing.T) {
	sv := V19()
	tests := []struct {
		tag  int
		raw  []byte
		name string
	}{
		{5, []byte{0x71, 0xC2}, "Platform Heading Angle"}, // 0x71C2 = 29122 → ~159.9° of 360
		{10, []byte("REAPER"), "Platform Designation"},
		{13, []byte{0x04, 0x5D, 0x6D, 0x00}, "Sensor Latitude"},
		{15, []byte{0x40, 0x00}, "Sensor True Altitude"},
		{65, []byte{19}, "UAS Datalink LS Version Number"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td, ok := sv.Tag(tt.tag)
			if !ok {
				t.Fatalf("tag %d missing", tt.tag)
			}
			if td.Name != tt.name {
				t.Errorf("name = %s, want %s", td.Name, tt.name)
			}
			// Every phase-1 tag has either Format or Decode set.
			if td.Format == specs.FormatNone && td.Decode == nil {
				t.Errorf("tag %d has no decoder", tt.tag)
			}
			if tt.tag == 13 && td.Format != specs.FormatInt32 {
				t.Errorf("tag 13 format = %v, want FormatInt32", td.Format)
			}
		})
	}
}

func TestV19IcingDetected(t *testing.T) {
	sv := V19()
	td, ok := sv.Tag(34)
	if !ok || td.Decode == nil {
		t.Fatalf("tag 34 missing or has no Decode")
	}
	v, err := td.Decode([]byte{1})
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	ev, ok := v.(record.EnumValue)
	if !ok {
		t.Fatalf("value type = %T, want EnumValue", v)
	}
	if ev.Label != "No Icing Detected" {
		t.Errorf("label = %s", ev.Label)
	}
}

func TestV19NestedLocalSetPassthrough(t *testing.T) {
	sv := V19()
	td, ok := sv.Tag(48) // Security Local Set
	if !ok || td.Decode == nil {
		t.Fatalf("tag 48 missing or has no Decode")
	}
	v, err := td.Decode([]byte{0x01, 0x02, 0x03})
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	nv, ok := v.(record.NestedValue)
	if !ok {
		t.Fatalf("value type = %T, want NestedValue", v)
	}
	if nv.SpecHint != "MISB ST 0102" {
		t.Errorf("specHint = %s", nv.SpecHint)
	}
}

func TestV19ComplexPackPassthrough(t *testing.T) {
	sv := V19()
	complexTags := []struct {
		tag  int
		name string
	}{
		{81, "Image Horizon Pixel Pack"},
		{102, "SDCC-FLP"},
		{115, "Control Command"},
		{116, "Control Command Verification List"},
		{121, "Active Wavelength List"},
		{122, "Country Codes"},
		{127, "Sensor Frame Rate Pack"},
		{128, "Wavelengths List"},
		{130, "Airbase Locations"},
		{138, "Payload List"},
		{139, "Active Payloads"},
		{140, "Weapons Stores"},
		{141, "Waypoint List"},
		{142, "View Domain"},
		{143, "Metadata Substream ID Pack"},
	}
	for _, tt := range complexTags {
		t.Run(tt.name, func(t *testing.T) {
			td, ok := sv.Tag(tt.tag)
			if !ok {
				t.Fatalf("tag %d not registered", tt.tag)
			}
			if td.Name != tt.name {
				t.Errorf("name = %q, want %q", td.Name, tt.name)
			}
			raw := []byte{0x01, 0x02, 0x03}
			v, err := klv.DecodeTag(td, raw)
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}
			bv, ok := v.(record.BytesValue)
			if !ok {
				t.Fatalf("type = %T, want BytesValue", v)
			}
			if !bytes.Equal([]byte(bv), raw) {
				t.Errorf("value = %x, want %x", []byte(bv), raw)
			}
		})
	}
}

func TestV19Decoding(t *testing.T) {
	sv := V19()
	tests := []struct {
		tag    int
		name   string
		raw    []byte
		assert func(t *testing.T, v record.Value)
	}{
		{1, "Checksum", []byte{0x8C, 0xED}, assertUint(36077)},
		{2, "Precision Time Stamp", []byte{0x00, 0x04, 0x59, 0xF4, 0xA6, 0xAA, 0x4A, 0xA8},
			assertTime(time.Date(2008, 10, 24, 0, 13, 29, 913000*1000, time.UTC))},
		{3, "Mission ID", []byte{0x4D, 0x49, 0x53, 0x53, 0x49, 0x4F, 0x4E, 0x30, 0x31}, assertString("MISSION01")},
		{4, "Platform Tail Number", []byte{0x41, 0x46, 0x2D, 0x31, 0x30, 0x31}, assertString("AF-101")},
		{5, "Platform Heading Angle", []byte{0x71, 0xC2}, assertFloat(159.974365, 1e-3)},
		{6, "Platform Pitch Angle", []byte{0xFD, 0x3D}, assertFloat(-0.431531724, 1e-6)},
		{7, "Platform Roll Angle", []byte{0x08, 0xB8}, assertFloat(3.40586566, 1e-5)},
		{10, "Platform Designation", []byte{0x4D, 0x51, 0x31, 0x2D, 0x42}, assertString("MQ1-B")},
		{11, "Image Source Sensor", []byte{0x45, 0x4F}, assertString("EO")},
		{12, "Image Coordinate System", []byte{0x47, 0x65, 0x6F, 0x64, 0x65, 0x74, 0x69, 0x63, 0x20, 0x57, 0x47, 0x53, 0x38, 0x34}, assertString("Geodetic WGS84")},
		{13, "Sensor Latitude", []byte{0x55, 0x95, 0xB6, 0x6D}, assertFloat(60.176822966978335, 1e-6)},
		{14, "Sensor Longitude", []byte{0x5B, 0x53, 0x60, 0xC4}, assertFloat(128.42675904204452, 1e-6)},
		{15, "Sensor True Altitude", []byte{0xC2, 0x21}, assertFloat(14190.7195, 1e-1)},
		{16, "Sensor Horizontal Field of View", []byte{0xCD, 0x9C}, assertFloat(144.571298, 1e-3)},
		{17, "Sensor Vertical Field of View", []byte{0xD9, 0x17}, assertFloat(152.643626, 1e-3)},
		{18, "Sensor Relative Azimuth Angle", []byte{0x72, 0x4A, 0x0A, 0x20}, assertFloat(160.71921143697557, 1e-6)},
		{19, "Sensor Relative Elevation Angle", []byte{0x87, 0xF8, 0x4B, 0x86}, assertFloat(-168.79232483394085, 1e-6)},
		{20, "Sensor Relative Roll Angle", []byte{0x7D, 0xC5, 0x5E, 0xCE}, assertFloat(176.86543764939194, 1e-6)},
		{23, "Frame Center Latitude", []byte{0xF1, 0x01, 0xA2, 0x29}, assertFloat(-10.542388633146132, 1e-6)},
		{24, "Frame Center Longitude", []byte{0x14, 0xBC, 0x08, 0x2B}, assertFloat(29.157890122923014, 1e-6)},
		{25, "Frame Center Elevation", []byte{0x34, 0xF3}, assertFloat(3216.03723, 1e-1)},
		{34, "Icing Detected", []byte{0x02}, assertEnum(2, "Icing Detected")},
		{38, "Density Altitude", []byte{0xCA, 0x35}, assertFloat(14818.6770, 1e-1)},
		{39, "Outside Air Temperature", []byte{0x54}, assertInt(84)},
		{40, "Target Location Latitude", []byte{0x8F, 0x69, 0x52, 0x62}, assertFloat(-79.163850051892850, 1e-6)},
		{41, "Target Location Longitude", []byte{0x76, 0x54, 0x57, 0xF2}, assertFloat(166.40081296041646, 1e-6)},
		{42, "Target Location Elevation", []byte{0xF8, 0x23}, assertFloat(18389.0471, 1e-1)},
		{55, "Relative Humidity", []byte{0x81}, assertFloat(50.5882353, 1e-4)},
		{65, "UAS Datalink LS Version Number", []byte{0x0D}, assertUint(13)},
		{72, "Event Start Time", []byte{0x00, 0x02, 0xD5, 0xD0, 0x24, 0x66, 0x01, 0x80},
			assertTime(time.Date(1995, 4, 16, 13, 44, 54, 0, time.UTC))},
		{75, "Sensor Ellipsoid Height", []byte{0xC2, 0x21}, assertFloat(14190.7195, 1e-1)},
		{77, "Operational Mode", []byte{0x01}, assertEnum(1, "Operational")},
		{82, "Corner Latitude Point 1 (Full)", []byte{0xF1, 0x06, 0x9B, 0x63}, assertFloat(-10.528728379108287, 1e-6)},
		{83, "Corner Longitude Point 1 (Full)", []byte{0x14, 0xBC, 0xB2, 0xC0}, assertFloat(29.161550376960857, 1e-6)},
		{84, "Corner Latitude Point 2 (Full)", []byte{0xF1, 0x00, 0x4D, 0x00}, assertFloat(-10.546048887183977, 1e-6)},
		{85, "Corner Longitude Point 2 (Full)", []byte{0x14, 0xBE, 0x84, 0xC8}, assertFloat(29.171550376960860, 1e-6)},
		{86, "Corner Latitude Point 3 (Full)", []byte{0xF0, 0xFD, 0x9B, 0x17}, assertFloat(-10.553450810972622, 1e-6)},
		{87, "Corner Longitude Point 3 (Full)", []byte{0x14, 0xBB, 0x17, 0xAF}, assertFloat(29.152729868885170, 1e-6)},
		{88, "Corner Latitude Point 4 (Full)", []byte{0xF1, 0x02, 0x05, 0x2A}, assertFloat(-10.541326455319641, 1e-6)},
		{89, "Corner Longitude Point 4 (Full)", []byte{0x14, 0xB9, 0xD1, 0x76}, assertFloat(29.145729868885170, 1e-6)},
		{90, "Platform Pitch Angle (Full)", []byte{0xFF, 0x62, 0xE2, 0xF2}, assertFloat(-0.43152510208614414, 1e-6)},
		{91, "Platform Roll Angle (Full)", []byte{0x04, 0xD8, 0x04, 0xDF}, assertFloat(3.4058139815022304, 1e-6)},
		{92, "Platform Angle of Attack (Full)", []byte{0xF3, 0xAB, 0x48, 0xEF}, assertFloat(-8.6701769841230370, 1e-6)},
		{131, "Take-off Time", []byte{0x05, 0x6F, 0x27, 0x1B, 0x5E, 0x41, 0xB7},
			assertTime(time.Date(2018, 6, 21, 13, 43, 57, 122999*1000, time.UTC))},
		{136, "Leap Seconds", []byte{0x1E}, assertInt(30)},
		{137, "Correction Offset", []byte{0x01, 0x2B, 0x8D, 0xC6, 0x35}, assertInt(5025678901)},
		// ---- Plain integers (Task 3) ----
		{8, "Platform True Airspeed", []byte{0x93}, assertUint(147)},
		{9, "Platform Indicated Airspeed", []byte{0x9F}, assertUint(159)},
		{47, "Generic Flag Data", []byte{0x31}, assertUint(49)},
		{56, "Platform Ground Speed", []byte{0x8C}, assertUint(140)},
		{60, "Weapon Load", []byte{0xAF, 0xD8}, assertUint(45016)},
		{61, "Weapon Fired", []byte{0xBA}, assertUint(186)},
		{62, "Laser PRF Code", []byte{0x06, 0xCF}, assertUint(1743)},
		{123, "Number of NAVSATs in View", []byte{0x07}, assertUint(7)},
		{124, "Positioning Method Source", []byte{0x03}, assertUint(3)},
		// ---- Unsigned scaled (Task 1) ----
		{21, "Slant Range", []byte{0x03, 0x83, 0x09, 0x26}, assertFloat(68590.983298744770, 1e-3)},
		{22, "Target Width", []byte{0x12, 0x81}, assertFloat(722.819867, 1e-3)},
		{35, "Wind Direction", []byte{0xA7, 0xC4}, assertFloat(235.924010, 1e-3)},
		{36, "Wind Speed", []byte{0xB2}, assertFloat(69.8039216, 1e-4)},
		{37, "Static Pressure", []byte{0xBE, 0xBA}, assertFloat(3725.18502, 1e-1)},
		{43, "Target Track Gate Width", []byte{0x03}, assertFloat(6.0, 1e-1)},
		{44, "Target Track Gate Height", []byte{0x0F}, assertFloat(30.0, 1e-1)},
		{45, "Target Error Estimate - CE90", []byte{0x1A, 0x95}, assertFloat(425.215152, 1e-3)},
		{46, "Target Error Estimate - LE90", []byte{0x26, 0x11}, assertFloat(608.9231, 1e-1)},
		{49, "Differential Pressure", []byte{0x3D, 0x07}, assertFloat(1191.95850, 1e-1)},
		{53, "Airfield Barometric Pressure", []byte{0x6A, 0xF4}, assertFloat(2088.96010, 1e-1)},
		{54, "Airfield Elevation", []byte{0x76, 0x70}, assertFloat(8306.80552, 1e-1)},
		{57, "Ground Range", []byte{0xB3, 0x8E, 0xAC, 0xF1}, assertFloat(3506979.0316063400, 1e-3)},
		{58, "Platform Fuel Remaining", []byte{0xA4, 0x5D}, assertFloat(6420.53864, 1e-1)},
		{64, "Platform Magnetic Heading", []byte{0xDD, 0xC5}, assertFloat(311.868162, 1e-3)},
		{69, "Alternate Platform Altitude", []byte{0x0B, 0xB3}, assertFloat(9.44533455, 1e-3)},
		{71, "Alternate Platform Heading", []byte{0x17, 0x2F}, assertFloat(32.6024262, 1e-3)},
		{76, "Alternate Platform Ellipsoid Height", []byte{0x0B, 0xB3}, assertFloat(9.44533455, 1e-3)},
		{78, "Frame Center Height Above Ellipsoid", []byte{0x0B, 0xB3}, assertFloat(9.44533455, 1e-3)},
		// ---- Signed scaled (Task 2) ----
		{26, "Offset Corner Latitude Point 1", []byte{0x17, 0x50}, assertFloat(0.0136602540, 1e-6)},
		{27, "Offset Corner Longitude Point 1", []byte{0x06, 0x3F}, assertFloat(0.0036602540, 1e-6)},
		{28, "Offset Corner Latitude Point 2", []byte{0xF9, 0xC1}, assertFloat(-0.0036602540, 1e-6)},
		{29, "Offset Corner Longitude Point 2", []byte{0x17, 0x50}, assertFloat(0.0136602540, 1e-6)},
		{30, "Offset Corner Latitude Point 3", []byte{0xED, 0x1F}, assertFloat(-0.011062196722312, 1e-6)},
		{31, "Offset Corner Longitude Point 3", []byte{0xF7, 0x32}, assertFloat(-0.005159154026917, 1e-6)},
		{32, "Offset Corner Latitude Point 4", []byte{0x01, 0xD0}, assertFloat(0.001062044129765, 1e-6)},
		{33, "Offset Corner Longitude Point 4", []byte{0xEB, 0x3F}, assertFloat(-0.012160863063448, 1e-6)},
		{50, "Platform Angle of Attack", []byte{0xC8, 0x83}, assertFloat(-8.67030854, 1e-4)},
		{51, "Platform Vertical Speed", []byte{0xD3, 0xFE}, assertFloat(-61.8878750, 1e-3)},
		{52, "Platform Sideslip Angle", []byte{0xDF, 0x79}, assertFloat(-5.08255257, 1e-4)},
		{67, "Alternate Platform Latitude", []byte{0x85, 0xA1, 0x5A, 0x39}, assertFloat(-86.041207348947040, 1e-6)},
		{68, "Alternate Platform Longitude", []byte{0x00, 0x1C, 0x50, 0x1C}, assertFloat(0.15552755452484243, 1e-6)},
		{79, "Sensor North Velocity", []byte{0x09, 0xFB}, assertFloat(25.4977569, 1e-4)},
		{80, "Sensor East Velocity", []byte{0x04, 0xBC}, assertFloat(12.1, 1e-1)},
		{93, "Platform Sideslip Angle (Full)", []byte{0xDE, 0x17, 0x93, 0x23}, assertFloat(-47.683, 1e-1)},
		// ---- Strings (Task 4) ----
		{59, "Platform Call Sign", []byte{0x54, 0x4F, 0x50, 0x20, 0x47, 0x55, 0x4E}, assertString("TOP GUN")},
		{70, "Alternate Platform Name", []byte{0x41, 0x50, 0x41, 0x43, 0x48, 0x45}, assertString("APACHE")},
		{106, "Stream Designator", []byte{0x42, 0x4C, 0x55, 0x45}, assertString("BLUE")},
		{107, "Operational Base", []byte{0x42, 0x41, 0x53, 0x45, 0x30, 0x31}, assertString("BASE01")},
		{108, "Broadcast Source", []byte{0x48, 0x4F, 0x4D, 0x45}, assertString("HOME")},
		{129, "Target ID", []byte{0x41, 0x31, 0x32, 0x33}, assertString("A123")},
		{135, "Communications Method", []byte("Frequency Modulation"), assertString("Frequency Modulation")},
		// ---- Bytes (Task 5) ----
		{66, "Deprecated", []byte{0xDE, 0xAD}, assertBytes([]byte{0xDE, 0xAD})},
		// ---- Enums (Task 6) ----
		{63, "Sensor Field of View Name", []byte{0x02}, assertEnum(2, "Medium")},
		{125, "Platform Status", []byte{0x09}, assertEnum(9, "Egress")},
		{126, "Sensor Control Mode", []byte{0x05}, assertEnum(5, "Auto - Holding Position")},
		// ---- Variable-length uint (Task 7) ----
		{110, "Time Airborne", []byte{0x4D, 0xAF}, assertUint(19887)},
		{111, "Propulsion Unit Speed", []byte{0x0B, 0xB8}, assertUint(3000)},
		{133, "On-board MI Storage Capacity", []byte{0x27, 0x10}, assertUint(10000)},
		// ---- IMAPB (Task 8) ----
		{96, "Target Width Extended", []byte{0x02, 0x5F, 0x3D}, assertFloat(13898.5463, 1e0)},
		{103, "Density Altitude Extended", []byte{0x98, 0x73, 0x26}, assertFloat(23456.24, 1e0)},
		{104, "Sensor Ellipsoid Height Extended", []byte{0x98, 0x73, 0x26}, assertFloat(23456.24, 1e0)},
		{105, "Alternate Platform Ellipsoid Height Extended", []byte{0x98, 0x73, 0x26}, assertFloat(23456.24, 1e0)},
		{109, "Range To Recovery Location", []byte{0x00, 0x05, 0x12}, assertFloat(1.625, 1e-1)},
		{112, "Platform Course Angle", []byte{0x58, 0xE3}, assertFloat(125.0, 1e0)},
		{113, "Altitude AGL", []byte{0x13, 0x17, 0x29}, assertFloat(2150.0, 1e0)},
		{114, "Radar Altimeter", []byte{0x13, 0x1E, 0x5F}, assertFloat(2154.50, 1e0)},
		{117, "Sensor Azimuth Rate", []byte{0x80, 0x20}, assertFloat(1.0, 1e0)},
		{118, "Sensor Elevation Rate", []byte{0x80, 0x00, 0x23}, assertFloat(0.004176, 1e-1)},
		{119, "Sensor Roll Rate", []byte{0x79, 0x99}, assertFloat(-50.0, 1e0)},
		{120, "On-board MI Storage Percent Full", []byte{0xB8, 0x51}, assertFloat(72.0, 1e0)},
		{132, "Transmission Frequency", []byte{0x06, 0x24, 0x3D}, assertFloat(2400.0, 1e0)},
		{134, "Zoom Percentage", []byte{0x8C, 0xCC}, assertFloat(55.0, 1e0)},
		// ---- Bytes (Task 5) ----
		{94, "MIIS Core Identifier",
			[]byte{0x01, 0x70, 0xF5, 0x92, 0xF0, 0x23, 0x73, 0x36, 0x4A, 0xF8, 0xAA, 0x91, 0x62, 0xC0, 0x0F, 0x2E, 0xB2, 0xDA, 0x16, 0xB7, 0x43, 0x41, 0x00, 0x08, 0x41, 0xA0, 0xBE, 0x36, 0x5B, 0x5A, 0xB9, 0x6A, 0x36, 0x45},
			assertBytes([]byte{0x01, 0x70, 0xF5, 0x92, 0xF0, 0x23, 0x73, 0x36, 0x4A, 0xF8, 0xAA, 0x91, 0x62, 0xC0, 0x0F, 0x2E, 0xB2, 0xDA, 0x16, 0xB7, 0x43, 0x41, 0x00, 0x08, 0x41, 0xA0, 0xBE, 0x36, 0x5B, 0x5A, 0xB9, 0x6A, 0x36, 0x45})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			td, ok := sv.Tag(tt.tag)
			if !ok {
				t.Fatalf("tag %d not registered", tt.tag)
			}
			if td.Name != tt.name {
				t.Errorf("name = %q, want %q", td.Name, tt.name)
			}
			v, err := klv.DecodeTag(td, tt.raw)
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}
			tt.assert(t, v)
		})
	}
}

func TestV19VariableIntSignExtension(t *testing.T) {
	sv := V19()
	td, ok := sv.Tag(137)
	if !ok {
		t.Fatalf("tag 137 missing")
	}
	v, err := klv.DecodeTag(td, []byte{0xFF})
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	iv, ok := v.(record.IntValue)
	if !ok {
		t.Fatalf("type = %T, want IntValue", v)
	}
	if int64(iv) != -1 {
		t.Errorf("value = %d, want -1", int64(iv))
	}
}

func assertFloat(want, tol float64) func(t *testing.T, v record.Value) {
	return func(t *testing.T, v record.Value) {
		t.Helper()
		fv, ok := v.(record.FloatValue)
		if !ok {
			t.Fatalf("type = %T, want FloatValue", v)
		}
		got := float64(fv)
		if diff := math.Abs(got - want); diff > tol {
			t.Errorf("value = %.15f, want %.15f (diff %.2e, tol %.2e)", got, want, diff, tol)
		}
	}
}

func assertUint(want uint64) func(t *testing.T, v record.Value) {
	return func(t *testing.T, v record.Value) {
		t.Helper()
		uv, ok := v.(record.UintValue)
		if !ok {
			t.Fatalf("type = %T, want UintValue", v)
		}
		if uint64(uv) != want {
			t.Errorf("value = %d, want %d", uint64(uv), want)
		}
	}
}

func assertInt(want int64) func(t *testing.T, v record.Value) {
	return func(t *testing.T, v record.Value) {
		t.Helper()
		iv, ok := v.(record.IntValue)
		if !ok {
			t.Fatalf("type = %T, want IntValue", v)
		}
		if int64(iv) != want {
			t.Errorf("value = %d, want %d", int64(iv), want)
		}
	}
}

func assertString(want string) func(t *testing.T, v record.Value) {
	return func(t *testing.T, v record.Value) {
		t.Helper()
		sv, ok := v.(record.StringValue)
		if !ok {
			t.Fatalf("type = %T, want StringValue", v)
		}
		if string(sv) != want {
			t.Errorf("value = %q, want %q", string(sv), want)
		}
	}
}

func assertTime(want time.Time) func(t *testing.T, v record.Value) {
	return func(t *testing.T, v record.Value) {
		t.Helper()
		tv, ok := v.(record.TimeValue)
		if !ok {
			t.Fatalf("type = %T, want TimeValue", v)
		}
		got := time.Time(tv)
		if !got.Equal(want) {
			t.Errorf("value = %v, want %v", got, want)
		}
	}
}

func assertBytes(want []byte) func(t *testing.T, v record.Value) {
	return func(t *testing.T, v record.Value) {
		t.Helper()
		bv, ok := v.(record.BytesValue)
		if !ok {
			t.Fatalf("type = %T, want BytesValue", v)
		}
		if !bytes.Equal([]byte(bv), want) {
			t.Errorf("value = %x, want %x", []byte(bv), want)
		}
	}
}

func assertEnum(code int64, label string) func(t *testing.T, v record.Value) {
	return func(t *testing.T, v record.Value) {
		t.Helper()
		ev, ok := v.(record.EnumValue)
		if !ok {
			t.Fatalf("type = %T, want EnumValue", v)
		}
		if ev.Code != code {
			t.Errorf("code = %d, want %d", ev.Code, code)
		}
		if ev.Label != label {
			t.Errorf("label = %q, want %q", ev.Label, label)
		}
	}
}
