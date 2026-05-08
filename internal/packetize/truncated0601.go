package packetize

import (
	"bytes"
	"fmt"

	"github.com/jacorbello/klvtool/internal/klv/specs/st0601"
)

// DetectTruncatedUASDatalink returns true when the payload appears to use the
// 11-byte trailing UL key (OID prefix stripped) plus BER length framing used
// by some ST 0601 elementary streams, rather than a full 16-byte SMPTE key at
// each packet boundary.
func DetectTruncatedUASDatalink(payload []byte) bool {
	suf := st0601.TruncatedUASDatalinkKeySuffix
	min := len(suf) + 1
	if len(payload) < min {
		return false
	}
	// Standard framing: first byte is always OID root; do not treat as truncated.
	if bytes.HasPrefix(payload, universalKeyPrefix) {
		return false
	}
	j := findTruncatedUASDatalinkSuffix(payload, 0)
	if j < 0 {
		return false
	}
	tail := payload[j+len(suf):]
	if len(tail) == 0 {
		return false
	}
	_, _, err := decodeBERLength(tail)
	return err == nil
}

// ParseTruncatedUASDatalinkStream packetizes payload that uses the truncated
// UL suffix at each record boundary. It returns synthetic Packet entries whose
// Key is the full 16-byte UASDatalinkUL for registry lookup; LengthStart and
// ValueStart refer to offsets in the original wire payload.
func ParseTruncatedUASDatalinkStream(req Request) (PacketizedStream, error) {
	mode := req.Mode
	if mode == "" {
		mode = ModeStrict
	} else if mode != ModeStrict && mode != ModeBestEffort {
		return PacketizedStream{}, fmt.Errorf("invalid packetization mode %q", mode)
	}

	stream := PacketizedStream{
		Source:        req.Record,
		Mode:          mode,
		ParserVersion: parserVersion,
		Packets:       make([]Packet, 0),
		Diagnostics:   make([]Diagnostic, 0),
	}

	payload := req.Record.Payload
	if len(payload) == 0 {
		return stream, nil
	}

	suf := st0601.TruncatedUASDatalinkKeySuffix
	ul := st0601.UASDatalinkUL

	offset := 0
	if !bytes.HasPrefix(payload, suf) {
		j := findTruncatedUASDatalinkSuffix(payload, 0)
		if j < 0 {
			d := malformedPacketDiagnostic("trunc0601_sync_missing", "truncated ST 0601 UL suffix not found", 0, 0)
			stream.Diagnostics = append(stream.Diagnostics, d)
			stream.ErrorCount++
			return stream, nil
		}
		if j > 0 {
			byteOffset := 0
			pktIdx := 0
			stream.Diagnostics = append(stream.Diagnostics, Diagnostic{
				Severity:    "warning",
				Code:        "trunc0601_preamble_skip",
				Message:     fmt.Sprintf("skipped %d bytes before first truncated UL suffix", j),
				Stage:       "packetize",
				PacketIndex: &pktIdx,
				ByteOffset:  &byteOffset,
			})
			stream.WarningCount++
			offset = j
		}
	}

	packetIndex := 0
	gapEvents := 0
	gapBytesTotal := 0
	for offset < len(payload) {
		if len(payload)-offset < len(suf)+1 {
			break
		}
		if !bytes.HasPrefix(payload[offset:], suf) {
			j := findTruncatedUASDatalinkSuffix(payload, offset)
			if j < 0 {
				recoveryFrom := offset
				d := malformedPacketDiagnostic(
					"trunc0601_resync_fail",
					fmt.Sprintf("truncated UL suffix missing after offset %d (%d trailing bytes)", offset, len(payload)-offset),
					offset,
					packetIndex,
				)
				stream.Diagnostics = append(stream.Diagnostics, d)
				stream.ErrorCount++
				if mode == ModeStrict {
					return stream, fmt.Errorf(d.Message)
				}
				byteOffset := recoveryFrom
				pktIdx := packetIndex
				stream.Diagnostics = append(stream.Diagnostics, Diagnostic{
					Severity:    "warning",
					Code:        "trunc0601_tail_skip",
					Message:     fmt.Sprintf("skipped %d unframed trailing bytes", len(payload)-recoveryFrom),
					Stage:       "packetize",
					PacketIndex: &pktIdx,
					ByteOffset:  &byteOffset,
				})
				stream.WarningCount++
				break
			}
			skipLen := j - offset
			if skipLen > 0 {
				gapEvents++
				gapBytesTotal += skipLen
			}
			offset = j
			stream.Recovered = true
			continue
		}

		keyStart := offset
		lengthStart := offset + len(suf)
		tail := payload[lengthStart:]
		length, lengthRead, err := decodeBERLength(tail)
		if err != nil {
			d := malformedPacketDiagnostic("invalid_ber_length", err.Error(), offset, packetIndex)
			stream.Diagnostics = append(stream.Diagnostics, d)
			stream.ErrorCount++
			if mode == ModeStrict {
				return stream, err
			}
			stream.Recovered = true
			next := findTruncatedUASDatalinkSuffix(payload, offset+1)
			if next < 0 {
				break
			}
			offset = next
			continue
		}
		valueStart, ok := safeAddInt(lengthStart, lengthRead)
		if !ok {
			d := malformedPacketDiagnostic("packet_bounds_overflow", "packet length exceeds supported bounds", offset, packetIndex)
			stream.Diagnostics = append(stream.Diagnostics, d)
			stream.ErrorCount++
			if mode == ModeStrict {
				return stream, fmt.Errorf(d.Message)
			}
			stream.Recovered = true
			break
		}
		end, ok := safeAddInt(valueStart, length)
		if !ok {
			d := malformedPacketDiagnostic("packet_bounds_overflow", "declared value length exceeds supported bounds", offset, packetIndex)
			stream.Diagnostics = append(stream.Diagnostics, d)
			stream.ErrorCount++
			if mode == ModeStrict {
				return stream, fmt.Errorf(d.Message)
			}
			stream.Recovered = true
			next := findTruncatedUASDatalinkSuffix(payload, offset+1)
			if next < 0 {
				break
			}
			offset = next
			continue
		}
		if end > len(payload) {
			d := malformedPacketDiagnostic(
				"value_out_of_range",
				"declared value length exceeds payload size",
				offset,
				packetIndex,
			)
			stream.Diagnostics = append(stream.Diagnostics, d)
			stream.ErrorCount++
			if mode == ModeStrict {
				return stream, fmt.Errorf(d.Message)
			}
			break
		}

		pkt := Packet{
			PacketIndex:        packetIndex,
			PacketStart:        keyStart,
			KeyStart:           keyStart,
			LengthStart:        lengthStart,
			ValueStart:         valueStart,
			PacketEndExclusive: end,
			Key:                append([]byte{}, ul...),
			Length:             length,
			Value:              append([]byte{}, payload[valueStart:end]...),
			Classification:     ClassificationUniversalSet,
			Diagnostics:        nil,
		}
		stream.Packets = append(stream.Packets, pkt)
		stream.ParsedCount++
		packetIndex++
		offset = end
	}

	if gapEvents > 0 {
		stream.Diagnostics = append(stream.Diagnostics, Diagnostic{
			Severity: "warning",
			Code:     "trunc0601_inter_record_gaps",
			Message: fmt.Sprintf(
				"skipped %d bytes in %d inter-record gaps between truncated UL payloads (typical PES/private data filler)",
				gapBytesTotal,
				gapEvents,
			),
			Stage: "packetize",
		})
		stream.WarningCount++
	}

	return stream, nil
}

