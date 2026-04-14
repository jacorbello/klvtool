// Package record defines the decoded-KLV data types produced by the engine.
// The Value type is a closed sum; consumers type-switch exhaustively.
package record

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"time"
)

// Record is the decoded form of one UAS Datalink LS packet.
type Record struct {
	Schema      string       `json:"schema"`
	UL          []byte       `json:"ul,omitempty"`
	LSVersion   int          `json:"lsVersion"`
	ValueLength int          `json:"valueLength"`
	Items       []Item       `json:"items"`
	Checksum    ChecksumInfo `json:"checksum"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Item is one decoded tag within a Record.
type Item struct {
	Tag   int    `json:"tag"`
	Name  string `json:"name"`
	Value Value  `json:"value"`
	Units string `json:"units,omitempty"`
	Raw   []byte `json:"raw,omitempty"`
}

// ChecksumInfo captures the expected and computed 16-bit packet checksum.
type ChecksumInfo struct {
	Expected uint16 `json:"expected"`
	Computed uint16 `json:"computed"`
	Valid    bool   `json:"valid"`
}

// Diagnostic describes a decode-time or validation-time issue.
// Shape mirrors packetize.Diagnostic and ts.Diagnostic for consistency.
type Diagnostic struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Tag      *int   `json:"tag,omitempty"`
	TagName  string `json:"tagName,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Expected string `json:"expected,omitempty"`
	Raw      string `json:"raw,omitempty"`
}

// Value is a closed sum of typed KLV values. Concrete implementations
// below each call isKLVValue to close the set.
type Value interface {
	isKLVValue()
}

type (
	IntValue    int64
	UintValue   uint64
	FloatValue  float64
	StringValue string
	BytesValue  []byte
	BoolValue   bool
	TimeValue   time.Time
)

// EnumValue pairs an integer code with its resolved label so consumers
// can render either representation.
type EnumValue struct {
	Code  int64  `json:"code"`
	Label string `json:"label"`
}

// NestedValue wraps a nested local set defined by another MISB spec.
// Phase 1 keeps these opaque with only a spec hint.
type NestedValue struct {
	SpecHint string `json:"specHint"`
	Raw      []byte `json:"raw"`
}

func (IntValue) isKLVValue()    {}
func (UintValue) isKLVValue()   {}
func (FloatValue) isKLVValue()  {}
func (StringValue) isKLVValue() {}
func (BytesValue) isKLVValue()  {}
func (BoolValue) isKLVValue()   {}
func (TimeValue) isKLVValue()   {}
func (EnumValue) isKLVValue()   {}
func (NestedValue) isKLVValue() {}

// JSON marshaling. Scalars marshal as primitives. BytesValue, NestedValue.Raw,
// and Item.Raw marshal as base64 strings. TimeValue uses RFC 3339 with
// microsecond precision.

func (v IntValue) MarshalJSON() ([]byte, error)  { return json.Marshal(int64(v)) }
func (v UintValue) MarshalJSON() ([]byte, error) { return json.Marshal(uint64(v)) }

// FloatValue marshals NaN/Inf as JSON null. ST 0601 "error indicator"
// sentinels decode to NaN (see applyScaleSigned); json.Marshal would
// otherwise reject the entire record and crash a decode run.
func (v FloatValue) MarshalJSON() ([]byte, error) {
	f := float64(v)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return []byte("null"), nil
	}
	return json.Marshal(f)
}
func (v StringValue) MarshalJSON() ([]byte, error) { return json.Marshal(string(v)) }
func (v BoolValue) MarshalJSON() ([]byte, error)   { return json.Marshal(bool(v)) }

func (v BytesValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(base64.StdEncoding.EncodeToString(v))
}

func (v TimeValue) MarshalJSON() ([]byte, error) {
	// Microsecond precision preserves ST 0601 Precision Time Stamp (tag 2)
	// which is sourced from a uint64 of microseconds since the MISP epoch.
	return json.Marshal(time.Time(v).UTC().Format("2006-01-02T15:04:05.000000Z"))
}

func (v NestedValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		SpecHint string `json:"specHint"`
		Raw      string `json:"raw"`
	}{
		SpecHint: v.SpecHint,
		Raw:      base64.StdEncoding.EncodeToString(v.Raw),
	})
}
