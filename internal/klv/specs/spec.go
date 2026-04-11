// Package specs defines the interfaces used by the klv engine to address
// MISB spec versions and their tag metadata. Concrete spec implementations
// live in subpackages (e.g. st0601).
package specs

import "github.com/jacorbello/klvtool/internal/klv/record"

// Format identifies the structural decoding strategy for a tag's raw bytes.
type Format int

const (
	// FormatNone means the TagDefinition's Decode function must handle
	// everything; no declarative format is applied.
	FormatNone Format = iota
	FormatUint8
	FormatUint16
	FormatUint32
	FormatUint64
	FormatInt8
	FormatInt16
	FormatInt32
	FormatInt64
	FormatUTF8
	FormatBytes
	FormatIMAPB
)

// LinearScale maps an unsigned/signed integer encoded value onto a physical
// range. For unsigned N-bit values, encoded 0 maps to Min and encoded (2^N-1)
// maps to Max. For signed N-bit values, -(2^(N-1)-1) maps to Min and
// (2^(N-1)-1) maps to Max; if ErrorIndicator is true, the value -2^(N-1)
// (0x80..0) is reserved to mean "out of range".
type LinearScale struct {
	Min, Max       float64
	ErrorIndicator bool
}

// TagDefinition describes a single KLV local set item's encoding, units,
// validation constraints, and decoder selection.
type TagDefinition struct {
	Tag   int
	Name  string
	Units string

	// Declarative decoder selection. Used when Decode is nil.
	Format Format
	Length int // 0 means variable-length
	Scale  *LinearScale

	// Function-pointer escape hatch. When non-nil, takes precedence over
	// Format for decoding. Length/Min/Max/Enum/Mandatory still apply for
	// validation regardless of which decoder path runs.
	Decode func(raw []byte) (record.Value, error)

	// Validation metadata.
	Mandatory bool
	Min, Max  *float64
	Enum      map[int64]string
}

// SpecVersion is one concrete version of one MISB spec. Implementations
// live in subpackages under internal/klv/specs.
type SpecVersion interface {
	URN() string
	UL() []byte
	VersionTag() int
	ExpectedVersion() int
	Tag(number int) (TagDefinition, bool)
	AllTags() []TagDefinition
}
