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

	for offset, packetIndex := 0, 0; offset < len(req.Record.Payload); packetIndex++ {
		packet, nextOffset, diag, err := parsePacket(req.Record.Payload, offset, packetIndex)
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
			stream.Diagnostics = append(stream.Diagnostics, recoverySkipDiagnostic(skipOffset, skipLen, packetIndex))
			offset = recovered
			continue
		}

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