// ParseAuto runs truncated ST 0601 UL transport framing when detected; otherwise
// it delegates to the standard SMPTE KLV packet scanner.
func (p *Parser) ParseAuto(req Request) (PacketizedStream, error) {
	if DetectTruncatedUASDatalink(req.Record.Payload) {
		return ParseTruncatedUASDatalinkStream(req)
	}
	return p.Parse(req)
}

func findTruncatedUASDatalinkSuffix(payload []byte, start int) int {
	suf := st0601.TruncatedUASDatalinkKeySuffix
	if start < 0 {
		start = 0
	}
	for start <= len(payload)-len(suf) {
		i := bytes.Index(payload[start:], suf)
		if i < 0 {
			return -1
		}
		j := start + i
		if !isFullUASDatalinkSuffix(payload, j) {
			return j
		}
		start = j + 1
	}
	return -1
}

func isFullUASDatalinkSuffix(payload []byte, suffixStart int) bool {
	sufLen := len(st0601.TruncatedUASDatalinkKeySuffix)
	prefixLen := len(st0601.UASDatalinkUL) - sufLen
	fullStart := suffixStart - prefixLen
	if fullStart < 0 || fullStart+len(st0601.UASDatalinkUL) > len(payload) {
		return false
	}
	return bytes.Equal(payload[fullStart:fullStart+len(st0601.UASDatalinkUL)], st0601.UASDatalinkUL)
}
