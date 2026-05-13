package packetize

import (
	"bytes"
	"fmt"
	"math"
	"slices"

	"github.com/jacorbello/klvtool/internal/extract"
	"github.com/jacorbello/klvtool/internal/model"
)

const parserVersion = "1"

type Request struct {
	Mode   Mode
	Record extract.RawPayloadRecord
	// KnownULs lists the 16-byte SMPTE Universal Labels the caller knows
	// about (typically derived from a klv.Registry). Used for the stripped-UL
	// detection probe; safe to leave nil to disable that detection.
	KnownULs [][]byte
	// RepairStrippedUL controls whether the parser rebuilds the payload to
	// re-attach the 5-byte SMPTE category prefix that some ffmpeg builds
	// strip when extracting KLV data streams via `-c copy -f data`. Has no
	// effect unless KnownULs is non-empty and the stripped pattern is
	// detected.
	RepairStrippedUL bool
}

type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) Parse(req Request) (PacketizedStream, error) {
	mode := req.Mode
	if mode == "" {
		mode = ModeStrict
	} else if mode != ModeStrict && mode != ModeBestEffort {
		return PacketizedStream{}, model.InvalidUsage(fmt.Errorf("invalid packetization mode %q", mode))
	}

	stream := PacketizedStream{
		Source:        req.Record,
		Mode:          mode,
		ParserVersion: parserVersion,
		Packets:       make([]Packet, 0),
		Diagnostics:   make([]Diagnostic, 0),
	}

	if len(req.Record.Payload) == 0 {
		return stream, nil
	}

	// Detection probe: some ffmpeg builds strip the first 5 bytes of the
	// 16-byte SMPTE UL when emitting KLV data streams via `-c copy -f data`.
	// We compare the payload's count of full ULs (any starting with
	// 06 0e 2b 34) against the count of known-UL tails. If there are zero
	// full ULs and at least one tail, the stream is almost certainly the
	// stripped variant; emit a stream-level diagnostic and (if requested)
	// repair the payload in-flight before the normal parse loop runs.
	if detected, hit := detectStrippedULPrefix(req.Record.Payload, req.KnownULs); detected {
		stream.Diagnostics = append(stream.Diagnostics, strippedULPrefixDiagnostic(hit))
		stream.WarningCount++
		if req.RepairStrippedUL {
			repaired, count := repairStrippedULPrefix(req.Record.Payload, req.KnownULs)
			req.Record.Payload = repaired
			stream.Source.Payload = repaired
			stream.Diagnostics = append(stream.Diagnostics, strippedULRepairedDiagnostic(count, hit))
			stream.Recovered = true
		}
	}

	for offset, attempt := 0, 0; offset < len(req.Record.Payload); attempt++ {
		packet, nextOffset, diag, err := parsePacket(req.Record.Payload, offset, attempt)
		if err != nil {
			stream.Diagnostics = append(stream.Diagnostics, diag)
			stream.ErrorCount++
			if mode == ModeStrict {
				return stream, model.PacketParse(err)
			}
			stream.Recovered = true
			recovered := scanForwardToKey(req.Record.Payload, offset+1)
			if recovered < 0 {
				break
			}
			skipOffset := offset
			skipLen := recovered
			skipDiag := recoverySkipDiagnostic(skipOffset, skipLen, attempt)
			stream.Diagnostics = append(stream.Diagnostics, skipDiag)
			stream.WarningCount++
			offset = recovered
			continue
		}

		packet.PacketIndex = len(stream.Packets)
		stream.Packets = append(stream.Packets, packet)
		stream.ParsedCount++
		offset = nextOffset
	}

	return stream, nil
}

func parsePacket(payload []byte, offset int, packetIndex int) (Packet, int, Diagnostic, error) {
	const keySize = 16

	if offset < 0 || offset >= len(payload) {
		diag := malformedPacketDiagnostic("packet_truncated", "payload ended before a complete packet key", offset, packetIndex)
		return Packet{}, 0, diag, fmt.Errorf(diag.Message)
	}
	if len(payload)-offset < keySize+1 {
		diag := malformedPacketDiagnostic("packet_truncated", "payload ended before a complete packet key", offset, packetIndex)
		return Packet{}, 0, diag, fmt.Errorf(diag.Message)
	}

	key := append([]byte(nil), payload[offset:offset+keySize]...)
	length, lengthRead, err := decodeBERLength(payload[offset+keySize:])
	if err != nil {
		diag := malformedPacketDiagnostic("invalid_ber_length", err.Error(), offset, packetIndex)
		return Packet{}, 0, diag, err
	}

	valueStart, ok := safeAddInt(offset, keySize)
	if !ok {
		diag := malformedPacketDiagnostic("packet_bounds_overflow", "packet start exceeds supported bounds", offset, packetIndex)
		return Packet{}, 0, diag, fmt.Errorf(diag.Message)
	}
	valueStart, ok = safeAddInt(valueStart, lengthRead)
	if !ok {
		diag := malformedPacketDiagnostic("packet_bounds_overflow", "packet length exceeds supported bounds", offset, packetIndex)
		return Packet{}, 0, diag, fmt.Errorf(diag.Message)
	}

	valueEnd, ok := safeAddInt(valueStart, length)
	if !ok {
		diag := malformedPacketDiagnostic("packet_bounds_overflow", "declared value length exceeds supported bounds", offset, packetIndex)
		return Packet{}, 0, diag, fmt.Errorf(diag.Message)
	}
	if valueEnd > len(payload) {
		diag := malformedPacketDiagnostic("value_out_of_range", "declared value length exceeds payload size", offset, packetIndex)
		return Packet{}, 0, diag, fmt.Errorf(diag.Message)
	}

	packet := Packet{
		PacketIndex:        packetIndex,
		PacketStart:        offset,
		KeyStart:           offset,
		LengthStart:        offset + keySize,
		ValueStart:         valueStart,
		PacketEndExclusive: valueEnd,
		Key:                key,
		Length:             length,
		Value:              append([]byte(nil), payload[valueStart:valueEnd]...),
		Classification:     classifyKey(key),
		Diagnostics:        []Diagnostic{},
	}

	return packet, valueEnd, Diagnostic{}, nil
}

