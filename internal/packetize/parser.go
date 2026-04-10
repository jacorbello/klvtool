package packetize

import (
	"fmt"
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
			break
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

	valueStart := offset + keySize + lengthRead
	valueEnd := valueStart + length
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
