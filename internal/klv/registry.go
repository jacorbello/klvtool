package klv

import (
	"errors"
	"fmt"

	"github.com/jacorbello/klvtool/internal/klv/specs"
)

// ErrUnknownSpec is returned when no registered SpecVersion matches a UL key.
var ErrUnknownSpec = errors.New("klv: unknown spec")

// Registry maps URNs to SpecVersions and resolves wire bytes to a SpecVersion
// via UL key matching (and, once multiple versions share a UL, via the LS
// Version tag in the value bytes).
type Registry struct {
	byURN map[string]specs.SpecVersion
	byUL  map[string][]specs.SpecVersion
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		byURN: make(map[string]specs.SpecVersion),
		byUL:  make(map[string][]specs.SpecVersion),
	}
}

// Register adds a SpecVersion to the registry. Duplicate URNs overwrite:
// if a SpecVersion with the same URN is already registered, its byUL entry
// is removed before the new SpecVersion is inserted so stale entries never
// accumulate (even when the new version has a different UL from the old).
func (r *Registry) Register(sv specs.SpecVersion) {
	if prev, ok := r.byURN[sv.URN()]; ok {
		prevKey := string(prev.UL())
		r.byUL[prevKey] = removeSpec(r.byUL[prevKey], sv.URN())
		if len(r.byUL[prevKey]) == 0 {
			delete(r.byUL, prevKey)
		}
	}
	r.byURN[sv.URN()] = sv
	key := string(sv.UL())
	r.byUL[key] = append(r.byUL[key], sv)
}

// removeSpec returns list with the SpecVersion matching urn removed.
// Preserves relative order of remaining entries.
func removeSpec(list []specs.SpecVersion, urn string) []specs.SpecVersion {
	out := list[:0]
	for _, sv := range list {
		if sv.URN() != urn {
			out = append(out, sv)
		}
	}
	return out
}

// Lookup returns the SpecVersion registered for the given URN, or ok=false.
func (r *Registry) Lookup(urn string) (specs.SpecVersion, bool) {
	sv, ok := r.byURN[urn]
	return sv, ok
}

// Resolve picks the right SpecVersion for a packet.
//
// Algorithm:
//  1. Find all SpecVersions registered under the exact UL byte match.
//  2. If zero match, return ErrUnknownSpec.
//  3. If one match, return it.
//  4. If multiple match, peek the LS Version tag in the value bytes and
//     return the SpecVersion whose ExpectedVersion equals the peeked value.
//     Returns ErrUnknownSpec if no version matches.
func (r *Registry) Resolve(ul []byte, value []byte) (specs.SpecVersion, error) {
	candidates := r.byUL[string(ul)]
	switch len(candidates) {
	case 0:
		return nil, fmt.Errorf("%w: no spec registered for UL", ErrUnknownSpec)
	case 1:
		return candidates[0], nil
	}
	// Multiple versions in the family: peek the LS Version tag.
	versionTag := candidates[0].VersionTag()
	peeked, ok := peekTag(value, versionTag)
	if !ok {
		return nil, fmt.Errorf("%w: cannot peek version tag %d", ErrUnknownSpec, versionTag)
	}
	for _, c := range candidates {
		if c.ExpectedVersion() == peeked {
			return c, nil
		}
	}
	return nil, fmt.Errorf("%w: version %d not registered", ErrUnknownSpec, peeked)
}

// peekTag walks the TLV stream in value looking for the given tag number,
// returning its single-byte value if found. Uses BER-OID for tags and
// BER short/long form for lengths. Stops at the first match or on any
// structural error. Returns 0/false when not found.
func peekTag(value []byte, target int) (int, bool) {
	cursor := 0
	for cursor < len(value) {
		tag, tagN, err := decodeBEROID(value[cursor:])
		if err != nil {
			return 0, false
		}
		cursor += tagN
		length, lenN, err := decodeBERLength(value[cursor:])
		if err != nil {
			return 0, false
		}
		cursor += lenN
		if cursor+length > len(value) {
			return 0, false
		}
		if tag == target && length >= 1 {
			return int(value[cursor]), true
		}
		cursor += length
	}
	return 0, false
}

// decodeBERLength decodes a BER length from the start of input. Mirrors
// packetize.decodeBERLength but kept local to avoid cross-package coupling.
func decodeBERLength(input []byte) (int, int, error) {
	if len(input) == 0 {
		return 0, 0, errors.New("ber length: empty")
	}
	first := input[0]
	if first&0x80 == 0 {
		return int(first), 1, nil
	}
	count := int(first & 0x7F)
	if count == 0 || count > 4 || 1+count > len(input) {
		return 0, 0, fmt.Errorf("ber length: invalid long form (count %d)", count)
	}
	length := 0
	for _, b := range input[1 : 1+count] {
		length = (length << 8) | int(b)
	}
	if length < 0 {
		return 0, 0, errors.New("ber length: overflow")
	}
	return length, 1 + count, nil
}