func classifyKey(key []byte) Classification {
	if len(key) != 16 {
		return ClassificationUnknown
	}
	if slices.Equal(key[:4], []byte{0x06, 0x0e, 0x2b, 0x34}) {
		return ClassificationUniversalSet
	}
	return ClassificationUnknown
}

// universalKeyPrefix is the 4-byte SMPTE universal label prefix used for
// scan-forward recovery in best-effort mode.
var universalKeyPrefix = []byte{0x06, 0x0e, 0x2b, 0x34}

// scanForwardToKey scans the payload starting at offset looking for the next
// occurrence of the SMPTE universal key prefix. Returns the offset of the match
// or -1 if no match is found.
func scanForwardToKey(payload []byte, offset int) int {
	if offset < 0 || offset >= len(payload) {
		return -1
	}
	idx := bytes.Index(payload[offset:], universalKeyPrefix)
	if idx < 0 {
		return -1
	}
	return offset + idx
}

func recoverySkipDiagnostic(skipFrom, skipTo, packetIndex int) Diagnostic {
	byteOffset := skipFrom
	pktIndex := packetIndex
	return Diagnostic{
		Severity:    "warning",
		Code:        "recovery_skip",
		Message:     fmt.Sprintf("skipped %d bytes to next key at offset %d", skipTo-skipFrom, skipTo),
		Stage:       "packetize",
		PacketIndex: &pktIndex,
		ByteOffset:  &byteOffset,
	}
}

func malformedPacketDiagnostic(code, message string, offset int, packetIndex int) Diagnostic {
	byteOffset := offset
	pktIndex := packetIndex
	return Diagnostic{
		Severity:    "error",
		Code:        code,
		Message:     message,
		Stage:       "packetize",
		PacketIndex: &pktIndex,
		ByteOffset:  &byteOffset,
	}
}

func safeAddInt(a, b int) (int, bool) {
	if b > 0 && a > math.MaxInt-b {
		return 0, false
	}
	if b < 0 && a < math.MinInt-b {
		return 0, false
	}
	return a + b, true
}

// strippedULMatch records which registered UL appears to have been stripped
// in the payload, plus the number of stub occurrences observed.
type strippedULMatch struct {
	UL    []byte
	Count int
}

// detectStrippedULPrefix probes a payload for the ffmpeg-strip pattern: the
// payload contains zero full SMPTE ULs (4-byte prefix 06 0e 2b 34) but at
// least one 11-byte tail of a registered UL. Returns (true, match) only when
// exactly that combination holds. Abstains when any full UL is present —
// mixing the two would suggest a payload whose framing we don't understand,
// and silent repair could corrupt valid packets.
func detectStrippedULPrefix(payload []byte, knownULs [][]byte) (bool, strippedULMatch) {
	if len(payload) == 0 || len(knownULs) == 0 {
		return false, strippedULMatch{}
	}
	if bytes.Contains(payload, universalKeyPrefix) {
		return false, strippedULMatch{}
	}
	best := strippedULMatch{}
	for _, ul := range knownULs {
		if len(ul) != 16 {
			continue
		}
		tail := ul[5:]
		count := bytes.Count(payload, tail)
		if count > best.Count {
			best = strippedULMatch{UL: ul, Count: count}
		}
	}
	if best.Count == 0 {
		return false, strippedULMatch{}
	}
	return true, best
}

// repairStrippedULPrefix rebuilds the payload by re-attaching the first
// 5 bytes (the SMPTE category prefix) of the best-matching known UL at every
// occurrence of that UL's 11-byte tail. Returns the rebuilt payload and the
// number of packets repaired. If detection abstains, returns the input
// unchanged.
func repairStrippedULPrefix(payload []byte, knownULs [][]byte) ([]byte, int) {
	ok, hit := detectStrippedULPrefix(payload, knownULs)
	if !ok {
		return payload, 0
	}
	prefix := hit.UL[:5]
	tail := hit.UL[5:]
	out := make([]byte, 0, len(payload)+5*hit.Count)
	cursor := 0
	repaired := 0
	for cursor < len(payload) {
		idx := bytes.Index(payload[cursor:], tail)
		if idx < 0 {
			out = append(out, payload[cursor:]...)
			break
		}
		out = append(out, payload[cursor:cursor+idx]...)
		out = append(out, prefix...)
		out = append(out, tail...)
		repaired++
		cursor += idx + len(tail)
	}
	return out, repaired
}

func strippedULPrefixDiagnostic(hit strippedULMatch) Diagnostic {
	return Diagnostic{
		Severity: "warning",
		Code:     DiagnosticStrippedULPrefix,
		Message: fmt.Sprintf(
			"payload appears to have the 5-byte SMPTE category prefix removed from every UL (matched %d stubs of %x); re-run with --repair-stripped-ul to recover, or use an ffmpeg build that preserves the full UL",
			hit.Count, hit.UL,
		),
		Stage: "packetize",
	}
}

func strippedULRepairedDiagnostic(count int, hit strippedULMatch) Diagnostic {
	return Diagnostic{
		Severity: "info",
		Code:     DiagnosticStrippedULRepaired,
		Message: fmt.Sprintf(
			"re-attached the 5-byte SMPTE category prefix to %d packet(s) against UL %x",
			count, hit.UL,
		),
		Stage: "packetize",
	}
}
